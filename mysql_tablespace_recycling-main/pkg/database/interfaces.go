package database

import (
	"database/sql"
	"time"
)

// RowScanner 行扫描器接口
type RowScanner interface {
	Scan(dest ...interface{}) error
}

// DBConnection 数据库连接接口
type DBConnection interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) RowScanner
	Close() error
	Ping() error
}

// ConnectionFactory 连接工厂接口
type ConnectionFactory interface {
	CreateConnection(config NodeConfigInterface) (DBConnection, error)
}

// NodeConfigInterface 节点配置接口
type NodeConfigInterface interface {
	NodeKey() string
	GetHost() string
	GetPort() int
	GetUsername() string
	GetPassword() string
	GetDatabase() string
	GetTimeout() time.Duration
	IsValid() bool
}