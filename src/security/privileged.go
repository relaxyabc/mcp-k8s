package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IsPrivilegedModeEnabled 检查是否启用了特权模式
func IsPrivilegedModeEnabled() bool {
	return PrivilegedMode
}

// RequirePrivilegedMode 检查特权模式，未启用则返回错误
func RequirePrivilegedMode() error {
	if !PrivilegedMode {
		return fmt.Errorf("此操作需要特权模式")
	}
	return nil
}

// ValidateUploadPath 验证上传目标路径是否在白名单中（如果配置了白名单）
func ValidateUploadPath(targetPath string, allowedPaths []string) error {
	// 如果没有配置白名单，允许所有路径
	if len(allowedPaths) == 0 {
		return nil
	}

	// 规范化路径（确保以 / 开头）
	targetPath = filepath.Clean(targetPath)
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}

	// 检查是否在白名单中
	for _, allowed := range allowedPaths {
		allowed = filepath.Clean(allowed)
		if !strings.HasPrefix(allowed, "/") {
			allowed = "/" + allowed
		}
		// 检查目标路径是否以允许的路径开头
		if strings.HasPrefix(targetPath, allowed) {
			return nil
		}
	}

	return fmt.Errorf("目标路径 %s 不在白名单内", targetPath)
}

// IsSensitivePath 检查路径是否为敏感目录
func IsSensitivePath(path string) bool {
	sensitivePaths := []string{
		"/etc/secrets",
		"/root",
		"~/.ssh",
		"/etc/ssh",
		"/var/run/secrets",
	}

	path = filepath.Clean(path)
	for _, sensitive := range sensitivePaths {
		if strings.HasPrefix(path, sensitive) || path == sensitive {
			return true
		}
	}
	return false
}

// ValidateUploadTarget 综合验证上传目标路径
func ValidateUploadTarget(targetPath string, allowedPaths []string) error {
	// 检查是否为敏感路径
	if IsSensitivePath(targetPath) {
		return fmt.Errorf("禁止上传到敏感目录: %s", targetPath)
	}

	// 检查白名单
	if err := ValidateUploadPath(targetPath, allowedPaths); err != nil {
		return err
	}

	return nil
}

// RequireClusterParameter 检查集群参数是否提供（不使用默认集群）
func RequireClusterParameter(cluster string) error {
	if cluster == "" {
		return fmt.Errorf("集群参数必需，请查看 mcp.json 配置文件指定目标集群")
	}
	return nil
}