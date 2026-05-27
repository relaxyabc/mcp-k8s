package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel 日志级别类型
type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

// Logger 基于 zap 的统一日志器
type Logger struct {
	zap    *zap.SugaredLogger
	level  zapcore.Level
	output *os.File
}

// NewLogger 创建新的日志实例
func NewLogger(level LogLevel, output *os.File) *Logger {
	var zapLevel zapcore.Level
	switch level {
	case Debug:
		zapLevel = zapcore.DebugLevel
	case Info:
		zapLevel = zapcore.InfoLevel
	case Warn:
		zapLevel = zapcore.WarnLevel
	case Error:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(output),
		zapLevel,
	)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return &Logger{
		zap:    zapLogger.Sugar(),
		level:  zapLevel,
		output: output,
	}
}

// NewDevelopmentLogger 创建开发环境的日志器（控制台输出）
func NewDevelopmentLogger() *Logger {
	zapLogger, _ := zap.NewDevelopment()
	return &Logger{
		zap:   zapLogger.Sugar(),
		level: zapcore.DebugLevel,
	}
}

// Debug 输出调试级别日志
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.zap.Debugw(msg, keysAndValues...)
}

// Info 输出信息级别日志
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.zap.Infow(msg, keysAndValues...)
}

// Warn 输出警告级别日志
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.zap.Warnw(msg, keysAndValues...)
}

// Error 输出错误级别日志
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.zap.Errorw(msg, keysAndValues...)
}

// Fatal 输出致命错误日志并退出程序
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.zap.Fatalw(msg, keysAndValues...)
}

// With 返回带有额外上下文字段的日志器
func (l *Logger) With(keysAndValues ...interface{}) *Logger {
	return &Logger{
		zap:    l.zap.With(keysAndValues...),
		level:  l.level,
		output: l.output,
	}
}

// Sync 刷新缓冲的日志条目
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// Named 添加子日志器名称
func (l *Logger) Named(name string) *Logger {
	return &Logger{
		zap:    l.zap.Named(name),
		level:  l.level,
		output: l.output,
	}
}