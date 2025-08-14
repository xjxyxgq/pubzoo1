# MySQL 表空间回收工具 - 使用示例

本文档提供了MySQL表空间回收工具的详细使用示例，涵盖从基础操作到高级场景的各种用法。

## 目录
- [基础使用示例](#基础使用示例)
- [集群环境使用](#集群环境使用)
- [高级配置示例](#高级配置示例)
- [编程接口示例](#编程接口示例)
- [故障处理示例](#故障处理示例)
- [性能优化示例](#性能优化示例)

## 基础使用示例

### 1. 快速开始 - 分析单个MySQL实例

```bash
# 基础分析命令
mysql-recycler analyze \
  --host=192.168.1.100 \
  --port=3306 \
  --user=admin \
  --password=mypassword

# 输出示例:
┌─────────────────────────────────────────────────────┐
│ MySQL表空间碎片分析报告                              │
├─────────────────────────────────────────────────────┤
│ 节点: 192.168.1.100:3306                           │
│ 总表数: 158                                         │
│ 碎片化表: 23                                        │
│ 总碎片空间: 2.5GB                                   │
│ 可回收空间: 2.1GB                                   │
└─────────────────────────────────────────────────────┘

详细碎片表信息:
┌──────────────┬───────────────┬─────────────┬─────────────┬─────────────┐
│ 数据库       │ 表名          │ 数据大小    │ 碎片大小    │ 碎片率      │
├──────────────┼───────────────┼─────────────┼─────────────┼─────────────┤
│ myapp        │ user_logs     │ 850MB       │ 320MB       │ 37.6%       │
│ myapp        │ order_history │ 1.2GB       │ 480MB       │ 40.0%       │
│ analytics    │ event_data    │ 2.1GB       │ 650MB       │ 31.0%       │
└──────────────┴───────────────┴─────────────┴─────────────┴─────────────┘
```

### 2. 执行表空间回收

```bash
# 基础回收命令
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --threshold=200MB

# 实时输出:
[2024-01-15 10:30:00] INFO: 开始表空间回收任务 (task-20240115-001)
[2024-01-15 10:30:01] INFO: 节点验证通过 - 192.168.1.101:3306 (从节点)
[2024-01-15 10:30:02] INFO: 发现 15 个需要回收的表
[2024-01-15 10:30:03] INFO: 开始回收表: myapp.user_logs (320MB碎片)
[2024-01-15 10:32:15] INFO: 完成回收表: myapp.user_logs (回收 315MB)
[2024-01-15 10:32:16] INFO: 开始回收表: myapp.order_history (480MB碎片)
[2024-01-15 10:35:42] INFO: 完成回收表: myapp.order_history (回收 465MB)
...
[2024-01-15 10:55:30] INFO: 任务完成 - 回收空间: 2.1GB, 处理表: 15, 用时: 25分30秒
```

### 3. 查看运行状态

```bash
# 查看所有活跃任务
mysql-recycler status

# 输出:
当前活跃任务: 2

┌─────────────────────────────────────────────────────────────────────────────┐
│ 任务ID: task-20240115-001                                                   │
│ 目标: 192.168.1.101:3306                                                   │
│ 模式: 独立节点模式                                                          │
│ 状态: 运行中                                                                │
│ 进度: 8/15 表 (53%)                                                        │
│ 当前: analytics.event_data                                                 │
│ 已回收: 1.2GB / 预计 2.1GB                                                 │
│ 预计完成: 10:55:30                                                         │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ 任务ID: task-20240115-002                                                   │
│ 目标: 192.168.1.102:3306                                                   │
│ 模式: 独立节点模式                                                          │
│ 状态: 等待中                                                                │
│ 进度: 0/23 表 (0%)                                                         │
│ 预计开始: 10:55:35                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 集群环境使用

### 1. 自动发现集群拓扑

```bash
# 发现集群拓扑结构
mysql-recycler topology \
  --host=192.168.1.100 \
  --user=admin \
  --password=mypassword \
  --output=json

# JSON输出:
{
  "master_node": {
    "host": "192.168.1.100",
    "port": 3306,
    "role": "主"
  },
  "slave_nodes": [
    {
      "host": "192.168.1.101",
      "port": 3306,
      "role": "从",
      "master_host": "192.168.1.100"
    },
    {
      "host": "192.168.1.102", 
      "port": 3306,
      "role": "从",
      "master_host": "192.168.1.101"
    }
  ],
  "relationships": {
    "192.168.1.100:3306": ["192.168.1.101:3306"],
    "192.168.1.101:3306": ["192.168.1.102:3306"]
  },
  "discovered_at": "2024-01-15T10:30:00Z"
}
```

### 2. 在从节点集群中执行回收

```bash
# 在整个从节点链中执行回收 (模式1)
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --mode=1 \
  --threshold=100MB \
  --concurrency=3

# 执行过程:
[2024-01-15 11:00:00] INFO: 发现集群拓扑 - 1主2从架构
[2024-01-15 11:00:01] INFO: 主节点: 192.168.1.100 (跳过)
[2024-01-15 11:00:02] INFO: 从节点: 192.168.1.101, 192.168.1.102
[2024-01-15 11:00:03] INFO: 在从节点上并发执行回收 (独立模式)
[2024-01-15 11:00:04] INFO: [节点101] 发现12个碎片表, 预计回收1.8GB
[2024-01-15 11:00:05] INFO: [节点102] 发现15个碎片表, 预计回收2.2GB  
[2024-01-15 11:00:06] INFO: 同时在2个节点上开始回收...
```

### 3. 使用复制同步模式

```bash
# 通过binlog同步执行回收 (模式2)
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --mode=2 \
  --threshold=200MB

# 执行过程:
[2024-01-15 11:30:00] INFO: 使用复制同步模式
[2024-01-15 11:30:01] INFO: 在指定节点执行, binlog将同步到下游
[2024-01-15 11:30:02] INFO: sql_log_bin保持开启状态
[2024-01-15 11:30:03] INFO: 开始在192.168.1.101上执行回收
[2024-01-15 11:30:04] INFO: 操作将自动同步到192.168.1.102
```

## 高级配置示例

### 1. 使用配置文件

创建 `config.yaml`:

```yaml
database:
  default_port: 3306
  timeout: 60s
  max_connections: 10
  conn_max_lifetime: 300s
  conn_max_idle_time: 60s

recycler:
  default_threshold: "500MB"
  max_concurrency: 5
  allow_master_operations: false
  default_mode: 1
  safety_check_interval: "30s"
  table_lock_timeout: "300s"

monitoring:
  persist_interval: "5m"
  status_file: "/var/log/mysql-recycler/status.json"
  max_history_records: 1000
  report_format: "json"

external_api:
  cluster_info_url: "http://cluster-api.internal/get_mysql_cluster_instance/"
  timeout: "10s"
  retry_times: 3
  enable_cache: true
  cache_timeout: "5m"
```

使用配置文件:

```bash
mysql-recycler --config=config.yaml reclaim --host=192.168.1.101
```

### 2. 环境变量配置

```bash
# 设置环境变量
export MYSQL_RECYCLER_HOST=192.168.1.101
export MYSQL_RECYCLER_USER=recycler_user
export MYSQL_RECYCLER_PASSWORD=secure_password
export MYSQL_RECYCLER_THRESHOLD=300MB
export MYSQL_RECYCLER_CONCURRENCY=8
export MYSQL_RECYCLER_LOG_LEVEL=debug

# 使用环境变量
mysql-recycler reclaim
```

### 3. 高级过滤和选择

```bash
# 只处理特定数据库
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --database=myapp,analytics

# 排除特定数据库
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --exclude-db=mysql,information_schema,performance_schema

# 处理特定表模式
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --table-pattern="*_logs,*_history"

# 模拟运行 (不实际执行)
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --dry-run
```

## 编程接口示例

### 1. Go语言集成示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "nucc.com/mysql_tablespace_recycling/cluster"
    "nucc.com/mysql_tablespace_recycling/recycler"
    "nucc.com/mysql_tablespace_recycling/monitor"
    "nucc.com/mysql_tablespace_recycling/config"
)

func main() {
    // 1. 加载配置
    cfg, err := config.LoadConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    
    // 2. 创建集群发现器
    discoverer := cluster.NewTopologyDiscoverer(cfg.GetClusterConfig())
    
    // 3. 发现集群拓扑
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    entryPoint := &cluster.NodeConfig{
        Host:     "192.168.1.100",
        Port:     3306,
        Username: "admin",
        Password: "mypassword",
        Database: "mysql",
    }
    
    topology, err := discoverer.DiscoverTopology(ctx, entryPoint)
    if err != nil {
        log.Fatalf("Failed to discover topology: %v", err)
    }
    
    fmt.Printf("发现集群: 主节点 %s, 从节点 %d个\n", 
        topology.MasterNode.Host, len(topology.SlaveNodes))
    
    // 4. 创建回收器
    recyclerInstance := recycler.NewRecycler(cfg.GetRecyclerConfig())
    
    // 5. 创建监控器
    monitorInstance := monitor.NewMonitor(cfg.GetMonitorConfig())
    
    // 6. 分析碎片
    for _, slave := range topology.SlaveNodes {
        target := &recycler.RecycleTarget{
            Node:      slave,
            Threshold: 100 * 1024 * 1024, // 100MB
            Mode:      recycler.ModeIndependent,
        }
        
        report, err := recyclerInstance.AnalyzeFragmentation(ctx, target)
        if err != nil {
            log.Printf("Failed to analyze node %s: %v", slave.Host, err)
            continue
        }
        
        fmt.Printf("节点 %s: 发现 %d 个碎片表, 可回收 %d MB\n",
            slave.Host, 
            len(report.FragmentedTables),
            report.EstimatedReclaimableSpace/1024/1024)
        
        // 7. 如果有碎片表，创建回收任务
        if len(report.FragmentedTables) > 0 {
            task := &recycler.RecycleTask{
                ID:       fmt.Sprintf("task-%d", time.Now().Unix()),
                Target:   target,
                Tables:   report.FragmentedTables,
                Priority: 1,
                Status:   recycler.StatusPending,
            }
            
            // 注册任务到监控器
            err = monitorInstance.StartTask(task)
            if err != nil {
                log.Printf("Failed to register task: %v", err)
                continue
            }
            
            // 执行回收
            go func(t *recycler.RecycleTask) {
                result, err := recyclerInstance.ExecuteRecycle(ctx, []*recycler.RecycleTask{t})
                if err != nil {
                    log.Printf("回收任务失败 %s: %v", t.ID, err)
                    return
                }
                
                fmt.Printf("任务 %s 完成: 回收 %d MB空间\n", 
                    t.ID, result.ReclaimedSpace/1024/1024)
            }(task)
        }
    }
    
    // 8. 等待所有任务完成
    for {
        tasks, err := monitorInstance.ListActiveTasks()
        if err != nil {
            log.Printf("Failed to get active tasks: %v", err)
            break
        }
        
        if len(tasks) == 0 {
            fmt.Println("所有任务已完成")
            break
        }
        
        fmt.Printf("等待 %d 个任务完成...\n", len(tasks))
        time.Sleep(10 * time.Second)
    }
}
```

### 2. 自定义回收策略示例

```go
// 自定义回收策略 - 优先回收大碎片表
type LargeFragmentStrategy struct {
    minFragmentSize int64
    maxTableSize    int64
}

func (s *LargeFragmentStrategy) ShouldRecycle(table *recycler.TableFragmentation) bool {
    // 只回收碎片超过最小阈值且表不超过最大限制的表
    return table.DataFree >= s.minFragmentSize && 
           (table.DataLength + table.IndexLength) <= s.maxTableSize
}

func (s *LargeFragmentStrategy) GetPriority(table *recycler.TableFragmentation) int {
    // 碎片越大优先级越高
    return int(table.DataFree / (1024 * 1024)) // 以MB为单位
}

func (s *LargeFragmentStrategy) PreRecycle(ctx context.Context, table *recycler.TableFragmentation) error {
    // 回收前的预处理，比如检查表锁状态
    fmt.Printf("准备回收表 %s.%s (碎片: %d MB)\n", 
        table.Schema, table.TableName, table.DataFree/1024/1024)
    return nil
}

func (s *LargeFragmentStrategy) PostRecycle(ctx context.Context, table *recycler.TableFragmentation, result *recycler.RecycleResult) error {
    // 回收后的处理，比如记录统计信息
    fmt.Printf("完成回收表 %s.%s (回收: %d MB)\n", 
        table.Schema, table.TableName, result.ReclaimedSpace/1024/1024)
    return nil
}

// 使用自定义策略
func main() {
    // ...初始化代码...
    
    recyclerInstance := recycler.NewRecycler(cfg.GetRecyclerConfig())
    
    // 注册自定义策略
    strategy := &LargeFragmentStrategy{
        minFragmentSize: 500 * 1024 * 1024, // 500MB
        maxTableSize:    50 * 1024 * 1024 * 1024, // 50GB
    }
    
    err := recyclerInstance.RegisterStrategy("large_fragment", strategy)
    if err != nil {
        log.Fatalf("Failed to register strategy: %v", err)
    }
}
```

### 3. 自定义监控钩子示例

```go
// 自定义监控钩子 - 发送通知
type NotificationHook struct {
    webhookURL string
    client     *http.Client
}

func (h *NotificationHook) OnTaskStart(task *recycler.RecycleTask) error {
    message := fmt.Sprintf("表空间回收任务开始: %s (目标: %s)", 
        task.ID, task.Target.Node.Host)
    return h.sendNotification(message)
}

func (h *NotificationHook) OnTaskProgress(taskID string, progress *monitor.ProgressUpdate) error {
    if progress.CompletedTables%5 == 0 { // 每完成5个表发送一次通知
        message := fmt.Sprintf("任务 %s 进度更新: 已完成 %d 个表", 
            taskID, progress.CompletedTables)
        return h.sendNotification(message)
    }
    return nil
}

func (h *NotificationHook) OnTaskComplete(taskID string, result *recycler.RecycleResult) error {
    message := fmt.Sprintf("任务 %s 完成: 回收空间 %d MB, 处理表 %d 个", 
        taskID, result.ReclaimedSpace/1024/1024, result.ProcessedTables)
    return h.sendNotification(message)
}

func (h *NotificationHook) OnTaskFailed(taskID string, err error) error {
    message := fmt.Sprintf("任务 %s 失败: %v", taskID, err)
    return h.sendNotification(message)
}

func (h *NotificationHook) sendNotification(message string) error {
    payload := map[string]string{
        "text": message,
        "timestamp": time.Now().Format(time.RFC3339),
    }
    
    data, _ := json.Marshal(payload)
    resp, err := h.client.Post(h.webhookURL, "application/json", bytes.NewBuffer(data))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("notification failed with status: %d", resp.StatusCode)
    }
    
    return nil
}

// 使用通知钩子
func main() {
    // ...初始化代码...
    
    monitorInstance := monitor.NewMonitor(cfg.GetMonitorConfig())
    
    // 注册通知钩子
    hook := &NotificationHook{
        webhookURL: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
        client:     &http.Client{Timeout: 10 * time.Second},
    }
    
    err := monitorInstance.RegisterHook(hook)
    if err != nil {
        log.Fatalf("Failed to register hook: %v", err)
    }
}
```

## 故障处理示例

### 1. 连接失败处理

```bash
# 使用重试和超时控制
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --connection-timeout=30s \
  --query-timeout=600s \
  --retry-times=3

# 输出示例:
[2024-01-15 14:30:00] WARN: 连接失败，2秒后重试 (1/3)
[2024-01-15 14:30:02] WARN: 连接失败，4秒后重试 (2/3) 
[2024-01-15 14:30:06] INFO: 连接成功
```

### 2. 权限不足处理

```bash
# 检查权限
mysql-recycler analyze \
  --host=192.168.1.101 \
  --user=limited_user \
  --password=mypassword \
  --check-permissions

# 输出:
权限检查结果:
✓ SELECT on information_schema.tables: OK
✗ ALTER on target databases: MISSING
✗ PROCESS (for SHOW SLAVE STATUS): MISSING

建议执行以下SQL授权:
GRANT ALTER ON *.* TO 'limited_user'@'%';
GRANT PROCESS ON *.* TO 'limited_user'@'%';
FLUSH PRIVILEGES;
```

### 3. 表锁定处理

```bash
# 使用表锁超时和跳过机制
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --table-lock-timeout=60s \
  --skip-locked-tables

# 输出示例:
[2024-01-15 15:00:00] INFO: 开始回收表: myapp.user_logs
[2024-01-15 15:00:30] WARN: 表 myapp.user_logs 被锁定，等待中...
[2024-01-15 15:01:00] WARN: 表 myapp.user_logs 锁定超时，跳过处理
[2024-01-15 15:01:01] INFO: 开始回收表: myapp.order_history
```

### 4. 主节点保护触发

```bash
# 当检测到主节点时的处理
mysql-recycler reclaim \
  --host=192.168.1.100 \
  --user=admin \
  --password=mypassword

# 输出:
[2024-01-15 16:00:00] ERROR: 操作被阻止 - 目标节点被识别为主节点
[2024-01-15 16:00:01] INFO: 验证结果:
  - read_only = OFF (主节点标识)
  - 外部API确认: instance_role = "主"
  - SHOW SLAVE STATUS: 无复制关系

解决方案:
1. 在从节点上执行: mysql-recycler reclaim --host=192.168.1.101
2. 强制在主节点执行: mysql-recycler reclaim --host=192.168.1.100 --allow-master
3. 检查节点角色: mysql-recycler topology --host=192.168.1.100
```

## 性能优化示例

### 1. 并发优化

```bash
# 调整并发参数
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --concurrency=10 \
  --max-connections=20

# 监控系统资源使用
mysql-recycler status --watch --format=detailed

# 输出:
实时性能监控:
┌─────────────────────────────────────────────────────────────────────────────┐
│ CPU使用率: 45% | 内存使用: 2.1GB | 网络IO: 50MB/s                           │
│ 数据库连接: 15/20 | 活跃goroutine: 25 | 队列任务: 3                        │
├─────────────────────────────────────────────────────────────────────────────┤
│ 任务性能统计:                                                               │
│ - 平均表回收时间: 2.5分钟                                                   │
│ - 回收效率: 850MB/分钟                                                      │
│ - 成功率: 98.5%                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2. 批量处理优化

```bash
# 使用批量处理提高效率
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --batch-size=50 \
  --batch-interval=1s

# 输出:
[2024-01-15 17:00:00] INFO: 使用批量处理模式 (batch-size: 50)
[2024-01-15 17:00:01] INFO: 批次1: 处理表 1-50
[2024-01-15 17:15:30] INFO: 批次1完成: 回收 1.2GB, 用时 15分30秒
[2024-01-15 17:15:31] INFO: 批次2: 处理表 51-100
```

### 3. 内存和连接池优化

创建优化配置文件 `performance.yaml`:

```yaml
database:
  max_connections: 50
  conn_max_lifetime: 1800s  # 30分钟
  conn_max_idle_time: 300s   # 5分钟
  query_timeout: 1800s       # 30分钟
  
recycler:
  max_concurrency: 15
  batch_size: 100
  worker_pool_size: 20
  table_lock_timeout: 300s
  safety_check_interval: 60s
  
monitoring:
  persist_interval: "30s"    # 更频繁的状态持久化
  memory_limit: "4GB"        # 内存使用限制
  gc_interval: "5m"          # 垃圾回收间隔
```

使用优化配置:

```bash
mysql-recycler --config=performance.yaml reclaim --host=192.168.1.101
```

### 4. 分时段执行优化

```bash
# 在业务低峰期执行
mysql-recycler reclaim \
  --host=192.168.1.101 \
  --user=admin \
  --password=mypassword \
  --schedule="02:00-06:00" \
  --max-duration=4h

# 输出:
[2024-01-15 01:59:59] INFO: 等待调度时间窗口 (02:00-06:00)
[2024-01-15 02:00:00] INFO: 进入执行时间窗口，开始回收任务
[2024-01-15 05:59:59] INFO: 接近时间窗口结束，准备停止新任务
[2024-01-15 06:00:00] INFO: 时间窗口结束，等待当前任务完成后停止
```

---

**提示**: 这些示例覆盖了工具的主要使用场景。在生产环境中使用时，建议：

1. 先在测试环境验证配置
2. 使用 `--dry-run` 参数预览操作
3. 在业务低峰期执行
4. 确保有完整的数据备份
5. 监控系统资源使用情况
6. 配置适当的告警和通知机制