package tools

import (
	"encoding/json"

	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/cluster"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
	"github.com/relaxyabc/mcp-k8s/src/security"
)

// RegisterAll 注册所有 MCP 工具
func RegisterAll(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	registerListResources(registry, clusterMgr, auditLogger, defaultNamespace)
	registerGetResource(registry, clusterMgr, auditLogger, defaultNamespace)
	registerReadPodLogs(registry, clusterMgr, auditLogger, defaultNamespace)
}

// registerListResources 注册 list_resources 工具
func registerListResources(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"cluster": {"type": "string", "description": "集群名称 (可选，默认使用 defaultCluster)"},
			"resourceType": {"type": "string", "enum": ["pods", "deployments", "services", "jobs", "configmaps", "namespaces"]},
			"namespace": {"type": "string", "description": "目标 namespace (namespace 资源类型时忽略此参数)"},
			"confirmed": {"type": "boolean", "description": "特权模式：用户已确认执行此操作"},
			"operationId": {"type": "string", "description": "特权模式：待确认操作的 ID"}
		},
		"required": ["resourceType"]
	}`)

	desc := "列出 Kubernetes 资源 (仅只读)。支持: pods, deployments, services, jobs, configmaps, namespaces"
	if security.PrivilegedMode {
		desc = "列出 Kubernetes 资源。特权模式下操作需用户确认"
	}

	registry.Register(
		"list_resources",
		desc,
		schema,
		MakeListResourcesHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}

// registerGetResource 注册 get_resource 工具
func registerGetResource(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"cluster": {"type": "string", "description": "集群名称 (可选)"},
			"resourceType": {"type": "string", "enum": ["pod", "deployment", "service", "job", "configmap", "secret"]},
			"namespace": {"type": "string"},
			"name": {"type": "string"},
			"confirmed": {"type": "boolean", "description": "特权模式：用户已确认执行此操作"},
			"operationId": {"type": "string", "description": "特权模式：待确认操作的 ID"}
		},
		"required": ["resourceType", "namespace", "name"]
	}`)

	desc := "获取 Kubernetes 资源详情 (仅只读)。自动脱敏 secrets"
	if security.PrivilegedMode {
		desc = "获取 Kubernetes 资源详情。特权模式下 Secret 数据不脱敏，操作需用户确认"
	}

	registry.Register(
		"get_resource",
		desc,
		schema,
		MakeGetResourceHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}

// registerReadPodLogs 注册 read_pod_logs 工具
func registerReadPodLogs(registry *mcp.Registry, clusterMgr *cluster.Manager, auditLogger *audit.Logger, defaultNamespace string) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"cluster": {"type": "string", "description": "集群名称 (可选)"},
			"namespace": {"type": "string"},
			"podName": {"type": "string"},
			"container": {"type": "string"},
			"logDir": {"type": "string"},
			"logFile": {"type": "string"},
			"operation": {"type": "string", "enum": ["tail", "head", "grep", "cat", "tail-grep", "cat-grep", "head-grep"], "default": "tail"},
			"lines": {"type": "integer", "default": 100},
			"pattern": {"type": "string"},
			"follow": {"type": "boolean", "default": false},
			"followDuration": {"type": "integer", "default": 10},
			"confirmed": {"type": "boolean", "description": "特权模式：用户已确认执行此操作"},
			"operationId": {"type": "string", "description": "特权模式：待确认操作的 ID"}
		},
		"required": ["namespace", "podName", "logDir", "logFile"]
	}`)

	desc := "进入 Pod 容器读取日志文件 (仅只读)。支持管道组合: tail | grep, cat | grep"
	if security.PrivilegedMode {
		desc = "进入 Pod 容器读取文件。特权模式下允许所有路径和命令，操作需用户确认"
	}

	registry.Register(
		"read_pod_logs",
		desc,
		schema,
		MakeReadPodLogsHandler(clusterMgr, auditLogger, defaultNamespace),
	)
}