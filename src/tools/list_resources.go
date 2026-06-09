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

// MakeListResourcesHandler 创建 list_resources 工具处理器
func MakeListResourcesHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		start := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.ListResourcesParams](params)
		if err != nil {
			auditLogger.LogError("list_resources", fmt.Sprintf("参数无效: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		// 特权模式确认流程
		if security.PrivilegedMode {
			if !p.Confirmed {
				message := fmt.Sprintf("特权模式：即将列出 %s 资源 (namespace: %s)", p.ResourceType, p.Namespace)
				op := security.CreateConfirmation("list_resources", p, message)
				return api.NewConfirmationResponse(op.ID, p, message), nil
			}
			// 验证确认
			if _, ok := security.ValidateConfirmation(p.OperationID); !ok {
				return api.NewErrorResponse(api.ErrConfirmationExpired, "操作未确认或已过期，请重新发起请求"), nil
			}
			// 记录审计日志
			defer auditLogger.LogPrivilegedOperation(ctx, &audit.PrivilegedOp{
				Type:      "list_resources",
				Resource:  p.ResourceType,
				Namespace: p.Namespace,
				Cluster:   p.Cluster,
				Details:   p,
				Confirmed: true,
				Status:    "success",
			})
		}

		// 获取集群
		loadedCluster, err := clusterMgr.GetCluster(p.Cluster)
		if err != nil {
			auditLogger.LogError("list_resources", err.Error())
			return api.NewErrorResponse(api.ErrClusterNotFound, fmt.Sprintf("%v。可用集群: %v", err, clusterMgr.ListClusters())), nil
		}

		// 创建资源处理器
		k8sClient, ok := loadedCluster.Client.(*k8s.Client)
		if !ok {
			return api.NewErrorResponse(api.ErrInternal, "客户端类型错误"), nil
		}
		handler := k8s.NewResourceHandler(k8sClient)

		// 确定 namespace
		ns := p.Namespace
		if ns == "" {
			ns = defaultNamespace
		}

		// 验证 namespace 访问权限 (特权模式下跳过)
		if !security.PrivilegedMode && p.ResourceType != "namespaces" {
			if err := clusterMgr.ValidateNamespace(p.Cluster, ns); err != nil {
				auditLogger.LogError("list_resources", err.Error())
				return api.NewErrorResponse(api.ErrNamespaceForbidden, err.Error()), nil
			}
		}

		// 执行查询
		var result []api.ResourceSummary
		var listErr error

		switch p.ResourceType {
		case "namespaces":
			result, listErr = handler.ListNamespaces(ctx)
		case "pods":
			result, listErr = handler.ListPods(ctx, ns)
		case "deployments":
			result, listErr = handler.ListDeployments(ctx, ns)
		case "services":
			result, listErr = handler.ListServices(ctx, ns)
		case "jobs":
			result, listErr = handler.ListJobs(ctx, ns)
		case "configmaps":
			result, listErr = handler.ListConfigMaps(ctx, ns)
		default:
			return api.NewErrorResponse(api.ErrInvalidInput, fmt.Sprintf("未知资源类型: %s", p.ResourceType)), nil
		}

		// 处理错误
		if listErr != nil {
			auditLogger.LogError("list_resources", listErr.Error())
			return api.NewErrorResponse(api.ErrInternal, listErr.Error()), nil
		}

		// 记录成功日志
		duration := time.Since(start).Milliseconds()
		auditLogger.LogToolCall("list_resources", p, "success", duration)

		return api.NewSuccessResponse(result), nil
	}
}