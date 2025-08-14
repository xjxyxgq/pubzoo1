package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogLevel(t *testing.T) {
	t.Run("LogLevel string representation", func(t *testing.T) {
		testCases := []struct {
			level    LogLevel
			expected string
		}{
			{DEBUG, "DEBUG"},
			{INFO, "INFO"},
			{WARN, "WARN"},
			{ERROR, "ERROR"},
			{LogLevel(999), "UNKNOWN"},
		}

		for _, tc := range testCases {
			assert.Equal(t, tc.expected, tc.level.String())
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	t.Run("Parse valid log levels", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected LogLevel
		}{
			{"debug", DEBUG},
			{"DEBUG", DEBUG},
			{"Debug", DEBUG},
			{"info", INFO},
			{"INFO", INFO},
			{"Info", INFO},
			{"warn", WARN},
			{"WARN", WARN},
			{"warning", WARN},
			{"WARNING", WARN},
			{"error", ERROR},
			{"ERROR", ERROR},
			{"Error", ERROR},
			{"invalid", INFO}, // 默认返回INFO
			{"", INFO},        // 空字符串默认返回INFO
		}

		for _, tc := range testCases {
			result := parseLogLevel(tc.input)
			assert.Equal(t, tc.expected, result, "parseLogLevel(%s) should return %v", tc.input, tc.expected)
		}
	})
}

func TestNewLogger(t *testing.T) {
	t.Run("Create logger with buffer output", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(DEBUG, &buf)
		
		assert.NotNil(t, logger)
		
		// 测试日志输出
		logger.Debug("test debug message")
		output := buf.String()
		
		assert.Contains(t, output, "[DEBUG]")
		assert.Contains(t, output, "test debug message")
	})

	t.Run("Create logger with nil output", func(t *testing.T) {
		logger := NewLogger(INFO, nil)
		assert.NotNil(t, logger)
		
		// 应该不会崩溃
		logger.Info("test message")
	})
}

func TestNewLoggerFromString(t *testing.T) {
	t.Run("Create logger from string level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLoggerFromString("debug", &buf)
		
		assert.NotNil(t, logger)
		
		logger.Debug("debug message")
		output := buf.String()
		
		assert.Contains(t, output, "[DEBUG]")
		assert.Contains(t, output, "debug message")
	})
}

func TestDefaultLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(DEBUG, &buf).(*DefaultLogger)

	t.Run("Set and get log level", func(t *testing.T) {
		logger.SetLevel(ERROR)
		
		// 清空缓冲区
		buf.Reset()
		
		// DEBUG、INFO、WARN 级别的日志不应该输出
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		
		assert.Empty(t, buf.String())
		
		// ERROR 级别的日志应该输出
		logger.Error("error message")
		output := buf.String()
		assert.Contains(t, output, "[ERROR]")
		assert.Contains(t, output, "error message")
	})

	t.Run("Test all log levels", func(t *testing.T) {
		logger.SetLevel(DEBUG)
		buf.Reset()
		
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")
		
		output := buf.String()
		assert.Contains(t, output, "[DEBUG]")
		assert.Contains(t, output, "[INFO]")
		assert.Contains(t, output, "[WARN]")
		assert.Contains(t, output, "[ERROR]")
	})

	t.Run("Test log with format arguments", func(t *testing.T) {
		logger.SetLevel(INFO)
		buf.Reset()
		
		logger.Info("user %s has %d items", "alice", 42)
		output := buf.String()
		
		assert.Contains(t, output, "[INFO]")
		assert.Contains(t, output, "user alice has 42 items")
	})

	t.Run("Test log level filtering", func(t *testing.T) {
		logger.SetLevel(WARN)
		buf.Reset()
		
		// DEBUG 和 INFO 应该被过滤
		logger.Debug("debug message")
		logger.Info("info message")
		
		assert.Empty(t, buf.String())
		
		// WARN 和 ERROR 应该输出
		logger.Warn("warn message")
		logger.Error("error message")
		
		output := buf.String()
		assert.Contains(t, output, "[WARN]")
		assert.Contains(t, output, "[ERROR]")
		assert.NotContains(t, output, "[DEBUG]")
		assert.NotContains(t, output, "[INFO]")
	})

	t.Run("Set output writer", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(INFO, &buf)
		
		// 记录一条消息到原缓冲区
		logger.Info("original test")
		assert.Contains(t, buf.String(), "original test")
		
		var newBuf bytes.Buffer
		logger.SetOutput(&newBuf)
		
		logger.Info("new output test")
		
		// 原缓冲区应该没有新内容
		assert.NotContains(t, buf.String(), "new output test")
		
		// 新缓冲区应该有内容
		assert.Contains(t, newBuf.String(), "new output test")
	})
}

func TestGlobalLogger(t *testing.T) {
	// 保存原始的全局日志器
	originalLogger := GetGlobalLogger()
	defer SetGlobalLogger(originalLogger)

	t.Run("Set and get global logger", func(t *testing.T) {
		var buf bytes.Buffer
		testLogger := NewLogger(DEBUG, &buf)
		
		SetGlobalLogger(testLogger)
		retrievedLogger := GetGlobalLogger()
		
		assert.Equal(t, testLogger, retrievedLogger)
	})

	t.Run("Global logging functions", func(t *testing.T) {
		var buf bytes.Buffer
		testLogger := NewLogger(DEBUG, &buf)
		SetGlobalLogger(testLogger)
		
		Debug("global debug")
		Info("global info")
		Warn("global warn")
		Error("global error")
		
		output := buf.String()
		assert.Contains(t, output, "global debug")
		assert.Contains(t, output, "global info")
		assert.Contains(t, output, "global warn")
		assert.Contains(t, output, "global error")
	})

	t.Run("Global logging with format", func(t *testing.T) {
		var buf bytes.Buffer
		testLogger := NewLogger(INFO, &buf)
		SetGlobalLogger(testLogger)
		
		Info("formatted message: %s = %d", "count", 100)
		output := buf.String()
		
		assert.Contains(t, output, "formatted message: count = 100")
	})
}

func TestLogOutput(t *testing.T) {
	t.Run("Log output format", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(INFO, &buf)
		
		logger.Info("test message")
		output := buf.String()
		
		// 验证输出包含时间戳、级别标识和消息
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 1)
		
		line := lines[0]
		assert.Contains(t, line, "[INFO]")
		assert.Contains(t, line, "test message")
		
		// 验证时间戳格式（应该包含日期和时间）
		assert.Regexp(t, `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, line)
	})

	t.Run("Multiple log lines", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(DEBUG, &buf)
		
		logger.Debug("line 1")
		logger.Info("line 2")
		logger.Warn("line 3")
		
		output := strings.TrimSpace(buf.String())
		lines := strings.Split(output, "\n")
		
		assert.Len(t, lines, 3)
		assert.Contains(t, lines[0], "line 1")
		assert.Contains(t, lines[1], "line 2")
		assert.Contains(t, lines[2], "line 3")
	})
}

func TestLoggerEdgeCases(t *testing.T) {
	t.Run("Empty log message", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(INFO, &buf)
		
		logger.Info("")
		output := buf.String()
		
		assert.Contains(t, output, "[INFO]")
	})

	t.Run("Log message with special characters", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(INFO, &buf)
		
		specialMessage := "Message with 中文, émojis 🚀, and symbols @#$%"
		logger.Info(specialMessage)
		output := buf.String()
		
		assert.Contains(t, output, specialMessage)
	})

	t.Run("Log with nil format args", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(INFO, &buf)
		
		// 这应该不会崩溃
		logger.Info("message without args")
		output := buf.String()
		
		assert.Contains(t, output, "message without args")
	})
}