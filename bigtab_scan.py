#!/usr/bin/env python3
import sys
import os
import argparse
import socket
from typing import List, Tuple

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


def find_target_database(conn, db_prefix: str) -> str:
    with conn.cursor() as cur:
        # Use information_schema for portability
        sql = (
            "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA "
            "WHERE SCHEMA_NAME LIKE %s ORDER BY SCHEMA_NAME"
        )
        cur.execute(sql, (db_prefix + '%',))
        rows = [r[0] for r in cur.fetchall()]
    if not rows:
        raise RuntimeError(f"未找到前缀为 {db_prefix} 的数据库")
    if len(rows) > 1:
        raise RuntimeError(
            f"找到多个前缀为 {db_prefix} 的数据库: {', '.join(rows)}"
        )
    return rows[0]


def list_tables_with_prefix(conn, db_name: str, table_prefix: str) -> List[str]:
    with conn.cursor() as cur:
        sql = (
            "SELECT TABLE_NAME FROM information_schema.TABLES "
            "WHERE TABLE_SCHEMA=%s AND TABLE_NAME LIKE %s "
            "ORDER BY TABLE_NAME"
        )
        cur.execute(sql, (db_name, table_prefix + '%'))
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
        "--db-prefix",
        default=os.getenv("DB_PREFIX", "clearing_branch"),
        help="数据库前缀，默认 clearing_branch",
    )
    parser.add_argument(
        "--table-prefix",
        default=os.getenv("TABLE_PREFIX", "t_p_trans"),
        help="数据表前缀，默认 t_p_trans",
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

    overall_ok = True

    for vip, ip in input_pairs:
        # Basic validation for IP/hostname
        try:
            socket.gethostbyname(ip)
        except Exception:
            eprint(f"[WARN] 无法解析 IP/主机名: {ip}，将继续尝试连接")

        try:
            conn = connect_mysql(ip, args.port, args.user, args.password, args.timeout)
        except Exception as ex:
            overall_ok = False
            eprint(f"[ERROR] 无法连接到 {ip}:{args.port} - {ex}")
            if args.strict:
                return 2
            continue

        try:
            try:
                dbname = find_target_database(conn, args.db_prefix)
            except Exception as ex:
                overall_ok = False
                eprint(f"[ERROR] {ip}: {ex}")
                if args.strict:
                    return 3
                conn.close()
                continue

            try:
                tables = list_tables_with_prefix(conn, dbname, args.table_prefix)
            except Exception as ex:
                overall_ok = False
                eprint(f"[ERROR] {ip}: 无法列出表: {ex}")
                if args.strict:
                    return 4
                conn.close()
                continue

            if not tables:
                eprint(f"[INFO] {ip}: 数据库 {dbname} 中未找到前缀 {args.table_prefix} 的表")
                conn.close()
                continue

            for t in tables:
                try:
                    cnt = count_table_rows(conn, dbname, t, args.timeout)
                except Exception as ex:
                    overall_ok = False
                    eprint(f"[ERROR] {ip}: 统计 {dbname}.{t} 失败: {ex}")
                    if args.strict:
                        conn.close()
                        return 5
                    continue

                if cnt >= args.threshold:
                    # Output as: vip, ip, db_name, table_name, count
                    print(f"{vip}, {ip}, {dbname}, {t}, {cnt}")
        finally:
            try:
                conn.close()
            except Exception:
                pass

    return 0 if overall_ok else 0  # Still exit 0 unless --strict


if __name__ == "__main__":
    sys.exit(main())
