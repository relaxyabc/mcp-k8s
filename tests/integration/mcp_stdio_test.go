package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
)

// Integration test for MCP stdio server

func TestMCPServerInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	stdin := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}\n`)
	stdout := &bytes.Buffer{}

	registry := mcp.NewRegistry()
	logger := audit.NewLogger(audit.Debug, os.Stdout)

	server := &MockMCPServer{
		stdin:   stdin,
		stdout:  stdout,
		registry: registry,
		logger:   logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.ProcessRequest(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	response := stdout.String()
	if response == "" {
		t.Error("expected non-empty response")
	}

	var resp mcp.Response
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		t.Errorf("failed to parse response: %v", err)
	}

	if resp.Jsonrpc != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %s", resp.Jsonrpc)
	}
}

func TestMCPServerToolsList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := mcp.NewRegistry()
	registry.Register(
		"test_tool",
		"Test tool for integration testing",
		json.RawMessage(`{"type":"object"}`),
		func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			return "test result", nil
		},
	)

	stdin := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"tools/list","id":2}\n`)
	stdout := &bytes.Buffer{}
	logger := audit.NewLogger(audit.Debug, os.Stdout)

	server := &MockMCPServer{
		stdin:   stdin,
		stdout:  stdout,
		registry: registry,
		logger:   logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.ProcessRequest(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	response := stdout.String()
	if response == "" {
		t.Error("expected non-empty response")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		t.Errorf("failed to parse response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Error("expected result object")
	}
	tools, ok := result["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Errorf("expected 1 tool in tools list")
	}
}

// MockMCPServer for testing without actual IO
type MockMCPServer struct {
	stdin   *bytes.Buffer
	stdout  *bytes.Buffer
	registry *mcp.Registry
	logger   *audit.Logger
}

func (s *MockMCPServer) ProcessRequest(ctx context.Context) error {
	line, err := s.stdin.ReadString('\n')
	if err != nil {
		return err
	}

	req, err := mcp.ParseRequest([]byte(line[:len(line)-1]))
	if err != nil {
		return err
	}

	switch req.Method {
	case "initialize":
		result, _ := s.registry.HandleInitialize(req.Params)
		resultBytes, _ := json.Marshal(result)
		resp := mcp.Response{
			Jsonrpc: "2.0",
			Result:  resultBytes,
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		s.stdout.Write(data)
		s.stdout.Write([]byte("\n"))
	case "tools/list":
		tools := s.registry.List()
		resultBytes, _ := json.Marshal(map[string]interface{}{"tools": tools})
		resp := mcp.Response{
			Jsonrpc: "2.0",
			Result:  resultBytes,
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		s.stdout.Write(data)
		s.stdout.Write([]byte("\n"))
	default:
		resp := mcp.Response{
			Jsonrpc: "2.0",
			Error:   &mcp.Error{Code: -32601, Message: "Method not found"},
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		s.stdout.Write(data)
		s.stdout.Write([]byte("\n"))
	}

	return nil
}