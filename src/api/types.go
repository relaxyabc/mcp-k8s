package api

import (
	"encoding/json"
)

// ListResourcesParams list_resources 工具参数
type ListResourcesParams struct {
	ResourceType string `json:"resourceType"`
	Namespace    string `json:"namespace,omitempty"`
}

// GetResourceParams get_resource 工具参数
type GetResourceParams struct {
	ResourceType string `json:"resourceType"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
}

// ReadPodLogsParams read_pod_logs 工具参数
type ReadPodLogsParams struct {
	Namespace      string `json:"namespace"`
	PodName        string `json:"podName"`
	Container      string `json:"container,omitempty"`
	LogDir         string `json:"logDir"`
	LogFile        string `json:"logFile"`
	Operation      string `json:"operation,omitempty"`
	Lines          int    `json:"lines,omitempty"`
	Pattern        string `json:"pattern,omitempty"`
	Follow         bool   `json:"follow,omitempty"`
	FollowDuration int    `json:"followDuration,omitempty"`
}

// ResourceSummary 列表结果中的资源摘要
type ResourceSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// ResourceDetail 获取结果中的资源详情
type ResourceDetail struct {
	Metadata map[string]interface{} `json:"metadata"`
	Spec     map[string]interface{} `json:"spec,omitempty"`
	Status   map[string]interface{} `json:"status,omitempty"`
}

// LogContent 日志文件内容
type LogContent struct {
	Lines      []string `json:"lines"`
	TotalCount int      `json:"totalCount"`
	Operation  string   `json:"operation"`
	Command    string   `json:"command,omitempty"`
	Truncated  bool     `json:"truncated,omitempty"`
	FilePath   string   `json:"filePath"`
	Followed   bool     `json:"followed,omitempty"`
}

// ToolResponse 标准 MCP 工具响应
type ToolResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ToolError  `json:"error,omitempty"`
}

// ToolError 工具响应中的错误
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// 错误代码常量
const (
	ErrInvalidInput        = "INVALID_INPUT"
	ErrNotFound            = "NOT_FOUND"
	ErrForbidden           = "FORBIDDEN"
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrInternal            = "INTERNAL_ERROR"
	ErrTimeout             = "TIMEOUT"
	ErrLogFileNotFound     = "LOG_FILE_NOT_FOUND"
	ErrSensitivePathDenied = "SENSITIVE_PATH_DENIED"
)

// NewSuccessResponse 创建成功的工具响应
func NewSuccessResponse(result interface{}) ToolResponse {
	return ToolResponse{
		Success: true,
		Result:  result,
	}
}

// NewErrorResponse 创建错误的工具响应
func NewErrorResponse(code, message string) ToolResponse {
	return ToolResponse{
		Success: false,
		Error: &ToolError{
			Code:    code,
			Message: message,
		},
	}
}

// ParseParams 将 JSON 参数解析为结构体
func ParseParams[T any](raw json.RawMessage) (T, error) {
	var params T
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, err
	}
	return params, nil
}