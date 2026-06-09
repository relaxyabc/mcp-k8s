package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/relaxyabc/mcp-k8s/src/logger"
)

// Server MCP stdio 服务器实现
type Server struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	registry *Registry
	audit    AuditLogger

	mu     sync.Mutex
	running bool
	log     *logger.Logger
}

// AuditLogger 审计日志接口
type AuditLogger interface {
	LogToolCall(toolName string, params any, result string, durationMs int64)
	LogError(toolName string, message string)
}

// NewServer 创建新的 MCP stdio 服务器
func NewServer(registry *Registry, audit AuditLogger) *Server {
	return &Server{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		registry: registry,
		audit:    audit,
		log:      logger.NewDevelopmentLogger(),
	}
}

// Start 开始监听 stdin 上的 MCP 请求
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	s.log.Info("server started, waiting for requests on stdin")

	scanner := bufio.NewScanner(s.stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			s.log.Info("received empty line, skipping")
			continue
		}

		// 记录收到的原始请求
		s.log.Info("received raw request", "content", string(line))

		// 解析 JSON-RPC 请求
		req, err := ParseRequest(line)
		if err != nil {
			s.log.Error("parse error", "error", err, "raw", string(line))
			s.writeError(nil, -32700, "Parse error")
			continue
		}

		s.log.Info("parsed request", "method", req.Method, "id", req.ID)

		// 处理请求
		response := s.handleRequest(ctx, req)

		// 通知类请求不需要响应
		if response == nil {
			s.log.Info("no response needed for notification")
			continue
		}

		// 写入响应
		respBytes, _ := json.Marshal(response)
		s.log.Info("sending response", "content", string(respBytes))
		if err := s.writeResponse(response); err != nil {
			s.log.Error("write response error", "error", err)
		}
	}

	if err := scanner.Err(); err != nil {
		s.log.Error("stdin scanner error", "error", err)
		return fmt.Errorf("stdin scanner error: %w", err)
	}

	s.log.Info("scanner ended, server shutting down")
	return nil
}

// handleRequest 处理 JSON-RPC 请求
func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
	s.log.Debug("handling request", "method", req.Method)

	switch req.Method {
	case "initialize":
		s.log.Debug("processing initialize request")
		result, err := s.registry.HandleInitialize(req.Params)
		if err != nil {
			s.log.Error("initialize error", "error", err)
			return &Response{
				Jsonrpc: JSONRPCVersion,
				Error:   err.(*Error),
				ID:      req.ID,
			}
		}
		resultBytes, _ := json.Marshal(result)
		s.log.Debug("initialize result", "content", string(resultBytes))
		return &Response{
			Jsonrpc: JSONRPCVersion,
			Result:  resultBytes,
			ID:      req.ID,
		}

	case "tools/list":
		s.log.Debug("processing tools/list request")
		tools := s.registry.List()
		resultBytes, _ := json.Marshal(map[string]any{
			"tools": tools,
		})
		s.log.Debug("tools/list result", "content", string(resultBytes))
		return &Response{
			Jsonrpc: JSONRPCVersion,
			Result:  resultBytes,
			ID:      req.ID,
		}

	case "tools/call":
		s.log.Debug("processing tools/call request")
		return s.handleToolCall(ctx, req)

	case "notifications/initialized":
		s.log.Debug("received initialized notification (no response needed)")
		return nil

	default:
		s.log.Warn("unknown method", "method", req.Method)
		return &Response{
			Jsonrpc: JSONRPCVersion,
			Error:   &Error{Code: -32601, Message: "Method not found"},
			ID:      req.ID,
		}
	}
}

// handleToolCall 执行工具调用
func (s *Server) handleToolCall(ctx context.Context, req *Request) *Response {
	s.log.Info("handleToolCall: 开始处理", "requestId", req.ID)
	// 解析工具调用参数
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Error("handleToolCall: 解析参数失败", "error", err)
		return &Response{
			Jsonrpc: JSONRPCVersion,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
			ID:      req.ID,
		}
	}

	s.log.Info("handleToolCall: 开始执行工具", "tool", params.Name, "arguments", string(params.Arguments))

	// 执行工具
	result, err := s.registry.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		s.log.Error("handleToolCall: 执行失败", "error", err)
		if e, ok := err.(*Error); ok {
			return &Response{
				Jsonrpc: JSONRPCVersion,
				Error:   e,
				ID:      req.ID,
			}
		}
		return &Response{
			Jsonrpc: JSONRPCVersion,
			Error:   &Error{Code: -32603, Message: err.Error()},
			ID:      req.ID,
		}
	}

	s.log.Info("handleToolCall: 执行成功", "requestId", req.ID)
	// 序列化结果为 JSON
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		s.log.Error("handleToolCall: JSON 序列化失败", "error", marshalErr)
		resultJSON = []byte(fmt.Sprintf("%v", result))
	}
	resultBytes, _ := json.Marshal(map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": string(resultJSON),
			},
		},
	})
	s.log.Info("handleToolCall: 返回响应", "requestId", req.ID)
	return &Response{
		Jsonrpc: JSONRPCVersion,
		Result:  resultBytes,
		ID:      req.ID,
	}
}

// writeResponse 向 stdout 写入 JSON-RPC 响应
func (s *Server) writeResponse(resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	s.stdout.Write(data)
	s.stdout.Write([]byte("\n"))
	return nil
}

// writeError 向 stdout 写入 JSON-RPC 错误
func (s *Server) writeError(id any, code int, message string) {
	resp := &Response{
		Jsonrpc: JSONRPCVersion,
		Error:   &Error{Code: code, Message: message},
		ID:      id,
	}
	s.writeResponse(resp)
}

// Stop 优雅关闭服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return nil
}