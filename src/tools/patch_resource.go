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

// MakePatchResourceHandler 创建 patch_resource 工具处理器
func MakePatchResourceHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		startTime := time.Now()

		// 必须在特权模式下使用
		if !security.PrivilegedMode {
			auditLogger.LogError("patch_resource", "不在特权模式")
			return api.NewErrorResponse(api.ErrForbidden, "patch_resource 仅在特权模式下可用"), nil
		}

		// 解析参数
		p, err := api.ParseParams[api.PatchResourceParams](params)
		if err != nil {
			auditLogger.LogError("patch_resource", fmt.Sprintf("参数解析失败: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		auditLogger.Info("patch_resource 开始", "cluster", p.Cluster, "namespace", p.Namespace, "resourceType", p.ResourceType, "name", p.Name)

		// 验证集群参数（必需）
		if err := security.RequireClusterParameter(p.Cluster); err != nil {
			return api.NewErrorResponse(api.ErrClusterParameterRequired, err.Error()), nil
		}

		// 验证必填字段
		if p.ResourceType == "" || p.Name == "" || p.Patch == "" {
			return api.NewErrorResponse(api.ErrInvalidInput, "resourceType, name 和 patch 是必填字段"), nil
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
		if p.Namespace != "" && !clusterMgr.IsNamespaceAllowed(p.Cluster, p.Namespace) {
			return api.NewErrorResponse(api.ErrNamespaceForbidden, fmt.Sprintf("命名空间 %s 不在白名单内", p.Namespace)), nil
		}

		// 创建写操作处理器
		writeHandler, err := k8s.NewWriteHandler(client)
		if err != nil {
			auditLogger.LogError("patch_resource", fmt.Sprintf("创建写处理器失败: %v", err))
			return api.NewErrorResponse(api.ErrInternal, err.Error()), nil
		}

		// 执行 Patch
		result, err := writeHandler.PatchResource(ctx, p.Namespace, p.ResourceType, p.Name, p.PatchType, []byte(p.Patch))
		if err != nil {
			auditLogger.LogError("patch_resource", fmt.Sprintf("Patch 失败: %v", err))
			auditLogger.LogToolCallPrivileged("patch_resource", p, "failed", time.Since(startTime).Milliseconds())
			return api.NewErrorResponse(api.ErrPatchFailed, err.Error()), nil
		}

		// 记录成功日志
		auditLogger.Info("patch_resource 完成", "kind", result.Kind, "name", result.Name)
		auditLogger.LogToolCallPrivileged("patch_resource", p, "success", time.Since(startTime).Milliseconds())

		return api.NewSuccessResponse(result), nil
	}
}

// registerPatchResource 注册 patch_resource 工具
func registerPatchResource(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"cluster": {"type": "string", "description": "目标集群名称（必需）"},
			"namespace": {"type": "string", "description": "目标命名空间"},
			"resourceType": {"type": "string", "description": "资源类型（如 deployment, service, configmap 等）"},
			"name": {"type": "string", "description": "资源名称"},
			"patchType": {"type": "string", "enum": ["merge", "json"], "description": "Patch 类型（默认 merge）"},
			"patch": {"type": "string", "description": "JSON 格式的 patch 内容"}
		},
		"required": ["cluster", "namespace", "resourceType", "name", "patch"]
	}`)

	registry.Register(
		"patch_resource",
		"对 Kubernetes 资源执行 Patch 操作。支持 merge patch 和 JSON patch。需要特权模式。",
		schema,
		MakePatchResourceHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}