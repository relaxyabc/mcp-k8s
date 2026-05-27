package security

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSanitizeSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	result := SanitizeSecret(secret)

	if result["name"] != "test-secret" {
		t.Errorf("expected name 'test-secret', got %v", result["name"])
	}
	if result["namespace"] != "default" {
		t.Errorf("expected namespace 'default', got %v", result["namespace"])
	}
	if result["data"] != "***REDACTED***" {
		t.Errorf("expected data to be redacted, got %v", result["data"])
	}
	keys := result["dataKeys"].([]string)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestSanitizeConfigMap(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
		},
		Data: map[string]string{
			"config.yaml":  "key: value",
			"password":     "should-be-redacted",
			"API_KEY":      "should-be-redacted",
			"normal-value": "normal-data",
			"app-name":     "my-app",
		},
	}

	result := SanitizeConfigMap(cm)

	if result["name"] != "test-config" {
		t.Errorf("expected name 'test-config', got %v", result["name"])
	}
	data := result["data"].(map[string]string)
	if data["password"] != "***REDACTED***" {
		t.Errorf("expected password to be redacted")
	}
	if data["API_KEY"] != "***REDACTED***" {
		t.Errorf("expected API_KEY to be redacted")
	}
	if data["normal-value"] != "normal-data" {
		t.Errorf("expected normal-value to not be redacted")
	}
	if data["config.yaml"] != "key: value" {
		t.Errorf("expected config.yaml to not be redacted")
	}
	if data["app-name"] != "my-app" {
		t.Errorf("expected app-name to not be redacted")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"passwd", true},
		{"token", true},
		{"secret", true},
		{"api_key", true},
		{"API_KEY", true},
		{"auth", true},
		{"credential", true},
		{"my-key", true},       // contains "key"
		{"encryption_key", true}, // contains "key"
		{"config", false},
		{"data", false},
		{"app-name", false},
		{"username", false},
		{"settings", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			if result != tt.expected {
				t.Errorf("isSensitiveKey(%s) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}