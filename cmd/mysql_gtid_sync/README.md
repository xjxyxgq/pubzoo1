# MySQL GTID 同步工具

这个工具用于同步MySQL复制集群中的GTID集合，确保所有节点的GTID集合一致。

## 功能特点

- 连接到多个MySQL实例，检查它们的GTID集合
- 分析主节点和从节点之间的GTID差异
- 生成一个SQL文件，用于在主节点上执行，通过注入空事务使所有节点的GTID集合一致
- 支持多个从节点同时同步
- 在SQL文件中用注释标识出每个GTID来自哪个从节点

## 使用方法

```bash
./mysql_gtid_sync -master=ip:port -slaves=ip1:port1,ip2:port2,... -user=用户名 -password=密码
```

### 参数说明

- `-master`: 主节点MySQL地址，格式为ip:port
- `-slaves`: 从节点MySQL地址列表，格式为ip1:port1,ip2:port2,...
- `-user`: MySQL用户名，默认为root
- `-password`: MySQL密码
- `-help`: 显示帮助信息

### 示例

```bash
./mysql_gtid_sync -master=192.168.1.100:3306 -slaves=192.168.1.101:3306,192.168.1.102:3306 -user=root -password=password123
```

## 工作原理

1. 连接到所有MySQL实例，获取它们的GTID集合
2. 分析主节点和从节点之间的GTID差异
3. 生成一个SQL文件，包含所有从节点缺失的GTID
4. 在主节点上执行生成的SQL文件，通过注入空事务使所有节点的GTID集合一致

## 注意事项

- 生成的SQL文件会禁用二进制日志，以避免在执行补全操作时产生新的二进制日志
- 执行生成的SQL文件前，建议先备份数据库
- SQL文件中会用注释标识出每个GTID来自哪个从节点

## 生成的SQL文件格式

```sql
-- GTID同步SQL文件 (用于在主节点执行)
-- 生成时间: 2023-01-01 12:00:00

-- 禁用二进制日志，避免生成新的事务
SET SESSION sql_log_bin = 0;

-- 以下是从节点 192.168.1.101:3306 缺失的GTIDs
-- UUID: 3E11FA47-71CA-11E1-9E33-C80AA9429562
SET GTID_NEXT = '3E11FA47-71CA-11E1-9E33-C80AA9429562:1';
BEGIN;
COMMIT;

-- 以下是从节点 192.168.1.102:3306 缺失的GTIDs
-- UUID: 3E11FA47-71CA-11E1-9E33-C80AA9429562
SET GTID_NEXT = '3E11FA47-71CA-11E1-9E33-C80AA9429562:2';
BEGIN;
COMMIT;

-- 恢复二进制日志和自动GTID分配
SET SESSION sql_log_bin = 1;
SET GTID_NEXT = 'AUTOMATIC';
```

## 执行生成的SQL文件

在主节点上执行生成的SQL文件：

```bash
mysql -h主节点地址 -u用户名 -p密码 < gtid_sync_master_主节点地址.sql
```

执行完成后，所有节点的GTID集合将一致，可以通过以下命令验证：

```sql
SHOW GLOBAL VARIABLES LIKE 'gtid_executed';
```