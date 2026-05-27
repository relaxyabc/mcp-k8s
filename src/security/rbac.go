package security

import (
	"context"
	"strings"
)

// ForbiddenPaths Pod 日志读取禁止访问的路径
var ForbiddenPaths = []string{
	"/etc/secrets",
	"/root",
	"/home",
	"/.ssh",
	"/.kube",
	"/var/run/secrets",
}

// ValidatePath 检查路径是否安全可访问
func ValidatePath(path string) bool {
	for _, forbidden := range ForbiddenPaths {
		if strings.HasPrefix(path, forbidden) || strings.Contains(path, forbidden) {
			return false
		}
	}
	// 同时检查主目录模式
	if strings.Contains(path, "~/.ssh") || strings.Contains(path, "~/.kube") {
		return false
	}
	return true
}

// CheckRBAC 在执行操作前验证 RBAC 权限
func CheckRBAC(ctx context.Context, verbs, resources, apiGroups []string) error {
	// TODO: 使用 client-go 实现实际的 RBAC 检查
	// 这需要 SubjectAccessReview 请求
	return nil
}