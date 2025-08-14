# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 提供在此代码库中工作的指导。

## 构建和开发命令

### 核心构建命令
```bash
# 构建主程序
go build -o mysql-recycler main.go

# 详细输出构建
go build -v -o mysql_tablespace_recycling main.go

# 安装依赖
go mod tidy

# 清理模块缓存（如需要）
go clean -modcache
```

### 测试命令
```bash
# 运行所有测试
go test ./...

# 详细输出运行测试
go test -v ./...

# 运行特定包的测试
go test ./cluster -v
go test ./recycler -v
go test ./monitor -v

# 运行特定测试函数
go test ./cluster -run TestTopologyDiscoverer -v

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 测试竞态条件
go test -race ./...
```

### 运行工具
```bash
# 显示帮助
./mysql-recycler --help

# 分析碎片（安全操作）
./mysql-recycler analyze --host=localhost --user=root --password=secret

# 发现集群拓扑
./mysql-recycler topology --host=192.168.1.100 --verbose

# 验证主节点安全性
./mysql-recycler validate --host=192.168.1.101

# 执行表空间回收（危险操作）
./mysql-recycler reclaim --host=192.168.1.101 --mode=1 --threshold=100MB --dry-run

# 监控任务状态
./mysql-recycler status --watch
```

## 架构概览

这是一个使用Go语言基于测试驱动开发原则构建的MySQL表空间碎片恢复工具。架构围绕安全性、并发性和集群拓扑感知设计。

### 核心架构组件

**四层架构：**
1. **CLI层** (`cmd/`): 基于Cobra的命令行界面，包含5个主要命令
2. **业务逻辑层** (`recycler/`, `cluster/`, `monitor/`): 核心域逻辑
3. **基础设施层** (`config/`, `pkg/`): 配置、日志和工具
4. **数据层**: MySQL连接和information_schema查询

### 关键领域模型

**集群管理 (`cluster/`)**:
- `NodeConfig`: 表示MySQL实例配置
- `ClusterTopology`: 通过`SHOW SLAVE HOSTS`发现的主从关系映射
- `MasterValidator`: 多层验证以防止在主节点上执行操作
- 发现使用并发goroutines和工作池进行集群扫描

**回收引擎 (`recycler/`)**:
- 两种操作模式：
  - 模式1（独立）: `sql_log_bin=OFF`，并发表处理
  - 模式2（复制）: `sql_log_bin=ON`，串行处理以防止复制延迟
- `TableFragmentation`: 分析`information_schema.tables.data_free`进行碎片计算
- 使用`ALTER TABLE name ENGINE=InnoDB`进行表空间压缩

**监控系统 (`monitor/`)**:
- 带JSON持久化的实时任务进度跟踪
- 通知钩子系统（支持Webhook）
- 每5分钟状态持久化的后台goroutine

### 关键安全机制

**主节点保护**: 三层验证系统
1. 数据库级别: `SELECT @@read_only`和复制状态检查
2. 外部API: POST到`/api/get_mysql_cluster_instance/`进行集群管理验证
3. 用户覆盖: `--allow-master`标志用于紧急操作

**并发安全**:
- 使用`make(chan struct{}, concurrency)`的信号量并发控制
- 使用`sync.WaitGroup`的适当goroutine生命周期管理
- 基于Context的取消支持

### 配置系统

**多源配置优先级**:
1. 命令行标志（最高）
2. 环境变量（`MYSQL_RECYCLER_*`）
3. YAML配置文件（`mysql-recycler.yaml`）
4. 默认值（最低）

配置文件搜索顺序：
- 当前目录: `./mysql-recycler.yaml`
- 用户主目录: `~/.mysql-recycler.yaml`
- 系统配置: `/etc/mysql-recycler/config.yaml`

## 开发模式

### 基于接口的设计
代码库广泛使用Go接口以提高可测试性：
- `DBConnection`接口用于数据库操作（允许模拟）
- `Monitor`接口用于任务监控
- `FragmentationAnalyzer`接口用于可定制的分析策略

### 错误处理模式
使用Go标准错误处理与context：
```go
if err != nil {
    return nil, fmt.Errorf("operation failed: %w", err)
}
```

### 并发处理模式
使用信号量的工作池实现：
```go
sem := make(chan struct{}, concurrency)
var wg sync.WaitGroup
// 使用控制并发处理项目
```

## 重要实现细节

### 数据库查询策略
- 使用`information_schema.tables`进行碎片分析
- 计算碎片为`data_free / (data_length + index_length)`
- 按可配置阈值和支持的引擎（InnoDB）过滤

### 集群发现算法
1. 从入口点节点开始
2. 执行`SHOW SLAVE HOSTS`查找下游节点
3. 在从节点上执行`SHOW SLAVE STATUS`映射上游关系
4. 构建双向拓扑图
5. 使用可配置TTL缓存结果

### 任务执行流程
1. 拓扑发现和主节点验证
2. 碎片分析和表优先级排序
3. 带进度跟踪的任务创建
4. 基于模式的并发/串行执行
5. 实时监控和状态持久化

### 测试策略
- 使用模拟对象对数据库操作进行单元测试
- 集成测试需要实际MySQL实例
- 测试覆盖成功和失败场景
- 使用`-race`标志进行竞态条件测试

## MySQL要求和权限

**所需MySQL版本**: 5.7+ 或 8.0+

**所需权限**:
- 对`information_schema.tables`的`SELECT`权限
- 对目标表的`ALTER`权限用于表空间操作
- `SHOW DATABASES`权限用于模式发现
- `REPLICATION CLIENT`权限用于拓扑发现（可选）

**外部API集成**:
该工具与集群管理API集成以进行额外的安全验证。预期的API合约在README.md中有记录。

## 不同环境的配置

**开发环境**: 广泛使用`--dry-run`标志和`--verbose`进行调试
**预发布环境**: 使用实际MySQL集群测试但使用较小的并发值
**生产环境**: 需要仔细调整阈值和主节点验证

该工具默认进行安全操作 - 默认情况下阻止主节点操作，干运行模式显示将要执行的操作而不做实际更改。