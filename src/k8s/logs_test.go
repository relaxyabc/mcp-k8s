package k8s

import (
	"testing"
)

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name        string
		logPath     string
		operation   string
		lines       int
		pattern     string
		expectedCmd string
	}{
		{
			name:        "tail operation",
			logPath:     "/var/log/app/info.log",
			operation:   "tail",
			lines:       100,
			pattern:     "",
			expectedCmd: "tail -100 /var/log/app/info.log",
		},
		{
			name:        "head operation",
			logPath:     "/var/log/app/info.log",
			operation:   "head",
			lines:       50,
			pattern:     "",
			expectedCmd: "head -50 /var/log/app/info.log",
		},
		{
			name:        "cat operation",
			logPath:     "/var/log/app/info.log",
			operation:   "cat",
			lines:       0,
			pattern:     "",
			expectedCmd: "cat /var/log/app/info.log",
		},
		{
			name:        "grep operation",
			logPath:     "/var/log/app/error.log",
			operation:   "grep",
			lines:       0,
			pattern:     "ERROR",
			expectedCmd: "grep ERROR /var/log/app/error.log",
		},
		{
			name:        "tail-grep operation",
			logPath:     "/var/log/app/error.log",
			operation:   "tail-grep",
			lines:       200,
			pattern:     "timeout",
			expectedCmd: "tail -200 /var/log/app/error.log | grep timeout",
		},
		{
			name:        "cat-grep operation",
			logPath:     "/var/log/app/error.log",
			operation:   "cat-grep",
			lines:       0,
			pattern:     "ERROR",
			expectedCmd: "cat /var/log/app/error.log | grep ERROR",
		},
		{
			name:        "head-grep operation",
			logPath:     "/var/log/app/lib.log",
			operation:   "head-grep",
			lines:       50,
			pattern:     "init",
			expectedCmd: "head -50 /var/log/app/lib.log | grep init",
		},
		{
			name:        "default operation (unknown)",
			logPath:     "/var/log/app/info.log",
			operation:   "unknown",
			lines:       100,
			pattern:     "",
			expectedCmd: "tail -100 /var/log/app/info.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCommand(tt.logPath, tt.operation, tt.lines, tt.pattern)
			if result != tt.expectedCmd {
				t.Errorf("buildCommand() = '%s', expected '%s'", result, tt.expectedCmd)
			}
		})
	}
}

func TestBuildCommandSecurity(t *testing.T) {
	operations := []string{"tail", "head", "cat", "grep", "tail-grep", "cat-grep", "head-grep"}
	for _, op := range operations {
		cmd := buildCommand("/var/log/app/test.log", op, 100, "pattern")
		// Verify no dangerous characters - simple check
		dangerous := []string{"rm", "mv", "cp", "chmod", "chown"}
		for _, d := range dangerous {
			if len(cmd) >= len(d) && cmd[:len(d)] == d {
				t.Errorf("buildCommand for operation '%s' produced dangerous command: %s", op, cmd)
			}
		}
	}
}