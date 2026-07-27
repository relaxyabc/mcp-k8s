package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/cluster"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
	"github.com/relaxyabc/mcp-k8s/src/security"
)

// MakeUploadFileHandler 创建 upload_file 工具处理器
func MakeUploadFileHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		startTime := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.UploadFileParams](params)
		if err != nil {
			return api.NewErrorResponse(api.ErrInvalidInput, fmt.Sprintf("参数解析失败: %v", err)), nil
		}

		auditLogger.Info("upload_file 开始", "cluster", p.Cluster, "pod", p.PodName, "localPath", p.LocalPath, "targetDir", p.TargetDir)

		// 验证集群参数（必需）
		if err := security.RequireClusterParameter(p.Cluster); err != nil {
			return api.NewErrorResponse(api.ErrClusterParameterRequired, err.Error()), nil
		}

		// 验证特权模式（上传需要特权模式）
		if err := security.RequirePrivilegedMode(); err != nil {
			return api.NewErrorResponse(api.ErrPrivilegedModeRequired, err.Error()), nil
		}

		// 验证本地文件存在性
		if err := security.ValidateLocalFileExists(p.LocalPath); err != nil {
			return api.NewErrorResponse(api.ErrFileNotFound, err.Error()), nil
		}

		auditLogger.Info("upload_file 验证文件大小", "localPath", p.LocalPath)

		// 验证文件大小（10MB 限制）
		if err := security.ValidateFileSize(p.LocalPath); err != nil {
			return api.NewErrorResponse(api.ErrFileTooLarge, err.Error()), nil
		}

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

		auditLogger.Info("upload_file 开始传输")

		// 执行上传（添加 60 秒超时）
		uploadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		auditLogger.Info("upload_file 调用 UploadFile 前", "namespace", p.Namespace, "pod", p.PodName, "targetDir", p.TargetDir)
		result, err := client.UploadFile(uploadCtx, p.Namespace, p.PodName, p.LocalPath, p.TargetDir, p.FileName)
		auditLogger.Info("upload_file UploadFile 返回", "error", err)

		if err != nil {
			// 记录失败日志
			auditLogger.LogError("upload_file", fmt.Sprintf("上传失败: %v", err))
			auditLogger.LogToolCallPrivileged("upload_file", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrUploadFailed, err.Error()), nil
		}

		// 记录成功日志
		auditLogger.Info("upload_file 完成", "targetPath", result.TargetPath, "fileSize", result.FileSize, "duration", result.Duration)
		auditLogger.LogToolCallPrivileged("upload_file", p, "success", time.Since(startTime).Milliseconds())

		// 返回成功响应
		return api.NewSuccessResponse(map[string]interface{}{
			"success":       true,
			"message":       "文件上传成功",
			"targetPath":    result.TargetPath,
			"backupCreated": result.BackupCreated,
			"backupPath":    result.BackupPath,
			"fileSize":      result.FileSize,
			"duration":      result.Duration,
			"privileged":    true,
		}), nil
	}
}

// RegisterUploadFile 注册 upload_file 工具
func registerUploadFile(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"localPath": {"type": "string", "description": "本地文件路径（绝对路径）"},
			"podName": {"type": "string", "description": "目标 Pod 名称"},
			"namespace": {"type": "string", "description": "目标命名空间"},
			"targetDir": {"type": "string", "description": "Pod 内目标目录路径（绝对路径）"},
			"cluster": {"type": "string", "description": "目标集群名称（必需）"},
			"fileName": {"type": "string", "description": "目标文件名（可选，如未指定使用本地文件名）"}
		},
		"required": ["localPath", "podName", "namespace", "targetDir", "cluster"]
	}`)

	registry.Register(
		"upload_file",
		"上传本地文件到 Kubernetes Pod 的指定目录。如果目标位置已存在文件，则自动创建时间戳备份。需要特权模式。",
		schema,
		MakeUploadFileHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}