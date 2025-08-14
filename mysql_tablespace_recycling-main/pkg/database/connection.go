package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mysql_tablespace_recycling/pkg/config"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// Instance MySQL实例信息
type Instance struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// String 返回实例的字符串表示
func (i *Instance) String() string {
	return fmt.Sprintf("%s:%d", i.Host, i.Port)
}

// DSN 生成数据库连接字符串
func (i *Instance) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=true",
		i.User, i.Password, i.Host, i.Port, i.Database, "10s", "30s", "30s")
}

// Connection MySQL连接管理器
type Connection struct {
	instance *Instance
	db       *sql.DB
	logger   *zap.Logger
	config   *config.DatabaseConfig
}

// NewConnection 创建新的数据库连接
func NewConnection(instance *Instance, cfg *config.DatabaseConfig, logger *zap.Logger) (*Connection, error) {
	db, err := sql.Open("mysql", instance.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// 配置连接池
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	conn := &Connection{
		instance: instance,
		db:       db,
		logger:   logger,
		config:   cfg,
	}

	logger.Info("Database connection established", 
		zap.String("instance", instance.String()))

	return conn, nil
}

// Close 关闭数据库连接
func (c *Connection) Close() error {
	if c.db != nil {
		c.logger.Info("Closing database connection", 
			zap.String("instance", c.instance.String()))
		return c.db.Close()
	}
	return nil
}

// GetDB 获取底层数据库连接
func (c *Connection) GetDB() *sql.DB {
	return c.db
}

// GetInstance 获取实例信息
func (c *Connection) GetInstance() *Instance {
	return c.instance
}

// QueryContext 执行查询语句
func (c *Connection) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	defer func() {
		c.logger.Debug("Query executed",
			zap.String("instance", c.instance.String()),
			zap.String("query", query),
			zap.Duration("duration", time.Since(start)))
	}()

	queryCtx, cancel := context.WithTimeout(ctx, c.config.QueryTimeout)
	defer cancel()

	return c.db.QueryContext(queryCtx, query, args...)
}

// QueryRowContext 执行查询单行语句
func (c *Connection) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	defer func() {
		c.logger.Debug("QueryRow executed",
			zap.String("instance", c.instance.String()),
			zap.String("query", query),
			zap.Duration("duration", time.Since(start)))
	}()

	queryCtx, cancel := context.WithTimeout(ctx, c.config.QueryTimeout)
	defer cancel()

	return c.db.QueryRowContext(queryCtx, query, args...)
}

// ExecContext 执行非查询语句
func (c *Connection) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	defer func() {
		c.logger.Debug("Exec executed",
			zap.String("instance", c.instance.String()),
			zap.String("query", query),
			zap.Duration("duration", time.Since(start)))
	}()

	execCtx, cancel := context.WithTimeout(ctx, c.config.QueryTimeout)
	defer cancel()

	return c.db.ExecContext(execCtx, query, args...)
}

// TableInfo 表信息结构
type TableInfo struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	DataFree   int64  `json:"data_free"`   // 碎片大小
	DataLength int64  `json:"data_length"` // 数据大小
	Engine     string `json:"engine"`      // 存储引擎
}

// GetTableFragmentInfo 获取表碎片信息
func (c *Connection) GetTableFragmentInfo(ctx context.Context, schema string) ([]TableInfo, error) {
	query := `
		SELECT 
			TABLE_SCHEMA,
			TABLE_NAME,
			COALESCE(DATA_FREE, 0) as DATA_FREE,
			COALESCE(DATA_LENGTH, 0) as DATA_LENGTH,
			ENGINE
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? 
			AND ENGINE = 'InnoDB' 
			AND TABLE_TYPE = 'BASE TABLE'
			AND DATA_FREE > 0
		ORDER BY DATA_FREE DESC`

	rows, err := c.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query table fragment info: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		err := rows.Scan(&table.Schema, &table.Table, &table.DataFree, &table.DataLength, &table.Engine)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tables, nil
}

// IsReadOnly 检查实例是否为只读
func (c *Connection) IsReadOnly(ctx context.Context) (bool, error) {
	var readOnly string
	query := "SELECT @@read_only"
	
	err := c.QueryRowContext(ctx, query).Scan(&readOnly)
	if err != nil {
		return false, fmt.Errorf("failed to check read_only status: %w", err)
	}

	return readOnly == "1" || readOnly == "ON", nil
}

// GetSlaveHosts 获取从库列表
func (c *Connection) GetSlaveHosts(ctx context.Context) ([]map[string]interface{}, error) {
	query := "SHOW SLAVE HOSTS"
	
	rows, err := c.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to show slave hosts: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var slaves []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan slave host: %w", err)
		}

		slave := make(map[string]interface{})
		for i, col := range columns {
			slave[col] = values[i]
		}
		slaves = append(slaves, slave)
	}

	return slaves, nil
}

// GetSlaveStatus 获取从库状态
func (c *Connection) GetSlaveStatus(ctx context.Context) (map[string]interface{}, error) {
	query := "SHOW SLAVE STATUS"
	
	rows, err := c.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to show slave status: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // 没有从库状态，说明不是从库
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan slave status: %w", err)
	}

	status := make(map[string]interface{})
	for i, col := range columns {
		status[col] = values[i]
	}

	return status, nil
}