package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogLevel 日志级别
type LogLevel int

const (
	// DEBUG 调试级别
	DEBUG LogLevel = iota
	// INFO 信息级别
	INFO
	// WARN 警告级别
	WARN
	// ERROR 错误级别
	ERROR
	// FATAL 致命级别
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

var levelFromString = map[string]LogLevel{
	"debug": DEBUG,
	"info":  INFO,
	"warn":  WARN,
	"error": ERROR,
	"fatal": FATAL,
}

// Logger 日志记录器
type Logger struct {
	level LogLevel
	zl    zerolog.Logger
}

// NewLogger 创建新的日志记录器
func NewLogger(level string) *Logger {
	logLevel := INFO
	if l, ok := levelFromString[strings.ToLower(level)]; ok {
		logLevel = l
	}

	// zerolog 默认使用 JSON；可通过环境变量切换为控制台输出
	zl := log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05.000"}).With().Timestamp().Logger()

	return &Logger{level: logLevel, zl: zl}
}

// log 记录日志
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	msg := fmt.Sprintf(format, args...)
	switch level {
	case DEBUG:
		l.zl.Debug().Msg(msg)
	case INFO:
		l.zl.Info().Msg(msg)
	case WARN:
		l.zl.Warn().Msg(msg)
	case ERROR:
		l.zl.Error().Msg(msg)
	case FATAL:
		// 不调用 os.Exit，统一使用 Error 级别并标记 fatal
		l.zl.Error().Str("level", "FATAL").Msg(msg)
	}
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level string) {
	if lvl, ok := levelFromString[strings.ToLower(level)]; ok {
		l.level = lvl
	}
}
