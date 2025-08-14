package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMasterValidator 测试主节点验证器
func TestMasterValidator(t *testing.T) {
	t.Run("Create master validator", func(t *testing.T) {
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockAPI := &MockExternalAPIClient{}
		mockFactory := &MockConnectionFactory{}
		
		validator := NewMasterValidator(mockDiscoverer, mockAPI, mockFactory, false)
		assert.NotNil(t, validator)
	})

	t.Run("Identify master node - high confidence", func(t *testing.T) {
		// 准备mock对象
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockAPI := &MockExternalAPIClient{}
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}

		node := &NodeConfig{
			Host:     "192.168.1.100",
			Port:     3306,
			Username: "root",
			Password: "password",
		}

		// 设置连接mock
		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		
		// 模拟主节点特征的查询结果
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'super_read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockValidatorRowResult("", uint32(1)))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'log_bin'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'binlog_format'", mock.Anything).Return(mockValidatorRowResult("", "ROW"))
		mockConn.On("Query", "SHOW PROCESSLIST", mock.Anything).Return(nil, nil)

		// 设置discoverer mock - 没有slave status（主节点特征）
		mockDiscoverer.On("GetSlaveStatus", mock.Anything, node).Return(nil, nil)
		
		// 设置discoverer mock - 有slave hosts（主节点特征）
		slaveHosts := []*SlaveHost{
			{ServerID: 2, Host: "192.168.1.101", Port: 3306, User: "repl"},
		}
		mockDiscoverer.On("GetSlaveHosts", mock.Anything, node).Return(slaveHosts, nil)

		// 设置外部API mock
		externalInfo := &ExternalNodeInfo{
			ClusterName:      "test-cluster",
			InstanceRole:     "主",
			InstanceReadOnly: "OFF",
		}
		mockAPI.On("GetNodeInfo", mock.Anything, "192.168.1.100").Return(externalInfo, nil)

		// 创建验证器
		validator := NewMasterValidator(mockDiscoverer, mockAPI, mockFactory, false)

		// 执行验证
		ctx := context.Background()
		isMaster, err := validator.IsMasterNode(ctx, node)

		// 验证结果
		assert.NoError(t, err)
		assert.True(t, isMaster)

		// 验证mock调用
		mockFactory.AssertExpectations(t)
		mockConn.AssertExpectations(t)
		mockDiscoverer.AssertExpectations(t)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Identify slave node - high confidence", func(t *testing.T) {
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockAPI := &MockExternalAPIClient{}
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}

		node := &NodeConfig{
			Host:     "192.168.1.101",
			Port:     3306,
			Username: "root",
			Password: "password",
		}

		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		
		// 模拟从节点特征
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'super_read_only'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockValidatorRowResult("", uint32(2)))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'log_bin'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'binlog_format'", mock.Anything).Return(mockValidatorRowResult("", "ROW"))
		mockConn.On("Query", "SHOW PROCESSLIST", mock.Anything).Return(nil, nil)

		// 从节点有slave status
		slaveStatus := &SlaveStatus{
			MasterHost:      "192.168.1.100",
			MasterPort:      3306,
			SlaveIORunning:  "Yes",
			SlaveSQLRunning: "Yes",
		}
		mockDiscoverer.On("GetSlaveStatus", mock.Anything, node).Return(slaveStatus, nil)
		mockDiscoverer.On("GetSlaveHosts", mock.Anything, node).Return([]*SlaveHost{}, nil)

		externalInfo := &ExternalNodeInfo{
			ClusterName:      "test-cluster",
			InstanceRole:     "从",
			InstanceReadOnly: "ON",
		}
		mockAPI.On("GetNodeInfo", mock.Anything, "192.168.1.101").Return(externalInfo, nil)

		validator := NewMasterValidator(mockDiscoverer, mockAPI, mockFactory, false)

		ctx := context.Background()
		isMaster, err := validator.IsMasterNode(ctx, node)

		assert.NoError(t, err)
		assert.False(t, isMaster)

		mockFactory.AssertExpectations(t)
		mockConn.AssertExpectations(t)
		mockDiscoverer.AssertExpectations(t)
		mockAPI.AssertExpectations(t)
	})

	t.Run("Validate master safety - block operation on master", func(t *testing.T) {
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockAPI := &MockExternalAPIClient{}
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}

		node := &NodeConfig{
			Host: "192.168.1.100",
			Port: 3306,
		}

		// 设置为主节点
		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'super_read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockValidatorRowResult("", uint32(1)))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'log_bin'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'binlog_format'", mock.Anything).Return(mockValidatorRowResult("", "ROW"))
		mockConn.On("Query", "SHOW PROCESSLIST", mock.Anything).Return(nil, nil)

		mockDiscoverer.On("GetSlaveStatus", mock.Anything, node).Return(nil, nil)
		mockDiscoverer.On("GetSlaveHosts", mock.Anything, node).Return([]*SlaveHost{}, nil)

		externalInfo := &ExternalNodeInfo{InstanceRole: "主"}
		mockAPI.On("GetNodeInfo", mock.Anything, "192.168.1.100").Return(externalInfo, nil)

		validator := NewMasterValidator(mockDiscoverer, mockAPI, mockFactory, false)

		ctx := context.Background()
		err := validator.ValidateMasterSafety(ctx, node)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation blocked: target node identified as master")
	})

	t.Run("Get detailed validation result", func(t *testing.T) {
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockAPI := &MockExternalAPIClient{}
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}

		node := &NodeConfig{
			Host: "192.168.1.100",
			Port: 3306,
		}

		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'super_read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockValidatorRowResult("", uint32(1)))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'log_bin'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'binlog_format'", mock.Anything).Return(mockValidatorRowResult("", "ROW"))
		mockConn.On("Query", "SHOW PROCESSLIST", mock.Anything).Return(nil, nil)

		mockDiscoverer.On("GetSlaveStatus", mock.Anything, node).Return(nil, nil)
		mockDiscoverer.On("GetSlaveHosts", mock.Anything, node).Return([]*SlaveHost{}, nil)

		externalInfo := &ExternalNodeInfo{InstanceRole: "主"}
		mockAPI.On("GetNodeInfo", mock.Anything, "192.168.1.100").Return(externalInfo, nil)

		validator := NewMasterValidator(mockDiscoverer, mockAPI, mockFactory, false)

		ctx := context.Background()
		result, err := validator.GetMasterValidationResult(ctx, node)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsMaster)
		assert.NotEmpty(t, result.ValidationMethods)
		assert.NotEmpty(t, result.Reasons)
		assert.NotNil(t, result.DatabaseStatus)
		assert.False(t, result.DatabaseStatus.ReadOnly)
	})

	t.Run("Strict mode validation", func(t *testing.T) {
		mockDiscoverer := &MockTopologyDiscoverer{}
		mockFactory := &MockConnectionFactory{}
		mockConn := &MockDBConnection{}

		node := &NodeConfig{
			Host: "192.168.1.100",
			Port: 3306,
		}

		// 设置低信心度的结果（只有read_only检查）
		mockFactory.On("CreateConnection", node).Return(mockConn, nil)
		mockConn.On("Close").Return(nil)
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'super_read_only'", mock.Anything).Return(mockValidatorRowResult("", "OFF"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'server_id'", mock.Anything).Return(mockValidatorRowResult("", uint32(1)))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'log_bin'", mock.Anything).Return(mockValidatorRowResult("", "ON"))
		mockConn.On("QueryRow", "SHOW VARIABLES LIKE 'binlog_format'", mock.Anything).Return(mockValidatorRowResult("", "ROW"))
		mockConn.On("Query", "SHOW PROCESSLIST", mock.Anything).Return(nil, nil)

		mockDiscoverer.On("GetSlaveStatus", mock.Anything, node).Return(nil, nil)
		mockDiscoverer.On("GetSlaveHosts", mock.Anything, node).Return([]*SlaveHost{}, nil)

		// 严格模式验证器
		validator := NewMasterValidator(mockDiscoverer, nil, mockFactory, true)

		ctx := context.Background()
		_, err := validator.IsMasterNode(ctx, node)

		// 在严格模式下，信心度不足应该报错
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "confidence too low")
	})
}

// MockTopologyDiscoverer Mock拓扑发现器
type MockTopologyDiscoverer struct {
	mock.Mock
}

func (m *MockTopologyDiscoverer) DiscoverTopology(ctx context.Context, entryPoint *NodeConfig) (*ClusterTopology, error) {
	args := m.Called(ctx, entryPoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ClusterTopology), args.Error(1)
}

func (m *MockTopologyDiscoverer) ValidateNode(ctx context.Context, node *NodeConfig) (*NodeStatus, error) {
	args := m.Called(ctx, node)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*NodeStatus), args.Error(1)
}

func (m *MockTopologyDiscoverer) GetSlaveHosts(ctx context.Context, node *NodeConfig) ([]*SlaveHost, error) {
	args := m.Called(ctx, node)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SlaveHost), args.Error(1)
}

func (m *MockTopologyDiscoverer) GetSlaveStatus(ctx context.Context, node *NodeConfig) (*SlaveStatus, error) {
	args := m.Called(ctx, node)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SlaveStatus), args.Error(1)
}

// mockValidatorRowResult 创建模拟的单行查询结果
func mockValidatorRowResult(column1, column2 interface{}) *MockSingleRow {
	return &MockSingleRow{
		scanValues: []interface{}{column1, column2},
		scanIndex:  0,
	}
}

// MockSingleRow 模拟单行结果
type MockSingleRow struct {
	scanValues []interface{}
	scanIndex  int
}

func (r *MockSingleRow) Scan(dest ...interface{}) error {
	for i, d := range dest {
		if r.scanIndex+i < len(r.scanValues) {
			val := r.scanValues[r.scanIndex+i]
			switch v := d.(type) {
			case *string:
				if s, ok := val.(string); ok {
					*v = s
				}
			case *uint32:
				if u, ok := val.(uint32); ok {
					*v = u
				}
			}
		}
	}
	return nil
}