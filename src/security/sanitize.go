package security

import (
	corev1 "k8s.io/api/core/v1"
	"strings"
)

// SensitiveKeyPatterns ConfigMap 脱敏的敏感键模式
var SensitiveKeyPatterns = []string{"password", "passwd", "token", "secret", "key", "credential", "auth", "api_key"}

// SanitizeSecret 隐藏 Kubernetes Secret 中的所有数据
func SanitizeSecret(secret *corev1.Secret) map[string]interface{} {
	return map[string]interface{}{
		"name":      secret.Name,
		"namespace": secret.Namespace,
		"type":      secret.Type,
		"labels":    secret.Labels,
		"createdAt": secret.CreationTimestamp,
		"dataKeys":  getKeys(secret.Data),
		"data":      "***REDACTED***",
	}
}

// SanitizeConfigMap 部分脱敏 Kubernetes ConfigMap
func SanitizeConfigMap(cm *corev1.ConfigMap) map[string]interface{} {
	result := map[string]interface{}{
		"name":      cm.Name,
		"namespace": cm.Namespace,
		"labels":    cm.Labels,
		"createdAt": cm.CreationTimestamp,
	}

	// 仅脱敏敏感键
	data := make(map[string]string)
	for key, value := range cm.Data {
		if isSensitiveKey(key) {
			data[key] = "***REDACTED***"
		} else {
			data[key] = value
		}
	}
	result["data"] = data

	return result
}

// isSensitiveKey 检查键名是否匹配敏感模式
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, pattern := range SensitiveKeyPatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	return false
}

// getKeys 返回数据 map 的键列表
func getKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}