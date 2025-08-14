# MySQL表空间回收工具 架构设计文档

## 目录
- [1. 系统概述](#1-系统概述)
- [2. 整体架构](#2-整体架构)
- [3. 模块设计](#3-模块设计)
- [4. 并发设计](#4-并发设计)
- [5. 安全设计](#5-安全设计)
- [6. 数据流设计](#6-数据流设计)
- [7. 存储设计](#7-存储设计)
- [8. 性能设计](#8-性能设计)
- [9. 可扩展性设计](#9-可扩展性设计)
- [10. 部署架构](#10-部署架构)

## 1. 系统概述

### 1.1 系统目标

MySQL表空间回收工具是一个高性能、高并发的数据库运维工具，专门用于安全地回收MySQL集群中的表空间碎片。系统采用测试驱动开发(TDD)方法，确保代码质量和系统稳定性。

### 1.2 核心特性

- **智能拓扑发现**：自动探测MySQL集群拓扑结构，支持多层主从复制架构
- **安全主节点保护**：多重机制识别和保护生产主节点，避免误操作
- **灵活工作模式**：支持binlog开关两种模式，满足不同场景需求
- **高并发处理**：基于goroutine的并发架构，提升处理效率
- **实时监控**：提供实时进度展示和状态监控能力
- **数据持久化**：内存数据定期持久化，确保任务状态不丢失

### 1.3 技术栈

- **开发语言**：Go 1.23+
- **数据库**：MySQL 5.7+ / 8.0+
- **并发模型**：Goroutine + Channel
- **API框架**：Gin / Echo (可选)
- **配置管理**：Viper
- **日志管理**：Logrus / Zap
- **测试框架**：Go Testing + Testify

## 2. 整体架构

### 2.1 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        MySQL表空间回收工具                          │
├─────────────────────────────────────────────────────────────────┤
│                           用户接口层                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │   CLI接口    │  │   REST API   │  │  WebSocket   │           │
│  │              │  │              │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│                          服务协调层                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │  任务调度器   │  │  状态管理器   │  │  配置管理器   │           │
│  │              │  │              │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│                          业务逻辑层                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 拓扑探测模块  │  │ 回收执行模块  │  │ 监控统计模块  │           │
│  │              │  │              │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│                          基础设施层                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 数据库连接池  │  │  并发控制器   │  │  持久化引擎   │           │
│  │              │  │              │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│                           存储层                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │  MySQL集群   │  │   本地存储    │  │   外部API    │           │
│  │              │  │              │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 分层架构说明

#### 2.2.1 用户接口层
- **CLI接口**：提供命令行工具，支持拓扑探测、表空间回收、状态查询等操作
- **REST API**：提供HTTP接口，支持Web管理平台集成
- **WebSocket**：提供实时推送能力，用于进度监控和状态更新

#### 2.2.2 服务协调层
- **任务调度器**：负责任务的创建、调度、执行和生命周期管理
- **状态管理器**：维护系统和任务状态，提供状态查询和更新接口
- **配置管理器**：管理系统配置，支持动态配置更新

#### 2.2.3 业务逻辑层
- **拓扑探测模块**：实现MySQL集群拓扑自动发现和主节点识别
- **回收执行模块**：执行表空间回收操作，支持不同工作模式
- **监控统计模块**：收集和分析执行过程中的各种指标

#### 2.2.4 基础设施层
- **数据库连接池**：管理MySQL连接，提供连接复用和负载均衡
- **并发控制器**：控制goroutine数量和任务并发度
- **持久化引擎**：负责数据的持久化存储和恢复

#### 2.2.5 存储层
- **MySQL集群**：目标数据库集群
- **本地存储**：任务状态和配置文件存储
- **外部API**：第三方系统集成接口

## 3. 模块设计

### 3.1 拓扑探测模块

```go
// 拓扑探测模块设计
type TopologyDiscoverer struct {
    connectionPool *ConnectionPool
    logger         Logger
    config         *DiscoveryConfig
}

type DiscoveryConfig struct {
    MaxDepth       int           // 最大探测深度
    Timeout        time.Duration // 超时时间
    ParallelDegree int           // 并行度
    RetryCount     int           // 重试次数
}

// 核心方法
func (d *TopologyDiscoverer) DiscoverTopology(ctx context.Context, entryPoint *DatabaseNode) (*ClusterTopology, error)
func (d *TopologyDiscoverer) ValidatePrimaryNode(ctx context.Context, node *DatabaseNode) (*PrimaryValidation, error)
func (d *TopologyDiscoverer) GetSlaveNodes(ctx context.Context, master *DatabaseNode) ([]*DatabaseNode, error)
func (d *TopologyDiscoverer) CheckReplicationLag(ctx context.Context, slave *DatabaseNode) (time.Duration, error)
```

#### 3.1.1 探测算法

```
探测算法流程：
1. 从入口节点开始，创建goroutine池
2. 并发执行以下操作：
   - 获取slave hosts (SHOW SLAVE HOSTS)
   - 获取slave status (SHOW SLAVE STATUS)
   - 验证节点状态 (SELECT @@read_only, @@server_id)
3. 递归探测下级节点，直到达到最大深度或无更多节点
4. 合并结果，构建完整拓扑树
5. 验证主节点身份
```

#### 3.1.2 主节点识别机制

```go
type PrimaryValidator struct {
    externalAPI ExternalAPIClient
    validators  []PrimaryValidationRule
}

// 验证规则
var defaultValidationRules = []PrimaryValidationRule{
    &ReadOnlyValidator{},      // 检查read_only状态
    &SlaveStatusValidator{},   // 检查是否有slave status
    &ExternalAPIValidator{},   // 调用外部API验证
    &RoleConsistencyValidator{}, // 角色一致性检查
}
```

### 3.2 回收执行模块

```go
// 回收执行模块设计
type RecycleExecutor struct {
    connectionPool *ConnectionPool
    jobManager     *JobManager
    statistics     *StatisticsCollector
    config         *RecycleConfig
}

// 工作模式接口
type WorkModeExecutor interface {
    Execute(ctx context.Context, job *RecycleJob) error
    ValidatePreConditions(ctx context.Context, target *DatabaseTarget) error
    GetEstimatedDuration(ctx context.Context, tableList []*TableInfo) time.Duration
}

// binlog-off模式实现
type BinlogOffExecutor struct {
    executor *RecycleExecutor
}

// binlog-on模式实现
type BinlogOnExecutor struct {
    executor *RecycleExecutor
}
```

#### 3.2.1 回收算法

```
回收算法流程：
1. 预检查阶段：
   - 验证目标节点状态
   - 检查表空间碎片情况
   - 估算回收时间和收益
   
2. 准备阶段：
   - 创建回收任务
   - 初始化统计信息
   - 设置工作模式
   
3. 执行阶段：
   - 批量处理表列表
   - 并发执行ALTER TABLE操作
   - 实时更新进度和统计
   
4. 清理阶段：
   - 恢复原始设置
   - 记录执行结果
   - 更新任务状态
```

#### 3.2.2 批处理设计

```go
type BatchProcessor struct {
    batchSize    int
    workerPool   *WorkerPool
    progress     *ProgressTracker
}

func (bp *BatchProcessor) ProcessTables(ctx context.Context, tables []*TableInfo) error {
    batches := bp.createBatches(tables)
    
    for _, batch := range batches {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := bp.processBatch(ctx, batch); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

### 3.3 监控统计模块

```go
// 监控统计模块设计
type StatisticsCollector struct {
    metrics    *MetricsRegistry
    storage    PersistentStorage
    notifier   EventNotifier
}

type MetricsRegistry struct {
    counters   map[string]*Counter
    gauges     map[string]*Gauge
    histograms map[string]*Histogram
    timers     map[string]*Timer
}

// 核心指标
var coreMetrics = []string{
    "jobs.total",                    // 总任务数
    "jobs.running",                  // 运行中任务数
    "jobs.completed",                // 已完成任务数
    "jobs.failed",                   // 失败任务数
    "tables.processed",              // 已处理表数
    "space.freed.bytes",             // 释放空间字节数
    "operation.duration.seconds",    // 操作耗时
    "concurrent.connections",        // 并发连接数
}
```

## 4. 并发设计

### 4.1 并发架构

```go
// 并发控制器设计
type ConcurrencyController struct {
    maxWorkers    int
    activeWorkers int32
    workerPool    chan struct{}
    jobQueue      chan Job
    resultChan    chan JobResult
    ctx           context.Context
    cancel        context.CancelFunc
    wg            sync.WaitGroup
}

func (cc *ConcurrencyController) Start() {
    for i := 0; i < cc.maxWorkers; i++ {
        cc.wg.Add(1)
        go cc.worker()
    }
    
    go cc.resultCollector()
}

func (cc *ConcurrencyController) worker() {
    defer cc.wg.Done()
    
    for {
        select {
        case job := <-cc.jobQueue:
            atomic.AddInt32(&cc.activeWorkers, 1)
            result := cc.processJob(job)
            cc.resultChan <- result
            atomic.AddInt32(&cc.activeWorkers, -1)
            
        case <-cc.ctx.Done():
            return
        }
    }
}
```

### 4.2 连接池管理

```go
// 数据库连接池设计
type ConnectionPool struct {
    pools    map[string]*sql.DB
    mu       sync.RWMutex
    config   *PoolConfig
}

type PoolConfig struct {
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
}

func (cp *ConnectionPool) GetConnection(target *DatabaseTarget) (*sql.DB, error) {
    key := cp.generateKey(target)
    
    cp.mu.RLock()
    if db, exists := cp.pools[key]; exists {
        cp.mu.RUnlock()
        return db, nil
    }
    cp.mu.RUnlock()
    
    cp.mu.Lock()
    defer cp.mu.Unlock()
    
    // 双重检查锁定模式
    if db, exists := cp.pools[key]; exists {
        return db, nil
    }
    
    db, err := cp.createConnection(target)
    if err != nil {
        return nil, err
    }
    
    cp.pools[key] = db
    return db, nil
}
```

### 4.3 goroutine生命周期管理

```go
// Goroutine管理器
type GoroutineManager struct {
    activeRoutines sync.Map
    shutdownChan   chan struct{}
    wg             sync.WaitGroup
}

func (gm *GoroutineManager) Go(name string, fn func()) {
    gm.wg.Add(1)
    gm.activeRoutines.Store(name, time.Now())
    
    go func() {
        defer gm.wg.Done()
        defer gm.activeRoutines.Delete(name)
        
        defer func() {
            if r := recover(); r != nil {
                log.Errorf("Goroutine %s panic: %v", name, r)
            }
        }()
        
        fn()
    }()
}

func (gm *GoroutineManager) Shutdown(timeout time.Duration) error {
    close(gm.shutdownChan)
    
    done := make(chan struct{})
    go func() {
        gm.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("shutdown timeout after %v", timeout)
    }
}
```

### 4.4 并发安全设计

```go
// 线程安全的任务状态管理
type SafeJobManager struct {
    jobs    sync.Map
    metrics *SafeMetrics
}

type SafeMetrics struct {
    mu      sync.RWMutex
    data    map[string]interface{}
    version int64
}

func (sm *SafeMetrics) Set(key string, value interface{}) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    sm.data[key] = value
    atomic.AddInt64(&sm.version, 1)
}

func (sm *SafeMetrics) Get(key string) (interface{}, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    value, exists := sm.data[key]
    return value, exists
}
```

## 5. 安全设计

### 5.1 主节点保护机制

```go
// 主节点保护器
type PrimaryProtector struct {
    validators []PrimaryValidator
    cache      *TTLCache
    alerter    AlertManager
}

func (pp *PrimaryProtector) IsPrimaryNode(ctx context.Context, target *DatabaseTarget) (*ValidationResult, error) {
    // 缓存检查
    if result := pp.cache.Get(target.Key()); result != nil {
        return result.(*ValidationResult), nil
    }
    
    // 多重验证
    var results []*ValidationResult
    for _, validator := range pp.validators {
        result, err := validator.Validate(ctx, target)
        if err != nil {
            return nil, err
        }
        results = append(results, result)
    }
    
    // 综合判断
    finalResult := pp.aggregateResults(results)
    
    // 缓存结果
    pp.cache.Set(target.Key(), finalResult, 5*time.Minute)
    
    // 如果是主节点，发送告警
    if finalResult.IsPrimary {
        pp.alerter.Send(AlertPrimaryNodeDetected, target)
    }
    
    return finalResult, nil
}
```

### 5.2 事务安全机制

```go
// 事务管理器
type TransactionManager struct {
    db           *sql.DB
    isolationLevel sql.IsolationLevel
    timeout       time.Duration
}

func (tm *TransactionManager) ExecuteInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
    tx, err := tm.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: tm.isolationLevel,
    })
    if err != nil {
        return err
    }
    
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()
    
    // 设置超时
    if tm.timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, tm.timeout)
        defer cancel()
    }
    
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit()
}
```

### 5.3 权限控制

```go
// 权限验证器
type PermissionValidator struct {
    requiredPrivileges []string
}

func (pv *PermissionValidator) ValidatePermissions(ctx context.Context, db *sql.DB) error {
    query := `SELECT PRIVILEGE_TYPE FROM INFORMATION_SCHEMA.USER_PRIVILEGES 
              WHERE GRANTEE = CURRENT_USER()`
    
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    var privileges []string
    for rows.Next() {
        var privilege string
        if err := rows.Scan(&privilege); err != nil {
            return err
        }
        privileges = append(privileges, privilege)
    }
    
    return pv.checkRequiredPrivileges(privileges)
}
```

## 6. 数据流设计

### 6.1 数据流架构

```
用户请求 → 接口层 → 服务层 → 业务层 → 数据层
    ↓           ↓        ↓        ↓        ↓
  验证参数   → 创建任务 → 执行逻辑 → 数据操作 → 返回结果
    ↓           ↓        ↓        ↓        ↓
  格式化     → 状态管理 → 进度更新 → 持久化   → 推送通知
```

### 6.2 事件驱动设计

```go
// 事件管理器
type EventManager struct {
    handlers map[EventType][]EventHandler
    queue    chan Event
    workers  int
}

type Event struct {
    Type      EventType
    Data      interface{}
    Timestamp time.Time
    Context   context.Context
}

type EventHandler func(context.Context, Event) error

// 事件类型定义
const (
    EventJobCreated    EventType = "job.created"
    EventJobStarted    EventType = "job.started"
    EventJobCompleted  EventType = "job.completed"
    EventJobFailed     EventType = "job.failed"
    EventTableProcessed EventType = "table.processed"
    EventProgressUpdate EventType = "progress.update"
)
```

### 6.3 消息传递机制

```go
// 消息总线
type MessageBus struct {
    subscribers map[string][]chan Message
    mu          sync.RWMutex
}

type Message struct {
    Topic   string
    Payload interface{}
    Headers map[string]string
}

func (mb *MessageBus) Subscribe(topic string, bufferSize int) <-chan Message {
    mb.mu.Lock()
    defer mb.mu.Unlock()
    
    ch := make(chan Message, bufferSize)
    mb.subscribers[topic] = append(mb.subscribers[topic], ch)
    
    return ch
}

func (mb *MessageBus) Publish(topic string, payload interface{}) {
    mb.mu.RLock()
    subscribers := mb.subscribers[topic]
    mb.mu.RUnlock()
    
    message := Message{
        Topic:   topic,
        Payload: payload,
        Headers: make(map[string]string),
    }
    
    for _, ch := range subscribers {
        select {
        case ch <- message:
        default:
            // 非阻塞发送，如果缓冲区满则丢弃
        }
    }
}
```

## 7. 存储设计

### 7.1 持久化架构

```go
// 持久化存储接口
type PersistentStorage interface {
    Save(key string, data interface{}) error
    Load(key string, data interface{}) error
    Delete(key string) error
    List(prefix string) ([]string, error)
    Exists(key string) bool
}

// 文件存储实现
type FileStorage struct {
    basePath string
    codec    Codec
    mu       sync.RWMutex
}

// 内存缓存 + 文件存储
type HybridStorage struct {
    cache   *TTLCache
    storage PersistentStorage
    syncInterval time.Duration
}
```

### 7.2 数据序列化

```go
// 编解码器接口
type Codec interface {
    Encode(data interface{}) ([]byte, error)
    Decode(data []byte, target interface{}) error
}

// JSON编解码器
type JSONCodec struct{}

func (c *JSONCodec) Encode(data interface{}) ([]byte, error) {
    return json.MarshalIndent(data, "", "  ")
}

func (c *JSONCodec) Decode(data []byte, target interface{}) error {
    return json.Unmarshal(data, target)
}
```

### 7.3 数据备份和恢复

```go
// 备份管理器
type BackupManager struct {
    storage    PersistentStorage
    retention  time.Duration
    encryption Encryptor
}

func (bm *BackupManager) CreateBackup(name string, data interface{}) error {
    backup := &Backup{
        Name:      name,
        Data:      data,
        Timestamp: time.Now(),
        Checksum:  bm.calculateChecksum(data),
    }
    
    if bm.encryption != nil {
        encryptedData, err := bm.encryption.Encrypt(backup)
        if err != nil {
            return err
        }
        backup.Data = encryptedData
    }
    
    return bm.storage.Save(fmt.Sprintf("backup/%s", name), backup)
}
```

## 8. 性能设计

### 8.1 性能优化策略

```go
// 性能监控器
type PerformanceMonitor struct {
    metrics    *MetricsCollector
    profiler   *Profiler
    optimizer  *QueryOptimizer
}

type QueryOptimizer struct {
    connectionPool *ConnectionPool
    queryCache     *QueryCache
    statementCache *PreparedStatementCache
}

// 批量处理优化
func (qo *QueryOptimizer) BatchExecute(queries []Query) error {
    // 按数据库分组
    grouped := qo.groupByDatabase(queries)
    
    // 并发执行
    var wg sync.WaitGroup
    errChan := make(chan error, len(grouped))
    
    for db, dbQueries := range grouped {
        wg.Add(1)
        go func(database string, queries []Query) {
            defer wg.Done()
            if err := qo.executeBatch(database, queries); err != nil {
                errChan <- err
            }
        }(db, dbQueries)
    }
    
    wg.Wait()
    close(errChan)
    
    for err := range errChan {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### 8.2 内存管理

```go
// 内存池管理器
type MemoryPoolManager struct {
    bufferPools map[int]*sync.Pool
    objectPools map[reflect.Type]*sync.Pool
}

func (mpm *MemoryPoolManager) GetBuffer(size int) []byte {
    pool, exists := mpm.bufferPools[size]
    if !exists {
        pool = &sync.Pool{
            New: func() interface{} {
                return make([]byte, size)
            },
        }
        mpm.bufferPools[size] = pool
    }
    
    return pool.Get().([]byte)
}

func (mpm *MemoryPoolManager) PutBuffer(buf []byte) {
    size := cap(buf)
    if pool, exists := mpm.bufferPools[size]; exists {
        // 重置缓冲区
        buf = buf[:0]
        pool.Put(buf)
    }
}
```

### 8.3 缓存策略

```go
// 多层缓存管理器
type MultiLevelCache struct {
    l1Cache *LRUCache      // 内存一级缓存
    l2Cache *RedisCache    // Redis二级缓存
    l3Cache *FileCache     // 文件三级缓存
    stats   *CacheStats
}

func (mlc *MultiLevelCache) Get(key string) (interface{}, bool) {
    // L1缓存
    if value, hit := mlc.l1Cache.Get(key); hit {
        mlc.stats.RecordHit("l1")
        return value, true
    }
    
    // L2缓存
    if value, hit := mlc.l2Cache.Get(key); hit {
        mlc.stats.RecordHit("l2")
        mlc.l1Cache.Set(key, value) // 回写L1
        return value, true
    }
    
    // L3缓存
    if value, hit := mlc.l3Cache.Get(key); hit {
        mlc.stats.RecordHit("l3")
        mlc.l1Cache.Set(key, value) // 回写L1
        mlc.l2Cache.Set(key, value) // 回写L2
        return value, true
    }
    
    mlc.stats.RecordMiss()
    return nil, false
}
```

## 9. 可扩展性设计

### 9.1 插件系统

```go
// 插件接口定义
type Plugin interface {
    Name() string
    Version() string
    Initialize(config map[string]interface{}) error
    Shutdown() error
}

// 执行器插件
type ExecutorPlugin interface {
    Plugin
    Execute(ctx context.Context, task *Task) (*Result, error)
    CanHandle(task *Task) bool
}

// 插件管理器
type PluginManager struct {
    plugins    map[string]Plugin
    executors  []ExecutorPlugin
    validators []ValidatorPlugin
}

func (pm *PluginManager) RegisterPlugin(plugin Plugin) error {
    name := plugin.Name()
    if _, exists := pm.plugins[name]; exists {
        return fmt.Errorf("plugin %s already registered", name)
    }
    
    if err := plugin.Initialize(nil); err != nil {
        return err
    }
    
    pm.plugins[name] = plugin
    
    // 类型断言注册到对应分类
    if executor, ok := plugin.(ExecutorPlugin); ok {
        pm.executors = append(pm.executors, executor)
    }
    
    return nil
}
```

### 9.2 配置热更新

```go
// 配置热更新管理器
type HotConfigManager struct {
    configPath    string
    watchers      []ConfigWatcher
    currentConfig *Config
    mu            sync.RWMutex
}

type ConfigWatcher interface {
    OnConfigChanged(oldConfig, newConfig *Config) error
}

func (hcm *HotConfigManager) StartWatching() error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    
    go func() {
        for {
            select {
            case event := <-watcher.Events:
                if event.Op&fsnotify.Write == fsnotify.Write {
                    hcm.reloadConfig()
                }
            case err := <-watcher.Errors:
                log.Errorf("Config watcher error: %v", err)
            }
        }
    }()
    
    return watcher.Add(hcm.configPath)
}

func (hcm *HotConfigManager) reloadConfig() error {
    newConfig, err := LoadConfig(hcm.configPath)
    if err != nil {
        return err
    }
    
    hcm.mu.Lock()
    oldConfig := hcm.currentConfig
    hcm.currentConfig = newConfig
    hcm.mu.Unlock()
    
    // 通知所有监听器
    for _, watcher := range hcm.watchers {
        if err := watcher.OnConfigChanged(oldConfig, newConfig); err != nil {
            log.Errorf("Config change notification failed: %v", err)
        }
    }
    
    return nil
}
```

### 9.3 水平扩展设计

```go
// 分布式协调器
type DistributedCoordinator struct {
    nodeID      string
    peers       map[string]*PeerNode
    consensus   ConsensusAlgorithm
    taskSplitter TaskSplitter
}

type TaskSplitter interface {
    Split(task *Task) ([]*SubTask, error)
    Merge(results []*SubTaskResult) (*TaskResult, error)
}

func (dc *DistributedCoordinator) DistributeTask(ctx context.Context, task *Task) (*TaskResult, error) {
    // 任务分割
    subTasks, err := dc.taskSplitter.Split(task)
    if err != nil {
        return nil, err
    }
    
    // 负载均衡分配
    assignments := dc.assignTasks(subTasks)
    
    // 并发执行
    var wg sync.WaitGroup
    results := make(chan *SubTaskResult, len(subTasks))
    errors := make(chan error, len(subTasks))
    
    for nodeID, nodeTasks := range assignments {
        wg.Add(1)
        go func(node string, tasks []*SubTask) {
            defer wg.Done()
            for _, task := range tasks {
                result, err := dc.executeOnNode(ctx, node, task)
                if err != nil {
                    errors <- err
                    return
                }
                results <- result
            }
        }(nodeID, nodeTasks)
    }
    
    wg.Wait()
    close(results)
    close(errors)
    
    // 检查错误
    select {
    case err := <-errors:
        return nil, err
    default:
    }
    
    // 合并结果
    var subResults []*SubTaskResult
    for result := range results {
        subResults = append(subResults, result)
    }
    
    return dc.taskSplitter.Merge(subResults)
}
```

## 10. 部署架构

### 10.1 单机部署

```yaml
# 单机部署架构
┌─────────────────────────────────────────┐
│              应用服务器                  │
│  ┌─────────────────────────────────────┐ │
│  │       MySQL回收工具进程              │ │
│  │  ┌─────────┐  ┌─────────┐          │ │
│  │  │ CLI模块 │  │ API服务 │          │ │
│  │  └─────────┘  └─────────┘          │ │
│  │  ┌─────────────────────────────────┐ │ │
│  │  │        业务逻辑层                │ │ │
│  │  └─────────────────────────────────┘ │ │
│  └─────────────────────────────────────┘ │
│  ┌─────────────────────────────────────┐ │
│  │         本地存储                     │ │
│  │  ┌─────────┐  ┌─────────┐          │ │
│  │  │ 配置文件│  │ 状态文件│          │ │
│  │  └─────────┘  └─────────┘          │ │
│  └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### 10.2 集群部署

```yaml
# 集群部署架构
┌─────────────────────────────────────────────────────────────┐
│                        负载均衡器                            │
└─────────────┬─────────────┬─────────────┬─────────────────┘
              │             │             │
    ┌─────────▼─────────┐ ┌─▼─────────────▼─┐ ┌─────────▼─────────┐
    │    主节点         │ │   工作节点1     │ │   工作节点2       │
    │  ┌─────────────┐  │ │  ┌────────────┐ │ │  ┌─────────────┐ │
    │  │ 任务调度器   │  │ │  │ 任务执行器  │ │ │  │ 任务执行器   │ │
    │  └─────────────┘  │ │  └────────────┘ │ │  └─────────────┘ │
    │  ┌─────────────┐  │ │  ┌────────────┐ │ │  ┌─────────────┐ │
    │  │ 状态管理器   │  │ │  │ 监控代理    │ │ │  │ 监控代理     │ │
    │  └─────────────┘  │ │  └────────────┘ │ │  └─────────────┘ │
    └───────────────────┘ └────────────────┘ └───────────────────┘
              │                    │                    │
    ┌─────────▼─────────────────────▼────────────────────▼───┐
    │                    共享存储                           │
    │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │
    │  │ 任务队列│  │ 配置存储│  │ 状态存储│              │
    │  └─────────┘  └─────────┘  └─────────┘              │
    └───────────────────────────────────────────────────────┘
```

### 10.3 容器化部署

```dockerfile
# Dockerfile示例
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mysql-recycler ./cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/mysql-recycler .
COPY --from=builder /app/configs/ ./configs/

EXPOSE 8080
CMD ["./mysql-recycler", "server", "--config", "./configs/config.yaml"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  mysql-recycler:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs
      - ./data:/app/data
      - ./logs:/app/logs
    environment:
      - RECYCLER_LOG_LEVEL=info
      - RECYCLER_DB_PASSWORD=${DB_PASSWORD}
    restart: unless-stopped
    
  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  redis_data:
```

### 10.4 Kubernetes部署

```yaml
# kubernetes部署配置
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mysql-recycler
  labels:
    app: mysql-recycler
spec:
  replicas: 3
  selector:
    matchLabels:
      app: mysql-recycler
  template:
    metadata:
      labels:
        app: mysql-recycler
    spec:
      containers:
      - name: mysql-recycler
        image: mysql-recycler:latest
        ports:
        - containerPort: 8080
        env:
        - name: RECYCLER_LOG_LEVEL
          value: "info"
        - name: RECYCLER_DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: password
        volumeMounts:
        - name: config-volume
          mountPath: /app/configs
        - name: data-volume
          mountPath: /app/data
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: mysql-recycler-config
      - name: data-volume
        persistentVolumeClaim:
          claimName: mysql-recycler-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: mysql-recycler-service
spec:
  selector:
    app: mysql-recycler
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

---

*本文档基于系统需求设计，随着开发进展会持续更新和完善。*