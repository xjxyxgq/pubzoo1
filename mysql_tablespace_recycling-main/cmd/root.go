package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mysql_tablespace_recycling/cluster"
	"mysql_tablespace_recycling/config"
	"mysql_tablespace_recycling/pkg/logger"
	"mysql_tablespace_recycling/recycler"

	"github.com/spf13/cobra"
)

// RootCmd 根命令
var RootCmd = &cobra.Command{
	Use:   "mysql-recycler",
	Short: "MySQL表空间碎片回收工具",
	Long: `MySQL表空间碎片回收工具
一个基于测试驱动开发的高性能MySQL表空间碎片回收工具，支持集群拓扑感知和安全的并发操作。`,
	PersistentPreRun: initializeLogger,
}

var appConfig *config.Config

// 全局选项
var (
	cfgFile     string
	logLevel    string
	logFormat   string
	verbose     bool
)

// 数据库连接选项
var (
	dbHost     string
	dbPort     int
	dbUser     string
	dbPassword string
	dbDatabase string
	dbTimeout  string
)

// 超时配置选项
var (
	connectTimeout   string  // 数据库连接超时
	queryTimeout     string  // 查询超时
	alterTimeout     string  // ALTER TABLE操作超时
	discoveryTimeout string  // 集群发现超时
)

func init() {
	// 全局标志
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认是 $HOME/.mysql-recycler.yaml)")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "日志级别 (debug, info, warn, error)")
	RootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "日志格式 (text, json)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")

	// 数据库连接标志
	RootCmd.PersistentFlags().StringVarP(&dbHost, "host", "H", "localhost", "MySQL主机地址")
	RootCmd.PersistentFlags().IntVarP(&dbPort, "port", "P", 3306, "MySQL端口")
	RootCmd.PersistentFlags().StringVarP(&dbUser, "user", "u", "root", "MySQL用户名")
	RootCmd.PersistentFlags().StringVarP(&dbPassword, "password", "p", "", "MySQL密码")
	RootCmd.PersistentFlags().StringVarP(&dbDatabase, "database", "d", "information_schema", "数据库名")
	RootCmd.PersistentFlags().StringVar(&dbTimeout, "timeout", "30s", "连接超时时间")

	// 超时配置标志
	RootCmd.PersistentFlags().StringVar(&connectTimeout, "connect-timeout", "30s", "数据库连接超时时间")
	RootCmd.PersistentFlags().StringVar(&queryTimeout, "query-timeout", "300s", "普通查询超时时间")
	RootCmd.PersistentFlags().StringVar(&alterTimeout, "alter-timeout", "3600s", "ALTER TABLE操作超时时间")
	RootCmd.PersistentFlags().StringVar(&discoveryTimeout, "discovery-timeout", "120s", "集群发现超时时间")

	// 添加子命令
	RootCmd.AddCommand(analyzeCmd)
	RootCmd.AddCommand(topologyCmd)
	RootCmd.AddCommand(validateCmd)
}

// initializeLogger 初始化日志系统
func initializeLogger(cmd *cobra.Command, args []string) {
	// 加载配置文件
	var err error
	if cfgFile == "" {
		cfgFile = config.FindConfigFile()
	}
	appConfig, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		appConfig = config.GetDefaultConfig()
	}
	
	// 从环境变量加载配置
	config.LoadConfigFromEnv(appConfig)
	
	// 初始化日志系统
	logLevelToUse := logLevel
	if verbose {
		logLevelToUse = "debug"
	}
	
	// 如果有配置文件，使用配置文件中的日志设置
	if appConfig != nil && logLevel == "info" { // 只有在用户没有明确指定时才使用配置文件
		logLevelToUse = appConfig.Logging.Level
	}
	
	var output *os.File
	if appConfig != nil && appConfig.Logging.OutputFile != "" {
		output, err = os.OpenFile(appConfig.Logging.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open log file %s: %v\n", appConfig.Logging.OutputFile, err)
			output = nil
		}
	}
	
	if output == nil {
		output = os.Stdout
	}
	
	globalLogger := logger.NewLoggerFromString(logLevelToUse, output)
	logger.SetGlobalLogger(globalLogger)
	
	if verbose || logLevelToUse == "debug" {
		logger.Debug("日志系统已初始化，级别: %s", logLevelToUse)
	}
}

// Execute 执行命令
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Connect   time.Duration // 连接超时
	Query     time.Duration // 查询超时
	Alter     time.Duration // ALTER TABLE超时
	Discovery time.Duration // 集群发现超时
}

// createTimeoutConfig 创建超时配置
func createTimeoutConfig() (*TimeoutConfig, error) {
	connectTimeout, err := time.ParseDuration(connectTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid connect-timeout format: %w", err)
	}

	queryTimeout, err := time.ParseDuration(queryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid query-timeout format: %w", err)
	}

	alterTimeout, err := time.ParseDuration(alterTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid alter-timeout format: %w", err)
	}

	discoveryTimeout, err := time.ParseDuration(discoveryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid discovery-timeout format: %w", err)
	}

	return &TimeoutConfig{
		Connect:   connectTimeout,
		Query:     queryTimeout,
		Alter:     alterTimeout,
		Discovery: discoveryTimeout,
	}, nil
}

// createNodeConfig 创建节点配置
func createNodeConfig() (*NodeConfig, error) {
	timeout, err := time.ParseDuration(dbTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}

	return &NodeConfig{
		Host:     dbHost,
		Port:     dbPort,
		Username: dbUser,
		Password: dbPassword,
		Database: dbDatabase,
		Timeout:  timeout,
	}, nil
}

// NodeConfig 适配器，实现recycler.NodeConfigInterface
type NodeConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	Timeout  time.Duration
}

func (n *NodeConfig) GetHost() string           { return n.Host }
func (n *NodeConfig) GetPort() int              { return n.Port }
func (n *NodeConfig) GetUsername() string       { return n.Username }
func (n *NodeConfig) GetPassword() string       { return n.Password }
func (n *NodeConfig) GetDatabase() string       { return n.Database }
func (n *NodeConfig) GetTimeout() time.Duration { return n.Timeout }
func (n *NodeConfig) NodeKey() string           { return fmt.Sprintf("%s:%d", n.Host, n.Port) }
func (n *NodeConfig) IsValid() bool            { return n.Host != "" && n.Port > 0 && n.Username != "" }

// toClusterNodeConfig 转换为cluster包的NodeConfig
func (n *NodeConfig) toClusterNodeConfig() *cluster.NodeConfig {
	return &cluster.NodeConfig{
		Host:     n.Host,
		Port:     n.Port,
		Username: n.Username,
		Password: n.Password,
		Database: n.Database,
		Timeout:  n.Timeout,
	}
}

// ConnectionFactory 连接工厂适配器
type ConnectionFactory struct{}

func (f *ConnectionFactory) CreateConnection(config recycler.NodeConfigInterface) (recycler.DBConnection, error) {
	nodeConfig := &cluster.NodeConfig{
		Host:     config.GetHost(),
		Port:     config.GetPort(),
		Username: config.GetUsername(),
		Password: config.GetPassword(),
		Database: config.GetDatabase(),
		Timeout:  config.GetTimeout(),
	}
	
	return cluster.NewMySQLConnection(nodeConfig)
}

// printJSON 以JSON格式输出
func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	}
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f分钟", d.Minutes())
	} else {
		return fmt.Sprintf("%.1f小时", d.Hours())
	}
}