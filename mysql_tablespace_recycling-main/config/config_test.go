package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultConfig(t *testing.T) {
	t.Run("Get default configuration", func(t *testing.T) {
		config := GetDefaultConfig()
		
		assert.NotNil(t, config)
		
		// 验证数据库默认配置
		assert.Equal(t, "localhost", config.Database.DefaultHost)
		assert.Equal(t, 3306, config.Database.DefaultPort)
		assert.Equal(t, "root", config.Database.DefaultUser)
		assert.Equal(t, "information_schema", config.Database.DefaultDatabase)
		assert.Equal(t, 30*time.Second, config.Database.Timeout)
		assert.Equal(t, 10, config.Database.MaxConnections)
		
		// 验证回收器默认配置
		assert.Equal(t, "100MB", config.Recycler.DefaultThreshold)
		assert.Equal(t, 1, config.Recycler.DefaultMode)
		assert.Equal(t, 10, config.Recycler.MaxConcurrency)
		assert.Equal(t, 3, config.Recycler.DefaultConcurrency)
		assert.False(t, config.Recycler.AllowMasterOperations)
		assert.Contains(t, config.Recycler.SupportedEngines, "InnoDB")
		assert.Contains(t, config.Recycler.ExcludeSchemas, "information_schema")
		
		// 验证监控默认配置
		assert.Equal(t, "/tmp/mysql-recycler-status.json", config.Monitoring.StatusFile)
		assert.Equal(t, 5*time.Minute, config.Monitoring.PersistInterval)
		assert.Equal(t, 1000, config.Monitoring.MaxHistoryRecords)
		assert.True(t, config.Monitoring.EnableAutoSave)
		assert.False(t, config.Monitoring.EnableNotification)
		
		// 验证外部API默认配置
		assert.Empty(t, config.ExternalAPI.ClusterInfoURL)
		assert.Equal(t, 10*time.Second, config.ExternalAPI.Timeout)
		assert.Equal(t, 3, config.ExternalAPI.RetryTimes)
		assert.True(t, config.ExternalAPI.EnableCache)
		
		// 验证日志默认配置
		assert.Equal(t, "info", config.Logging.Level)
		assert.Equal(t, "text", config.Logging.Format)
		assert.Empty(t, config.Logging.OutputFile)
		assert.Equal(t, 100, config.Logging.MaxSize)
		assert.Equal(t, 3, config.Logging.MaxBackups)
		assert.Equal(t, 7, config.Logging.MaxAge)
		assert.True(t, config.Logging.Compress)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid default configuration", func(t *testing.T) {
		config := GetDefaultConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})
	
	t.Run("Invalid database port", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Database.DefaultPort = -1
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid database port")
	})
	
	t.Run("Invalid database port - too high", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Database.DefaultPort = 99999
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid database port")
	})
	
	t.Run("Invalid database timeout", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Database.Timeout = -1 * time.Second
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database timeout must be positive")
	})
	
	t.Run("Invalid max concurrency", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Recycler.MaxConcurrency = 0
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max concurrency must be positive")
	})
	
	t.Run("Invalid default concurrency", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Recycler.DefaultConcurrency = 0
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "default concurrency must be positive")
	})
	
	t.Run("Default concurrency exceeds max", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Recycler.DefaultConcurrency = 15
		config.Recycler.MaxConcurrency = 10
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "default concurrency cannot exceed max concurrency")
	})
	
	t.Run("Invalid max history records", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Monitoring.MaxHistoryRecords = 0
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max history records must be positive")
	})
	
	t.Run("Invalid log level", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Logging.Level = "invalid"
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log level")
	})
	
	t.Run("Invalid log format", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Logging.Format = "invalid"
		
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log format")
	})
	
	t.Run("Valid log levels", func(t *testing.T) {
		config := GetDefaultConfig()
		validLevels := []string{"debug", "info", "warn", "error"}
		
		for _, level := range validLevels {
			config.Logging.Level = level
			err := config.Validate()
			assert.NoError(t, err, "Level %s should be valid", level)
		}
	})
	
	t.Run("Valid log formats", func(t *testing.T) {
		config := GetDefaultConfig()
		validFormats := []string{"text", "json"}
		
		for _, format := range validFormats {
			config.Logging.Format = format
			err := config.Validate()
			assert.NoError(t, err, "Format %s should be valid", format)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("Load config with empty file path", func(t *testing.T) {
		config, err := LoadConfig("")
		assert.NoError(t, err)
		assert.NotNil(t, config)
		
		// 应该返回默认配置
		defaultConfig := GetDefaultConfig()
		assert.Equal(t, defaultConfig.Database.DefaultHost, config.Database.DefaultHost)
	})
	
	t.Run("Load config with non-existent file", func(t *testing.T) {
		config, err := LoadConfig("/non/existent/file.yaml")
		assert.NoError(t, err)
		assert.NotNil(t, config)
		
		// 应该返回默认配置
		defaultConfig := GetDefaultConfig()
		assert.Equal(t, defaultConfig.Database.DefaultHost, config.Database.DefaultHost)
	})
	
	t.Run("Load valid YAML config", func(t *testing.T) {
		// 创建临时配置文件
		tempFile, err := os.CreateTemp("", "config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		yamlContent := `
database:
  default_host: "custom-host"
  default_port: 3307
recycler:
  default_threshold: "200MB"
  max_concurrency: 20
logging:
  level: "debug"
`
		
		_, err = tempFile.WriteString(yamlContent)
		require.NoError(t, err)
		tempFile.Close()
		
		config, err := LoadConfig(tempFile.Name())
		assert.NoError(t, err)
		assert.NotNil(t, config)
		
		// 验证自定义值被加载
		assert.Equal(t, "custom-host", config.Database.DefaultHost)
		assert.Equal(t, 3307, config.Database.DefaultPort)
		assert.Equal(t, "200MB", config.Recycler.DefaultThreshold)
		assert.Equal(t, 20, config.Recycler.MaxConcurrency)
		assert.Equal(t, "debug", config.Logging.Level)
		
		// 验证未设置的值使用默认值
		assert.Equal(t, "root", config.Database.DefaultUser)
	})
	
	t.Run("Load invalid YAML config", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		// 写入无效的YAML
		invalidYAML := `
database:
  default_host: "test"
  invalid_yaml: [
`
		
		_, err = tempFile.WriteString(invalidYAML)
		require.NoError(t, err)
		tempFile.Close()
		
		config, err := LoadConfig(tempFile.Name())
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})
	
	t.Run("Load config with validation error", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		// 写入导致验证错误的配置
		invalidConfig := `
database:
  default_port: -1
`
		
		_, err = tempFile.WriteString(invalidConfig)
		require.NoError(t, err)
		tempFile.Close()
		
		config, err := LoadConfig(tempFile.Name())
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid configuration")
	})
}

func TestSaveConfig(t *testing.T) {
	t.Run("Save config to file", func(t *testing.T) {
		config := GetDefaultConfig()
		config.Database.DefaultHost = "test-host"
		config.Recycler.DefaultThreshold = "500MB"
		
		tempDir, err := os.MkdirTemp("", "config-test-")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		configFile := filepath.Join(tempDir, "test-config.yaml")
		
		err = SaveConfig(config, configFile)
		assert.NoError(t, err)
		
		// 验证文件存在
		assert.FileExists(t, configFile)
		
		// 验证文件内容可以重新加载
		loadedConfig, err := LoadConfig(configFile)
		assert.NoError(t, err)
		assert.Equal(t, "test-host", loadedConfig.Database.DefaultHost)
		assert.Equal(t, "500MB", loadedConfig.Recycler.DefaultThreshold)
	})
	
	t.Run("Save config with directory creation", func(t *testing.T) {
		config := GetDefaultConfig()
		
		tempDir, err := os.MkdirTemp("", "config-test-")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		// 使用不存在的嵌套目录
		configFile := filepath.Join(tempDir, "nested", "dir", "config.yaml")
		
		err = SaveConfig(config, configFile)
		assert.NoError(t, err)
		assert.FileExists(t, configFile)
	})
	
	t.Run("Save config to invalid path", func(t *testing.T) {
		config := GetDefaultConfig()
		
		// 使用无效路径（试图在只读目录下创建文件）
		err := SaveConfig(config, "/root/cannot-write.yaml")
		assert.Error(t, err)
	})
}

func TestLoadConfigFromEnv(t *testing.T) {
	// 保存原始环境变量
	originalEnv := make(map[string]string)
	envVars := []string{
		"MYSQL_RECYCLER_HOST", "MYSQL_RECYCLER_PORT", "MYSQL_RECYCLER_USER",
		"MYSQL_RECYCLER_DATABASE", "MYSQL_RECYCLER_THRESHOLD", "MYSQL_RECYCLER_CONCURRENCY",
		"MYSQL_RECYCLER_LOG_LEVEL", "MYSQL_RECYCLER_LOG_FORMAT", "MYSQL_RECYCLER_STATUS_FILE",
		"MYSQL_RECYCLER_WEBHOOK_URL",
	}
	
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	
	// 测试后恢复环境变量
	defer func() {
		for env, val := range originalEnv {
			if val != "" {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	}()
	
	t.Run("Load configuration from environment variables", func(t *testing.T) {
		// 设置测试环境变量
		os.Setenv("MYSQL_RECYCLER_HOST", "env-host")
		os.Setenv("MYSQL_RECYCLER_PORT", "3307")
		os.Setenv("MYSQL_RECYCLER_USER", "env-user")
		os.Setenv("MYSQL_RECYCLER_DATABASE", "env-db")
		os.Setenv("MYSQL_RECYCLER_THRESHOLD", "500MB")
		os.Setenv("MYSQL_RECYCLER_CONCURRENCY", "8")
		os.Setenv("MYSQL_RECYCLER_LOG_LEVEL", "debug")
		os.Setenv("MYSQL_RECYCLER_LOG_FORMAT", "json")
		os.Setenv("MYSQL_RECYCLER_STATUS_FILE", "/tmp/env-status.json")
		os.Setenv("MYSQL_RECYCLER_WEBHOOK_URL", "http://example.com/webhook")
		
		config := GetDefaultConfig()
		LoadConfigFromEnv(config)
		
		assert.Equal(t, "env-host", config.Database.DefaultHost)
		assert.Equal(t, 3307, config.Database.DefaultPort)
		assert.Equal(t, "env-user", config.Database.DefaultUser)
		assert.Equal(t, "env-db", config.Database.DefaultDatabase)
		assert.Equal(t, "500MB", config.Recycler.DefaultThreshold)
		assert.Equal(t, 8, config.Recycler.DefaultConcurrency)
		assert.Equal(t, "debug", config.Logging.Level)
		assert.Equal(t, "json", config.Logging.Format)
		assert.Equal(t, "/tmp/env-status.json", config.Monitoring.StatusFile)
		assert.Equal(t, "http://example.com/webhook", config.Monitoring.WebhookURL)
		assert.True(t, config.Monitoring.EnableNotification) // 设置webhook URL时自动启用通知
	})
	
	t.Run("Handle invalid environment values", func(t *testing.T) {
		config := GetDefaultConfig()
		originalPort := config.Database.DefaultPort
		originalConcurrency := config.Recycler.DefaultConcurrency
		
		// 设置无效的环境变量值
		os.Setenv("MYSQL_RECYCLER_PORT", "invalid-port")
		os.Setenv("MYSQL_RECYCLER_CONCURRENCY", "invalid-concurrency")
		
		LoadConfigFromEnv(config)
		
		// 无效值应该被忽略，保持原始默认值
		assert.Equal(t, originalPort, config.Database.DefaultPort)
		assert.Equal(t, originalConcurrency, config.Recycler.DefaultConcurrency)
	})
}

func TestGetConfigSearchPaths(t *testing.T) {
	t.Run("Get configuration search paths", func(t *testing.T) {
		paths := GetConfigSearchPaths()
		assert.NotEmpty(t, paths)
		
		// 应该包含当前目录的路径
		foundCurrentDir := false
		for _, path := range paths {
			if filepath.Base(path) == "mysql-recycler.yaml" && filepath.Dir(path) != "/" {
				foundCurrentDir = true
				break
			}
		}
		assert.True(t, foundCurrentDir, "Should include current directory paths")
		
		// 应该包含系统配置路径
		assert.Contains(t, paths, "/etc/mysql-recycler/config.yaml")
		assert.Contains(t, paths, "/etc/mysql-recycler/config.yml")
		
		// 检查路径格式
		for _, path := range paths {
			assert.True(t, filepath.IsAbs(path) || filepath.Dir(path) != "", 
				"All paths should be absolute or relative with directory: %s", path)
		}
	})
}

func TestFindConfigFile(t *testing.T) {
	t.Run("Find existing config file", func(t *testing.T) {
		// 在临时目录中创建一个配置文件
		tempDir, err := os.MkdirTemp("", "config-search-test-")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		// 切换到临时目录
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)
		
		err = os.Chdir(tempDir)
		require.NoError(t, err)
		
		// 创建配置文件
		configFile := "mysql-recycler.yaml"
		err = os.WriteFile(configFile, []byte("test: true"), 0644)
		require.NoError(t, err)
		
		found := FindConfigFile()
		assert.NotEmpty(t, found)
		assert.True(t, filepath.IsAbs(found))
		assert.FileExists(t, found)
	})
	
	t.Run("No config file found", func(t *testing.T) {
		// 切换到一个空的临时目录
		tempDir, err := os.MkdirTemp("", "config-search-empty-")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalDir)
		
		err = os.Chdir(tempDir)
		require.NoError(t, err)
		
		found := FindConfigFile()
		assert.Empty(t, found)
	})
}

func TestGenerateExampleConfig(t *testing.T) {
	t.Run("Generate example configuration", func(t *testing.T) {
		example := GenerateExampleConfig()
		assert.NotEmpty(t, example)
		
		// 验证示例包含关键配置项
		assert.Contains(t, example, "database:")
		assert.Contains(t, example, "recycler:")
		assert.Contains(t, example, "monitoring:")
		assert.Contains(t, example, "external_api:")
		assert.Contains(t, example, "logging:")
		
		// 验证示例包含注释说明
		assert.Contains(t, example, "# MySQL表空间回收工具配置文件示例")
		assert.Contains(t, example, "# 数据库连接配置")
		assert.Contains(t, example, "# 默认碎片回收阈值")
		
		// 验证可以解析为有效的YAML
		tempFile, err := os.CreateTemp("", "example-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		
		_, err = tempFile.WriteString(example)
		require.NoError(t, err)
		tempFile.Close()
		
		// 尝试加载示例配置
		config, err := LoadConfig(tempFile.Name())
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}

func TestParseHelperFunctions(t *testing.T) {
	t.Run("Parse valid port", func(t *testing.T) {
		port, err := parsePort("3306")
		assert.NoError(t, err)
		assert.Equal(t, 3306, port)
		
		port, err = parsePort("8080")
		assert.NoError(t, err)
		assert.Equal(t, 8080, port)
	})
	
	t.Run("Parse invalid port", func(t *testing.T) {
		testCases := []string{"0", "-1", "65536", "99999", "abc", "3306.5", ""}
		
		for _, tc := range testCases {
			_, err := parsePort(tc)
			assert.Error(t, err, "Port %s should be invalid", tc)
		}
	})
	
	t.Run("Parse valid integer", func(t *testing.T) {
		value, err := parseIntValue("42")
		assert.NoError(t, err)
		assert.Equal(t, 42, value)
		
		value, err = parseIntValue("0")
		assert.NoError(t, err)
		assert.Equal(t, 0, value)
	})
	
	t.Run("Parse invalid integer", func(t *testing.T) {
		testCases := []string{"-1", "abc", "3.14", "", "1.0"}
		
		for _, tc := range testCases {
			_, err := parseIntValue(tc)
			assert.Error(t, err, "Value %s should be invalid", tc)
		}
	})
}

func TestConfigMarshalUnmarshal(t *testing.T) {
	t.Run("Round trip YAML marshal/unmarshal", func(t *testing.T) {
		originalConfig := GetDefaultConfig()
		
		// 修改一些值
		originalConfig.Database.DefaultHost = "test-host"
		originalConfig.Recycler.DefaultThreshold = "500MB"
		originalConfig.Monitoring.EnableNotification = true
		
		// 创建临时文件
		tempFile, err := os.CreateTemp("", "roundtrip-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()
		
		// 保存配置
		err = SaveConfig(originalConfig, tempFile.Name())
		require.NoError(t, err)
		
		// 重新加载配置
		loadedConfig, err := LoadConfig(tempFile.Name())
		require.NoError(t, err)
		
		// 验证关键值
		assert.Equal(t, originalConfig.Database.DefaultHost, loadedConfig.Database.DefaultHost)
		assert.Equal(t, originalConfig.Recycler.DefaultThreshold, loadedConfig.Recycler.DefaultThreshold)
		assert.Equal(t, originalConfig.Monitoring.EnableNotification, loadedConfig.Monitoring.EnableNotification)
		assert.Equal(t, originalConfig.Logging.Level, loadedConfig.Logging.Level)
	})
}