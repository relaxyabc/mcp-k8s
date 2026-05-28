package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/logger"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
	"github.com/relaxyabc/mcp-k8s/src/security"
	"github.com/urfave/cli/v2"
)

// 版本信息（通过 ldflags 在构建时设置）
var (
	Version   = "dev"
	BuildTime = ""
)

func main() {
	log := logger.NewDevelopmentLogger()

	app := &cli.App{
		Name:    "k8s-mcp",
		Usage:   "Kubernetes 只读 MCP 服务器",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "kubeconfig",
				Aliases: []string{"k"},
				Value:   "~/.kube/config",
				Usage:   "kubeconfig 文件路径",
			},
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "查询的默认命名空间",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "日志级别: debug|info|warn|error",
			},
			&cli.StringFlag{
				Name:    "log-file",
				Aliases: []string{"f"},
				Usage:   "审计日志文件路径（默认: stdout）",
			},
		},
		Action: func(ctx *cli.Context) error {
			return runMCPServer(ctx, log)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal("应用错误", "error", err)
	}
}

// runMCPServer 启动 MCP stdio 服务器
func runMCPServer(ctx *cli.Context, log *logger.Logger) error {
	log.Info("启动 k8s-mcp 服务器")

	// 解析日志级别
	level := logger.Info
	switch ctx.String("log-level") {
	case "debug":
		level = logger.Debug
	case "warn":
		level = logger.Warn
	case "error":
		level = logger.Error
	}

	// 设置审计日志器
	output := os.Stdout
	if ctx.String("log-file") != "" {
		f, err := os.OpenFile(ctx.String("log-file"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Error("打开日志文件失败", "error", err)
			return fmt.Errorf("打开日志文件失败: %w", err)
		}
		output = f
	}
	auditLogger := audit.NewLogger(level, output)
	log.Info("审计日志器已初始化", "level", ctx.String("log-level"))

	// 加载 kubeconfig
	kubeconfigPath := ctx.String("kubeconfig")
	log.Info("加载 kubeconfig", "path", kubeconfigPath)
	config, err := k8s.LoadKubeconfig(kubeconfigPath)
	if err != nil {
		log.Error("加载 kubeconfig 失败", "error", err)
		auditLogger.LogError("startup", fmt.Sprintf("加载 kubeconfig 失败: %v", err))
		return fmt.Errorf("加载 kubeconfig 失败: %w", err)
	}
	log.Info("kubeconfig 加载成功")

	// 创建 Kubernetes 客户端
	log.Debug("创建 Kubernetes 客户端")
	client, err := k8s.NewClient(config)
	if err != nil {
		log.Error("创建 K8s 客户端失败", "error", err)
		auditLogger.LogError("startup", fmt.Sprintf("创建 K8s 客户端失败: %v", err))
		return fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}
	log.Info("Kubernetes 客户端创建成功")

	// 创建资源处理器
	resourceHandler := k8s.NewResourceHandler(client)
	log.Debug("资源处理器已创建")

	// 创建日志处理器
	logHandler := k8s.NewLogHandler(client)
	log.Debug("日志处理器已创建")

	// 创建工具注册器
	registry := mcp.NewRegistry()
	log.Debug("MCP 注册器已创建")

	// 注册 MCP 工具
	registerTools(registry, resourceHandler, logHandler, auditLogger)
	log.Info("MCP 工具已注册")

	// 创建 MCP 服务器
	server := mcp.NewServer(registry, auditLogger)
	log.Debug("MCP 服务器实例已创建")

	// 设置带取消的上下文
	serverCtx, cancel := context.WithCancel(context.Background())

	// 处理关闭信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("关闭 MCP 服务器")
		cancel()
		server.Stop()
	}()

	// 启动服务器
	log.Info("启动 MCP stdio 服务器", "kubeconfig", kubeconfigPath)
	return server.Start(serverCtx)
}

// registerTools 注册所有 MCP 工具到注册器
func registerTools(registry *mcp.Registry, resourceHandler *k8s.ResourceHandler, logHandler *k8s.LogHandler, auditLogger *audit.Logger) {
	// 注册 list_resources 工具
	listResourcesSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resourceType": {"type": "string", "enum": ["pods", "deployments", "services", "jobs", "configmaps", "namespaces"]},
			"namespace": {"type": "string", "description": "目标 namespace (namespace 资源类型时忽略此参数)"}
		},
		"required": ["resourceType"]
	}`)

	registry.Register(
		"list_resources",
		"列出 Kubernetes 资源 (仅只读)。支持: pods, deployments, services, jobs, configmaps, namespaces",
		listResourcesSchema,
		makeListResourcesHandler(resourceHandler, auditLogger),
	)

	// 注册 get_resource 工具
	getResourceSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resourceType": {"type": "string", "enum": ["pod", "deployment", "service", "job", "configmap", "secret"]},
			"namespace": {"type": "string"},
			"name": {"type": "string"}
		},
		"required": ["resourceType", "namespace", "name"]
	}`)

	registry.Register(
		"get_resource",
		"获取 Kubernetes 资源详情 (仅只读)。自动脱敏 secrets",
		getResourceSchema,
		makeGetResourceHandler(resourceHandler, auditLogger),
	)

	// 注册 read_pod_logs 工具
	readPodLogsSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"namespace": {"type": "string"},
			"podName": {"type": "string"},
			"container": {"type": "string"},
			"logDir": {"type": "string"},
			"logFile": {"type": "string"},
			"operation": {"type": "string", "enum": ["tail", "head", "grep", "cat", "tail-grep", "cat-grep", "head-grep"], "default": "tail"},
			"lines": {"type": "integer", "default": 100},
			"pattern": {"type": "string"},
			"follow": {"type": "boolean", "default": false},
			"followDuration": {"type": "integer", "default": 10}
		},
		"required": ["namespace", "podName", "logDir", "logFile"]
	}`)

	registry.Register(
		"read_pod_logs",
		"进入 Pod 容器读取日志文件 (仅只读)。支持管道组合: tail | grep, cat | grep",
		readPodLogsSchema,
		makeReadPodLogsHandler(logHandler, auditLogger),
	)
}

// makeListResourcesHandler 创建 list_resources 工具处理器
func makeListResourcesHandler(handler *k8s.ResourceHandler, auditLogger *audit.Logger) mcp.ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		start := time.Now()

		// 解析参数
		p, err := api.ParseParams[api.ListResourcesParams](params)
		if err != nil {
			auditLogger.LogError("list_resources", fmt.Sprintf("参数无效: %v", err))
			return api.NewErrorResponse(api.ErrInvalidInput, "参数无效"), nil
		}

		// 验证资源类型
		var result []api.ResourceSummary
		var listErr error

		switch p.ResourceType {
		case "namespaces":
			result, listErr = handler.ListNamespaces(ctx)
		case "pods":
			ns := p.Namespace
			if ns == "" {
				ns = "default"
			}
			result, listErr = handler.ListPods(ctx, ns)
		case "deployments":
			ns := p.Namespace
			if ns == "" {
				ns = "default"
			}
			result, listErr = handler.ListDeployments(ctx, ns)
		case "services":
			ns := p.Namespace
			if ns == "" {
				ns = "default"
			}
			result, listErr = handler.ListServices(ctx, ns)
		case "jobs":
			ns := p.Namespace
			if ns == "" {
				ns = "default"
			}
			result, listErr = handler.ListJobs(ctx, ns)
		case "configmaps":
			ns := p.Namespace
			if ns == "" {
				ns = "default"
			}
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

// makeGetResourceHandler 创建 get_resource 工具处理器
func makeGetResourceHandler(handler *k8s.ResourceHandler, auditLogger *audit.Logger) mcp.ToolHandler {
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
				// 脱敏 ConfigMap
				result = security.SanitizeConfigMap(cm)
			}
		case "secret":
			secret, err := handler.GetSecret(ctx, p.Namespace, p.Name)
			if err != nil {
				getErr = err
			} else {
				// 脱敏 Secret - 隐藏所有数据
				result = security.SanitizeSecret(secret)
			}
		default:
			return api.NewErrorResponse(api.ErrInvalidInput, fmt.Sprintf("未知资源类型: %s", p.ResourceType)), nil
		}

		// 处理错误
		if getErr != nil {
			auditLogger.LogError("get_resource", getErr.Error())
			// 检查是否为未找到错误
			if isNotFoundError(getErr) {
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

// isNotFoundError 检查错误是否为 Kubernetes 未找到错误
func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "not found" || containsNotFound(err.Error()))
}

// containsNotFound 检查字符串是否包含 "not found" 模式
func containsNotFound(s string) bool {
	return len(s) > 0 && (s == "not found" || (len(s) >= 10 && s[len(s)-10:] == "not found"))
}

// makeReadPodLogsHandler 创建 read_pod_logs 工具处理器
func makeReadPodLogsHandler(handler *k8s.LogHandler, auditLogger *audit.Logger) mcp.ToolHandler {
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
			// 检查是否为禁止路径
			if strings.Contains(readErr.Error(), "因安全原因被禁止访问") {
				return api.NewErrorResponse(api.ErrSensitivePathDenied, readErr.Error()), nil
			}
			// 检查是否为未找到
			if isNotFoundError(readErr) || strings.Contains(readErr.Error(), "no such file") {
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