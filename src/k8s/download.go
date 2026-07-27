package k8s

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadResult 文件下载结果
type DownloadResult struct {
	LocalPath     string // 本地文件路径
	BackupCreated  bool   // 是否创建了本地备份
	BackupPath     string // 备份文件路径
	FileSize       int64  // 文件大小
	Duration       int64  // 耗时（毫秒）
}

// DownloadFile 从 Pod 下载文件到本地
func (c *Client) DownloadFile(ctx context.Context, namespace, podName, remotePath, localPath string) (*DownloadResult, error) {
	startTime := time.Now()

	// 检查 Pod 内文件是否存在
	fileExists, err := c.checkFileExistsInPod(ctx, namespace, podName, remotePath)
	if err != nil {
		return nil, fmt.Errorf("检查远程文件失败: %w", err)
	}
	if !fileExists {
		return nil, fmt.Errorf("Pod 内文件不存在: %s", remotePath)
	}

	// 获取远程文件大小
	remoteFileSize, err := c.getRemoteFileSize(ctx, namespace, podName, remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取远程文件大小失败: %w", err)
	}

	// 决定是否压缩下载
	// .log 文件 或 > 1MB 的文件需要压缩
	isLogFile := strings.HasSuffix(remotePath, ".log")
	shouldCompress := isLogFile || remoteFileSize > 1*1024*1024

	// 检查本地文件是否存在
	var backupPath string
	var backupCreated bool

	if _, err := os.Stat(localPath); err == nil {
		// 文件存在，创建备份
		backupPath = fmt.Sprintf("%s.backup.%s", localPath, time.Now().Format("20060102-150405"))
		if err := backupLocalFile(localPath, backupPath); err != nil {
			return nil, fmt.Errorf("创建本地备份失败: %w", err)
		}
		backupCreated = true
	}

	// 确保本地父目录存在
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("创建本地目录失败: %w", err)
	}

	var stdout, stderr *bytes.Buffer
		var shellCmd string

		if shouldCompress {
			// 压缩下载：tar + gzip
			shellCmd = fmt.Sprintf("tar czf - %s", remotePath)
		} else {
			// 直接下载：cat
			shellCmd = fmt.Sprintf("cat %s", remotePath)
		}

		// 使用 LogHandler.ExecCommandBytes（处理二进制数据）
		handler := NewLogHandler(c)
		stdout, stderr, err = handler.ExecCommandBytes(ctx, namespace, podName, "", shellCmd)
		if err != nil {
			return nil, fmt.Errorf("下载文件失败: %w, stderr: %s", err, stderr.String())
		}

		// 处理下载内容
		var fileContent []byte
		if shouldCompress {
			// 解压 tar.gz 归档
			fileContent, err = extractFileFromTarGz(stdout, filepath.Base(remotePath))
			if err != nil {
				return nil, fmt.Errorf("解压文件失败: %w", err)
			}
		} else {
			// 直接使用文件内容
			fileContent = stdout.Bytes()
		}

	// 写入本地文件
	if err := os.WriteFile(localPath, fileContent, 0644); err != nil {
		return nil, fmt.Errorf("写入本地文件失败: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()

	return &DownloadResult{
		LocalPath:     localPath,
		BackupCreated:  backupCreated,
		BackupPath:     backupPath,
		FileSize:       remoteFileSize,
		Duration:       duration,
	}, nil
}

// getRemoteFileSize 获取 Pod 内文件大小（内部方法）
func (c *Client) getRemoteFileSize(ctx context.Context, namespace, podName, filePath string) (int64, error) {
	// 使用 LogHandler.ExecCommand（和 read_pod_logs 一致）
	handler := NewLogHandler(c)
	shellCmd := fmt.Sprintf("stat -c %%s %s", filePath)

	stdout, stderr, err := handler.ExecCommand(ctx, namespace, podName, "", shellCmd)
	if err != nil {
		return 0, fmt.Errorf("获取文件大小失败: %w, stderr: %s", err, stderr)
	}

	// 解析输出
	var size int64
	_, err = fmt.Sscanf(stdout, "%d", &size)
	if err != nil {
		return 0, fmt.Errorf("解析文件大小失败: %w", err)
	}

	return size, nil
}

// GetRemoteFileSize 获取 Pod 内文件大小（公开方法）
func GetRemoteFileSize(ctx context.Context, client *Client, namespace, podName, filePath string) (int64, error) {
	return client.getRemoteFileSize(ctx, namespace, podName, filePath)
}

// backupLocalFile 备份本地文件
func backupLocalFile(sourcePath, backupPath string) error {
	input, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, input, 0644)
}

// extractFileFromTarGz 从 tar.gz 归档中提取文件
func extractFileFromTarGz(tarGzData io.Reader, fileName string) ([]byte, error) {
	// 创建 gzip reader
	gzReader, err := gzip.NewReader(tarGzData)
	if err != nil {
		return nil, fmt.Errorf("解压 gzip 失败: %w", err)
	}
	defer gzReader.Close()

	// 创建 tar reader
	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// 查找目标文件
		if filepath.Base(header.Name) == fileName {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			return content, nil
		}
	}

	return nil, fmt.Errorf("在压缩包中未找到文件: %s", fileName)
}