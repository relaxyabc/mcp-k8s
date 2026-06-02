package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/cluster"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
)

// MakeReadPodLogsHandler 创建 read_pod_logs 工具处理器
func MakeReadPodLogsHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		start := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.ReadPodLogsParams](params)
		if err != nil {
			auditLogger.LogError("read_pod_logs", fmt.Sprintf("参数无效: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		// 验证必填字段
		if p.Namespace == "" || p.PodName == "" || p.LogDir == "" || p.LogFile == "" {
			return api.NewErrorResponse(api.ErrInvalidInput, "namespace, podName, logDir 和 logFile 是必填字段"), nil
		}

		// 获取集群
		loadedCluster, err := clusterMgr.GetCluster(p.Cluster)
		if err != nil {
			auditLogger.LogError("read_pod_logs", err.Error())
			return api.NewErrorResponse(api.ErrClusterNotFound, fmt.Sprintf("%v。可用集群: %v", err, clusterMgr.ListClusters())), nil
		}

		// 验证 namespace 访问权限
		if err := clusterMgr.ValidateNamespace(p.Cluster, p.Namespace); err != nil {
			auditLogger.LogError("read_pod_logs", err.Error())
			return api.NewErrorResponse(api.ErrNamespaceForbidden, err.Error()), nil
		}

		// 创建日志处理器
		k8sClient, ok := loadedCluster.Client.(*k8s.Client)
		if !ok {
			return api.NewErrorResponse(api.ErrInternal, "客户端类型错误"), nil
		}
		handler := k8s.NewLogHandler(k8sClient)

		// 构建完整日志路径
		logPath := p.LogDir + "/" + p.LogFile

		// 如果未指定则获取默认容器
		container := p.Container
		if container == "" {
			container, err = handler.GetDefaultContainer(ctx, p.Namespace, p.PodName)
			if err != nil {
				auditLogger.LogError("read_pod_logs", err.Error())
				return api.NewErrorResponse(api.ErrNotFound, err.Error()), nil
			}
		}

		var result *api.LogContent
		var readErr error

		// 处理跟踪模式
		if p.Follow {
			followDuration := p.FollowDuration
			if followDuration <= 0 {
				followDuration = 10
			}
			if followDuration > 60 {
				followDuration = 60
			}
			pattern := ""
			if p.Operation == "tail-grep" {
				pattern = p.Pattern
			}
			result, readErr = handler.ReadLogFollow(ctx, p.Namespace, p.PodName, container, logPath, followDuration, pattern)
		} else {
			// 正常读取操作
			operation := p.Operation
			if operation == "" {
				operation = "tail"
			}
			lines := p.Lines
			if lines <= 0 {
				lines = 100
			}
			result, readErr = handler.ReadLogWithOperation(ctx, p.Namespace, p.PodName, container, logPath, operation, lines, p.Pattern)
		}

		// 处理错误
		if readErr != nil {
			auditLogger.LogError("read_pod_logs", readErr.Error())
			if strings.Contains(readErr.Error(), "因安全原因被禁止访问") {
				return api.NewErrorResponse(api.ErrSensitivePathDenied, readErr.Error()), nil
			}
			if IsNotFoundError(readErr) || strings.Contains(readErr.Error(), "no such file") {
				return api.NewErrorResponse(api.ErrLogFileNotFound, readErr.Error()), nil
			}
			return api.NewErrorResponse(api.ErrInternal, readErr.Error()), nil
		}

		// 记录成功日志
		duration := time.Since(start).Milliseconds()
		auditLogger.LogToolCall("read_pod_logs", p, "success", duration)

		return api.NewSuccessResponse(result), nil
	}
}