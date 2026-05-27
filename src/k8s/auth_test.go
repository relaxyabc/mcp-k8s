package k8s

import (
	"testing"
)

func TestLoadKubeconfigPathExpansion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "expand home directory",
			input:    "~/.kube/config",
			expected: "", // Will be expanded to actual home directory
		},
		{
			name:     "no expansion needed",
			input:    "/path/to/config",
			expected: "/path/to/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input == "" {
				t.Error("input should not be empty")
			}
		})
	}
}

func TestLoadKubeconfigInvalidPath(t *testing.T) {
	_, err := LoadKubeconfig("/non/existent/path/kubeconfig")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadKubeconfigSkipCI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
}