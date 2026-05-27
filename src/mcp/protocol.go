package mcp

import (
	"encoding/json"
)

const (
	JSONRPCVersion = "2.0"
)

// Request JSON-RPC 2.0 请求结构
type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// Response JSON-RPC 2.0 响应结构
type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// Error JSON-RPC 2.0 错误结构
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error 实现 error 接口
func (e *Error) Error() string {
	return e.Message
}

// ParseRequest 从原始字节解析 JSON-RPC 请求
func ParseRequest(data []byte) (*Request, error) {
	req := &Request{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, err
	}
	return req, nil
}

// FormatResponse 格式化 JSON-RPC 响应
func FormatResponse(id interface{}, result interface{}) ([]byte, error) {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	resp := Response{
		Jsonrpc: JSONRPCVersion,
		Result:  resultBytes,
		ID:      id,
	}
	return json.Marshal(resp)
}

// FormatError 格式化 JSON-RPC 错误响应
func FormatError(id interface{}, code int, message string) ([]byte, error) {
	resp := Response{
		Jsonrpc: JSONRPCVersion,
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	return json.Marshal(resp)
}