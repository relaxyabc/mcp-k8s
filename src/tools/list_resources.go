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

		// 验证 namespace 访问权限 (namespace 类型不需要验证)
		if p.ResourceType != "namespaces" {
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