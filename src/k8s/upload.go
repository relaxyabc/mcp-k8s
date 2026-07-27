package k8s

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// UploadResult 文件上传结果
type UploadResult struct {
	TargetPath    string // 目标文件路径
	BackupCreated bool   // 是否创建了备份
	BackupPath    string // 备份文件路径
	FileSize      int64  // 文件大小
	Duration      int64  // 耗时（毫秒）
}

// UploadFile 上传本地文件到 Pod
func (c *Client) UploadFile(ctx context.Context, namespace, podName, localPath, targetDir, targetFileName string) (*UploadResult, error) {
	startTime := time.Now()

	// 获取本地文件信息
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取本地文件: %w", err)
	}

	// 确定目标文件名
	if targetFileName == "" {
		targetFileName = filepath.Base(localPath)
	}

	// 构造目标路径（使用 path.Join 确保使用 / 分隔符）
	targetPath := path.Join(targetDir, targetFileName)

	// 检查目标文件是否已存在
	fileExists, err := c.checkFileExistsInPod(ctx, namespace, podName, targetPath)
	if err != nil {
		return nil, fmt.Errorf("检查目标文件失败: %w", err)
	}

	var backupPath string
	var backupCreated bool

	// 如果文件存在，创建备份
	if fileExists {
		backupPath = fmt.Sprintf("%s.backup.%s", targetPath, time.Now().Format("20060102-150405"))
		if err := c.backupFileInPod(ctx, namespace, podName, targetPath, backupPath); err != nil {
			return nil, fmt.Errorf("创建备份失败: %w", err)
		}
		backupCreated = true
	}

	// 读取本地文件内容
	fileContent, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("读取本地文件失败: %w", err)
	}

	// 根据文件类型选择上传方式
	isArchive := isArchiveFile(targetFileName)

	if isArchive {
		// 压缩包：使用 tar 解压方式
		if err := c.uploadAsArchive(ctx, namespace, podName, targetDir, targetFileName, fileContent, fileInfo); err != nil {
			return nil, err
		}
	} else {
		// 普通文件：直接写入
		if err := c.uploadDirectly(ctx, namespace, podName, targetPath, fileContent); err != nil {
			return nil, err
		}
	}

	duration := time.Since(startTime).Milliseconds()

	return &UploadResult{
		TargetPath:    targetPath,
		BackupCreated: backupCreated,
		BackupPath:    backupPath,
		FileSize:      fileInfo.Size(),
		Duration:      duration,
	}, nil
}

// isArchiveFile 判断是否是压缩包
func isArchiveFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	archiveExts := map[string]bool{
		".tar":    true,
		".tar.gz": true,
		".tgz":    true,
		".tar.bz": true,
		".tbz":    true,
		".tar.xz": true,
		".txz":    true,
		".gz":     true,
		".bz":     true,
		".bz2":    true,
		".xz":     true,
		".zip":    true,
	}

	// 检查双扩展名（如 .tar.gz）
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			doubleExt := strings.ToLower(filename[i:])
			if archiveExts[doubleExt] {
				return true
			}
			break
		}
	}

	return archiveExts[ext]
}

// uploadAsArchive 以压缩包方式上传（解压）
func (c *Client) uploadAsArchive(ctx context.Context, namespace, podName, targetDir, targetFileName string, fileContent []byte, fileInfo os.FileInfo) error {
	// 创建 tar 归档（包含单个文件）
	tarBuffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(tarBuffer)

	header := &tar.Header{
		Name: targetFileName,
		Mode: 0644,
		Size: fileInfo.Size(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("创建 tar 头失败: %w", err)
	}

	if _, err := tarWriter.Write(fileContent); err != nil {
		return fmt.Errorf("写入 tar 内容失败: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("关闭 tar 写入器失败: %w", err)
	}

	// 暂不支持 stdin 传输
	return fmt.Errorf("压缩包上传暂不支持（需要 stdin 传输）")
}

// uploadDirectly 直接写入文件
func (c *Client) uploadDirectly(ctx context.Context, namespace, podName, targetPath string, fileContent []byte) error {
	// 使用 LogHandler.ExecCommand（和 download.go 一致）
	handler := NewLogHandler(c)

	// 小于 10KB，通过命令参数传输
	if len(fileContent) < 10240 {
		tmpPath := targetPath + ".tmp"

		// 使用 base64 编码传输（避免 shell 转义问题）
		encoded := encodeBase64(fileContent)
		shellCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, tmpPath)

		_, stderr, err := handler.ExecCommand(ctx, namespace, podName, "", shellCmd)
		if err != nil {
			return fmt.Errorf("写入文件失败: %w, stderr: %s", err, stderr)
		}

		// 移动临时文件
		mvCmd := fmt.Sprintf("mv %s %s", tmpPath, targetPath)
		_, stderr, err = handler.ExecCommand(ctx, namespace, podName, "", mvCmd)
		if err != nil {
			return fmt.Errorf("移动文件失败: %w, stderr: %s", err, stderr)
		}
		return nil
	}

	// 大文件：暂不支持
	return fmt.Errorf("大文件上传暂不支持（超过 10KB）")
}

// encodeBase64 简单的 base64 编码
func encodeBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				base64Chars[n&0x3F],
			)
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				'=',
			)
		} else {
			n = uint32(data[i])<<16
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				'=',
				'=',
			)
		}
	}

	return string(result)
}

// checkFileExistsInPod 检查 Pod 内文件是否存在
func (c *Client) checkFileExistsInPod(ctx context.Context, namespace, podName, filePath string) (bool, error) {
	// 使用 LogHandler.ExecCommand（和 download.go 一致）
	handler := NewLogHandler(c)
	shellCmd := fmt.Sprintf("ls %s 2>/dev/null", filePath)

	stdout, stderr, err := handler.ExecCommand(ctx, namespace, podName, "", shellCmd)
	if err != nil {
		// 区分超时错误和其他错误
		if strings.Contains(err.Error(), "超时") || strings.Contains(err.Error(), "timeout") {
			return false, fmt.Errorf("检查文件存在性超时: %w, stderr: %s", err, stderr)
		}
		return false, nil
	}
	// ls 成功，说明文件存在
	return len(stdout) > 0, nil
}

// backupFileInPod 在 Pod 内创建文件备份
func (c *Client) backupFileInPod(ctx context.Context, namespace, podName, sourcePath, backupPath string) error {
	// 使用 LogHandler.ExecCommand
	handler := NewLogHandler(c)
	shellCmd := fmt.Sprintf("cp %s %s", sourcePath, backupPath)

	_, stderr, err := handler.ExecCommand(ctx, namespace, podName, "", shellCmd)
	if err != nil {
		return fmt.Errorf("备份文件失败: %w, stderr: %s", err, stderr)
	}

	return nil
}