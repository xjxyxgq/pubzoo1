# 起始节点功能使用示例

新增的`--start-node`参数允许你指定一个特定的MySQL节点作为起点，然后只在该节点及其下游节点上执行表空间回收操作。

## 功能说明

当使用`--start-node`参数时，工具会：

1. 发现完整的集群拓扑关系
2. 从指定的起始节点开始，递归获取所有下游节点
3. 只对这些节点执行表空间回收操作

这对于以下场景特别有用：
- 希望从某个特定的从节点开始进行表空间回收
- 避免在上游节点进行可能影响下游复制的操作
- 分阶段进行集群维护

## 使用示例

### 1. 基本用法

```bash
# 从slave1.db:3306开始，对该节点及其所有下游节点执行回收
./mysql-recycler reclaim \
  --host=entry.db \
  --port=3306 \
  --user=admin \
  --password=secret \
  --start-node=slave1.db:3306 \
  --dry-run
```

### 2. 结合其他参数使用

```bash
# 指定起始节点，设置回收阈值和并发数
./mysql-recycler reclaim \
  --host=entry.db \
  --port=3306 \
  --user=admin \
  --password=secret \
  --start-node=slave2.db:3306 \
  --threshold=500MB \
  --concurrency=2 \
  --mode=1
```

### 3. 监控模式

```bash
# 使用监控模式查看详细进度
./mysql-recycler reclaim \
  --host=entry.db \
  --port=3306 \
  --user=admin \
  --password=secret \
  --start-node=slave1.db:3306 \
  --watch \
  --verbose
```

## 集群拓扑示例

假设有以下集群拓扑：

```
master.db:3306
├── slave1.db:3306
│   ├── slave1a.db:3306
│   └── slave1b.db:3306
└── slave2.db:3306
    ├── slave2a.db:3306
    └── slave2b.db:3306
```

### 不同起始节点的影响范围

1. **不指定起始节点**（默认行为）：
   - 只对连接的入口节点执行回收

2. **--start-node=master.db:3306**：
   - 对所有节点执行回收：master.db, slave1.db, slave1a.db, slave1b.db, slave2.db, slave2a.db, slave2b.db

3. **--start-node=slave1.db:3306**：
   - 对slave1及其下游执行回收：slave1.db, slave1a.db, slave1b.db

4. **--start-node=slave2.db:3306**：
   - 对slave2及其下游执行回收：slave2.db, slave2a.db, slave2b.db

5. **--start-node=slave1a.db:3306**：
   - 只对slave1a执行回收：slave1a.db

## 输出示例

当使用`--start-node`参数时，工具会显示拓扑发现过程和目标节点：

```
发现集群拓扑以确定回收范围...
找到 3 个目标节点进行回收: slave1.db:3306, slave1a.db:3306, slave1b.db:3306

正在分析节点 slave1.db:3306 的表空间碎片... (1/3)
节点 slave1.db:3306: 发现 5 个碎片化表，预计可回收 256 MB 空间

正在分析节点 slave1a.db:3306 的表空间碎片... (2/3)
节点 slave1a.db:3306: 发现 3 个碎片化表，预计可回收 128 MB 空间

正在分析节点 slave1b.db:3306 的表空间碎片... (3/3)
节点 slave1b.db:3306: 发现 2 个碎片化表，预计可回收 64 MB 空间

共发现 10 个碎片化表，预计可回收 448 MB 空间

回收计划:
========
总任务数: 3
总待处理表数: 10

任务 1: slave1.db:3306
- 任务ID: reclaim-slave1.db:3306-1692123456
- 工作模式: 独立节点模式 (sql_log_bin=OFF)
- 并发数: 3
- 待处理表数: 5
- 运行模式: 模拟运行 (不实际执行)

...
```

## 注意事项

1. **权限要求**：确保连接用户对所有目标节点都有适当的权限
2. **网络连通性**：确保工具能够连接到所有目标节点
3. **拓扑发现**：工具需要能够通过`SHOW SLAVE HOSTS`等命令发现集群拓扑
4. **安全检查**：即使指定了起始节点，主节点安全检查仍然生效（除非使用`--allow-master`）

## 故障排除

### 常见错误

1. **"未找到起始节点"**：
   - 检查节点地址格式是否正确（host:port）
   - 确认节点在集群拓扑中存在

2. **"拓扑发现失败"**：
   - 检查网络连通性
   - 确认数据库用户权限
   - 查看详细错误信息（使用`--verbose`）

3. **"连接超时"**：
   - 增加连接超时时间
   - 检查防火墙设置
   - 确认MySQL服务正常运行