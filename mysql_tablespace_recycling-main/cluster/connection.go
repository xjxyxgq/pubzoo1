package cluster

import (
	"fmt"
	"time"

	"mysql_tablespace_recycling/pkg/database"
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
)

// DBConnection 数据库连接接口 - 继承通用接口
type DBConnection = database.DBConnection

// MySQLConnection MySQL连接实现
type MySQLConnection struct {
	db *sql.DB
}

// NewMySQLConnection 创建MySQL连接
func NewMySQLConnection(node *NodeConfig) (*MySQLConnection, error) {
	if !node.IsValid() {
		return nil, fmt.Errorf("invalid node config: %+v", node)
	}

	// 确定要使用的超时值
	connectTimeout := node.Timeout  // 默认连接超时
	readTimeout := node.Timeout    // 默认读超时
	writeTimeout := node.Timeout   // 默认写超时
	
	// 如果设置了具体的超时配置，则使用它们
	if node.ConnectTimeout > 0 {
		connectTimeout = node.ConnectTimeout
	}
	if node.QueryTimeout > 0 {
		readTimeout = node.QueryTimeout
		writeTimeout = node.QueryTimeout
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=true",
		node.Username,
		node.Password,
		node.Host,
		node.Port,
		node.Database,
		connectTimeout.String(),
		readTimeout.String(),
		writeTimeout.String(),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to %s:%d: %w", node.Host, node.Port, err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	conn := &MySQLConnection{db: db}

	// 测试连接
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping %s:%d: %w", node.Host, node.Port, err)
	}

	return conn, nil
}

// Query 执行查询
func (c *MySQLConnection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}

// QueryRow 执行单行查询
func (c *MySQLConnection) QueryRow(query string, args ...interface{}) database.RowScanner {
	return c.db.QueryRow(query, args...)
}

// Close 关闭连接
func (c *MySQLConnection) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping 测试连接
func (c *MySQLConnection) Ping() error {
	return c.db.Ping()
}

// ConnectionFactory 连接工厂接口 - 使用通用接口
type ConnectionFactory = database.ConnectionFactory

// DefaultConnectionFactory 默认连接工厂
type DefaultConnectionFactory struct{}

// CreateConnection 创建数据库连接
func (f *DefaultConnectionFactory) CreateConnection(config database.NodeConfigInterface) (database.DBConnection, error) {
	// 转换为cluster的NodeConfig类型
	nodeConfig, ok := config.(*NodeConfig)
	if !ok {
		return nil, fmt.Errorf("config must be of type *NodeConfig")
	}
	return NewMySQLConnection(nodeConfig)
}