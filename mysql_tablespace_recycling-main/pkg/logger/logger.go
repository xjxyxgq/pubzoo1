package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger 简单的日志接口
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	SetLevel(level LogLevel)
	SetOutput(w io.Writer)
}

// DefaultLogger 默认日志实现
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger 创建新的日志器
func NewLogger(level LogLevel, output io.Writer) Logger {
	if output == nil {
		output = os.Stdout
	}
	
	return &DefaultLogger{
		level:  level,
		logger: log.New(output, "", log.LstdFlags),
	}
}

// NewLoggerFromString 从字符串创建日志器
func NewLoggerFromString(levelStr string, output io.Writer) Logger {
	level := parseLogLevel(levelStr)
	return NewLogger(level, output)
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// SetLevel 设置日志级别
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.level = level
}

// SetOutput 设置输出
func (l *DefaultLogger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// Debug 输出调试日志
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, msg, args...)
	}
}

// Info 输出信息日志
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, msg, args...)
	}
}

// Warn 输出警告日志
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, msg, args...)
	}
}

// Error 输出错误日志
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, msg, args...)
	}
}

// log 内部日志方法
func (l *DefaultLogger) log(level LogLevel, msg string, args ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", level.String())
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	l.logger.Print(prefix + msg)
}

// 全局日志器实例
var globalLogger Logger = NewLogger(INFO, os.Stdout)

// SetGlobalLogger 设置全局日志器
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger 获取全局日志器
func GetGlobalLogger() Logger {
	return globalLogger
}

// 全局日志函数
func Debug(msg string, args ...interface{}) {
	globalLogger.Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	globalLogger.Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	globalLogger.Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	globalLogger.Error(msg, args...)
}