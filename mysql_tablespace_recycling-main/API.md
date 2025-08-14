# MySQL 表空间回收工具 - API文档

本文档详细描述了MySQL表空间回收工具的API接口和核心包结构。

## 目录
- [核心包结构](#核心包结构)
- [命令行接口](#命令行接口)
- [核心API](#核心api)
- [配置API](#配置api)
- [监控API](#监控api)
- [外部集成API](#外部集成api)

## 核心包结构

```go
package main

import (
    "nucc.com/mysql_tablespace_recycling/cluster"
    "nucc.com/mysql_tablespace_recycling/recycler" 
    "nucc.com/mysql_tablespace_recycling/monitor"
    "nucc.com/mysql_tablespace_recycling/config"
)
```

### cluster 包 - 集群拓扑管理

#### 主要接口

```go
// TopologyDiscoverer 集群拓扑发现器接口
type TopologyDiscoverer interface {
    // DiscoverTopology 发现集群拓扑结构
    DiscoverTopology(ctx context.Context, entryPoint *NodeConfig) (*ClusterTopology, error)
    
    // ValidateNode 验证节点角色和状态
    ValidateNode(ctx context.Context, node *NodeConfig) (*NodeStatus, error)
    
    // GetDownstreamNodes 获取下游节点列表
    GetDownstreamNodes(ctx context.Context, node *NodeConfig) ([]*NodeConfig, error)
}
```

#### 核心数据结构

```go
// NodeConfig MySQL节点配置
type NodeConfig struct {
    Host        string `json:"host" yaml:"host"`
    Port        int    `json:"port" yaml:"port"`
    Username    string `json:"username" yaml:"username"`
    Password    string `json:"password" yaml:"password"`
    Database    string `json:"database" yaml:"database"`
    Timeout     time.Duration `json:"timeout" yaml:"timeout"`
}

// NodeStatus 节点状态信息
type NodeStatus struct {
    IsMaster     bool              `json:"is_master"`
    IsReadOnly   bool              `json:"is_read_only"`
    Role         string            `json:"role"` // "主", "从", "unknown"
    SlaveStatus  *SlaveStatus      `json:"slave_status,omitempty"`
    Hosts        []string          `json:"slave_hosts,omitempty"`
    LastCheck    time.Time         `json:"last_check"`
}

// ClusterTopology 集群拓扑结构
type ClusterTopology struct {
    MasterNode   *NodeConfig       `json:"master_node"`
    SlaveNodes   []*NodeConfig     `json:"slave_nodes"`
    Relationships map[string][]string `json:"relationships"` // 父子关系映射
    DiscoveredAt time.Time         `json:"discovered_at"`
}
```

#### API方法

```go
// NewTopologyDiscoverer 创建拓扑发现器
func NewTopologyDiscoverer(cfg *config.ClusterConfig) TopologyDiscoverer

// ConcurrentDiscovery 并发发现集群拓扑
func (d *DefaultDiscoverer) ConcurrentDiscovery(
    ctx context.Context, 
    entryPoint *NodeConfig,
    maxConcurrency int,
) (*ClusterTopology, error)

// ValidateMasterNode 多重验证主节点身份
func (d *DefaultDiscoverer) ValidateMasterNode(
    ctx context.Context,
    node *NodeConfig,
    externalAPI ExternalAPIClient,
) error
```

### recycler 包 - 表空间回收核心

#### 主要接口

```go
// TablespaceRecycler 表空间回收器接口
type TablespaceRecycler interface {
    // AnalyzeFragmentation 分析表空间碎片
    AnalyzeFragmentation(ctx context.Context, target *RecycleTarget) (*FragmentationReport, error)
    
    // ExecuteRecycle 执行表空间回收
    ExecuteRecycle(ctx context.Context, tasks []*RecycleTask) (*RecycleResult, error)
    
    // GetProgress 获取回收进度
    GetProgress(taskID string) (*ProgressInfo, error)
}
```

#### 核心数据结构

```go
// RecycleTarget 回收目标配置
type RecycleTarget struct {
    Node         *cluster.NodeConfig `json:"node"`
    Databases    []string           `json:"databases,omitempty"`
    Tables       []string           `json:"tables,omitempty"`
    ExcludeDBs   []string           `json:"exclude_dbs,omitempty"`
    ExcludeTables []string          `json:"exclude_tables,omitempty"`
    Mode         RecycleMode        `json:"mode"`
    Threshold    int64              `json:"threshold"` // bytes
}

// RecycleMode 回收模式
type RecycleMode int

const (
    ModeIndependent RecycleMode = 1 // 独立节点模式
    ModeReplication RecycleMode = 2 // 复制同步模式
)

// FragmentationReport 碎片分析报告
type FragmentationReport struct {
    NodeInfo     *cluster.NodeConfig `json:"node_info"`
    TotalTables  int                `json:"total_tables"`
    FragmentedTables []*TableFragmentation `json:"fragmented_tables"`
    TotalFragmentSize int64         `json:"total_fragment_size"`
    EstimatedReclaimableSpace int64 `json:"estimated_reclaimable_space"`
    GeneratedAt  time.Time          `json:"generated_at"`
}

// TableFragmentation 表碎片信息
type TableFragmentation struct {
    Schema       string    `json:"schema"`
    TableName    string    `json:"table_name"`
    Engine       string    `json:"engine"`
    DataLength   int64     `json:"data_length"`
    IndexLength  int64     `json:"index_length"`
    DataFree     int64     `json:"data_free"`
    FragmentRatio float64  `json:"fragment_ratio"`
}

// RecycleTask 回收任务
type RecycleTask struct {
    ID           string              `json:"id"`
    Target       *RecycleTarget      `json:"target"`
    Tables       []*TableFragmentation `json:"tables"`
    Priority     int                 `json:"priority"`
    CreatedAt    time.Time           `json:"created_at"`
    Status       TaskStatus          `json:"status"`
}

// TaskStatus 任务状态
type TaskStatus int

const (
    StatusPending    TaskStatus = iota
    StatusRunning
    StatusCompleted
    StatusFailed
    StatusCancelled
)
```

#### API方法

```go
// NewRecycler 创建回收器实例
func NewRecycler(cfg *config.RecyclerConfig) TablespaceRecycler

// BatchRecycle 批量回收表空间
func (r *DefaultRecycler) BatchRecycle(
    ctx context.Context,
    tasks []*RecycleTask,
    concurrency int,
) (*RecycleResult, error)

// SafeRecycleTable 安全回收单个表
func (r *DefaultRecycler) SafeRecycleTable(
    ctx context.Context,
    node *cluster.NodeConfig,
    schema, table string,
    mode RecycleMode,
) error
```

### monitor 包 - 状态监控

#### 主要接口

```go
// ProgressMonitor 进度监控器接口
type ProgressMonitor interface {
    // StartTask 开始监控任务
    StartTask(task *recycler.RecycleTask) error
    
    // UpdateProgress 更新任务进度
    UpdateProgress(taskID string, progress *ProgressUpdate) error
    
    // GetTaskStatus 获取任务状态
    GetTaskStatus(taskID string) (*TaskProgress, error)
    
    // ListActiveTasks 列出活跃任务
    ListActiveTasks() ([]*TaskProgress, error)
    
    // PersistStatus 持久化状态到文件
    PersistStatus() error
}
```

#### 核心数据结构

```go
// TaskProgress 任务进度信息
type TaskProgress struct {
    TaskID           string                  `json:"task_id"`
    Target           *recycler.RecycleTarget `json:"target"`
    Status           recycler.TaskStatus     `json:"status"`
    TotalTables      int                     `json:"total_tables"`
    CompletedTables  int                     `json:"completed_tables"`
    CurrentTable     string                  `json:"current_table"`
    StartTime        time.Time               `json:"start_time"`
    EstimatedEndTime *time.Time              `json:"estimated_end_time,omitempty"`
    ReclaimedSpace   int64                   `json:"reclaimed_space"`
    ErrorMessage     string                  `json:"error_message,omitempty"`
    LastUpdate       time.Time               `json:"last_update"`
}

// ProgressUpdate 进度更新信息
type ProgressUpdate struct {
    CompletedTables  int    `json:"completed_tables,omitempty"`
    CurrentTable     string `json:"current_table,omitempty"`
    ReclaimedSpace   int64  `json:"reclaimed_space,omitempty"`
    ErrorMessage     string `json:"error_message,omitempty"`
}

// MonitorStatus 监控器状态
type MonitorStatus struct {
    ActiveTasks      int       `json:"active_tasks"`
    CompletedTasks   int       `json:"completed_tasks"`
    TotalReclaimedSpace int64  `json:"total_reclaimed_space"`
    LastPersistTime  time.Time `json:"last_persist_time"`
}
```

#### API方法

```go
// NewMonitor 创建监控器实例
func NewMonitor(cfg *config.MonitorConfig) ProgressMonitor

// StartBackgroundPersistence 启动后台持久化
func (m *DefaultMonitor) StartBackgroundPersistence(
    ctx context.Context,
    interval time.Duration,
) error

// GenerateReport 生成进度报告
func (m *DefaultMonitor) GenerateReport() (*ProgressReport, error)
```

### config 包 - 配置管理

#### 核心数据结构

```go
// Config 主配置结构
type Config struct {
    Database    DatabaseConfig  `yaml:"database"`
    Recycler    RecyclerConfig  `yaml:"recycler"`
    Monitoring  MonitorConfig   `yaml:"monitoring"`
    ExternalAPI ExternalAPIConfig `yaml:"external_api"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
    DefaultPort     int           `yaml:"default_port"`
    Timeout         time.Duration `yaml:"timeout"`
    MaxConnections  int           `yaml:"max_connections"`
    ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
    ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// RecyclerConfig 回收器配置
type RecyclerConfig struct {
    DefaultThreshold        int64         `yaml:"default_threshold"`
    MaxConcurrency         int           `yaml:"max_concurrency"`
    AllowMasterOperations  bool          `yaml:"allow_master_operations"`
    DefaultMode            int           `yaml:"default_mode"`
    SafetyCheckInterval    time.Duration `yaml:"safety_check_interval"`
    TableLockTimeout       time.Duration `yaml:"table_lock_timeout"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
    PersistInterval    time.Duration `yaml:"persist_interval"`
    StatusFile         string        `yaml:"status_file"`
    MaxHistoryRecords  int           `yaml:"max_history_records"`
    ReportFormat       string        `yaml:"report_format"`
}

// ExternalAPIConfig 外部API配置
type ExternalAPIConfig struct {
    ClusterInfoURL string        `yaml:"cluster_info_url"`
    Timeout        time.Duration `yaml:"timeout"`
    RetryTimes     int           `yaml:"retry_times"`
    EnableCache    bool          `yaml:"enable_cache"`
    CacheTimeout   time.Duration `yaml:"cache_timeout"`
}
```

#### API方法

```go
// LoadConfig 加载配置文件
func LoadConfig(configFile string) (*Config, error)

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() (*Config, error)

// Validate 验证配置有效性
func (c *Config) Validate() error

// GetDatabaseConfig 获取数据库配置
func (c *Config) GetDatabaseConfig() *DatabaseConfig

// GetRecyclerConfig 获取回收器配置
func (c *Config) GetRecyclerConfig() *RecyclerConfig
```

## 命令行接口

### 主命令

```bash
mysql-recycler [global-options] <command> [command-options]
```

#### 全局选项

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `--config` | string | 配置文件路径 | `config.yaml` |
| `--log-level` | string | 日志级别 | `info` |
| `--log-format` | string | 日志格式 | `json` |
| `--verbose` | bool | 详细输出 | `false` |

### 子命令

#### 1. analyze - 分析表空间碎片

```bash
mysql-recycler analyze [options]
```

**选项:**

| 参数 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `--host` | string | MySQL主机地址 | ✓ |
| `--port` | int | MySQL端口 | - |
| `--user` | string | MySQL用户名 | ✓ |
| `--password` | string | MySQL密码 | ✓ |
| `--database` | string | 指定数据库 | - |
| `--table` | string | 指定表名 | - |
| `--threshold` | string | 碎片阈值 | - |
| `--output` | string | 输出格式(json/table) | - |

**示例:**

```bash
# 分析指定实例的所有表
mysql-recycler analyze --host=192.168.1.100 --user=admin --password=secret

# 分析特定数据库
mysql-recycler analyze --host=192.168.1.100 --user=admin --password=secret --database=myapp

# 输出JSON格式报告
mysql-recycler analyze --host=192.168.1.100 --user=admin --password=secret --output=json
```

#### 2. reclaim - 执行表空间回收

```bash
mysql-recycler reclaim [options]
```

**选项:**

| 参数 | 类型 | 说明 | 必需 |
|------|------|------|------|
| `--host` | string | MySQL主机地址 | ✓ |
| `--port` | int | MySQL端口 | - |
| `--user` | string | MySQL用户名 | ✓ |
| `--password` | string | MySQL密码 | ✓ |
| `--mode` | int | 操作模式(1/2) | - |
| `--threshold` | string | 碎片回收阈值 | - |
| `--concurrency` | int | 并发数 | - |
| `--allow-master` | bool | 允许主节点操作 | - |
| `--dry-run` | bool | 模拟运行 | - |
| `--database` | string | 指定数据库 | - |
| `--exclude-db` | stringSlice | 排除数据库 | - |

**示例:**

```bash
# 基本回收操作
mysql-recycler reclaim --host=192.168.1.101 --user=admin --password=secret

# 指定模式和阈值
mysql-recycler reclaim --host=192.168.1.101 --user=admin --password=secret --mode=1 --threshold=500MB

# 模拟运行
mysql-recycler reclaim --host=192.168.1.101 --user=admin --password=secret --dry-run
```

#### 3. status - 查看状态

```bash
mysql-recycler status [options]
```

**选项:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `--task-id` | string | 特定任务ID |
| `--format` | string | 输出格式 |
| `--watch` | bool | 持续监控 |

**示例:**

```bash
# 查看所有活跃任务
mysql-recycler status

# 查看特定任务
mysql-recycler status --task-id=task-001

# 持续监控
mysql-recycler status --watch
```

#### 4. topology - 集群拓扑

```bash
mysql-recycler topology [options]
```

**选项:**

| 参数 | 类型 | 说明 |
|------|------|------|
| `--host` | string | 入口节点地址 |
| `--user` | string | MySQL用户名 |
| `--password` | string | MySQL密码 |
| `--output` | string | 输出格式 |

## 核心API

### 错误处理

所有API方法都使用标准的Go错误处理模式：

```go
// 自定义错误类型
type RecyclerError struct {
    Code    ErrorCode `json:"code"`
    Message string    `json:"message"`
    Cause   error     `json:"cause,omitempty"`
}

// 错误码定义
type ErrorCode int

const (
    ErrInvalidConfig ErrorCode = 1001
    ErrConnectionFailed ErrorCode = 1002
    ErrMasterNodeOperation ErrorCode = 1003
    ErrTableLocked ErrorCode = 1004
    ErrInsufficientPrivileges ErrorCode = 1005
)
```

### 上下文管理

所有长时间运行的操作都支持context.Context：

```go
// 带超时的操作
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

result, err := recycler.ExecuteRecycle(ctx, tasks)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Printf("操作超时")
    }
    return err
}
```

### 并发安全

所有API都是线程安全的，支持并发调用：

```go
// 并发分析多个节点
var wg sync.WaitGroup
results := make(chan *FragmentationReport, len(nodes))

for _, node := range nodes {
    wg.Add(1)
    go func(n *cluster.NodeConfig) {
        defer wg.Done()
        report, err := recycler.AnalyzeFragmentation(ctx, &RecycleTarget{
            Node: n,
            Threshold: threshold,
        })
        if err == nil {
            results <- report
        }
    }(node)
}

wg.Wait()
close(results)
```

## 外部集成API

### 集群信息API

**接口地址:** `POST /api/get_mysql_cluster_instance/`

**请求格式:**
```json
{
  "search_ins_ip": "192.168.1.100"
}
```

**响应格式:**
```json
{
  "status": "ok",
  "data": {
    "cluster_name": "prod-cluster-01",
    "cluster_group_name": "business-group",
    "instance_role": "从",
    "instance_read_only": "ON"
  }
}
```

**集成示例:**
```go
type ExternalAPIClient interface {
    GetClusterInstance(ctx context.Context, ip string) (*ClusterInstanceInfo, error)
}

type ClusterInstanceInfo struct {
    ClusterName      string `json:"cluster_name"`
    ClusterGroupName string `json:"cluster_group_name"`
    InstanceRole     string `json:"instance_role"`
    InstanceReadOnly string `json:"instance_read_only"`
}

// 实现客户端
client := NewExternalAPIClient(&ExternalAPIConfig{
    ClusterInfoURL: "http://api.internal/get_mysql_cluster_instance/",
    Timeout:        10 * time.Second,
})

info, err := client.GetClusterInstance(ctx, "192.168.1.100")
```

## 扩展接口

### 自定义回收策略

```go
// RecycleStrategy 回收策略接口
type RecycleStrategy interface {
    // ShouldRecycle 判断表是否需要回收
    ShouldRecycle(table *TableFragmentation) bool
    
    // GetPriority 获取回收优先级
    GetPriority(table *TableFragmentation) int
    
    // PreRecycle 回收前处理
    PreRecycle(ctx context.Context, table *TableFragmentation) error
    
    // PostRecycle 回收后处理
    PostRecycle(ctx context.Context, table *TableFragmentation, result *RecycleResult) error
}

// 注册自定义策略
func (r *DefaultRecycler) RegisterStrategy(name string, strategy RecycleStrategy) error
```

### 自定义监控器

```go
// MonitorHook 监控钩子接口
type MonitorHook interface {
    // OnTaskStart 任务开始时调用
    OnTaskStart(task *recycler.RecycleTask) error
    
    // OnTaskProgress 任务进度更新时调用
    OnTaskProgress(taskID string, progress *ProgressUpdate) error
    
    // OnTaskComplete 任务完成时调用
    OnTaskComplete(taskID string, result *recycler.RecycleResult) error
    
    // OnTaskFailed 任务失败时调用
    OnTaskFailed(taskID string, err error) error
}

// 注册监控钩子
func (m *DefaultMonitor) RegisterHook(hook MonitorHook) error
```

---

**注意:** 本API文档会随着项目开发进度持续更新。所有接口都保证向后兼容性，重大变更会通过版本号体现。