package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestInstance tests the Instance struct
func TestInstance(t *testing.T) {
	t.Run("Instance string representation", func(t *testing.T) {
		instance := &Instance{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "password",
			Database: "test_db",
		}
		
		expected := "localhost:3306"
		assert.Equal(t, expected, instance.String())
	})
	
	t.Run("Generate DSN", func(t *testing.T) {
		instance := &Instance{
			Host:     "database.example.com",
			Port:     3306,
			User:     "user",
			Password: "pass",
			Database: "mydb",
		}
		
		dsn := instance.DSN()
		
		// 验证DSN包含必要的组件
		assert.Contains(t, dsn, "user:pass")
		assert.Contains(t, dsn, "tcp(database.example.com:3306)")
		assert.Contains(t, dsn, "/mydb")
		assert.Contains(t, dsn, "timeout=10s")
		assert.Contains(t, dsn, "readTimeout=30s")
		assert.Contains(t, dsn, "writeTimeout=30s")
		assert.Contains(t, dsn, "parseTime=true")
	})
	
	t.Run("DSN with special characters", func(t *testing.T) {
		instance := &Instance{
			Host:     "localhost",
			Port:     3306,
			User:     "user@domain",
			Password: "p@ssw0rd!",
			Database: "test-db",
		}
		
		dsn := instance.DSN()
		
		// DSN应该包含特殊字符（在实际使用中可能需要URL编码）
		assert.Contains(t, dsn, "user@domain:p@ssw0rd!")
		assert.Contains(t, dsn, "/test-db")
	})
}

// TestTableInfo tests the TableInfo struct
func TestTableInfo(t *testing.T) {
	t.Run("TableInfo structure", func(t *testing.T) {
		table := TableInfo{
			Schema:     "test_schema",
			Table:      "test_table",
			DataFree:   1024 * 1024 * 100, // 100MB
			DataLength: 1024 * 1024 * 500, // 500MB
			Engine:     "InnoDB",
		}
		
		assert.Equal(t, "test_schema", table.Schema)
		assert.Equal(t, "test_table", table.Table)
		assert.Equal(t, int64(1024*1024*100), table.DataFree)
		assert.Equal(t, int64(1024*1024*500), table.DataLength)
		assert.Equal(t, "InnoDB", table.Engine)
	})
}

// TestNodeConfigInterface tests node config interface methods
func TestNodeConfigInterface(t *testing.T) {
	t.Run("Mock node config implementation", func(t *testing.T) {
		mockConfig := &MockNodeConfig{
			host:     "localhost",
			port:     3306,
			username: "root",
			password: "password",
			database: "test_db",
			timeout:  30 * time.Second,
		}
		
		assert.Equal(t, "localhost:3306", mockConfig.NodeKey())
		assert.Equal(t, "localhost", mockConfig.GetHost())
		assert.Equal(t, 3306, mockConfig.GetPort())
		assert.Equal(t, "root", mockConfig.GetUsername())
		assert.Equal(t, "password", mockConfig.GetPassword())
		assert.Equal(t, "test_db", mockConfig.GetDatabase())
		assert.Equal(t, 30*time.Second, mockConfig.GetTimeout())
		assert.True(t, mockConfig.IsValid())
	})
	
	t.Run("Invalid node config", func(t *testing.T) {
		invalidConfigs := []*MockNodeConfig{
			{host: "", port: 3306, username: "root"}, // 空主机名
			{host: "localhost", port: 0, username: "root"}, // 端口为0
			{host: "localhost", port: 3306, username: ""}, // 空用户名
		}
		
		for i, config := range invalidConfigs {
			assert.False(t, config.IsValid(), "Config %d should be invalid", i)
		}
	})
}

// MockNodeConfig 实现NodeConfigInterface接口用于测试
type MockNodeConfig struct {
	host     string
	port     int
	username string
	password string
	database string
	timeout  time.Duration
}

func (m *MockNodeConfig) NodeKey() string {
	return fmt.Sprintf("%s:%d", m.host, m.port)
}

func (m *MockNodeConfig) GetHost() string {
	return m.host
}

func (m *MockNodeConfig) GetPort() int {
	return m.port
}

func (m *MockNodeConfig) GetUsername() string {
	return m.username
}

func (m *MockNodeConfig) GetPassword() string {
	return m.password
}

func (m *MockNodeConfig) GetDatabase() string {
	return m.database
}

func (m *MockNodeConfig) GetTimeout() time.Duration {
	return m.timeout
}

func (m *MockNodeConfig) IsValid() bool {
	return m.host != "" && m.port > 0 && m.username != ""
}