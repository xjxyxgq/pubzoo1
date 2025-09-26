#!/usr/bin/env python3
import sys
import os
import argparse
import socket
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import List, Tuple, Dict

try:
    import pymysql
except Exception as e:  # pragma: no cover
    pymysql = None


def eprint(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)


def parse_input_lines(lines: List[str]) -> List[Tuple[str, str]]:
    pairs: List[Tuple[str, str]] = []
    for ln in lines:
        s = ln.strip()
        if not s or s.startswith('#'):
            continue
        # Accept comma, whitespace, or tab separated two columns: vip, ip
        if ',' in s:
            parts = [p.strip() for p in s.split(',') if p.strip()]
        else:
            parts = s.split()
        if len(parts) < 2:
            eprint(f"[WARN] 跳过无法解析的行: {ln.rstrip()}")
            continue
        vip, ip = parts[0], parts[1]
        pairs.append((vip, ip))
    return pairs


def connect_mysql(host: str, port: int, user: str, password: str, timeout: int):
    if pymysql is None:
        raise RuntimeError(
            "未安装 PyMySQL，请先执行: pip install PyMySQL"
        )
    return pymysql.connect(
        host=host,
        port=port,
        user=user,
        password=password,
        connect_timeout=timeout,
        read_timeout=timeout,
        write_timeout=timeout,
        autocommit=True,
        cursorclass=pymysql.cursors.Cursor,
    )


def find_database_by_regex(conn, db_regex: str) -> str:
    with conn.cursor() as cur:
        sql = (
            "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA "
            "WHERE SCHEMA_NAME REGEXP %s ORDER BY SCHEMA_NAME"
        )
        cur.execute(sql, (db_regex,))
        rows = [r[0] for r in cur.fetchall()]
    if not rows:
        raise RuntimeError(f"未找到匹配正则 {db_regex} 的数据库")
    if len(rows) > 1:
        raise RuntimeError(
            f"匹配正则 {db_regex} 的数据库不唯一: {', '.join(rows)}"
        )
    return rows[0]


def list_tables_by_regex(conn, db_name: str, table_regex: str) -> List[str]:
    with conn.cursor() as cur:
        sql = (
            "SELECT TABLE_NAME FROM information_schema.TABLES "
            "WHERE TABLE_SCHEMA=%s AND TABLE_NAME REGEXP %s "
            "ORDER BY TABLE_NAME"
        )
        cur.execute(sql, (db_name, table_regex))
        return [r[0] for r in cur.fetchall()]


def count_table_rows(conn, db_name: str, table_name: str, timeout: int) -> int:
    # Ensure database and identifiers are quoted properly
    ident_db = f"`{db_name.replace('`', '``')}`"
    ident_tbl = f"`{table_name.replace('`', '``')}`"
    sql = f"SELECT COUNT(*) FROM {ident_db}.{ident_tbl}"
    # Use a dedicated cursor to allow per-query timeout via session if supported
    with conn.cursor() as cur:
        try:
            # Attempt to set statement timeout if server supports (MySQL 8.0.3+)
            # This won't raise if variable doesn't exist due to SQL_MODE, so ignore errors.
            try:
                cur.execute("SET SESSION MAX_EXECUTION_TIME=%s", (timeout * 1000,))
            except Exception:
                pass
            cur.execute(sql)
            row = cur.fetchone()
            return int(row[0]) if row and row[0] is not None else 0
        finally:
            try:
                cur.execute("SET SESSION MAX_EXECUTION_TIME=0")
            except Exception:
                pass


def main():
    parser = argparse.ArgumentParser(
        description=(
            "统计多个 MySQL 实例中 clearing_branch* 库里的 t_p_trans* 大表行数，"
            "并输出超过阈值的表: vip, ip, db_name, table_name, count"
        )
    )
    parser.add_argument(
        "--user",
        default=os.getenv("MYSQL_USER", "root"),
        help="MySQL 用户名，默认取环境变量 MYSQL_USER 或 root",
    )
    parser.add_argument(
        "--password",
        default=os.getenv("MYSQL_PASSWORD", ""),
        help="MySQL 密码，默认取环境变量 MYSQL_PASSWORD",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.getenv("MYSQL_PORT", "3306")),
        help="MySQL 端口，默认 3306",
    )
    parser.add_argument(
        "--threshold",
        type=int,
        default=int(os.getenv("ROW_THRESHOLD", "3000000")),
        help="超过该行数才输出，默认 3000000",
    )
    parser.add_argument(
        "--db-regex",
        default=os.getenv("DB_REGEX", r"^clearing_branch_[0-9]$"),
        help="数据库名正则，默认 ^clearing_branch_[0-9]$",
    )
    parser.add_argument(
        "--table-regex",
        default=os.getenv("TABLE_REGEX", r"^t_p_trans_[0-9]{2}_[0-9]{2}$"),
        help="表名正则，默认 ^t_p_trans_[0-9]{2}_[0-9]{2}$",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=int(os.getenv("MYSQL_TIMEOUT", "30")),
        help="连接与查询超时秒数，默认 30",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="严格模式：出现任何错误即退出非零",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=max(4, (os.cpu_count() or 2) * 2),
        help="并发线程数，默认为 2x CPU 核心数（不小于 4）",
    )
    parser.add_argument(
        "--out-dir",
        default=os.getenv("OUT_DIR", "."),
        help="结果输出目录（按 vip 前缀命名文件）",
    )
    parser.add_argument(
        "--file-suffix",
        default=os.getenv("OUT_SUFFIX", "_bigtab.csv"),
        help="结果文件名后缀，默认 _bigtab.csv",
    )
    parser.add_argument(
        "--input",
        default=None,
        help="包含 vip, ip 列表的文件路径；未提供则读取标准输入",
    )
    args = parser.parse_args()

    if not args.password and os.getenv("MYSQL_PASSWORD_FILE"):
        try:
            with open(os.getenv("MYSQL_PASSWORD_FILE"), "r", encoding="utf-8") as f:
                args.password = f.read().strip()
        except Exception as ex:
            eprint(f"[WARN] 无法从 MYSQL_PASSWORD_FILE 读取密码: {ex}")

    # Read input lines from file or stdin
    try:
        if args.input:
            with open(args.input, "r", encoding="utf-8") as f:
                src_lines = f.readlines()
        else:
            src_lines = sys.stdin.readlines()
    except Exception as ex:
        eprint(f"[ERROR] 无法读取输入: {ex}")
        return 10

    input_pairs = parse_input_lines(src_lines)
    if not input_pairs:
        eprint("[INFO] 未提供任何 vip, ip 输入；从标准输入按行提供，如: 'vip1, 10.0.0.1'")
        return 1

    # 准备输出目录
    out_dir = Path(args.out_dir)
    try:
        out_dir.mkdir(parents=True, exist_ok=True)
    except Exception as ex:
        eprint(f"[ERROR] 无法创建输出目录 {out_dir}: {ex}")
        return 11

    # 每个 vip 一个锁，保证写文件时串行
    vip_locks: Dict[str, threading.Lock] = {}
    vip_locks_guard = threading.Lock()

    def get_vip_lock(vip: str) -> threading.Lock:
        if vip in vip_locks:
            return vip_locks[vip]
        with vip_locks_guard:
            if vip not in vip_locks:
                vip_locks[vip] = threading.Lock()
            return vip_locks[vip]

    def sanitize_filename(name: str) -> str:
        safe = []
        for ch in name:
            if ch.isalnum() or ch in ('-', '_', '.'):
                safe.append(ch)
            else:
                safe.append('_')
        return ''.join(safe) or 'vip'

    def output_path_for_vip(vip: str) -> Path:
        return out_dir / f"{sanitize_filename(vip)}{args.file_suffix}"

    def write_result_line(vip: str, line: str):
        path = output_path_for_vip(vip)
        lock = get_vip_lock(vip)
        with lock:
            with open(path, 'a', encoding='utf-8') as f:
                f.write(line + "\n")

    def progress(msg: str):
        # 仅将进度输出到标准输出
        print(msg)

    def process_one(vip: str, ip: str) -> bool:
        progress(f"[START] {vip}@{ip}: 开始处理")
        try:
            try:
                socket.gethostbyname(ip)
            except Exception:
                progress(f"[WARN] {vip}@{ip}: 主机名无法解析，尝试直连")

            try:
                conn = connect_mysql(ip, args.port, args.user, args.password, args.timeout)
            except Exception as ex:
                eprint(f"[ERROR] 无法连接到 {ip}:{args.port} - {ex}")
                return False

            try:
                dbname = find_database_by_regex(conn, args.db_regex)
                progress(f"[INFO] {vip}@{ip}: 目标库 {dbname}")
            except Exception as ex:
                eprint(f"[ERROR] {ip}: {ex}")
                try:
                    conn.close()
                except Exception:
                    pass
                return False

            try:
                tables = list_tables_by_regex(conn, dbname, args.table_regex)
                progress(f"[INFO] {vip}@{ip}: 匹配表数量 {len(tables)}")
            except Exception as ex:
                eprint(f"[ERROR] {ip}: 无法列出表: {ex}")
                try:
                    conn.close()
                except Exception:
                    pass
                return False

            if not tables:
                progress(f"[INFO] {vip}@{ip}: 未找到匹配正则 {args.table_regex} 的表")
                try:
                    conn.close()
                except Exception:
                    pass
                return True

            written = 0
            total = len(tables)
            for idx, t in enumerate(tables, 1):
                try:
                    cnt = count_table_rows(conn, dbname, t, args.timeout)
                except Exception as ex:
                    eprint(f"[ERROR] {ip}: 统计 {dbname}.{t} 失败: {ex}")
                    if args.strict:
                        try:
                            conn.close()
                        except Exception:
                            pass
                        return False
                    continue

                # 每张表的进度（不输出统计值到标准输出）
                progress(f"[PROG] {vip}@{ip}: {idx}/{total} 统计 {t}")

                if cnt >= args.threshold:
                    line = f"{vip}, {ip}, {dbname}, {t}, {cnt}"
                    write_result_line(vip, line)
                    written += 1
                    progress(f"[NOTE] {vip}@{ip}: {t} 超过阈值，已写入文件")

            try:
                conn.close()
            except Exception:
                pass

            path = output_path_for_vip(vip)
            if written > 0:
                progress(f"[DONE] {vip}@{ip}: 完成，写入 {written} 条到 {path}")
            else:
                progress(f"[DONE] {vip}@{ip}: 完成，没有超过阈值的表")
            return True
        except Exception as ex:
            eprint(f"[ERROR] {vip}@{ip}: 未处理异常 {ex}")
            return False

    overall_ok = True
    # 并发执行
    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = [pool.submit(process_one, vip, ip) for vip, ip in input_pairs]
        for fut in as_completed(futures):
            try:
                ok = fut.result()
            except Exception as ex:
                eprint(f"[ERROR] 任务执行异常: {ex}")
                ok = False
            overall_ok = overall_ok and ok

    return 0 if overall_ok or not args.strict else 1


if __name__ == "__main__":
    sys.exit(main())
