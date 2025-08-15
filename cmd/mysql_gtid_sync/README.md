# MySQL GTID 同步工具

这个工具用于同步MySQL复制集群中的GTID集合，确保所有节点的GTID集合一致。

## 功能特点

- 连接到多个MySQL实例，检查它们的GTID集合
- 计算所有节点（包括主节点和所有从节点）的GTID并集
- 分析每个节点与GTID并集的差异
- 生成一个SQL文件，用于在主节点上执行，通过注入空事务使所有节点的GTID集合一致
- 支持多个从节点同时同步
- 在SQL文件中用注释标识出每个GTID来自哪个节点
- 保持二进制日志开启，确保事务能够正常复制到所有从节点

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
2. 计算所有节点的GTID并集
3. 分析每个节点（包括主节点）与GTID并集的差异
4. 生成一个SQL文件，包含所有节点缺失的GTID
5. 在主节点上执行生成的SQL文件，通过注入空事务使所有节点的GTID集合一致
6. 由于保持二进制日志开启，这些事务会通过复制同步到所有从节点

## 注意事项

- 生成的SQL文件会保持二进制日志开启，确保事务能够正常复制到所有从节点
- 执行生成的SQL文件前，建议先备份数据库
- SQL文件中会用注释标识出每个GTID来自哪个节点
- 此工具不仅补充从节点缺少的GTID，还会将所有节点（包括主节点和所有从节点）的GTID集合取并集，确保整个集群的GTID集合一致

## 生成的SQL文件格式

```sql
-- GTID同步SQL文件 (用于在主节点执行)
-- 生成时间: 2023-01-01 12:00:00
-- 注意: 此文件将在主节点上执行，并通过复制同步到所有从节点

-- 确保GTID_NEXT可以手动设置
SET SESSION GTID_NEXT_AUTOMATIC_POSITION = 0;

-- 以下是节点 192.168.1.100:3306 缺失的GTIDs
-- UUID: 3E11FA47-71CA-11E1-9E33-C80AA9429562
SET GTID_NEXT = '3E11FA47-71CA-11E1-9E33-C80AA9429562:1';
BEGIN;
COMMIT;

-- 以下是节点 192.168.1.101:3306 缺失的GTIDs
-- UUID: 3E11FA47-71CA-11E1-9E33-C80AA9429562
SET GTID_NEXT = '3E11FA47-71CA-11E1-9E33-C80AA9429562:2';
BEGIN;
COMMIT;

-- 以下是节点 192.168.1.102:3306 缺失的GTIDs
-- UUID: 5F11FA47-71CA-11E1-9E33-C80AA9429562
SET GTID_NEXT = '5F11FA47-71CA-11E1-9E33-C80AA9429562:3';
BEGIN;
COMMIT;

-- 恢复自动GTID分配
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