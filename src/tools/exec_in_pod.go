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

// MakeExecInPodHandler 创建 exec_in_pod 工具处理器（仅特权模式）
func MakeExecInPodHandler(clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		start := time.Now()

		// 必须在特权模式下使用
		if !security.PrivilegedMode {
			return api.NewErrorResponse(api.ErrForbidden, "exec_in_pod 仅在特权模式下可用"), nil
		}

		// 解析参数
		p, err := api.ParseParams[api.ExecInPodParams](params)
		if err != nil {
			auditLogger.LogError("exec_in_pod", fmt.Sprintf("参数无效: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		// 验证必填字段
		if p.Namespace == "" || p.PodName == "" || p.Command == "" {
			return api.NewErrorResponse(api.ErrInvalidInput, "namespace, podName 和 command 是必填字段"), nil
		}

		// 确认流程
		if !p.Confirmed {
			message := fmt.Sprintf("特权模式：即将在 Pod '%s' 中执行命令 '%s' (namespace: %s)", p.PodName, p.Command, p.Namespace)
			op := security.CreateConfirmation("exec_in_pod", p, message)
			return api.NewConfirmationResponse(op.ID, p, message), nil
		}

		// 验证确认
		if _, ok := security.ValidateConfirmation(p.OperationID); !ok {
			return api.NewErrorResponse(api.ErrConfirmationExpired, "操作未确认或已过期，请重新发起请求"), nil
		}

		// 获取集群
		loadedCluster, err := clusterMgr.GetCluster(p.Cluster)
		if err != nil {
			auditLogger.LogError("exec_in_pod", err.Error())
			return api.NewErrorResponse(api.ErrClusterNotFound, fmt.Sprintf("%v。可用集群: %v", err, clusterMgr.ListClusters())), nil
		}

		// 创建日志处理器
		k8sClient, ok := loadedCluster.Client.(*k8s.Client)
		if !ok {
			return api.NewErrorResponse(api.ErrInternal, "客户端类型错误"), nil
		}
		handler := k8s.NewLogHandler(k8sClient)

		// 如果未指定则获取默认容器
		container := p.Container
		if container == "" {
			container, err = handler.GetDefaultContainer(ctx, p.Namespace, p.PodName)
			if err != nil {
				auditLogger.LogError("exec_in_pod", err.Error())
				return api.NewErrorResponse(api.ErrNotFound, err.Error()), nil
			}
		}

		// 执行命令
		stdout, stderr, execErr := handler.ExecCommand(ctx, p.Namespace, p.PodName, container, p.Command)

		// 记录审计日志
		status := "success"
		errMsg := ""
		if execErr != nil {
			status = "failed"
			errMsg = execErr.Error()
		}
		auditLogger.LogPrivilegedOperation(ctx, &audit.PrivilegedOp{
			Type:      "exec_in_pod",
			Resource:  p.PodName,
			Namespace: p.Namespace,
			Cluster:   p.Cluster,
			Details:   fmt.Sprintf("command=%s, container=%s", p.Command, container),
			Confirmed: true,
			Status:    status,
			Error:     errMsg,
		})

		// 处理错误
		if execErr != nil {
			auditLogger.LogError("exec_in_pod", execErr.Error())
			return api.NewErrorResponse(api.ErrInternal, fmt.Sprintf("执行失败: %v, stderr: %s", execErr, stderr)), nil
		}

		// 记录成功日志
		duration := time.Since(start).Milliseconds()
		auditLogger.LogToolCall("exec_in_pod", p, "success", duration)

		return api.NewSuccessResponse(&api.ExecInPodResult{
			Stdout:    stdout,
			Stderr:    stderr,
			Command:   p.Command,
			PodName:   p.PodName,
			Container: container,
		}), nil
	}
}