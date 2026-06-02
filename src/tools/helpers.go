package tools

// IsNotFoundError 检查错误是否为 Kubernetes 未找到错误
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "not found" || containsNotFound(msg)
}

// containsNotFound 检查字符串是否包含 "not found" 模式
func containsNotFound(s string) bool {
	return len(s) > 0 && (s == "not found" || (len(s) >= 10 && s[len(s)-10:] == "not found"))
}