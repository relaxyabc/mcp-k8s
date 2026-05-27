package mcp

import (
	"encoding/json"
	"testing"
)

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Request
		hasError bool
	}{
		{
			name:  "valid initialize request",
			input: `{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}`,
			expected: &Request{
				Jsonrpc: "2.0",
				Method:  "initialize",
				ID:      float64(1),
			},
			hasError: false,
		},
		{
			name:  "valid tools/list request",
			input: `{"jsonrpc":"2.0","method":"tools/list","id":2}`,
			expected: &Request{
				Jsonrpc: "2.0",
				Method:  "tools/list",
				ID:      float64(2),
			},
			hasError: false,
		},
		{
			name:     "invalid JSON",
			input:    `{invalid}`,
			expected: nil,
			hasError: true,
		},
		{
			name:     "empty input",
			input:    ``,
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(tt.input))
			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if req.Jsonrpc != tt.expected.Jsonrpc {
				t.Errorf("Jsonrpc mismatch: got %s, expected %s", req.Jsonrpc, tt.expected.Jsonrpc)
			}
			if req.Method != tt.expected.Method {
				t.Errorf("Method mismatch: got %s, expected %s", req.Method, tt.expected.Method)
			}
		})
	}
}

func TestFormatResponse(t *testing.T) {
	id := float64(1)
	result := map[string]string{"status": "ok"}

	data, err := FormatResponse(id, result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
		return
	}

	if resp.Jsonrpc != "2.0" {
		t.Errorf("expected Jsonrpc '2.0', got %s", resp.Jsonrpc)
	}
	if resp.ID != id {
		t.Errorf("expected ID %v, got %v", id, resp.ID)
	}
}

func TestFormatError(t *testing.T) {
	id := float64(1)
	code := -32602
	message := "Invalid params"

	data, err := FormatError(id, code, message)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
		return
	}

	if resp.Error == nil {
		t.Error("expected error in response")
		return
	}
	if resp.Error.Code != code {
		t.Errorf("expected error code %d, got %d", code, resp.Error.Code)
	}
	if resp.Error.Message != message {
		t.Errorf("expected error message '%s', got '%s'", message, resp.Error.Message)
	}
}

func TestErrorInterface(t *testing.T) {
	err := &Error{
		Code:    -32601,
		Message: "Method not found",
	}

	if err.Error() != "Method not found" {
		t.Errorf("Error() returned '%s', expected 'Method not found'", err.Error())
	}
}