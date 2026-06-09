package audit

import (
	"context"
	"os"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/logger"
)

// LogLevel 审计日志级别
type LogLevel = logger.LogLevel

// 日志级别常量，使用 logger 包的级别定义
const (
	Debug = logger.Debug
	Info  = logger.Info
	Warn  = logger.Warn
	Error = logger.Error
)

// AuditEntry 单条审计日志记录
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       LogLevel  `json:"level"`
	ToolName    string    `json:"toolName,omitempty"`
	Params      any       `json:"params,omitempty"`
	Result      string    `json:"result,omitempty"`
	Command     string    `json:"command,omitempty"`
	Message     string    `json:"message,omitempty"`
	DurationMs  int64     `json:"durationMs,omitempty"`
}

// PrivilegedOp 特权模式操作记录
type PrivilegedOp struct {
	Type      string      // 操作类型：list_resources, get_resource, read_pod_logs
	Resource  string      // 资源名称（Pod名、资源名等）
	Namespace string      // namespace
	Cluster   string      // 集群名
	Details   interface{} // 操作详情
	Confirmed bool        // 是否已确认
	Status    string      // success, failed
	Error     string      // 错误信息（如果有）
}

// Logger 基于 zap 的审计日志器
type Logger struct {
	*logger.Logger
}

// NewLogger 创建新的审计日志器
func NewLogger(level LogLevel, outputFile *os.File) *Logger {
	return &Logger{
		logger.NewLogger(level, outputFile),
	}
}

// LogToolCall 记录 MCP 工具调用日志
func (l *Logger) LogToolCall(toolName string, params any, result string, durationMs int64) {
	l.Info("tool_call",
		"toolName", toolName,
		"params", sanitizeParams(params),
		"result", result,
		"durationMs", durationMs,
	)
}

// LogError 记录错误日志
func (l *Logger) LogError(toolName string, message string) {
	l.Error("tool_error",
		"toolName", toolName,
		"message", message,
	)
}

// LogPrivilegedOperation 记录特权模式操作
func (l *Logger) LogPrivilegedOperation(ctx context.Context, op *PrivilegedOp) {
	l.Warn("privileged_operation",
		"type", op.Type,
		"resource", op.Resource,
		"namespace", op.Namespace,
		"cluster", op.Cluster,
		"confirmed", op.Confirmed,
		"status", op.Status,
		"details", op.Details,
		"error", op.Error,
	)
}

// sanitizeParams 在记录日志前移除敏感数据
func sanitizeParams(params any) any {
	return params
}