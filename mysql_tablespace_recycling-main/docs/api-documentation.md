# MySQL表空间回收工具 API接口文档

## 目录
- [1. 概述](#1-概述)
- [2. 命令行接口](#2-命令行接口)
- [3. REST API接口](#3-rest-api接口)
- [4. 配置文件接口](#4-配置文件接口)
- [5. 数据结构定义](#5-数据结构定义)
- [6. 错误码定义](#6-错误码定义)
- [7. 使用示例](#7-使用示例)

## 1. 概述

MySQL表空间回收工具提供了丰富的接口，包括命令行接口、REST API接口和配置文件接口，支持集群拓扑探测、表空间回收、实时监控等功能。

### 1.1 接口特性

- **高并发支持**：所有接口支持并发调用，使用goroutine池管理并发请求
- **安全保障**：内置主节点识别和保护机制，避免在生产主节点执行回收操作
- **实时监控**：提供实时进度展示和状态监控接口
- **灵活配置**：支持多种工作模式和参数配置

## 2. 命令行接口

### 2.1 主命令

```bash
mysql-tablespace-recycling [global flags] <command> [command flags] [arguments...]
```

#### 全局参数

| 参数 | 类型 | 默认值 | 描述 |
|-----|------|-------|------|
| `--config` | string | `./config.yaml` | 配置文件路径 |
| `--log-level` | string | `info` | 日志级别 (debug/info/warn/error) |
| `--log-file` | string | `./logs/recycling.log` | 日志文件路径 |
| `--dry-run` | bool | `false` | 干跑模式，不执行实际操作 |
| `--parallel` | int | `10` | 并发处理数量 |

### 2.2 集群拓扑探测命令

```bash
mysql-tablespace-recycling topology [flags]
```

#### 参数说明

| 参数 | 类型 | 必需 | 描述 |
|-----|------|-----|------|
| `--host` | string | 是 | MySQL主机地址 |
| `--port` | int | 否 | MySQL端口 (默认3306) |
| `--user` | string | 是 | MySQL用户名 |
| `--password` | string | 是 | MySQL密码 |
| `--output-format` | string | 否 | 输出格式 (json/yaml/table) |
| `--output-file` | string | 否 | 输出文件路径 |
| `--max-depth` | int | 否 | 最大探测深度 (默认10) |

#### 输出示例

```json
{
  "cluster_name": "mysql-cluster-prod",
  "topology": {
    "master": {
      "host": "192.168.1.100",
      "port": 3306,
      "role": "master",
      "read_only": false,
      "slaves": [
        {
          "host": "192.168.1.101",
          "port": 3306,
          "role": "slave",
          "read_only": true,
          "lag_seconds": 0
        }
      ]
    }
  }
}
```

### 2.3 表空间回收命令

```bash
mysql-tablespace-recycling recycle [flags]
```

#### 参数说明

| 参数 | 类型 | 必需 | 描述 |
|-----|------|-----|------|
| `--host` | string | 是 | 目标MySQL主机地址 |
| `--port` | int | 否 | MySQL端口 (默认3306) |
| `--user` | string | 是 | MySQL用户名 |
| `--password` | string | 是 | MySQL密码 |
| `--database` | string | 否 | 指定数据库 (为空则处理所有数据库) |
| `--table` | string | 否 | 指定表名 (为空则处理所有表) |
| `--threshold-mb` | int | 否 | 碎片阈值(MB) (默认100) |
| `--threshold-percent` | float | 否 | 碎片百分比阈值 (默认10.0) |
| `--mode` | string | 否 | 工作模式 (binlog-off/binlog-on) |
| `--exclude-primary` | bool | 否 | 排除主节点 (默认true) |
| `--batch-size` | int | 否 | 批处理大小 (默认5) |
| `--timeout` | duration | 否 | 操作超时时间 (默认1h) |

#### 工作模式说明

- **binlog-off模式**：关闭binlog，独立连接每个节点进行回收
- **binlog-on模式**：保持binlog开启，通过主从复制传播回收操作

### 2.4 监控命令

```bash
mysql-tablespace-recycling monitor [flags]
```

#### 参数说明

| 参数 | 类型 | 必需 | 描述 |
|-----|------|-----|------|
| `--refresh-interval` | duration | 否 | 刷新间隔 (默认5s) |
| `--output-format` | string | 否 | 输出格式 (table/json) |
| `--show-completed` | bool | 否 | 显示已完成任务 (默认false) |

### 2.5 状态查询命令

```bash
mysql-tablespace-recycling status [flags]
```

#### 参数说明

| 参数 | 类型 | 必需 | 描述 |
|-----|------|-----|------|
| `--job-id` | string | 否 | 任务ID (为空则显示所有任务) |
| `--format` | string | 否 | 输出格式 (table/json/yaml) |

## 3. REST API接口

### 3.1 服务器启动

```bash
mysql-tablespace-recycling server --port 8080
```

### 3.2 API端点

#### 3.2.1 集群拓扑探测

**请求**
```http
POST /api/v1/topology/discover
Content-Type: application/json

{
  "host": "192.168.1.100",
  "port": 3306,
  "username": "root",
  "password": "password",
  "max_depth": 10
}
```

**响应**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "code": 0,
  "message": "success",
  "data": {
    "cluster_id": "cluster-001",
    "discover_time": "2024-01-01T10:00:00Z",
    "topology": {
      "master": {
        "host": "192.168.1.100",
        "port": 3306,
        "server_id": 1,
        "read_only": false,
        "slaves": [...]
      }
    }
  }
}
```

#### 3.2.2 创建回收任务

**请求**
```http
POST /api/v1/recycle/jobs
Content-Type: application/json

{
  "target": {
    "host": "192.168.1.101",
    "port": 3306,
    "username": "recycler",
    "password": "password"
  },
  "config": {
    "mode": "binlog-off",
    "threshold_mb": 100,
    "threshold_percent": 10.0,
    "exclude_primary": true,
    "databases": ["test_db"],
    "tables": ["user_table"],
    "batch_size": 5,
    "timeout": "1h"
  }
}
```

**响应**
```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "code": 0,
  "message": "job created successfully",
  "data": {
    "job_id": "job-20240101-001",
    "status": "pending",
    "created_at": "2024-01-01T10:00:00Z"
  }
}
```

#### 3.2.3 查询任务状态

**请求**
```http
GET /api/v1/recycle/jobs/{job_id}
```

**响应**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "code": 0,
  "message": "success",
  "data": {
    "job_id": "job-20240101-001",
    "status": "running",
    "progress": {
      "total_tables": 100,
      "processed_tables": 45,
      "failed_tables": 2,
      "percentage": 45.0
    },
    "statistics": {
      "space_freed_mb": 1024,
      "time_elapsed": "00:15:30",
      "estimated_remaining": "00:18:45"
    }
  }
}
```

#### 3.2.4 取消任务

**请求**
```http
DELETE /api/v1/recycle/jobs/{job_id}
```

**响应**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "code": 0,
  "message": "job cancelled successfully"
}
```

#### 3.2.5 获取任务列表

**请求**
```http
GET /api/v1/recycle/jobs?status=running&limit=10&offset=0
```

**响应**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "code": 0,
  "message": "success",
  "data": {
    "total": 25,
    "jobs": [
      {
        "job_id": "job-20240101-001",
        "status": "running",
        "created_at": "2024-01-01T10:00:00Z",
        "progress": 45.0
      }
    ]
  }
}
```

#### 3.2.6 健康检查

**请求**
```http
GET /api/v1/health
```

**响应**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "code": 0,
  "message": "healthy",
  "data": {
    "version": "1.0.0",
    "uptime": "2h30m15s",
    "active_jobs": 3,
    "system_load": 0.75
  }
}
```

### 3.3 WebSocket接口

#### 3.3.1 实时监控

**连接**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws/monitor');

ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Job status update:', data);
};
```

**消息格式**
```json
{
  "type": "job_update",
  "job_id": "job-20240101-001",
  "status": "running",
  "progress": 47.5,
  "timestamp": "2024-01-01T10:05:00Z"
}
```

## 4. 配置文件接口

### 4.1 配置文件结构

```yaml
# config.yaml
# MySQL表空间回收工具配置文件

# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"

# 日志配置
logging:
  level: "info"
  file: "./logs/recycling.log"
  max_size: 100        # MB
  max_backups: 5
  max_age: 30         # 天
  compress: true

# 数据库配置
database:
  # 默认连接参数
  default:
    timeout: "10s"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: "1h"
  
  # 集群配置
  clusters:
    - name: "prod-cluster"
      primary:
        host: "192.168.1.100"
        port: 3306
        username: "recycler"
        password: "password"
      
# 回收配置
recycling:
  # 默认参数
  defaults:
    threshold_mb: 100
    threshold_percent: 10.0
    batch_size: 5
    timeout: "1h"
    mode: "binlog-off"
    exclude_primary: true
  
  # 并发控制
  concurrency:
    max_workers: 10
    queue_size: 100
    worker_timeout: "30m"

# 监控配置
monitoring:
  # 指标收集
  metrics:
    enabled: true
    interval: "30s"
    retention: "7d"
  
  # 报警配置
  alerts:
    enabled: true
    webhook_url: "http://alert-manager:9093/api/v1/alerts"

# 持久化配置
persistence:
  # 任务状态持久化
  job_state:
    enabled: true
    file: "./data/jobs.json"
    interval: "10s"
  
  # 指标持久化
  metrics:
    enabled: true
    file: "./data/metrics.db"
```

### 4.2 环境变量配置

| 变量名 | 描述 | 默认值 |
|-------|------|-------|
| `RECYCLER_CONFIG_FILE` | 配置文件路径 | `./config.yaml` |
| `RECYCLER_LOG_LEVEL` | 日志级别 | `info` |
| `RECYCLER_SERVER_PORT` | 服务端口 | `8080` |
| `RECYCLER_DB_PASSWORD` | 数据库密码 | - |
| `RECYCLER_ALERT_WEBHOOK` | 报警Webhook地址 | - |

## 5. 数据结构定义

### 5.1 核心数据结构

```go
// ClusterTopology 集群拓扑结构
type ClusterTopology struct {
    ClusterID    string         `json:"cluster_id"`
    DiscoverTime time.Time      `json:"discover_time"`
    Master       *DatabaseNode  `json:"master"`
    Slaves       []*DatabaseNode `json:"slaves,omitempty"`
}

// DatabaseNode 数据库节点信息
type DatabaseNode struct {
    Host         string         `json:"host"`
    Port         int           `json:"port"`
    ServerID     uint32        `json:"server_id"`
    ReadOnly     bool          `json:"read_only"`
    Role         string        `json:"role"`
    LagSeconds   int64         `json:"lag_seconds"`
    Slaves       []*DatabaseNode `json:"slaves,omitempty"`
}

// RecycleJob 回收任务
type RecycleJob struct {
    JobID       string           `json:"job_id"`
    Status      JobStatus        `json:"status"`
    Config      *RecycleConfig   `json:"config"`
    Progress    *JobProgress     `json:"progress"`
    Statistics  *JobStatistics   `json:"statistics"`
    CreatedAt   time.Time        `json:"created_at"`
    StartedAt   *time.Time       `json:"started_at,omitempty"`
    CompletedAt *time.Time       `json:"completed_at,omitempty"`
    Error       string           `json:"error,omitempty"`
}

// RecycleConfig 回收配置
type RecycleConfig struct {
    Target          DatabaseTarget `json:"target"`
    Mode            WorkMode       `json:"mode"`
    ThresholdMB     int           `json:"threshold_mb"`
    ThresholdPercent float64       `json:"threshold_percent"`
    ExcludePrimary  bool          `json:"exclude_primary"`
    Databases       []string      `json:"databases,omitempty"`
    Tables          []string      `json:"tables,omitempty"`
    BatchSize       int           `json:"batch_size"`
    Timeout         time.Duration `json:"timeout"`
}

// JobProgress 任务进度
type JobProgress struct {
    TotalTables     int     `json:"total_tables"`
    ProcessedTables int     `json:"processed_tables"`
    FailedTables    int     `json:"failed_tables"`
    Percentage      float64 `json:"percentage"`
    CurrentTable    string  `json:"current_table,omitempty"`
}

// JobStatistics 任务统计
type JobStatistics struct {
    SpaceFreedMB       int64         `json:"space_freed_mb"`
    TimeElapsed        time.Duration `json:"time_elapsed"`
    EstimatedRemaining time.Duration `json:"estimated_remaining"`
    TablesProcessed    []TableResult `json:"tables_processed"`
}
```

### 5.2 枚举定义

```go
// JobStatus 任务状态
type JobStatus string

const (
    JobStatusPending   JobStatus = "pending"
    JobStatusRunning   JobStatus = "running"
    JobStatusCompleted JobStatus = "completed"
    JobStatusFailed    JobStatus = "failed"
    JobStatusCancelled JobStatus = "cancelled"
)

// WorkMode 工作模式
type WorkMode string

const (
    ModeBinlogOff WorkMode = "binlog-off"
    ModeBinlogOn  WorkMode = "binlog-on"
)

// NodeRole 节点角色
type NodeRole string

const (
    RoleMaster NodeRole = "master"
    RoleSlave  NodeRole = "slave"
)
```

## 6. 错误码定义

| 错误码 | 含义 | HTTP状态码 |
|-------|------|-----------|
| 0 | 成功 | 200 |
| 1001 | 参数错误 | 400 |
| 1002 | 数据库连接失败 | 500 |
| 1003 | 拓扑探测失败 | 500 |
| 1004 | 主节点检测失败 | 500 |
| 1005 | 任务创建失败 | 500 |
| 1006 | 任务不存在 | 404 |
| 1007 | 任务状态错误 | 400 |
| 1008 | 权限不足 | 403 |
| 1009 | 操作超时 | 408 |
| 1010 | 系统忙碌 | 503 |

### 错误响应格式

```json
{
  "code": 1002,
  "message": "数据库连接失败",
  "details": {
    "error": "connection refused",
    "host": "192.168.1.100",
    "port": 3306
  },
  "timestamp": "2024-01-01T10:00:00Z"
}
```

## 7. 使用示例

### 7.1 命令行使用示例

```bash
# 1. 探测集群拓扑
mysql-tablespace-recycling topology \
  --host 192.168.1.100 \
  --user recycler \
  --password password \
  --output-format json \
  --output-file topology.json

# 2. 执行表空间回收（干跑模式）
mysql-tablespace-recycling recycle \
  --host 192.168.1.101 \
  --user recycler \
  --password password \
  --threshold-mb 100 \
  --mode binlog-off \
  --dry-run

# 3. 监控任务进度
mysql-tablespace-recycling monitor \
  --refresh-interval 5s \
  --output-format table

# 4. 查询任务状态
mysql-tablespace-recycling status \
  --job-id job-20240101-001 \
  --format json
```

### 7.2 API使用示例

```bash
# 1. 启动API服务器
mysql-tablespace-recycling server --port 8080

# 2. 探测集群拓扑
curl -X POST http://localhost:8080/api/v1/topology/discover \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.100",
    "port": 3306,
    "username": "recycler",
    "password": "password"
  }'

# 3. 创建回收任务
curl -X POST http://localhost:8080/api/v1/recycle/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "target": {
      "host": "192.168.1.101",
      "port": 3306,
      "username": "recycler",
      "password": "password"
    },
    "config": {
      "mode": "binlog-off",
      "threshold_mb": 100,
      "exclude_primary": true
    }
  }'

# 4. 查询任务状态
curl -X GET http://localhost:8080/api/v1/recycle/jobs/job-20240101-001
```

### 7.3 配置文件示例

```yaml
# 生产环境配置示例
server:
  host: "0.0.0.0"
  port: 8080

logging:
  level: "info"
  file: "/var/log/mysql-recycler/app.log"

database:
  default:
    timeout: "30s"
    max_open_conns: 20
    max_idle_conns: 10

recycling:
  defaults:
    threshold_mb: 500
    threshold_percent: 15.0
    batch_size: 10
    exclude_primary: true
  
  concurrency:
    max_workers: 20
    queue_size: 200

monitoring:
  metrics:
    enabled: true
    interval: "30s"
  
  alerts:
    enabled: true
    webhook_url: "http://alert-manager:9093/api/v1/alerts"
```

---

*本文档持续更新中，如有疑问请联系开发团队。*