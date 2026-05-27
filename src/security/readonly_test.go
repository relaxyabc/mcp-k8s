package security

import (
	"testing"
)

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		command  string
		expected bool
	}{
		{"cat", true},
		{"tail", true},
		{"head", true},
		{"grep", true},
		{"ls", true},
		{"rm", false},
		{"mv", false},
		{"cp", false},
		{"write", false},
		{"edit", false},
		{"chmod", false},
		{"chown", false},
		{"echo", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := ValidateCommand(tt.command)
			if result != tt.expected {
				t.Errorf("ValidateCommand(%s) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestValidateShellCommand(t *testing.T) {
	tests := []struct {
		command  string
		expected bool
	}{
		{"tail -100 /var/log/app.log", true},
		{"cat /var/log/app.log | grep ERROR", true},
		{"tail -f /var/log/app.log | grep timeout", true},
		{"head -50 /var/log/app.log", true},
		{"rm /var/log/app.log", false},
		{"mv /var/log/app.log /tmp/", false},
		{"cat /var/log/app.log > /tmp/output", false},
		{"echo 'test' >> /var/log/app.log", false},
		{"chmod 777 /var/log/app.log", false},
		{"cat /var/log/app.log | grep error", true},
		// 测试路径中包含禁止命令子字符串的情况（不应被误判）
		{"grep 'pattern' /apps/logs/titan-security-center-blue-7666774dc7-lkpzh/*", true},
		{"tail -100 /var/log/titan-chown-test/app.log", true},
		{"cat /var/log/app-rm-backup.log", true},
		// 测试管道中包含禁止命令
		{"cat /var/log/app.log | rm something", false},
		{"tail -100 /var/log/app.log | chmod 777", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := ValidateShellCommand(tt.command)
			if result != tt.expected {
				t.Errorf("ValidateShellCommand(%s) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/var/log/app/info.log", true},
		{"/var/log/app/error.log", true},
		{"/app/logs/request.log", true},
		{"/etc/secrets/credentials.txt", false},
		{"/root/.bashrc", false},
		{"/home/user/.ssh/id_rsa", false},
		{"~/.kube/config", false},
		{"~/.ssh/known_hosts", false},
		{"/var/run/secrets/kubernetes.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ValidatePath(tt.path)
			if result != tt.expected {
				t.Errorf("ValidatePath(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsReadOnlyVerb(t *testing.T) {
	tests := []struct {
		verb     string
		expected bool
	}{
		{"get", true},
		{"list", true},
		{"watch", true},
		{"create", false},
		{"update", false},
		{"delete", false},
		{"patch", false},
	}

	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			result := IsReadOnlyVerb(tt.verb)
			if result != tt.expected {
				t.Errorf("IsReadOnlyVerb(%s) = %v, expected %v", tt.verb, result, tt.expected)
			}
		})
	}
}