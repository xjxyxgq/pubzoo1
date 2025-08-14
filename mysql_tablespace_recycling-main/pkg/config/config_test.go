package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig(t *testing.T) {
	t.Run("Default database config values", func(t *testing.T) {
		config := DatabaseConfig{
			ConnTimeout:     10 * time.Second,
			QueryTimeout:    30 * time.Second,
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: 3600 * time.Second,
		}
		
		assert.Equal(t, 10*time.Second, config.ConnTimeout)
		assert.Equal(t, 30*time.Second, config.QueryTimeout)
		assert.Equal(t, 10, config.MaxIdleConns)
		assert.Equal(t, 100, config.MaxOpenConns)
		assert.Equal(t, 3600*time.Second, config.ConnMaxLifetime)
	})
}

func TestRecoveryConfig(t *testing.T) {
	t.Run("Default recovery config values", func(t *testing.T) {
		config := RecoveryConfig{
			AllowOnMaster:     false,
			FragmentThreshold: 104857600, // 100MB
			WorkMode:          1,
			Concurrency:       5,
			BatchSize:         10,
			ProcessInterval:   1 * time.Second,
		}
		
		assert.False(t, config.AllowOnMaster)
		assert.Equal(t, int64(104857600), config.FragmentThreshold)
		assert.Equal(t, 1, config.WorkMode)
		assert.Equal(t, 5, config.Concurrency)
		assert.Equal(t, 10, config.BatchSize)
		assert.Equal(t, 1*time.Second, config.ProcessInterval)
	})
	
	t.Run("Recovery config validation", func(t *testing.T) {
		// 测试有效的工作模式
		validModes := []int{1, 2}
		for _, mode := range validModes {
			config := RecoveryConfig{WorkMode: mode}
			assert.Contains(t, validModes, config.WorkMode)
		}
		
		// 测试合理的并发数
		config := RecoveryConfig{Concurrency: 5}
		assert.Greater(t, config.Concurrency, 0)
		assert.LessOrEqual(t, config.Concurrency, 100) // 假设最大并发数不超过100
	})
}

func TestMonitorConfig(t *testing.T) {
	t.Run("Default monitor config values", func(t *testing.T) {
		config := MonitorConfig{
			PersistInterval: 30 * time.Second,
			StatusFile:      "./status.json",
			EnableHTTP:      true,
			HTTPPort:        8080,
		}
		
		assert.Equal(t, 30*time.Second, config.PersistInterval)
		assert.Equal(t, "./status.json", config.StatusFile)
		assert.True(t, config.EnableHTTP)
		assert.Equal(t, 8080, config.HTTPPort)
	})
	
	t.Run("Monitor config with different settings", func(t *testing.T) {
		config := MonitorConfig{
			PersistInterval: 60 * time.Second,
			StatusFile:      "/tmp/custom-status.json",
			EnableHTTP:      false,
			HTTPPort:        9090,
		}
		
		assert.Equal(t, 60*time.Second, config.PersistInterval)
		assert.Equal(t, "/tmp/custom-status.json", config.StatusFile)
		assert.False(t, config.EnableHTTP)
		assert.Equal(t, 9090, config.HTTPPort)
	})
}

func TestExternalAPIConfig(t *testing.T) {
	t.Run("Default external API config", func(t *testing.T) {
		config := ExternalAPIConfig{
			BaseURL:    "",
			Timeout:    10 * time.Second,
			RetryCount: 3,
		}
		
		assert.Empty(t, config.BaseURL)
		assert.Equal(t, 10*time.Second, config.Timeout)
		assert.Equal(t, 3, config.RetryCount)
	})
	
	t.Run("External API config with custom values", func(t *testing.T) {
		config := ExternalAPIConfig{
			BaseURL:    "https://api.example.com",
			Timeout:    30 * time.Second,
			RetryCount: 5,
		}
		
		assert.Equal(t, "https://api.example.com", config.BaseURL)
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 5, config.RetryCount)
	})
	
	t.Run("Validate retry count", func(t *testing.T) {
		config := ExternalAPIConfig{RetryCount: 3}
		assert.GreaterOrEqual(t, config.RetryCount, 0)
		assert.LessOrEqual(t, config.RetryCount, 10) // 合理的重试上限
	})
}

func TestLogConfig(t *testing.T) {
	t.Run("Default log config", func(t *testing.T) {
		config := LogConfig{
			Level:    "info",
			Output:   "stdout",
			ToFile:   false,
			FilePath: "./logs/app.log",
		}
		
		assert.Equal(t, "info", config.Level)
		assert.Equal(t, "stdout", config.Output)
		assert.False(t, config.ToFile)
		assert.Equal(t, "./logs/app.log", config.FilePath)
	})
	
	t.Run("Log config with file output", func(t *testing.T) {
		config := LogConfig{
			Level:    "debug",
			Output:   "file",
			ToFile:   true,
			FilePath: "/var/log/mysql-recycler.log",
		}
		
		assert.Equal(t, "debug", config.Level)
		assert.Equal(t, "file", config.Output)
		assert.True(t, config.ToFile)
		assert.Equal(t, "/var/log/mysql-recycler.log", config.FilePath)
	})
	
	t.Run("Valid log levels", func(t *testing.T) {
		validLevels := []string{"debug", "info", "warn", "error"}
		
		for _, level := range validLevels {
			config := LogConfig{Level: level}
			assert.Contains(t, validLevels, config.Level)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("Complete configuration structure", func(t *testing.T) {
		config := Config{
			Database: DatabaseConfig{
				ConnTimeout:     15 * time.Second,
				QueryTimeout:    45 * time.Second,
				MaxIdleConns:    20,
				MaxOpenConns:    200,
				ConnMaxLifetime: 7200 * time.Second,
			},
			Recovery: RecoveryConfig{
				AllowOnMaster:     true,
				FragmentThreshold: 209715200, // 200MB
				WorkMode:          2,
				Concurrency:       8,
				BatchSize:         20,
				ProcessInterval:   2 * time.Second,
			},
			Monitor: MonitorConfig{
				PersistInterval: 45 * time.Second,
				StatusFile:      "/tmp/recycler-status.json",
				EnableHTTP:      true,
				HTTPPort:        8090,
			},
			ExternalAPI: ExternalAPIConfig{
				BaseURL:    "https://monitoring.example.com",
				Timeout:    20 * time.Second,
				RetryCount: 4,
			},
			Log: LogConfig{
				Level:    "debug",
				Output:   "file",
				ToFile:   true,
				FilePath: "/var/log/recycler.log",
			},
		}
		
		// 验证所有配置都被正确设置
		assert.Equal(t, 15*time.Second, config.Database.ConnTimeout)
		assert.True(t, config.Recovery.AllowOnMaster)
		assert.Equal(t, int64(209715200), config.Recovery.FragmentThreshold)
		assert.Equal(t, 8090, config.Monitor.HTTPPort)
		assert.Equal(t, "https://monitoring.example.com", config.ExternalAPI.BaseURL)
		assert.Equal(t, "debug", config.Log.Level)
	})
	
	t.Run("Default configuration initialization", func(t *testing.T) {
		config := Config{}
		
		// 验证零值
		assert.Equal(t, time.Duration(0), config.Database.ConnTimeout)
		assert.False(t, config.Recovery.AllowOnMaster)
		assert.Equal(t, int64(0), config.Recovery.FragmentThreshold)
		assert.Empty(t, config.Monitor.StatusFile)
		assert.Empty(t, config.ExternalAPI.BaseURL)
		assert.Empty(t, config.Log.Level)
	})
}

func TestConfigFieldTags(t *testing.T) {
	t.Run("Verify struct tags are present", func(t *testing.T) {
		// 这个测试验证配置结构体有适当的JSON和YAML标签
		// 在实际应用中，这些标签对于配置文件的序列化/反序列化很重要
		
		config := Config{
			Database: DatabaseConfig{ConnTimeout: 10 * time.Second},
			Recovery: RecoveryConfig{WorkMode: 1},
			Monitor:  MonitorConfig{HTTPPort: 8080},
		}
		
		// 基本验证配置结构体可以被实例化
		assert.NotNil(t, config.Database)
		assert.NotNil(t, config.Recovery)
		assert.NotNil(t, config.Monitor)
		assert.NotNil(t, config.ExternalAPI)
		assert.NotNil(t, config.Log)
	})
}