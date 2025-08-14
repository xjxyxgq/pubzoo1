package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"mysql_tablespace_recycling/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDBConnection 模拟数据库连接
type MockDBConnection struct {
	mock.Mock
}

func (m *MockDBConnection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	callArgs := m.Called(query, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(*sql.Rows), callArgs.Error(1)
}

func (m *MockDBConnection) QueryRow(query string, args ...interface{}) database.RowScanner {
	callArgs := m.Called(query, args)
	if callArgs.Get(0) == nil {
		return nil
	}
	return callArgs.Get(0).(database.RowScanner)
}

func (m *MockDBConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDBConnection) Ping() error {
	args := m.Called()
	return args.Error(0)
}

// MockConnectionFactory 模拟连接工厂
type MockConnectionFactory struct {
	mock.Mock
}

func (m *MockConnectionFactory) CreateConnection(config database.NodeConfigInterface) (database.DBConnection, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(database.DBConnection), args.Error(1)
}

// MockExternalAPIClient 模拟外部API客户端
type MockExternalAPIClient struct {
	mock.Mock
}

func (m *MockExternalAPIClient) GetNodeInfo(ctx context.Context, host string) (*ExternalNodeInfo, error) {
	args := m.Called(ctx, host)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ExternalNodeInfo), args.Error(1)
}

// TestNodeConfig 测试节点配置
func TestNodeConfig(t *testing.T) {
	t.Run("Valid node config", func(t *testing.T) {
		node := &NodeConfig{
			Host:     "192.168.1.100",
			Port:     3306,
			Username: "root",
			Password: "password",
		}
		assert.True(t, node.IsValid())
		assert.Equal(t, "192.168.1.100:3306", node.NodeKey())
	})

	t.Run("Invalid node config - empty host", func(t *testing.T) {
		node := &NodeConfig{
			Port:     3306,
			Username: "root",
		}
		assert.False(t, node.IsValid())
	})

	t.Run("Invalid node config - zero port", func(t *testing.T) {
		node := &NodeConfig{
			Host:     "192.168.1.100",
			Username: "root",
		}
		assert.False(t, node.IsValid())
	})
}

// TestNodeStatus 测试节点状态
func TestNodeStatus(t *testing.T) {
	t.Run("Master node status", func(t *testing.T) {
		status := &NodeStatus{
			IsMaster:   true,
			IsReadOnly: false,
			Role:       "主",
			SlaveHosts: []string{"192.168.1.101:3306"},
		}
		assert.True(t, status.IsMasterRole())
		assert.True(t, status.HasSlaves())
	})

	t.Run("Slave node status", func(t *testing.T) {
		status := &NodeStatus{
			IsMaster:   false,
			IsReadOnly: true,
			Role:       "从",
			SlaveStatus: &SlaveStatus{
				MasterHost:          "192.168.1.100",
				MasterPort:          3306,
				SlaveIORunning:      "Yes",
				SlaveSQLRunning:     "Yes",
				SecondsBehindMaster: intPtr(0),
			},
		}
		assert.False(t, status.IsMasterRole())
		assert.False(t, status.HasSlaves())
		assert.True(t, status.SlaveStatus.IsHealthy())
		assert.Equal(t, 0, status.SlaveStatus.GetReplicationDelay())
	})
}

// TestClusterTopology 测试集群拓扑
func TestClusterTopology(t *testing.T) {
	t.Run("Add master and slave nodes", func(t *testing.T) {
		topology := &ClusterTopology{
			Relationships: make(map[string][]string),
		}

		masterNode := &NodeConfig{
			Host: "192.168.1.100",
			Port: 3306,
		}
		masterStatus := &NodeStatus{
			IsMaster:   true,
			IsReadOnly: false,
			Role:       "主",
		}

		slaveNode := &NodeConfig{
			Host: "192.168.1.101",
			Port: 3306,
		}
		slaveStatus := &NodeStatus{
			IsMaster:   false,
			IsReadOnly: true,
			Role:       "从",
		}

		topology.AddNode(masterNode, masterStatus)
		topology.AddNode(slaveNode, slaveStatus)

		assert.Equal(t, 2, topology.TotalNodes)
		assert.Equal(t, masterNode, topology.FindMasterNode())
		assert.Len(t, topology.GetSlaveNodes(), 1)
		assert.Equal(t, slaveNode, topology.GetSlaveNodes()[0])
	})

	t.Run("Get downstream nodes", func(t *testing.T) {
		topology := &ClusterTopology{
			Relationships: map[string][]string{
				"192.168.1.100:3306": {"192.168.1.101:3306"},
			},
		}

		masterNode := &NodeConfig{Host: "192.168.1.100", Port: 3306}
		slaveNode := &NodeConfig{Host: "192.168.1.101", Port: 3306}
		topology.SlaveNodes = []*NodeConfig{slaveNode}

		downstreams := topology.GetDownstreamNodes(masterNode)
		assert.Len(t, downstreams, 1)
		assert.Equal(t, slaveNode, downstreams[0])
	})
}

// TestTopologyDiscoverer 测试拓扑发现器基础功能
func TestTopologyDiscoverer(t *testing.T) {
	t.Run("Create topology discoverer", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		options := &DiscoveryOptions{
			MaxConcurrency: 5,
			Timeout:        30 * time.Second,
			MaxDepth:       3,
		}

		discoverer := NewTopologyDiscoverer(mockFactory, options)
		assert.NotNil(t, discoverer)
	})

	t.Run("Validate invalid node config", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		discoverer := NewTopologyDiscoverer(mockFactory, nil)

		invalidNode := &NodeConfig{
			Host: "", // 无效的空主机名
			Port: 3306,
		}

		ctx := context.Background()
		topology, err := discoverer.DiscoverTopology(ctx, invalidNode)

		assert.Error(t, err)
		assert.Nil(t, topology)
		assert.Contains(t, err.Error(), "invalid entry point")
	})

	t.Run("Handle connection failure", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		mockFactory.On("CreateConnection", mock.Anything).Return(nil, assert.AnError)

		discoverer := NewTopologyDiscoverer(mockFactory, &DiscoveryOptions{
			MaxConcurrency: 1,
			Timeout:        5 * time.Second,
			MaxDepth:       1,
		})

		node := &NodeConfig{
			Host:     "192.168.1.100",
			Port:     3306,
			Username: "root",
			Password: "password",
		}

		ctx := context.Background()
		topology, err := discoverer.DiscoverTopology(ctx, node)

		assert.Error(t, err)
		assert.Nil(t, topology)
		mockFactory.AssertExpectations(t)
	})
}

// TestExternalAPIClient 测试外部API客户端
func TestExternalAPIClient(t *testing.T) {
	t.Run("Create HTTP external API client", func(t *testing.T) {
		client := NewHTTPExternalAPIClient(
			"http://api.test.com/get_mysql_cluster_instance/",
			10*time.Second,
			5*time.Minute,
		)
		assert.NotNil(t, client)
	})

	t.Run("Cache functionality", func(t *testing.T) {
		client := NewHTTPExternalAPIClient(
			"http://api.test.com/get_mysql_cluster_instance/",
			10*time.Second,
			100*time.Millisecond, // 短TTL用于测试
		)

		nodeInfo := &ExternalNodeInfo{
			ClusterName:      "test-cluster",
			InstanceRole:     "从",
			InstanceReadOnly: "ON",
		}

		// 缓存节点信息
		client.cacheNodeInfo("192.168.1.100", nodeInfo)

		// 立即获取应该成功
		cached := client.getCachedNodeInfo("192.168.1.100")
		assert.NotNil(t, cached)
		assert.Equal(t, "test-cluster", cached.ClusterName)

		// 等待缓存过期
		time.Sleep(150 * time.Millisecond)
		cached = client.getCachedNodeInfo("192.168.1.100")
		assert.Nil(t, cached)
	})

	t.Run("Clear cache", func(t *testing.T) {
		client := NewHTTPExternalAPIClient(
			"http://api.test.com/get_mysql_cluster_instance/",
			10*time.Second,
			5*time.Minute,
		)

		nodeInfo := &ExternalNodeInfo{
			ClusterName: "test-cluster",
		}

		client.cacheNodeInfo("192.168.1.100", nodeInfo)
		assert.NotNil(t, client.getCachedNodeInfo("192.168.1.100"))

		client.ClearCache()
		assert.Nil(t, client.getCachedNodeInfo("192.168.1.100"))
	})
}

// TestSlaveStatus 测试从库状态
func TestSlaveStatus(t *testing.T) {
	t.Run("Healthy slave status", func(t *testing.T) {
		status := &SlaveStatus{
			SlaveIORunning:      "Yes",
			SlaveSQLRunning:     "Yes",
			SecondsBehindMaster: intPtr(0),
		}

		assert.True(t, status.IsHealthy())
		assert.Equal(t, 0, status.GetReplicationDelay())
	})

	t.Run("Unhealthy slave status", func(t *testing.T) {
		status := &SlaveStatus{
			SlaveIORunning:      "No",
			SlaveSQLRunning:     "Yes",
			SecondsBehindMaster: intPtr(100),
		}

		assert.False(t, status.IsHealthy())
		assert.Equal(t, 100, status.GetReplicationDelay())
	})

	t.Run("Unknown replication delay", func(t *testing.T) {
		status := &SlaveStatus{
			SlaveIORunning:      "Yes",
			SlaveSQLRunning:     "Yes",
			SecondsBehindMaster: nil,
		}

		assert.True(t, status.IsHealthy())
		assert.Equal(t, -1, status.GetReplicationDelay())
	})
}

// 辅助函数
func intPtr(i int) *int {
	return &i
}

// nodeConfigMatcher 创建NodeConfig的匹配器
func nodeConfigMatcher(expectedHost string) interface{} {
	return mock.MatchedBy(func(node *NodeConfig) bool {
		return node.Host == expectedHost
	})
}

// TestDefaultConnectionFactory 测试默认连接工厂
func TestDefaultConnectionFactory(t *testing.T) {
	t.Run("Create connection with valid config", func(t *testing.T) {
		factory := &DefaultConnectionFactory{}
		
		// 这个测试需要真实的数据库连接，所以我们只测试类型转换逻辑
		invalidConfig := &struct {
			database.NodeConfigInterface
		}{}
		
		_, err := factory.CreateConnection(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be of type *NodeConfig")
	})
}

// TestMySQLConnection 测试MySQL连接
func TestMySQLConnection(t *testing.T) {
	t.Run("Create connection with invalid config", func(t *testing.T) {
		invalidNode := &NodeConfig{
			Host: "", // 无效配置
			Port: 0,
		}
		
		_, err := NewMySQLConnection(invalidNode)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid node config")
	})
}

// TestTopologyDiscovererComplex 测试拓扑发现器的复杂场景
func TestTopologyDiscovererComplex(t *testing.T) {
	t.Run("Discover topology with timeout", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		
		options := &DiscoveryOptions{
			MaxConcurrency: 1,
			Timeout:        100 * time.Millisecond, // 很短的超时
			MaxDepth:       1,
		}
		
		discoverer := NewTopologyDiscoverer(mockFactory, options)
		
		// 模拟连接延迟
		mockFactory.On("CreateConnection", mock.Anything).Return(nil, fmt.Errorf("connection timeout"))
		
		entryNode := &NodeConfig{
			Host:     "slow.example.com",
			Port:     3306,
			Username: "root",
			Password: "password",
		}
		
		ctx := context.Background()
		_, err := discoverer.DiscoverTopology(ctx, entryNode)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to validate entry point")
	})
	
	t.Run("Validate single node", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}
		
		discoverer := NewTopologyDiscoverer(mockFactory, nil)
		
		node := &NodeConfig{
			Host:     "test.example.com",
			Port:     3306,
			Username: "root",
			Password: "password",
		}
		
		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockRowResult("read_only", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockRowResult("server_id", uint32(1)))
		mockConn.On("Query", "SHOW SLAVE STATUS", mock.Anything).Return(createEmptyRows(), nil)
		mockConn.On("Query", "SHOW SLAVE HOSTS", mock.Anything).Return(createEmptyRows(), nil)
		
		ctx := context.Background()
		status, err := discoverer.ValidateNode(ctx, node)
		
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.True(t, status.IsMaster)
		assert.False(t, status.IsReadOnly)
		assert.Equal(t, uint32(1), status.ServerID)
		assert.Equal(t, "主", status.Role)
	})
	
	t.Run("Handle connection errors", func(t *testing.T) {
		mockFactory := &MockConnectionFactory{}
		discoverer := NewTopologyDiscoverer(mockFactory, nil)
		
		node := &NodeConfig{
			Host:     "unreachable.example.com",
			Port:     3306,
			Username: "root",
			Password: "password",
		}
		
		mockFactory.On("CreateConnection", node).Return(nil, fmt.Errorf("connection failed"))
		
		ctx := context.Background()
		
		_, err := discoverer.ValidateNode(ctx, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect")
		
		_, err = discoverer.GetSlaveHosts(ctx, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect")
		
		_, err = discoverer.GetSlaveStatus(ctx, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect")
	})
}

// TestDiscoveryOptions 测试发现选项
func TestDiscoveryOptions(t *testing.T) {
	t.Run("Default discovery options", func(t *testing.T) {
		factory := &MockConnectionFactory{}
		discoverer := NewTopologyDiscoverer(factory, nil).(*DefaultTopologyDiscoverer)
		
		assert.Equal(t, 5, discoverer.options.MaxConcurrency)
		assert.Equal(t, 30*time.Second, discoverer.options.Timeout)
		assert.Equal(t, 3, discoverer.options.MaxDepth)
	})
	
	t.Run("Custom discovery options", func(t *testing.T) {
		factory := &MockConnectionFactory{}
		options := &DiscoveryOptions{
			MaxConcurrency: 10,
			Timeout:        60 * time.Second,
			MaxDepth:       5,
		}
		
		discoverer := NewTopologyDiscoverer(factory, options).(*DefaultTopologyDiscoverer)
		
		assert.Equal(t, 10, discoverer.options.MaxConcurrency)
		assert.Equal(t, 60*time.Second, discoverer.options.Timeout)
		assert.Equal(t, 5, discoverer.options.MaxDepth)
	})
}

// 测试辅助函数

// createEmptyRows 创建空的查询结果
func createEmptyRows() *sql.Rows {
	// 这个函数在实际测试中需要创建一个空的sql.Rows
	// 由于sql.Rows不能直接实例化，我们返回nil表示空结果
	return nil
}

// createSlaveHostRows 创建SHOW SLAVE HOSTS的查询结果
func createSlaveHostRows(slaves []*SlaveHost) *sql.Rows {
	// 在实际实现中，这里需要创建包含slave host数据的sql.Rows
	// 由于测试的复杂性，我们简化处理
	return nil
}

// createSlaveStatusRows 创建SHOW SLAVE STATUS的查询结果  
func createSlaveStatusRows(status *SlaveStatus) *sql.Rows {
	// 在实际实现中，这里需要创建包含slave status数据的sql.Rows
	return nil
}

// TestClusterTopologyAdvanced 测试集群拓扑的高级功能
func TestClusterTopologyAdvanced(t *testing.T) {
	t.Run("Find master node from multiple candidates", func(t *testing.T) {
		topology := &ClusterTopology{
			Relationships: make(map[string][]string),
			NodeStatuses:  make(map[string]*NodeStatus),
		}
		
		// 添加两个可能的主节点候选
		masterNode1 := &NodeConfig{Host: "master1.example.com", Port: 3306}
		masterStatus1 := &NodeStatus{
			IsMaster:   true,
			IsReadOnly: false,
			Role:       "主",
			SlaveStatus: &SlaveStatus{}, // 有slave status，说明不是真正的主节点
		}
		
		masterNode2 := &NodeConfig{Host: "master2.example.com", Port: 3306}
		masterStatus2 := &NodeStatus{
			IsMaster:   true,
			IsReadOnly: false,
			Role:       "主",
			SlaveStatus: nil, // 没有slave status，说明是真正的主节点
		}
		
		topology.AddNode(masterNode1, masterStatus1)
		topology.AddNode(masterNode2, masterStatus2)
		
		// 手动触发分析
		discoverer := &DefaultTopologyDiscoverer{}
		err := discoverer.analyzeMasterSlave(topology)
		assert.NoError(t, err)
		
		// 应该选择没有slave status的节点作为主节点
		assert.NotNil(t, topology.MasterNode)
		assert.Equal(t, "master2.example.com", topology.MasterNode.Host)
	})
	
	t.Run("Handle topology with no clear master", func(t *testing.T) {
		topology := &ClusterTopology{
			Relationships: make(map[string][]string),
			NodeStatuses:  make(map[string]*NodeStatus),
		}
		
		// 只添加从节点
		slaveNode1 := &NodeConfig{Host: "slave1.example.com", Port: 3306}
		slaveStatus1 := &NodeStatus{
			IsMaster:   false,
			IsReadOnly: true,
			Role:       "从",
		}
		
		slaveNode2 := &NodeConfig{Host: "slave2.example.com", Port: 3306}
		slaveStatus2 := &NodeStatus{
			IsMaster:   false,
			IsReadOnly: true,
			Role:       "从",
		}
		
		topology.AddNode(slaveNode1, slaveStatus1)
		topology.AddNode(slaveNode2, slaveStatus2)
		
		discoverer := &DefaultTopologyDiscoverer{}
		err := discoverer.analyzeMasterSlave(topology)
		assert.NoError(t, err)
		
		// 没有主节点
		assert.Nil(t, topology.MasterNode)
		assert.Len(t, topology.SlaveNodes, 2)
	})
}

// TestNodeConfigAdvanced 测试NodeConfig的高级功能
func TestNodeConfigAdvanced(t *testing.T) {
	t.Run("Node key generation", func(t *testing.T) {
		node := &NodeConfig{
			Host: "example.com",
			Port: 3306,
		}
		
		expected := "example.com:3306"
		assert.Equal(t, expected, node.NodeKey())
	})
	
	t.Run("Node validation edge cases", func(t *testing.T) {
		// 测试各种无效配置
		testCases := []struct {
			name string
			node *NodeConfig
			valid bool
		}{
			{
				name: "Missing host",
				node: &NodeConfig{Port: 3306, Username: "root"},
				valid: false,
			},
			{
				name: "Zero port",
				node: &NodeConfig{Host: "localhost", Username: "root"},
				valid: false,
			},
			{
				name: "Missing username", 
				node: &NodeConfig{Host: "localhost", Port: 3306},
				valid: false,
			},
			{
				name: "Valid minimal config",
				node: &NodeConfig{Host: "localhost", Port: 3306, Username: "root"},
				valid: true,
			},
			{
				name: "Valid complete config",
				node: &NodeConfig{
					Host: "localhost", Port: 3306, Username: "root", 
					Password: "pass", Database: "test",
				},
				valid: true,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.valid, tc.node.IsValid(), 
					"Expected IsValid()=%v for %+v", tc.valid, tc.node)
			})
		}
	})
}

// MockRowScanner 实现 RowScanner 接口
type MockRowScanner struct {
	value interface{}
}

func (m *MockRowScanner) Scan(dest ...interface{}) error {
	if len(dest) > 0 {
		switch v := m.value.(type) {
		case string:
			if ptr, ok := dest[0].(*string); ok {
				*ptr = v
			}
		case uint32:
			if ptr, ok := dest[0].(*uint32); ok {
				*ptr = v
			}
		case bool:
			if ptr, ok := dest[0].(*bool); ok {
				*ptr = v
			}
		}
	}
	return nil
}

// mockRowResult 创建模拟的行结果
func mockRowResult(column string, value interface{}) *MockRowScanner {
	return &MockRowScanner{value: value}
}