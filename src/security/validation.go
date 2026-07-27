package security

import (
	"fmt"
	"os"
)

const (
	// DefaultMaxFileSizeMB 默认文件大小限制（MB）
	DefaultMaxFileSizeMB = 10

	// BytesPerMB 每MB的字节数
	BytesPerMB = 1024 * 1024
)

// ValidateFileSize 验证文件大小是否超过限制（固定 10MB）
func ValidateFileSize(filePath string) error {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("本地文件不存在: %s", filePath)
		}
		return fmt.Errorf("无法读取文件信息: %s", filePath)
	}

	// 检查文件大小
	fileSizeMB := fileInfo.Size() / BytesPerMB
	maxSizeBytes := int64(DefaultMaxFileSizeMB) * BytesPerMB

	if fileInfo.Size() > maxSizeBytes {
		return fmt.Errorf("文件大小 %dMB 超过限制 %dMB，建议先使用 grep 过滤", fileSizeMB, DefaultMaxFileSizeMB)
	}

	return nil
}

// ValidateLocalFileExists 验证本地文件是否存在
func ValidateLocalFileExists(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("本地文件不存在: %s", filePath)
		}
		return fmt.Errorf("无法访问文件: %s", filePath)
	}
	return nil
}

// ValidateLocalFileReadable 验证本地文件是否可读
func ValidateLocalFileReadable(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("无法读取文件: %s", filePath)
	}
	file.Close()
	return nil
}

// ValidateLocalDirWritable 验证本地目录是否可写
func ValidateLocalDirWritable(dirPath string) error {
	// 检查目录是否存在
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("本地目录不存在: %s", dirPath)
		}
		return fmt.Errorf("无法访问目录: %s", dirPath)
	}

	// 检查是否是目录
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", dirPath)
	}

	// 尝试创建临时文件验证写权限
	tmpFile := dirPath + "/.write_test"
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("目录无写入权限: %s", dirPath)
	}
	f.Close()
	os.Remove(tmpFile)

	return nil
}

// GetFileSizeMB 获取文件大小（MB）
func GetFileSizeMB(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size() / BytesPerMB, nil
}