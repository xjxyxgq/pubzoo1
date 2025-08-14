# MySQL 表空间碎片回收工具

一个基于测试驱动开发（TDD）的高性能 MySQL 表空间碎片回收工具，专为生产环境设计，支持集群拓扑感知和安全的并发操作。

## 简介

该工具通过智能分析 `information_schema.tables` 表中的 `data_free` 字段，自动识别并回收MySQL表空间碎片。当表空间碎片大小超过设定阈值时，工具会安全地执行 `ALTER TABLE NAME ENGINE=InnoDB` 命令进行表空间压缩。

### 核心特性

- 🔍 **智能集群拓扑发现** - 自动探测MySQL主从复制关系
- ⚡ **高性能并发处理** - 基于goroutine的高效并发架构
- 🛡️ **主节点保护** - 多重验证机制确保不在主节点执行危险操作
- 📊 **实时监控** - 提供详细的操作进度和状态信息
- 🧪 **测试驱动开发** - 完整的测试覆盖确保代码质量

## 系统要求

- Go 1.23+
- MySQL 5.7+ 或 MySQL 8.0+
- 具有相应权限的MySQL账户（SELECT, ALTER权限）

## 快速开始

### 安装

```bash
git clone <repository-url>
cd mysql_tablespace_recycling
go mod tidy
go build -o mysql-recycler main.go
```

### 基本使用

```bash
# 分析指定MySQL实例的表空间碎片
./mysql-recycler analyze --host=192.168.1.100 --port=3306 --user=admin --password=secret

# 执行表空间回收（模式1：独立节点模式）
./mysql-recycler reclaim --host=192.168.1.100 --mode=1 --threshold=100MB

# 查看当前运行状态
./mysql-recycler status
```

## 架构设计

### 核心模块

```
├── cluster/          # 集群拓扑发现和管理
├── recycler/         # 表空间回收核心逻辑
├── monitor/          # 状态监控和进度跟踪
├── config/           # 配置管理
└── utils/           # 工具函数
```

### 工作流程

1. **拓扑发现** - 通过 `SHOW SLAVE HOSTS` 和 `SHOW SLAVE STATUS` 构建集群拓扑图
2. **主节点识别** - 多重验证确保主节点安全
3. **碎片分析** - 计算表空间碎片并识别需要回收的表
4. **并发执行** - 在安全节点上并发执行表空间回收
5. **进度监控** - 实时跟踪操作进度并持久化状态

## 功能详解

### 1. 集群拓扑探测

工具具备强大的集群拓扑自动发现能力：

- **并发探测**: 使用goroutine池并发查询集群节点信息
- **关系映射**: 自动构建主从复制关系图谱
- **状态缓存**: 智能缓存拓扑信息，减少重复查询

### 2. 主节点安全保护

多层次验证机制确保不会在主节点执行危险操作：

- **read_only检查**: 验证节点的只读状态
- **外部API验证**: 通过集群管理API获取节点角色信息
- **复制状态验证**: 确认节点不存在slave复制关系

#### 外部API接口规范

```json
POST /api/get_mysql_cluster_instance/
{
  "search_ins_ip": "192.168.1.100"
}

响应：
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

### 3. 双模式操作

#### 模式1 (默认): 独立节点模式
- 设置 `sql_log_bin=OFF` 关闭binlog记录
- 在每个节点独立执行表空间回收
- 各节点独立计算碎片阈值
- 适用于从节点维护场景

#### 模式2: 主从同步模式
- 保持 `sql_log_bin=ON`
- 仅在指定节点执行操作
- 通过binlog同步到下游节点
- 在指定节点统一计算阈值
- 适用于一致性要求较高的场景

### 4. 实时监控

提供详细的操作状态监控：

```bash
./mysql-recycler status
```

输出示例：
```
当前运行任务:
┌─────────────────────────────────────────────────────────────┐
│ 任务ID: task-001                                            │
│ 目标节点: 192.168.1.101:3306                               │
│ 模式: 独立节点模式                                          │
│ 进度: 15/50 (30%)                                          │
│ 当前表: user_data.user_logs                                │
│ 预计完成时间: 2024-01-15 14:30:00                          │
│ 已回收空间: 2.5GB                                          │
└─────────────────────────────────────────────────────────────┘
```

## 配置参数

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--host` | MySQL主机地址 | localhost |
| `--port` | MySQL端口 | 3306 |
| `--user` | MySQL用户名 | root |
| `--password` | MySQL密码 | - |
| `--mode` | 操作模式 (1/2) | 1 |
| `--threshold` | 碎片回收阈值 | 100MB |
| `--concurrency` | 并发数 | 5 |
| `--allow-master` | 允许在主节点执行 | false |

### 配置文件

支持YAML格式配置文件 `config.yaml`:

```yaml
database:
  default_port: 3306
  timeout: 30s
  max_connections: 10

recycler:
  default_threshold: "100MB"
  max_concurrency: 5
  allow_master_operations: false
  
monitoring:
  persist_interval: "5m"
  status_file: "/tmp/mysql-recycler-status.json"

external_api:
  cluster_info_url: "http://api.internal/get_mysql_cluster_instance/"
  timeout: "10s"
```

## 开发指南

### 测试驱动开发

项目严格遵循TDD原则：

```bash
# 运行所有测试
go test ./...

# 运行特定模块测试
go test ./cluster -v

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 代码规范

- **函数复杂度**: 单个函数不超过50行
- **并发安全**: 所有goroutine必须正确管理生命周期
- **错误处理**: 使用标准Go错误处理模式
- **日志记录**: 使用结构化日志（推荐zap）

### 扩展开发

项目设计为可扩展的模块化架构：

```go
// 自定义回收策略
type CustomRecycler struct {
    // 实现Recycler接口
}

// 自定义监控器
type CustomMonitor struct {
    // 实现Monitor接口
}
```

## 性能优化

### 并发控制

- 使用worker pool模式管理goroutine
- 通过信号量控制并发数量
- 实现优雅关闭机制

### 内存管理

- 及时释放数据库连接
- 使用对象池复用常用对象
- 避免内存泄漏

### 网络优化

- 连接池复用数据库连接
- 批量操作减少网络往返
- 超时控制防止连接挂起

## 故障排除

### 常见问题

1. **连接被拒绝**
   ```
   Error: dial tcp 192.168.1.100:3306: connect: connection refused
   ```
   解决方案: 检查MySQL服务状态和网络连接

2. **权限不足**
   ```
   Error: Access denied for user 'recycler'@'%' to database 'information_schema'
   ```
   解决方案: 确保用户具有SELECT和ALTER权限

3. **主节点保护触发**
   ```
   Error: Operation blocked: target node identified as master
   ```
   解决方案: 使用 `--allow-master` 参数或在从节点执行

### 日志分析

日志文件位置: `/tmp/mysql-recycler.log`

日志级别:
- `DEBUG`: 详细的调试信息
- `INFO`: 一般操作信息
- `WARN`: 警告信息
- `ERROR`: 错误信息

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

### 代码贡献要求

- 所有新功能必须有对应的测试用例
- 测试覆盖率不得低于80%
- 代码必须通过golint和go vet检查
- 提交信息遵循约定式提交规范

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 联系方式

- 项目维护者: [您的姓名]
- 邮箱: [your.email@domain.com]
- 问题反馈: [项目Issues页面]

---

**⚠️ 警告**: 表空间回收操作会锁定表，请在业务低峰期执行，并确保有完整的数据备份。