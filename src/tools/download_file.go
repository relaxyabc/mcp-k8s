package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/cluster"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
	"github.com/relaxyabc/mcp-k8s/src/security"
)

// MakeDownloadFileHandler 创建 download_file 工具处理器
func MakeDownloadFileHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		startTime := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.DownloadFileParams](params)
		if err != nil {
			return api.NewErrorResponse(api.ErrInvalidInput, fmt.Sprintf("参数解析失败: %v", err)), nil
		}

		auditLogger.Info("download_file 开始", "cluster", p.Cluster, "pod", p.PodName, "remotePath", p.RemotePath, "localPath", p.LocalPath)

		// 验证集群参数（必需）
		if err := security.RequireClusterParameter(p.Cluster); err != nil {
			return api.NewErrorResponse(api.ErrClusterParameterRequired, err.Error()), nil
		}

		// 下载不需要特权模式，但需要验证文件大小
		// 获取集群客户端
		loadedCluster, err := clusterMgr.GetCluster(p.Cluster)
		if err != nil {
			return api.NewErrorResponse(api.ErrClusterNotFound, fmt.Sprintf("集群未找到: %s", p.Cluster)), nil
		}

		// 类型断言获取 k8s.Client
		client, ok := loadedCluster.Client.(*k8s.Client)
		if !ok {
			return api.NewErrorResponse(api.ErrInternal, "客户端类型错误"), nil
		}

		// 验证命名空间访问权限
		if !clusterMgr.IsNamespaceAllowed(p.Cluster, p.Namespace) {
			return api.NewErrorResponse(api.ErrNamespaceForbidden, fmt.Sprintf("命名空间 %s 不在白名单内", p.Namespace)), nil
		}

		auditLogger.Info("download_file 检查远程文件", "remotePath", p.RemotePath)

		// 预检查文件大小（添加 30 秒超时）
		fileSizeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		remoteFileSize, err := k8s.GetRemoteFileSize(fileSizeCtx, client, p.Namespace, p.PodName, p.RemotePath)
		cancel()
		if err != nil {
			auditLogger.LogError("download_file", fmt.Sprintf("获取远程文件信息失败: %v", err))
			auditLogger.LogToolCall("download_file", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrFileNotFound, fmt.Sprintf("无法获取远程文件信息: %v", err)), nil
		}

		auditLogger.Info("download_file 远程文件大小", "sizeBytes", remoteFileSize, "sizeKB", remoteFileSize/1024)

		// 验证文件大小（10MB 限制）
		maxSizeBytes := int64(security.DefaultMaxFileSizeMB) * 1024 * 1024
		if remoteFileSize > maxSizeBytes {
			errMsg := fmt.Sprintf("文件大小 %dMB 超过 %dMB 限制，建议使用 read_pod_logs 工具先过滤", remoteFileSize/(1024*1024), security.DefaultMaxFileSizeMB)
			auditLogger.LogError("download_file", errMsg)
			auditLogger.LogToolCall("download_file", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrFileTooLarge, errMsg), nil
		}

		// 验证本地父目录存在性
		localDir := filepath.Dir(p.LocalPath)
		if err := security.ValidateLocalDirWritable(localDir); err != nil {
			auditLogger.LogError("download_file", fmt.Sprintf("本地目录验证失败: %v", err))
			auditLogger.LogToolCall("download_file", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrInvalidInput, err.Error()), nil
		}

		auditLogger.Info("download_file 开始传输", "shouldCompress", remoteFileSize > 1*1024*1024 || strings.HasSuffix(p.RemotePath, ".log"))

		// 执行下载（添加 60 秒超时）
		downloadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		result, err := client.DownloadFile(downloadCtx, p.Namespace, p.PodName, p.RemotePath, p.LocalPath)
		if err != nil {
			auditLogger.LogError("download_file", fmt.Sprintf("下载失败: %v", err))
			auditLogger.LogToolCall("download_file", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrDownloadFailed, err.Error()), nil
		}

		// 记录成功日志
		auditLogger.Info("download_file 完成", "fileSize", result.FileSize, "duration", result.Duration)
		auditLogger.LogToolCall("download_file", p, "success", time.Since(startTime).Milliseconds())

		// 返回成功响应
		return api.NewSuccessResponse(map[string]interface{}{
			"success":       true,
			"message":       "文件下载成功",
			"localPath":     result.LocalPath,
			"backupCreated": result.BackupCreated,
			"backupPath":    result.BackupPath,
			"fileSize":      result.FileSize,
			"duration":      result.Duration,
			"privileged":    false,
		}), nil
	}
}

// RegisterDownloadFile 注册 download_file 工具
func registerDownloadFile(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"podName": {"type": "string", "description": "源 Pod 名称"},
			"namespace": {"type": "string", "description": "源命名空间"},
			"remotePath": {"type": "string", "description": "Pod 内文件路径（绝对路径）"},
			"localPath": {"type": "string", "description": "本地目标路径（绝对路径）"},
			"cluster": {"type": "string", "description": "源集群名称（必需）"}
		},
		"required": ["podName", "namespace", "remotePath", "localPath", "cluster"]
	}`)

	registry.Register(
		"download_file",
		"从 Kubernetes Pod 下载文件到本地。如果本地已存在同名文件，则自动创建时间戳备份。不需要特权模式。",
		schema,
		MakeDownloadFileHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}