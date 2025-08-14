package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Database    DatabaseConfig    `yaml:"database" json:"database"`
	Recycler    RecyclerConfig    `yaml:"recycler" json:"recycler"`
	Monitoring  MonitoringConfig  `yaml:"monitoring" json:"monitoring"`
	ExternalAPI ExternalAPIConfig `yaml:"external_api" json:"external_api"`
	Logging     LoggingConfig     `yaml:"logging" json:"logging"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DefaultHost     string        `yaml:"default_host" json:"default_host"`
	DefaultPort     int           `yaml:"default_port" json:"default_port"`
	DefaultUser     string        `yaml:"default_user" json:"default_user"`
	DefaultDatabase string        `yaml:"default_database" json:"default_database"`
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`
	MaxConnections  int           `yaml:"max_connections" json:"max_connections"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

// RecyclerConfig 回收器配置
type RecyclerConfig struct {
	DefaultThreshold        string        `yaml:"default_threshold" json:"default_threshold"`
	DefaultMode             int           `yaml:"default_mode" json:"default_mode"`
	MaxConcurrency         int           `yaml:"max_concurrency" json:"max_concurrency"`
	DefaultConcurrency     int           `yaml:"default_concurrency" json:"default_concurrency"`
	AllowMasterOperations  bool          `yaml:"allow_master_operations" json:"allow_master_operations"`
	SafetyCheckInterval    time.Duration `yaml:"safety_check_interval" json:"safety_check_interval"`
	TableLockTimeout       time.Duration `yaml:"table_lock_timeout" json:"table_lock_timeout"`
	MinTableSize           string        `yaml:"min_table_size" json:"min_table_size"`
	MaxTableSize           string        `yaml:"max_table_size" json:"max_table_size"`
	SupportedEngines       []string      `yaml:"supported_engines" json:"supported_engines"`
	ExcludeSchemas         []string      `yaml:"exclude_schemas" json:"exclude_schemas"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	StatusFile         string        `yaml:"status_file" json:"status_file"`
	PersistInterval    time.Duration `yaml:"persist_interval" json:"persist_interval"`
	MaxHistoryRecords  int           `yaml:"max_history_records" json:"max_history_records"`
	EnableAutoSave     bool          `yaml:"enable_auto_save" json:"enable_auto_save"`
	EnableNotification bool          `yaml:"enable_notification" json:"enable_notification"`
	WebhookURL         string        `yaml:"webhook_url" json:"webhook_url"`
}

// ExternalAPIConfig 外部API配置
type ExternalAPIConfig struct {
	ClusterInfoURL string        `yaml:"cluster_info_url" json:"cluster_info_url"`
	Timeout        time.Duration `yaml:"timeout" json:"timeout"`
	RetryTimes     int           `yaml:"retry_times" json:"retry_times"`
	EnableCache    bool          `yaml:"enable_cache" json:"enable_cache"`
	CacheTimeout   time.Duration `yaml:"cache_timeout" json:"cache_timeout"`
	AuthToken      string        `yaml:"auth_token" json:"auth_token"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	OutputFile string `yaml:"output_file" json:"output_file"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`       // MB
	MaxBackups int    `yaml:"max_backups" json:"max_backups"` // 备份文件数
	MaxAge     int    `yaml:"max_age" json:"max_age"`         // 天数
	Compress   bool   `yaml:"compress" json:"compress"`
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			DefaultHost:     "localhost",
			DefaultPort:     3306,
			DefaultUser:     "root",
			DefaultDatabase: "information_schema",
			Timeout:         30 * time.Second,
			MaxConnections:  10,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
		Recycler: RecyclerConfig{
			DefaultThreshold:       "100MB",
			DefaultMode:            1,
			MaxConcurrency:         10,
			DefaultConcurrency:     3,
			AllowMasterOperations:  false,
			SafetyCheckInterval:    30 * time.Second,
			TableLockTimeout:       5 * time.Minute,
			MinTableSize:           "10MB",
			MaxTableSize:           "",
			SupportedEngines:       []string{"InnoDB"},
			ExcludeSchemas:        []string{"information_schema", "performance_schema", "mysql", "sys"},
		},
		Monitoring: MonitoringConfig{
			StatusFile:         "/tmp/mysql-recycler-status.json",
			PersistInterval:    5 * time.Minute,
			MaxHistoryRecords:  1000,
			EnableAutoSave:     true,
			EnableNotification: false,
			WebhookURL:         "",
		},
		ExternalAPI: ExternalAPIConfig{
			ClusterInfoURL: "",
			Timeout:        10 * time.Second,
			RetryTimes:     3,
			EnableCache:    true,
			CacheTimeout:   5 * time.Minute,
			AuthToken:      "",
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			OutputFile: "",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(configFile string) (*Config, error) {
	config := GetDefaultConfig()

	if configFile == "" {
		return config, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return config, nil // 使用默认配置
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv(config *Config) {
	if host := os.Getenv("MYSQL_RECYCLER_HOST"); host != "" {
		config.Database.DefaultHost = host
	}
	if port := os.Getenv("MYSQL_RECYCLER_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			config.Database.DefaultPort = p
		}
	}
	if user := os.Getenv("MYSQL_RECYCLER_USER"); user != "" {
		config.Database.DefaultUser = user
	}
	if database := os.Getenv("MYSQL_RECYCLER_DATABASE"); database != "" {
		config.Database.DefaultDatabase = database
	}
	if threshold := os.Getenv("MYSQL_RECYCLER_THRESHOLD"); threshold != "" {
		config.Recycler.DefaultThreshold = threshold
	}
	if concurrency := os.Getenv("MYSQL_RECYCLER_CONCURRENCY"); concurrency != "" {
		if c, err := parseIntValue(concurrency); err == nil {
			config.Recycler.DefaultConcurrency = c
		}
	}
	if logLevel := os.Getenv("MYSQL_RECYCLER_LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("MYSQL_RECYCLER_LOG_FORMAT"); logFormat != "" {
		config.Logging.Format = logFormat
	}
	if statusFile := os.Getenv("MYSQL_RECYCLER_STATUS_FILE"); statusFile != "" {
		config.Monitoring.StatusFile = statusFile
	}
	if webhookURL := os.Getenv("MYSQL_RECYCLER_WEBHOOK_URL"); webhookURL != "" {
		config.Monitoring.WebhookURL = webhookURL
		config.Monitoring.EnableNotification = true
	}
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, configFile string) error {
	// 确保目录存在
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	// 验证数据库配置
	if c.Database.DefaultPort <= 0 || c.Database.DefaultPort > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.DefaultPort)
	}
	
	if c.Database.Timeout <= 0 {
		return fmt.Errorf("database timeout must be positive")
	}

	// 验证回收器配置
	if c.Recycler.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive")
	}
	
	if c.Recycler.DefaultConcurrency <= 0 {
		return fmt.Errorf("default concurrency must be positive")
	}
	
	if c.Recycler.DefaultConcurrency > c.Recycler.MaxConcurrency {
		return fmt.Errorf("default concurrency cannot exceed max concurrency")
	}

	// 验证监控配置
	if c.Monitoring.MaxHistoryRecords <= 0 {
		return fmt.Errorf("max history records must be positive")
	}

	// 验证日志级别
	validLogLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, level := range validLogLevels {
		if c.Logging.Level == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	// 验证日志格式
	if c.Logging.Format != "text" && c.Logging.Format != "json" {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

// GetConfigSearchPaths 获取配置文件搜索路径
func GetConfigSearchPaths() []string {
	paths := []string{}

	// 当前目录
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "mysql-recycler.yaml"))
		paths = append(paths, filepath.Join(cwd, "mysql-recycler.yml"))
		paths = append(paths, filepath.Join(cwd, ".mysql-recycler.yaml"))
		paths = append(paths, filepath.Join(cwd, ".mysql-recycler.yml"))
	}

	// 用户主目录
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".mysql-recycler.yaml"))
		paths = append(paths, filepath.Join(home, ".mysql-recycler.yml"))
		paths = append(paths, filepath.Join(home, ".config", "mysql-recycler", "config.yaml"))
		paths = append(paths, filepath.Join(home, ".config", "mysql-recycler", "config.yml"))
	}

	// 系统配置目录
	paths = append(paths, "/etc/mysql-recycler/config.yaml")
	paths = append(paths, "/etc/mysql-recycler/config.yml")

	return paths
}

// FindConfigFile 查找配置文件
func FindConfigFile() string {
	for _, path := range GetConfigSearchPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// GenerateExampleConfig 生成示例配置文件
func GenerateExampleConfig() string {
	return `# MySQL表空间回收工具配置文件示例

# 数据库连接配置
database:
  default_host: "localhost"
  default_port: 3306
  default_user: "root"
  default_database: "information_schema"
  timeout: "30s"
  max_connections: 10
  conn_max_lifetime: "5m"
  conn_max_idle_time: "1m"

# 回收器配置
recycler:
  default_threshold: "100MB"        # 默认碎片回收阈值
  default_mode: 1                   # 1=独立节点模式, 2=复制同步模式
  max_concurrency: 10               # 最大并发数
  default_concurrency: 3            # 默认并发数
  allow_master_operations: false    # 是否允许在主节点执行操作
  safety_check_interval: "30s"      # 安全检查间隔
  table_lock_timeout: "5m"          # 表锁超时时间
  min_table_size: "10MB"            # 最小表大小
  max_table_size: ""                # 最大表大小（空表示无限制）
  supported_engines:                # 支持的存储引擎
    - "InnoDB"
  exclude_schemas:                  # 默认排除的数据库
    - "information_schema"
    - "performance_schema"
    - "mysql"
    - "sys"

# 监控配置
monitoring:
  status_file: "/tmp/mysql-recycler-status.json"
  persist_interval: "5m"
  max_history_records: 1000
  enable_auto_save: true
  enable_notification: false
  webhook_url: ""                   # Webhook通知URL

# 外部API配置
external_api:
  cluster_info_url: ""              # 集群信息API URL
  timeout: "10s"
  retry_times: 3
  enable_cache: true
  cache_timeout: "5m"
  auth_token: ""                    # 认证令牌

# 日志配置
logging:
  level: "info"                     # debug, info, warn, error
  format: "text"                    # text, json
  output_file: ""                   # 空表示输出到控制台
  max_size: 100                     # 日志文件最大大小(MB)
  max_backups: 3                    # 保留的备份文件数
  max_age: 7                        # 日志文件最大保留天数
  compress: true                    # 是否压缩旧日志文件
`
}

// 辅助函数

func parsePort(portStr string) (int, error) {
	port := 0
	_, err := fmt.Sscanf(portStr, "%d", &port)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", portStr)
	}
	return port, nil
}

func parseIntValue(valueStr string) (int, error) {
	value := 0
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid integer value: %s", valueStr)
	}
	return value, nil
}