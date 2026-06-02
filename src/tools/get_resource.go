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

// MakeGetResourceHandler 创建 get_resource 工具处理器
func MakeGetResourceHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		start := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.GetResourceParams](params)
		if err != nil {
			auditLogger.LogError("get_resource", fmt.Sprintf("参数无效: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		// 验证必填字段
		if p.ResourceType == "" || p.Namespace == "" || p.Name == "" {
			return api.NewErrorResponse(api.ErrInvalidInput, "resourceType, namespace 和 name 是必填字段"), nil
		}

		// 获取集群
		loadedCluster, err := clusterMgr.GetCluster(p.Cluster)
		if err != nil {
			auditLogger.LogError("get_resource", err.Error())
			return api.NewErrorResponse(api.ErrClusterNotFound, fmt.Sprintf("%v。可用集群: %v", err, clusterMgr.ListClusters())), nil
		}

		// 验证 namespace 访问权限
		if err := clusterMgr.ValidateNamespace(p.Cluster, p.Namespace); err != nil {
			auditLogger.LogError("get_resource", err.Error())
			return api.NewErrorResponse(api.ErrNamespaceForbidden, err.Error()), nil
		}

		// 创建资源处理器
		k8sClient, ok := loadedCluster.Client.(*k8s.Client)
		if !ok {
			return api.NewErrorResponse(api.ErrInternal, "客户端类型错误"), nil
		}
		handler := k8s.NewResourceHandler(k8sClient)

		var result any
		var getErr error

		switch p.ResourceType {
		case "pod":
			result, getErr = handler.GetPod(ctx, p.Namespace, p.Name)
		case "deployment":
			result, getErr = handler.GetDeployment(ctx, p.Namespace, p.Name)
		case "service":
			result, getErr = handler.GetService(ctx, p.Namespace, p.Name)
		case "job":
			result, getErr = handler.GetJob(ctx, p.Namespace, p.Name)
		case "configmap":
			cm, err := handler.GetConfigMap(ctx, p.Namespace, p.Name)
			if err != nil {
				getErr = err
			} else {
				result = security.SanitizeConfigMap(cm)
			}
		case "secret":
			secret, err := handler.GetSecret(ctx, p.Namespace, p.Name)
			if err != nil {
				getErr = err
			} else {
				result = security.SanitizeSecret(secret)
			}
		default:
			return api.NewErrorResponse(api.ErrInvalidInput, fmt.Sprintf("未知资源类型: %s", p.ResourceType)), nil
		}

		// 处理错误
		if getErr != nil {
			auditLogger.LogError("get_resource", getErr.Error())
			if IsNotFoundError(getErr) {
				return api.NewErrorResponse(api.ErrNotFound, getErr.Error()), nil
			}
			return api.NewErrorResponse(api.ErrInternal, getErr.Error()), nil
		}

		// 记录成功日志
		duration := time.Since(start).Milliseconds()
		auditLogger.LogToolCall("get_resource", p, "success", duration)

		return api.NewSuccessResponse(result), nil
	}
}