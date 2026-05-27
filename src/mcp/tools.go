package mcp

import (
	"context"
	"encoding/json"
)

// Tool MCP 工具定义
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolHandler 工具执行函数签名
type ToolHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// Registry MCP 工具注册管理器
type Registry struct {
	tools    map[string]Tool
	handlers map[string]ToolHandler
}

// NewRegistry 创建新的工具注册器
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// Register 注册工具到注册器
func (r *Registry) Register(name, description string, inputSchema json.RawMessage, handler ToolHandler) {
	r.tools[name] = Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
	r.handlers[name] = handler
}

// List 返回所有已注册的工具
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Execute 根据名称执行工具
func (r *Registry) Execute(ctx context.Context, name string, params json.RawMessage) (interface{}, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return nil, &Error{
			Code:    -32601,
			Message: "Method not found",
		}
	}
	return handler(ctx, params)
}

// HandleInitialize 处理 MCP initialize 方法
func (r *Registry) HandleInitialize(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "k8s-mcp",
			"version": "1.0.0",
		},
	}, nil
}