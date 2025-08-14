package config

import (
	"time"
)

// Config 表空间回收工具配置
type Config struct {
	// 数据库连接配置
	Database DatabaseConfig `yaml:"database" json:"database"`
	
	// 回收配置
	Recovery RecoveryConfig `yaml:"recovery" json:"recovery"`
	
	// 监控配置
	Monitor MonitorConfig `yaml:"monitor" json:"monitor"`
	
	// 外部API配置
	ExternalAPI ExternalAPIConfig `yaml:"external_api" json:"external_api"`
	
	// 日志配置
	Log LogConfig `yaml:"log" json:"log"`
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	// 连接超时时间
	ConnTimeout time.Duration `yaml:"conn_timeout" json:"conn_timeout" default:"10s"`
	
	// 查询超时时间
	QueryTimeout time.Duration `yaml:"query_timeout" json:"query_timeout" default:"30s"`
	
	// 最大空闲连接数
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns" default:"10"`
	
	// 最大打开连接数
	MaxOpenConns int `yaml:"max_open_conns" json:"max_open_conns" default:"100"`
	
	// 连接最大生命周期
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime" default:"3600s"`
}

// RecoveryConfig 表空间回收配置
type RecoveryConfig struct {
	// 是否允许在主节点执行回收
	AllowOnMaster bool `yaml:"allow_on_master" json:"allow_on_master" default:"false"`
	
	// 碎片阈值（字节）
	FragmentThreshold int64 `yaml:"fragment_threshold" json:"fragment_threshold" default:"104857600"` // 100MB
	
	// 工作模式 1: 独立模式, 2: 复制模式
	WorkMode int `yaml:"work_mode" json:"work_mode" default:"1"`
	
	// 并发度
	Concurrency int `yaml:"concurrency" json:"concurrency" default:"5"`
	
	// 单次处理表的数量限制
	BatchSize int `yaml:"batch_size" json:"batch_size" default:"10"`
	
	// 处理间隔时间
	ProcessInterval time.Duration `yaml:"process_interval" json:"process_interval" default:"1s"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	// 状态持久化间隔
	PersistInterval time.Duration `yaml:"persist_interval" json:"persist_interval" default:"30s"`
	
	// 状态文件路径
	StatusFile string `yaml:"status_file" json:"status_file" default:"./status.json"`
	
	// 启用HTTP监控服务
	EnableHTTP bool `yaml:"enable_http" json:"enable_http" default:"true"`
	
	// HTTP监控端口
	HTTPPort int `yaml:"http_port" json:"http_port" default:"8080"`
}

// ExternalAPIConfig 外部API配置
type ExternalAPIConfig struct {
	// API base URL
	BaseURL string `yaml:"base_url" json:"base_url"`
	
	// 请求超时时间
	Timeout time.Duration `yaml:"timeout" json:"timeout" default:"10s"`
	
	// 重试次数
	RetryCount int `yaml:"retry_count" json:"retry_count" default:"3"`
}

// LogConfig 日志配置
type LogConfig struct {
	// 日志级别
	Level string `yaml:"level" json:"level" default:"info"`
	
	// 日志输出路径
	Output string `yaml:"output" json:"output" default:"stdout"`
	
	// 是否输出到文件
	ToFile bool `yaml:"to_file" json:"to_file" default:"false"`
	
	// 日志文件路径
	FilePath string `yaml:"file_path" json:"file_path" default:"./logs/app.log"`
}