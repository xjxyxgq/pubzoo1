# MySQL表空间回收工具 TDD开发指南

## 目录
- [1. TDD概述](#1-tdd概述)
- [2. 测试策略](#2-测试策略)
- [3. 测试环境搭建](#3-测试环境搭建)
- [4. 测试用例设计原则](#4-测试用例设计原则)
- [5. 单元测试指南](#5-单元测试指南)
- [6. 集成测试指南](#6-集成测试指南)
- [7. 性能测试指南](#7-性能测试指南)
- [8. 测试工具和框架](#8-测试工具和框架)
- [9. 持续集成](#9-持续集成)
- [10. 测试最佳实践](#10-测试最佳实践)

## 1. TDD概述

### 1.1 TDD开发流程

测试驱动开发(TDD)遵循"红-绿-重构"循环：

```
红阶段 (Red) → 绿阶段 (Green) → 重构阶段 (Refactor)
     ↑                                    ↓
     └────────────── 循环迭代 ←──────────────┘
```

1. **红阶段**：编写一个失败的测试用例
2. **绿阶段**：编写最小可行代码使测试通过
3. **重构阶段**：优化代码结构，保持测试通过

### 1.2 TDD在本项目中的应用

```go
// TDD开发示例：拓扑探测功能

// 第一步：编写失败的测试
func TestTopologyDiscoverer_DiscoverCluster(t *testing.T) {
    // 设置测试数据
    mockDB := setupMockDatabase(t)
    discoverer := NewTopologyDiscoverer(mockDB)
    
    // 执行测试
    topology, err := discoverer.DiscoverTopology(context.Background(), &DatabaseNode{
        Host: "master.example.com",
        Port: 3306,
    })
    
    // 断言结果
    assert.NoError(t, err)
    assert.NotNil(t, topology)
    assert.Equal(t, "master", topology.Master.Role)
    assert.Len(t, topology.Master.Slaves, 2)
}

// 第二步：编写实现代码
func (d *TopologyDiscoverer) DiscoverTopology(ctx context.Context, entry *DatabaseNode) (*ClusterTopology, error) {
    // 最小实现，让测试通过
    return &ClusterTopology{
        Master: &DatabaseNode{
            Host: entry.Host,
            Port: entry.Port,
            Role: "master",
            Slaves: make([]*DatabaseNode, 2),
        },
    }, nil
}

// 第三步：重构代码，添加真实逻辑
func (d *TopologyDiscoverer) DiscoverTopology(ctx context.Context, entry *DatabaseNode) (*ClusterTopology, error) {
    // 实际的拓扑探测逻辑
    master, err := d.identifyMaster(ctx, entry)
    if err != nil {
        return nil, err
    }
    
    slaves, err := d.discoverSlaves(ctx, master)
    if err != nil {
        return nil, err
    }
    
    return &ClusterTopology{
        Master: master,
        Slaves: slaves,
        DiscoverTime: time.Now(),
    }, nil
}
```

## 2. 测试策略

### 2.1 测试金字塔

```
        /\
       /  \
      / E2E \         <- 端到端测试 (少量，高价值)
     /______\
    /        \
   /Integration\      <- 集成测试 (适中，关键路径)
  /____________\
 /              \
/   Unit Tests   \    <- 单元测试 (大量，快速反馈)
/________________\
```

### 2.2 测试分层策略

| 测试层级 | 测试范围 | 测试比例 | 执行频率 | 反馈时间 |
|---------|---------|---------|---------|---------|
| 单元测试 | 函数/方法级别 | 70% | 每次提交 | <5秒 |
| 集成测试 | 模块间交互 | 20% | 每日构建 | 5-30秒 |
| 系统测试 | 完整功能场景 | 8% | 发版前 | 1-5分钟 |
| E2E测试 | 真实环境验证 | 2% | 发版前 | 5-30分钟 |

### 2.3 测试覆盖率目标

```go
// 覆盖率目标配置
var coverageTargets = map[string]float64{
    "pkg/topology":   90.0,  // 核心模块要求高覆盖率
    "pkg/recycler":   90.0,  // 核心模块要求高覆盖率
    "pkg/monitor":    85.0,  // 监控模块
    "pkg/api":        80.0,  // API层
    "pkg/utils":      75.0,  // 工具函数
    "overall":        85.0,  // 整体覆盖率
}

// 覆盖率检查脚本
func checkCoverage(packagePath string, threshold float64) error {
    cmd := exec.Command("go", "test", "-coverprofile=coverage.out", packagePath)
    if err := cmd.Run(); err != nil {
        return err
    }
    
    coverage := parseCoverageReport("coverage.out")
    if coverage < threshold {
        return fmt.Errorf("coverage %.2f%% below threshold %.2f%% for %s", 
            coverage, threshold, packagePath)
    }
    
    return nil
}
```

## 3. 测试环境搭建

### 3.1 测试数据库环境

```go
// 测试数据库管理器
type TestDatabaseManager struct {
    containers map[string]*testcontainers.Container
    configs    map[string]*DatabaseConfig
}

func NewTestDatabaseManager() *TestDatabaseManager {
    return &TestDatabaseManager{
        containers: make(map[string]*testcontainers.Container),
        configs:    make(map[string]*DatabaseConfig),
    }
}

func (tm *TestDatabaseManager) SetupMySQLCluster(t *testing.T, topology *ClusterTopology) error {
    ctx := context.Background()
    
    // 启动主节点
    masterReq := testcontainers.ContainerRequest{
        Image:        "mysql:8.0",
        ExposedPorts: []string{"3306/tcp"},
        Env: map[string]string{
            "MYSQL_ROOT_PASSWORD": "rootpassword",
            "MYSQL_DATABASE":      "testdb",
        },
        WaitingFor: wait.ForLog("ready for connections"),
    }
    
    masterContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: masterReq,
        Started:         true,
    })
    if err != nil {
        return err
    }
    
    // 配置主从复制
    if err := tm.setupReplication(ctx, masterContainer, topology.Slaves); err != nil {
        return err
    }
    
    tm.containers["master"] = &masterContainer
    return nil
}

func (tm *TestDatabaseManager) Cleanup(t *testing.T) {
    ctx := context.Background()
    for name, container := range tm.containers {
        if err := (*container).Terminate(ctx); err != nil {
            t.Logf("Failed to terminate container %s: %v", name, err)
        }
    }
}
```

### 3.2 Mock对象管理

```go
// Mock工厂
type MockFactory struct {
    databases map[string]*MockDatabase
    apis      map[string]*MockAPIClient
}

// Mock数据库
type MockDatabase struct {
    mock.Mock
    queries map[string][]map[string]interface{}
}

func (m *MockDatabase) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    callArgs := m.Called(ctx, query, args)
    return callArgs.Get(0).(*sql.Rows), callArgs.Error(1)
}

func (m *MockDatabase) SetupQueryResult(query string, results []map[string]interface{}) {
    m.queries[query] = results
    m.On("Query", mock.Anything, query, mock.Anything).Return(
        createMockRows(results), nil)
}

// 使用示例
func TestTopologyDiscoverer_GetSlaveHosts(t *testing.T) {
    // 准备Mock
    mockDB := &MockDatabase{queries: make(map[string][]map[string]interface{})}
    mockDB.SetupQueryResult("SHOW SLAVE HOSTS", []map[string]interface{}{
        {"Server_id": 2, "Host": "slave1.example.com", "Port": 3306},
        {"Server_id": 3, "Host": "slave2.example.com", "Port": 3306},
    })
    
    // 执行测试
    discoverer := NewTopologyDiscoverer(mockDB)
    slaves, err := discoverer.GetSlaveHosts(context.Background())
    
    // 验证结果
    assert.NoError(t, err)
    assert.Len(t, slaves, 2)
    assert.Equal(t, "slave1.example.com", slaves[0].Host)
}
```

### 3.3 测试数据管理

```go
// 测试数据构建器
type TestDataBuilder struct {
    clusters   []*ClusterTopology
    jobs       []*RecycleJob
    tables     []*TableInfo
}

func NewTestDataBuilder() *TestDataBuilder {
    return &TestDataBuilder{}
}

func (tdb *TestDataBuilder) WithCluster(name string) *ClusterBuilder {
    return &ClusterBuilder{
        builder: tdb,
        cluster: &ClusterTopology{
            ClusterID: name,
        },
    }
}

type ClusterBuilder struct {
    builder *TestDataBuilder
    cluster *ClusterTopology
}

func (cb *ClusterBuilder) WithMaster(host string, port int) *ClusterBuilder {
    cb.cluster.Master = &DatabaseNode{
        Host: host,
        Port: port,
        Role: "master",
    }
    return cb
}

func (cb *ClusterBuilder) WithSlave(host string, port int, lag time.Duration) *ClusterBuilder {
    slave := &DatabaseNode{
        Host: host,
        Port: port,
        Role: "slave",
        LagSeconds: int64(lag.Seconds()),
    }
    cb.cluster.Master.Slaves = append(cb.cluster.Master.Slaves, slave)
    return cb
}

func (cb *ClusterBuilder) Build() *ClusterTopology {
    cb.builder.clusters = append(cb.builder.clusters, cb.cluster)
    return cb.cluster
}

// 使用示例
func setupTestData(t *testing.T) *TestDataBuilder {
    return NewTestDataBuilder().
        WithCluster("test-cluster").
        WithMaster("master.test.com", 3306).
        WithSlave("slave1.test.com", 3306, 100*time.Millisecond).
        WithSlave("slave2.test.com", 3306, 200*time.Millisecond).
        Build()
}
```

## 4. 测试用例设计原则

### 4.1 FIRST原则

- **Fast (快速)**：测试应该快速运行
- **Independent (独立)**：测试间不应相互依赖
- **Repeatable (可重复)**：测试结果应该一致
- **Self-Validating (自我验证)**：测试应该有明确的通过/失败结果
- **Timely (及时)**：测试应该在产品代码之前编写

### 4.2 测试用例命名规范

```go
// 命名格式：Test[FunctionName]_[Scenario]_[ExpectedResult]
func TestTopologyDiscoverer_DiscoverCluster_WithValidMaster_ReturnsCompleteTopology(t *testing.T) {}
func TestTopologyDiscoverer_DiscoverCluster_WithUnreachableNode_ReturnsError(t *testing.T) {}
func TestRecycleExecutor_ExecuteJob_WithBinlogOffMode_SuccessfullyRecycles(t *testing.T) {}
func TestRecycleExecutor_ExecuteJob_WithInsufficientPrivileges_ReturnsError(t *testing.T) {}
```

### 4.3 测试用例结构 (AAA模式)

```go
func TestRecycleExecutor_CalculateFragmentation_ValidTable_ReturnsCorrectValue(t *testing.T) {
    // Arrange (准备)
    executor := NewRecycleExecutor(&RecycleConfig{})
    tableInfo := &TableInfo{
        Schema:    "testdb",
        TableName: "users",
        DataLength: 1024000,  // 1MB
        IndexLength: 256000,  // 256KB
        DataFree:   102400,   // 100KB
    }
    
    // Act (执行)
    fragmentation, err := executor.CalculateFragmentation(tableInfo)
    
    // Assert (断言)
    assert.NoError(t, err)
    assert.Equal(t, 8.0, fragmentation) // 100KB / 1.25MB * 100 = 8%
}
```

### 4.4 边界条件测试

```go
func TestTableFragmentationCalculator_CalculateFragmentation(t *testing.T) {
    testCases := []struct {
        name           string
        dataLength     int64
        indexLength    int64
        dataFree       int64
        expectedResult float64
        expectError    bool
    }{
        {
            name:           "正常情况",
            dataLength:     1000000,
            indexLength:    200000,
            dataFree:       100000,
            expectedResult: 8.33,
        },
        {
            name:           "零碎片",
            dataLength:     1000000,
            indexLength:    200000,
            dataFree:       0,
            expectedResult: 0.0,
        },
        {
            name:           "全部碎片",
            dataLength:     0,
            indexLength:    0,
            dataFree:       100000,
            expectedResult: 100.0,
        },
        {
            name:        "负数输入",
            dataLength:  -1000,
            indexLength: 200000,
            dataFree:    100000,
            expectError: true,
        },
        {
            name:           "极大值",
            dataLength:     math.MaxInt64,
            indexLength:    1000000,
            dataFree:       1000000,
            expectedResult: 0.0, // 近似为0
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            calculator := NewFragmentationCalculator()
            tableInfo := &TableInfo{
                DataLength:  tc.dataLength,
                IndexLength: tc.indexLength,
                DataFree:    tc.dataFree,
            }
            
            result, err := calculator.Calculate(tableInfo)
            
            if tc.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.InDelta(t, tc.expectedResult, result, 0.01)
            }
        })
    }
}
```

## 5. 单元测试指南

### 5.1 核心模块单元测试

```go
// 拓扑探测模块测试
func TestTopologyDiscoverer_IdentifyPrimaryNode(t *testing.T) {
    tests := []struct {
        name           string
        nodeConfig     *DatabaseNode
        readOnly       bool
        slaveStatus    *SlaveStatus
        externalResult *ExternalValidation
        expectedResult bool
        expectError    bool
    }{
        {
            name: "确认主节点",
            nodeConfig: &DatabaseNode{
                Host: "master.test.com",
                Port: 3306,
            },
            readOnly:    false,
            slaveStatus: nil, // 无slave状态
            externalResult: &ExternalValidation{
                Role:     "主",
                ReadOnly: "OFF",
            },
            expectedResult: true,
        },
        {
            name: "确认从节点",
            nodeConfig: &DatabaseNode{
                Host: "slave.test.com",
                Port: 3306,
            },
            readOnly: true,
            slaveStatus: &SlaveStatus{
                MasterHost: "master.test.com",
                MasterPort: 3306,
            },
            externalResult: &ExternalValidation{
                Role:     "从",
                ReadOnly: "ON",
            },
            expectedResult: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 设置Mock
            mockDB := setupMockDatabase(t)
            mockAPI := setupMockExternalAPI(t)
            
            setupDatabaseMocks(mockDB, tt.readOnly, tt.slaveStatus)
            setupAPIResponse(mockAPI, tt.nodeConfig, tt.externalResult)
            
            // 创建探测器
            discoverer := &TopologyDiscoverer{
                db:          mockDB,
                externalAPI: mockAPI,
            }
            
            // 执行测试
            isPrimary, err := discoverer.IdentifyPrimaryNode(context.Background(), tt.nodeConfig)
            
            // 验证结果
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedResult, isPrimary)
            }
        })
    }
}
```

### 5.2 并发模块测试

```go
// 并发控制器测试
func TestConcurrencyController_ProcessTasks(t *testing.T) {
    controller := NewConcurrencyController(5) // 5个worker
    
    // 创建测试任务
    taskCount := 100
    tasks := make([]*Task, taskCount)
    for i := 0; i < taskCount; i++ {
        tasks[i] = &Task{
            ID:      fmt.Sprintf("task-%d", i),
            Payload: fmt.Sprintf("payload-%d", i),
        }
    }
    
    // 并发处理任务
    results := make(chan *TaskResult, taskCount)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    start := time.Now()
    for _, task := range tasks {
        go func(t *Task) {
            result, err := controller.ProcessTask(ctx, t)
            assert.NoError(t, err)
            results <- result
        }(task)
    }
    
    // 收集结果
    var completedTasks []*TaskResult
    for i := 0; i < taskCount; i++ {
        select {
        case result := <-results:
            completedTasks = append(completedTasks, result)
        case <-ctx.Done():
            t.Fatal("Test timeout")
        }
    }
    
    duration := time.Since(start)
    
    // 验证结果
    assert.Len(t, completedTasks, taskCount)
    assert.Less(t, duration, 5*time.Second, "并发处理应该更快")
    
    // 验证没有数据竞争
    taskIDs := make(map[string]bool)
    for _, result := range completedTasks {
        assert.False(t, taskIDs[result.TaskID], "任务ID重复: %s", result.TaskID)
        taskIDs[result.TaskID] = true
    }
}
```

### 5.3 错误处理测试

```go
// 错误处理和重试机制测试
func TestRecycleExecutor_RetryMechanism(t *testing.T) {
    mockDB := &MockDatabase{}
    
    // 设置前两次调用失败，第三次成功
    mockDB.On("Exec", mock.Anything, "ALTER TABLE users ENGINE=InnoDB").
        Return(nil, errors.New("connection lost")).Once()
    mockDB.On("Exec", mock.Anything, "ALTER TABLE users ENGINE=InnoDB").
        Return(nil, errors.New("deadlock")).Once()
    mockDB.On("Exec", mock.Anything, "ALTER TABLE users ENGINE=InnoDB").
        Return(&sql.Result{}, nil).Once()
    
    config := &RecycleConfig{
        RetryCount:    3,
        RetryInterval: 100 * time.Millisecond,
    }
    
    executor := NewRecycleExecutor(mockDB, config)
    
    // 执行测试
    err := executor.RecycleTable(context.Background(), &TableInfo{
        Schema:    "testdb",
        TableName: "users",
    })
    
    // 验证结果
    assert.NoError(t, err)
    mockDB.AssertExpectations(t)
}

// 超时处理测试
func TestRecycleExecutor_TimeoutHandling(t *testing.T) {
    mockDB := &MockDatabase{}
    
    // 设置长时间运行的操作
    mockDB.On("Exec", mock.Anything, mock.Anything).
        Run(func(args mock.Arguments) {
            time.Sleep(2 * time.Second) // 模拟长时间操作
        }).
        Return(&sql.Result{}, nil)
    
    config := &RecycleConfig{
        OperationTimeout: 500 * time.Millisecond,
    }
    
    executor := NewRecycleExecutor(mockDB, config)
    
    // 执行测试
    start := time.Now()
    err := executor.RecycleTable(context.Background(), &TableInfo{
        Schema:    "testdb",
        TableName: "large_table",
    })
    duration := time.Since(start)
    
    // 验证结果
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "timeout")
    assert.Less(t, duration, 1*time.Second, "应该在超时时间内返回")
}
```

## 6. 集成测试指南

### 6.1 数据库集成测试

```go
// 真实MySQL环境集成测试
func TestTopologyDiscoverer_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }
    
    // 设置测试集群
    testCluster := setupIntegrationCluster(t)
    defer testCluster.Cleanup()
    
    // 创建探测器
    discoverer := NewTopologyDiscoverer(&DatabaseConfig{
        ConnectTimeout: 10 * time.Second,
        QueryTimeout:   5 * time.Second,
    })
    
    // 执行拓扑发现
    topology, err := discoverer.DiscoverTopology(context.Background(), &DatabaseNode{
        Host:     testCluster.MasterHost,
        Port:     testCluster.MasterPort,
        Username: "root",
        Password: "testpass",
    })
    
    // 验证结果
    require.NoError(t, err)
    require.NotNil(t, topology)
    
    // 验证主节点
    assert.Equal(t, testCluster.MasterHost, topology.Master.Host)
    assert.Equal(t, "master", topology.Master.Role)
    assert.False(t, topology.Master.ReadOnly)
    
    // 验证从节点
    require.Len(t, topology.Master.Slaves, 2)
    for _, slave := range topology.Master.Slaves {
        assert.Equal(t, "slave", slave.Role)
        assert.True(t, slave.ReadOnly)
        assert.GreaterOrEqual(t, slave.LagSeconds, int64(0))
    }
}

// 集群环境设置
type IntegrationCluster struct {
    MasterHost   string
    MasterPort   int
    SlaveConfigs []SlaveConfig
    containers   []*testcontainers.Container
}

type SlaveConfig struct {
    Host string
    Port int
}

func setupIntegrationCluster(t *testing.T) *IntegrationCluster {
    ctx := context.Background()
    
    // 启动主节点
    masterContainer := startMySQLContainer(ctx, t, "master", map[string]string{
        "MYSQL_ROOT_PASSWORD": "testpass",
        "server-id":          "1",
        "log-bin":            "mysql-bin",
    })
    
    masterHost := getContainerHost(ctx, t, masterContainer)
    masterPort := getContainerPort(ctx, t, masterContainer, "3306/tcp")
    
    // 启动从节点
    var slaves []SlaveConfig
    var containers []*testcontainers.Container
    
    for i := 2; i <= 3; i++ {
        slaveContainer := startMySQLContainer(ctx, t, fmt.Sprintf("slave%d", i-1), map[string]string{
            "MYSQL_ROOT_PASSWORD": "testpass",
            "server-id":          fmt.Sprintf("%d", i),
            "relay-log":          "relay-bin",
        })
        
        slaveHost := getContainerHost(ctx, t, slaveContainer)
        slavePort := getContainerPort(ctx, t, slaveContainer, "3306/tcp")
        
        // 配置复制
        setupReplication(t, masterHost, masterPort, slaveHost, slavePort)
        
        slaves = append(slaves, SlaveConfig{
            Host: slaveHost,
            Port: slavePort,
        })
        containers = append(containers, &slaveContainer)
    }
    
    containers = append(containers, &masterContainer)
    
    return &IntegrationCluster{
        MasterHost:   masterHost,
        MasterPort:   masterPort,
        SlaveConfigs: slaves,
        containers:   containers,
    }
}
```

### 6.2 API集成测试

```go
// API集成测试
func TestAPI_Integration(t *testing.T) {
    // 启动测试服务器
    server := setupTestServer(t)
    defer server.Close()
    
    client := &http.Client{Timeout: 30 * time.Second}
    baseURL := server.URL
    
    // 测试拓扑发现API
    t.Run("DiscoverTopology", func(t *testing.T) {
        reqBody := map[string]interface{}{
            "host":     "test-master.example.com",
            "port":     3306,
            "username": "testuser",
            "password": "testpass",
        }
        
        resp, err := httpPost(client, baseURL+"/api/v1/topology/discover", reqBody)
        require.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        var result map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&result)
        require.NoError(t, err)
        
        assert.Equal(t, float64(0), result["code"])
        assert.NotNil(t, result["data"])
    })
    
    // 测试任务创建和查询API
    t.Run("JobLifecycle", func(t *testing.T) {
        // 创建任务
        createReq := map[string]interface{}{
            "target": map[string]interface{}{
                "host":     "test-slave.example.com",
                "port":     3306,
                "username": "testuser",
                "password": "testpass",
            },
            "config": map[string]interface{}{
                "mode":             "binlog-off",
                "threshold_mb":     100,
                "threshold_percent": 10.0,
            },
        }
        
        resp, err := httpPost(client, baseURL+"/api/v1/recycle/jobs", createReq)
        require.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var createResult map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&createResult)
        require.NoError(t, err)
        
        jobID := createResult["data"].(map[string]interface{})["job_id"].(string)
        assert.NotEmpty(t, jobID)
        
        // 查询任务状态
        resp2, err := client.Get(baseURL + "/api/v1/recycle/jobs/" + jobID)
        require.NoError(t, err)
        defer resp2.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp2.StatusCode)
        
        var statusResult map[string]interface{}
        err = json.NewDecoder(resp2.Body).Decode(&statusResult)
        require.NoError(t, err)
        
        jobData := statusResult["data"].(map[string]interface{})
        assert.Equal(t, jobID, jobData["job_id"])
        assert.Contains(t, []string{"pending", "running", "completed"}, jobData["status"])
    })
}
```

## 7. 性能测试指南

### 7.1 基准测试

```go
// 拓扑探测性能基准测试
func BenchmarkTopologyDiscoverer_DiscoverTopology(b *testing.B) {
    discoverer := setupBenchmarkDiscoverer(b)
    testNode := &DatabaseNode{
        Host: "benchmark.test.com",
        Port: 3306,
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, err := discoverer.DiscoverTopology(context.Background(), testNode)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// 并发性能测试
func BenchmarkRecycleExecutor_ConcurrentRecycle(b *testing.B) {
    executor := setupBenchmarkExecutor(b)
    tables := generateBenchmarkTables(100) // 100个表
    
    b.ResetTimer()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            for _, table := range tables {
                err := executor.RecycleTable(context.Background(), table)
                if err != nil {
                    b.Fatal(err)
                }
            }
        }
    })
}

// 内存分配基准测试
func BenchmarkMemoryAllocation_TableProcessing(b *testing.B) {
    processor := NewTableProcessor()
    tables := generateLargeTables(1000)
    
    b.ReportAllocs()
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        results := processor.ProcessTables(tables)
        if len(results) != 1000 {
            b.Fatal("Unexpected result count")
        }
    }
}
```

### 7.2 负载测试

```go
// 负载测试框架
type LoadTestRunner struct {
    concurrency  int
    duration     time.Duration
    rampUpTime   time.Duration
    target       string
    scenarios    []LoadScenario
}

type LoadScenario struct {
    Name        string
    Weight      int // 权重
    RequestFunc func(ctx context.Context) error
}

func (ltr *LoadTestRunner) RunLoadTest(ctx context.Context) (*LoadTestResult, error) {
    result := &LoadTestResult{
        StartTime: time.Now(),
    }
    
    // 创建worker池
    workers := make(chan struct{}, ltr.concurrency)
    for i := 0; i < ltr.concurrency; i++ {
        workers <- struct{}{}
    }
    
    // 统计收集器
    stats := NewStatsCollector()
    
    // 执行负载测试
    testCtx, cancel := context.WithTimeout(ctx, ltr.duration)
    defer cancel()
    
    var wg sync.WaitGroup
    
    for {
        select {
        case <-testCtx.Done():
            goto done
        case <-workers:
            wg.Add(1)
            go func() {
                defer wg.Done()
                defer func() { workers <- struct{}{} }()
                
                scenario := ltr.selectScenario()
                start := time.Now()
                err := scenario.RequestFunc(testCtx)
                duration := time.Since(start)
                
                stats.Record(scenario.Name, duration, err)
            }()
        }
    }
    
done:
    wg.Wait()
    
    result.EndTime = time.Now()
    result.Statistics = stats.GetStatistics()
    
    return result, nil
}

// 使用示例
func TestLoadTest_APIEndpoints(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过负载测试")
    }
    
    runner := &LoadTestRunner{
        concurrency: 50,
        duration:    2 * time.Minute,
        target:      "http://localhost:8080",
        scenarios: []LoadScenario{
            {
                Name:   "DiscoverTopology",
                Weight: 30,
                RequestFunc: func(ctx context.Context) error {
                    return callDiscoverTopologyAPI(runner.target)
                },
            },
            {
                Name:   "CreateJob",
                Weight: 50,
                RequestFunc: func(ctx context.Context) error {
                    return callCreateJobAPI(runner.target)
                },
            },
            {
                Name:   "QueryStatus",
                Weight: 20,
                RequestFunc: func(ctx context.Context) error {
                    return callQueryStatusAPI(runner.target)
                },
            },
        },
    }
    
    result, err := runner.RunLoadTest(context.Background())
    require.NoError(t, err)
    
    // 验证性能指标
    assert.Less(t, result.Statistics["DiscoverTopology"].AvgResponseTime, 1*time.Second)
    assert.Less(t, result.Statistics["CreateJob"].P95ResponseTime, 2*time.Second)
    assert.Greater(t, result.Statistics["QueryStatus"].ThroughputRPS, float64(100))
}
```

### 7.3 压力测试

```go
// 压力测试：测试系统极限
func TestStressTest_MaxConcurrency(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过压力测试")
    }
    
    // 逐步增加并发数，找到系统极限
    maxConcurrencies := []int{100, 200, 500, 1000, 2000}
    
    for _, maxConcurrency := range maxConcurrencies {
        t.Run(fmt.Sprintf("Concurrency_%d", maxConcurrency), func(t *testing.T) {
            success, avgLatency := runConcurrencyTest(t, maxConcurrency)
            
            t.Logf("并发数: %d, 成功率: %.2f%%, 平均延迟: %v", 
                maxConcurrency, success*100, avgLatency)
            
            // 定义可接受的性能阈值
            assert.Greater(t, success, 0.95, "成功率应该大于95%")
            assert.Less(t, avgLatency, 5*time.Second, "平均延迟应该小于5秒")
        })
    }
}

func runConcurrencyTest(t *testing.T, concurrency int) (successRate float64, avgLatency time.Duration) {
    var successCount int64
    var totalLatency int64
    var totalRequests int64 = int64(concurrency)
    
    var wg sync.WaitGroup
    wg.Add(concurrency)
    
    for i := 0; i < concurrency; i++ {
        go func() {
            defer wg.Done()
            
            start := time.Now()
            err := executeStressTestRequest()
            latency := time.Since(start)
            
            atomic.AddInt64(&totalLatency, int64(latency))
            if err == nil {
                atomic.AddInt64(&successCount, 1)
            }
        }()
    }
    
    wg.Wait()
    
    successRate = float64(successCount) / float64(totalRequests)
    avgLatency = time.Duration(totalLatency / totalRequests)
    
    return successRate, avgLatency
}
```

## 8. 测试工具和框架

### 8.1 测试框架选择

```go
// 主要测试依赖
import (
    "testing"                           // Go标准测试框架
    "github.com/stretchr/testify/assert" // 断言库
    "github.com/stretchr/testify/require" // 必要断言库
    "github.com/stretchr/testify/mock"   // Mock框架
    "github.com/stretchr/testify/suite"  // 测试套件
    "github.com/testcontainers/testcontainers-go" // 容器测试
    "github.com/testcontainers/testcontainers-go/wait" // 等待策略
    "github.com/golang/mock/gomock"      // Go mock生成器
    "github.com/DATA-DOG/go-sqlmock"    // SQL mock
)

// 测试套件示例
type RecyclerTestSuite struct {
    suite.Suite
    db        *sql.DB
    mock      sqlmock.Sqlmock
    recycler  *RecycleExecutor
    cleanup   func()
}

func (suite *RecyclerTestSuite) SetupSuite() {
    // 测试套件级别的设置
    var err error
    suite.db, suite.mock, err = sqlmock.New()
    suite.Require().NoError(err)
    
    suite.recycler = NewRecycleExecutor(suite.db, &RecycleConfig{})
}

func (suite *RecyclerTestSuite) TearDownSuite() {
    // 测试套件级别的清理
    suite.db.Close()
    if suite.cleanup != nil {
        suite.cleanup()
    }
}

func (suite *RecyclerTestSuite) SetupTest() {
    // 每个测试方法前的设置
    suite.mock.ExpectBegin()
}

func (suite *RecyclerTestSuite) TearDownTest() {
    // 每个测试方法后的清理
    suite.Assert().NoError(suite.mock.ExpectationsWereMet())
}

func TestRecyclerTestSuite(t *testing.T) {
    suite.Run(t, new(RecyclerTestSuite))
}
```

### 8.2 测试辅助工具

```go
// 测试工具集
type TestUtils struct {
    logger     *logrus.Logger
    containers map[string]testcontainers.Container
    tempDirs   []string
}

func NewTestUtils() *TestUtils {
    return &TestUtils{
        logger:     logrus.New(),
        containers: make(map[string]testcontainers.Container),
    }
}

// HTTP测试客户端
func (tu *TestUtils) NewHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            MaxIdleConns:        10,
            IdleConnTimeout:     30 * time.Second,
            DisableCompression:  true,
        },
    }
}

// 临时目录管理
func (tu *TestUtils) CreateTempDir(t *testing.T) string {
    dir, err := ioutil.TempDir("", "recycler-test-")
    require.NoError(t, err)
    
    tu.tempDirs = append(tu.tempDirs, dir)
    return dir
}

// 清理资源
func (tu *TestUtils) Cleanup(t *testing.T) {
    // 清理容器
    ctx := context.Background()
    for name, container := range tu.containers {
        if err := container.Terminate(ctx); err != nil {
            t.Logf("Failed to terminate container %s: %v", name, err)
        }
    }
    
    // 清理临时目录
    for _, dir := range tu.tempDirs {
        if err := os.RemoveAll(dir); err != nil {
            t.Logf("Failed to remove temp dir %s: %v", dir, err)
        }
    }
}

// 等待条件满足
func (tu *TestUtils) WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    timeoutChan := time.After(timeout)
    
    for {
        select {
        case <-timeoutChan:
            t.Fatalf("Timeout waiting for condition: %s", message)
        case <-ticker.C:
            if condition() {
                return
            }
        }
    }
}
```

### 8.3 测试数据生成器

```go
// 测试数据生成器
type TestDataGenerator struct {
    faker *gofakeit.Faker
}

func NewTestDataGenerator(seed int64) *TestDataGenerator {
    faker := gofakeit.New(seed)
    return &TestDataGenerator{faker: faker}
}

func (tg *TestDataGenerator) GenerateClusterTopology(masterCount, slaveCount int) *ClusterTopology {
    topology := &ClusterTopology{
        ClusterID:    tg.faker.UUID(),
        DiscoverTime: time.Now(),
    }
    
    // 生成主节点
    topology.Master = &DatabaseNode{
        Host:     tg.faker.IPv4Address(),
        Port:     3306,
        ServerID: tg.faker.Uint32(),
        Role:     "master",
        ReadOnly: false,
    }
    
    // 生成从节点
    for i := 0; i < slaveCount; i++ {
        slave := &DatabaseNode{
            Host:       tg.faker.IPv4Address(),
            Port:       3306,
            ServerID:   tg.faker.Uint32(),
            Role:       "slave",
            ReadOnly:   true,
            LagSeconds: tg.faker.Int64Range(0, 300), // 0-300秒延迟
        }
        topology.Master.Slaves = append(topology.Master.Slaves, slave)
    }
    
    return topology
}

func (tg *TestDataGenerator) GenerateTableInfo(schema, table string) *TableInfo {
    dataLength := tg.faker.Int64Range(1000000, 100000000)   // 1MB - 100MB
    indexLength := tg.faker.Int64Range(100000, 10000000)    // 100KB - 10MB
    dataFree := tg.faker.Int64Range(0, dataLength/10)       // 0 - 10% fragmentation
    
    return &TableInfo{
        Schema:      schema,
        TableName:   table,
        Engine:      "InnoDB",
        DataLength:  dataLength,
        IndexLength: indexLength,
        DataFree:    dataFree,
        Rows:        tg.faker.Int64Range(1000, 1000000),
        CreateTime:  tg.faker.DateRange(time.Now().AddDate(-1, 0, 0), time.Now()),
    }
}

func (tg *TestDataGenerator) GenerateRecycleJob() *RecycleJob {
    return &RecycleJob{
        JobID:  tg.faker.UUID(),
        Status: JobStatus(tg.faker.RandomString([]string{
            string(JobStatusPending),
            string(JobStatusRunning),
            string(JobStatusCompleted),
        })),
        Config: &RecycleConfig{
            ThresholdMB:      tg.faker.IntRange(50, 500),
            ThresholdPercent: tg.faker.Float64Range(5.0, 30.0),
            BatchSize:        tg.faker.IntRange(5, 20),
            Mode:             WorkMode(tg.faker.RandomString([]string{
                string(ModeBinlogOff),
                string(ModeBinlogOn),
            })),
        },
        CreatedAt: tg.faker.DateRange(time.Now().AddDate(0, 0, -7), time.Now()),
    }
}
```

## 9. 持续集成

### 9.1 CI/CD管道配置

```yaml
# .github/workflows/ci.yml
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      mysql-master:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: rootpass
          MYSQL_DATABASE: testdb
        ports:
          - 3306:3306
        options: >-
          --health-cmd="mysqladmin ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=3
          
      mysql-slave:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: rootpass
          MYSQL_DATABASE: testdb
        ports:
          - 3307:3306
        options: >-
          --health-cmd="mysqladmin ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=3
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
        
    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
          
    - name: Install dependencies
      run: |
        go mod download
        go install github.com/golang/mock/mockgen@latest
        
    - name: Generate mocks
      run: |
        go generate ./...
        
    - name: Run linter
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --timeout=5m
        
    - name: Run unit tests
      run: |
        go test -race -coverprofile=coverage.out ./...
        
    - name: Run integration tests
      run: |
        go test -tags=integration -race ./...
      env:
        MYSQL_MASTER_HOST: localhost
        MYSQL_MASTER_PORT: 3306
        MYSQL_SLAVE_HOST: localhost
        MYSQL_SLAVE_PORT: 3307
        MYSQL_PASSWORD: rootpass
        
    - name: Check test coverage
      run: |
        go tool cover -html=coverage.out -o coverage.html
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
        echo "Coverage: ${COVERAGE}%"
        if (( $(echo "$COVERAGE < 85" | bc -l) )); then
          echo "Coverage ${COVERAGE}% is below threshold 85%"
          exit 1
        fi
        
    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        
    - name: Run benchmarks
      run: |
        go test -bench=. -benchmem -run=^$ ./... > benchmark.txt
        
    - name: Performance regression check
      run: |
        # 比较基准测试结果，检查性能回归
        ./scripts/check-performance-regression.sh
        
  build:
    needs: test
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
        
    - name: Build binary
      run: |
        CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o mysql-recycler ./cmd/main.go
        
    - name: Build Docker image
      run: |
        docker build -t mysql-recycler:${{ github.sha }} .
        
    - name: Run security scan
      uses: securecodewarrior/github-action-add-sarif@v1
      with:
        sarif-file: 'security-scan-results.sarif'
```

### 9.2 测试报告生成

```go
// 测试报告生成器
type TestReporter struct {
    results    []TestResult
    coverage   *CoverageReport
    benchmarks []BenchmarkResult
}

type TestResult struct {
    Package   string        `json:"package"`
    TestName  string        `json:"test_name"`
    Status    string        `json:"status"`
    Duration  time.Duration `json:"duration"`
    Error     string        `json:"error,omitempty"`
}

type CoverageReport struct {
    OverallCoverage float64                    `json:"overall_coverage"`
    PackageCoverage map[string]float64         `json:"package_coverage"`
    UncoveredLines  map[string][]UncoveredLine `json:"uncovered_lines"`
}

type BenchmarkResult struct {
    Name         string  `json:"name"`
    Iterations   int     `json:"iterations"`
    NsPerOp      int64   `json:"ns_per_op"`
    BytesPerOp   int64   `json:"bytes_per_op"`
    AllocsPerOp  int64   `json:"allocs_per_op"`
    MemoryMB     float64 `json:"memory_mb"`
}

func (tr *TestReporter) GenerateReport(outputFormat string) error {
    report := map[string]interface{}{
        "timestamp":  time.Now(),
        "summary":    tr.generateSummary(),
        "tests":      tr.results,
        "coverage":   tr.coverage,
        "benchmarks": tr.benchmarks,
    }
    
    switch outputFormat {
    case "json":
        return tr.generateJSONReport(report)
    case "html":
        return tr.generateHTMLReport(report)
    case "junit":
        return tr.generateJUnitReport(report)
    default:
        return fmt.Errorf("unsupported format: %s", outputFormat)
    }
}

func (tr *TestReporter) generateSummary() map[string]interface{} {
    var passed, failed, skipped int
    var totalDuration time.Duration
    
    for _, result := range tr.results {
        totalDuration += result.Duration
        switch result.Status {
        case "PASS":
            passed++
        case "FAIL":
            failed++
        case "SKIP":
            skipped++
        }
    }
    
    return map[string]interface{}{
        "total":     len(tr.results),
        "passed":    passed,
        "failed":    failed,
        "skipped":   skipped,
        "duration":  totalDuration,
        "pass_rate": float64(passed) / float64(len(tr.results)) * 100,
    }
}
```

### 9.3 质量门禁

```bash
#!/bin/bash
# scripts/quality-gate.sh

set -e

echo "=== Quality Gate Checks ==="

# 1. 测试覆盖率检查
echo "Checking test coverage..."
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
COVERAGE_THRESHOLD=85

if (( $(echo "$COVERAGE < $COVERAGE_THRESHOLD" | bc -l) )); then
    echo "❌ Coverage $COVERAGE% is below threshold $COVERAGE_THRESHOLD%"
    exit 1
fi
echo "✅ Coverage $COVERAGE% passes threshold"

# 2. 代码质量检查
echo "Running static analysis..."
golangci-lint run --timeout=5m
echo "✅ Static analysis passed"

# 3. 安全扫描
echo "Running security scan..."
gosec ./...
echo "✅ Security scan passed"

# 4. 依赖检查
echo "Checking for vulnerable dependencies..."
go list -json -m all | nancy sleuth
echo "✅ Dependency check passed"

# 5. 性能回归检查
echo "Checking for performance regression..."
if [ -f "benchmark-baseline.txt" ]; then
    benchcmp benchmark-baseline.txt benchmark.txt
    if [ $? -ne 0 ]; then
        echo "❌ Performance regression detected"
        exit 1
    fi
fi
echo "✅ Performance check passed"

# 6. 集成测试
echo "Running integration tests..."
go test -tags=integration ./...
echo "✅ Integration tests passed"

echo "=== All Quality Gates Passed ==="
```

## 10. 测试最佳实践

### 10.1 测试原则

1. **独立性**：每个测试应该独立运行，不依赖其他测试的状态
2. **可重复性**：测试结果应该一致和可重复
3. **快速反馈**：单元测试应该快速运行，提供即时反馈
4. **有意义的断言**：使用清晰的断言和错误消息
5. **测试数据管理**：使用Builder模式或工厂模式管理测试数据

### 10.2 常见反模式和解决方案

```go
// ❌ 反模式：测试间相互依赖
func TestA(t *testing.T) {
    globalState = "modified"
}

func TestB(t *testing.T) {
    // 依赖TestA修改的状态
    assert.Equal(t, "modified", globalState)
}

// ✅ 正确做法：每个测试独立设置状态
func TestA(t *testing.T) {
    state := "modified" // 本地状态
    // 测试逻辑
}

func TestB(t *testing.T) {
    state := setupRequiredState() // 独立设置
    // 测试逻辑
}

// ❌ 反模式：测试过于复杂
func TestComplexScenario(t *testing.T) {
    // 100行的复杂测试逻辑
    // 测试多个功能点
    // 难以理解测试意图
}

// ✅ 正确做法：分解为多个简单测试
func TestScenario_PartA(t *testing.T) {
    // 专注测试一个功能点
}

func TestScenario_PartB(t *testing.T) {
    // 专注测试另一个功能点
}

// ❌ 反模式：魔法数字和硬编码
func TestCalculation(t *testing.T) {
    result := calculate(42, 3.14)
    assert.Equal(t, 131.88, result)
}

// ✅ 正确做法：使用有意义的常量
func TestCalculation(t *testing.T) {
    const (
        inputValue    = 42
        multiplier    = 3.14
        expectedResult = 131.88
    )
    
    result := calculate(inputValue, multiplier)
    assert.Equal(t, expectedResult, result)
}
```

### 10.3 测试维护策略

```go
// 测试维护工具
type TestMaintainer struct {
    testSuites map[string]*TestSuite
    metrics    *TestMetrics
}

type TestMetrics struct {
    TestCount        int
    FailureRate      float64
    AverageRunTime   time.Duration
    CoverageRate     float64
    FlakyTests       []string
    SlowTests        []string
}

func (tm *TestMaintainer) AnalyzeTestHealth() *HealthReport {
    report := &HealthReport{
        Timestamp: time.Now(),
    }
    
    // 分析测试稳定性
    report.FlakyTests = tm.identifyFlakyTests()
    
    // 分析测试性能
    report.SlowTests = tm.identifySlowTests()
    
    // 分析测试覆盖率
    report.CoverageGaps = tm.identifyCoverageGaps()
    
    // 生成改进建议
    report.Recommendations = tm.generateRecommendations(report)
    
    return report
}

// 测试清理工具
func (tm *TestMaintainer) CleanupObsoleteTests() error {
    obsoleteTests := tm.findObsoleteTests()
    
    for _, test := range obsoleteTests {
        if err := tm.removeTest(test); err != nil {
            return err
        }
    }
    
    return tm.updateTestSuite()
}
```

### 10.4 文档化测试

```go
// 使用行为驱动开发(BDD)风格的测试文档
func TestTopologyDiscovery_BehaviorSpecification(t *testing.T) {
    // Given: 一个包含主从节点的MySQL集群
    Given(t, "一个包含主从节点的MySQL集群", func(ctx *TestContext) {
        ctx.SetupCluster(&ClusterConfig{
            Master: NodeConfig{Host: "master.test", Port: 3306},
            Slaves: []NodeConfig{
                {Host: "slave1.test", Port: 3306},
                {Host: "slave2.test", Port: 3306},
            },
        })
    })
    
    // When: 执行拓扑发现
    When(t, "执行拓扑发现", func(ctx *TestContext) {
        discoverer := NewTopologyDiscoverer(ctx.GetDB())
        result, err := discoverer.DiscoverTopology(context.Background(), 
            ctx.GetMasterNode())
        ctx.SetResult("topology", result)
        ctx.SetResult("error", err)
    })
    
    // Then: 应该返回完整的集群拓扑
    Then(t, "应该返回完整的集群拓扑", func(ctx *TestContext) {
        topology := ctx.GetResult("topology").(*ClusterTopology)
        err := ctx.GetResult("error").(error)
        
        assert.NoError(t, err)
        assert.NotNil(t, topology)
        assert.Equal(t, "master.test", topology.Master.Host)
        assert.Len(t, topology.Master.Slaves, 2)
    })
}

// 测试文档生成
func GenerateTestDocumentation() error {
    tests := collectAllTests()
    
    doc := &TestDocumentation{
        GeneratedAt: time.Now(),
        TestSuites:  make(map[string]*TestSuiteDoc),
    }
    
    for _, test := range tests {
        suite := getOrCreateSuite(doc, test.Package)
        
        testDoc := &TestDoc{
            Name:        test.Name,
            Description: extractDescription(test),
            Scenarios:   extractScenarios(test),
            Examples:    extractExamples(test),
        }
        
        suite.Tests = append(suite.Tests, testDoc)
    }
    
    return renderDocumentation(doc)
}
```

---

*本TDD开发指南为MySQL表空间回收工具项目提供全面的测试指导，帮助开发团队建立高质量的测试实践。*