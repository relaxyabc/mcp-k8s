package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/relaxyabc/mcp-k8s/src/audit"
	"github.com/relaxyabc/mcp-k8s/src/cluster"
	"github.com/relaxyabc/mcp-k8s/src/config"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/logger"
	"github.com/relaxyabc/mcp-k8s/src/mcp"
	"github.com/relaxyabc/mcp-k8s/src/security"
	"github.com/relaxyabc/mcp-k8s/src/tools"
	"github.com/urfave/cli/v2"
)

// 版本信息（通过 ldflags 在构建时设置）
var (
	Version   = "dev"
	BuildTime = ""
)

func main() {
	log := logger.NewDevelopmentLogger()

	// 构建完整版本字符串
	fullVersion := Version
	if BuildTime != "" {
		fullVersion = fmt.Sprintf("%s (构建时间: %s)", Version, BuildTime)
	}

	// 自定义版本输出
	cli.VersionPrinter = func(ctx *cli.Context) {
		fmt.Fprintf(ctx.App.Writer, "版本: %s\n", ctx.App.Version)
	}

	app := &cli.App{
		Name:    "k8s-mcp",
		Usage:   "Kubernetes 只读 MCP 服务器",
		Version: fullVersion,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "配置文件路径 (JSON/YAML)，启用多集群模式",
			},
			&cli.StringFlag{
				Name:    "kubeconfig",
				Aliases: []string{"k"},
				Value:   "~/.kube/config",
				Usage:   "kubeconfig 文件路径 (单集群模式)",
			},
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "查询的默认命名空间 (单集群模式)",
			},
			&cli.BoolFlag{
				Name:  "privileged",
				Usage: "启用特权模式，禁用所有只读限制（警告：所有操作需用户确认后执行）",
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

	// 处理特权模式
	if ctx.Bool("privileged") {
		log.Warn("========================================")
		log.Warn("  特权模式已启用！")
		log.Warn("  所有安全限制已禁用")
		log.Warn("  每个操作都需要用户确认后执行")
		log.Warn("  所有操作将记录审计日志")
		log.Warn("========================================")
		security.SetPrivilegedMode(true)
	}

	clusterMgr, auditLogger, defaultNamespace, configPath, err := initializeCluster(ctx, log)
	if err != nil {
		return err
	}

	// 创建工具注册器
	registry := mcp.NewRegistry()
	log.Debug("MCP 注册器已创建")

	// 注册 MCP 工具
	tools.RegisterAll(registry, clusterMgr, auditLogger, defaultNamespace)
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
	if configPath != "" {
		log.Info("启动 MCP stdio 服务器 (多集群模式)", "config", configPath)
	} else {
		log.Info("启动 MCP stdio 服务器 (单集群模式)", "kubeconfig", ctx.String("kubeconfig"))
	}
	return server.Start(serverCtx)
}

// initializeCluster 初始化集群管理器和审计日志器
func initializeCluster(ctx *cli.Context, log *logger.Logger) (*cluster.Manager, *audit.Logger, string, string, error) {
	configPath := ctx.String("config")

	if configPath != "" {
		return initMultiClusterMode(ctx, log, configPath)
	}
	return initSingleClusterMode(ctx, log)
}

// initMultiClusterMode 初始化多集群模式
func initMultiClusterMode(ctx *cli.Context, log *logger.Logger, configPath string) (*cluster.Manager, *audit.Logger, string, string, error) {
	log.Info("使用配置文件模式", "path", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Error("加载配置文件失败", "error", err)
		return nil, nil, "", "", fmt.Errorf("加载配置文件失败: %w", err)
	}
	log.Info("配置文件加载成功", "clusters", len(cfg.Clusters), "default", cfg.DefaultCluster)

	clusterMgr, err := cluster.NewManager(cfg)
	if err != nil {
		log.Error("创建集群管理器失败", "error", err)
		return nil, nil, "", "", fmt.Errorf("创建集群管理器失败: %w", err)
	}
	log.Info("集群管理器创建成功", "clusters", clusterMgr.ListClusters())

	// 设置审计日志器
	level := parseLogLevel(cfg.GetLogLevel())
	output := os.Stdout
	if cfg.GetLogFile() != "" {
		f, err := os.OpenFile(cfg.GetLogFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Error("打开日志文件失败", "error", err)
			return nil, nil, "", "", fmt.Errorf("打开日志文件失败: %w", err)
		}
		output = f
	}
	auditLogger := audit.NewLogger(level, output)
	log.Info("审计日志器已初始化", "level", cfg.GetLogLevel(), "file", cfg.GetLogFile())

	// 警告：忽略单集群参数
	if ctx.String("kubeconfig") != "~/.kube/config" {
		log.Warn("多集群模式下 --kubeconfig 参数被忽略")
	}
	if ctx.String("namespace") != "" {
		log.Warn("多集群模式下 --namespace 参数被忽略")
	}

	return clusterMgr, auditLogger, "", configPath, nil
}

// initSingleClusterMode 初始化单集群模式
func initSingleClusterMode(ctx *cli.Context, log *logger.Logger) (*cluster.Manager, *audit.Logger, string, string, error) {
	log.Info("使用单集群模式 (向后兼容)")

	kubeconfigPath := ctx.String("kubeconfig")
	log.Info("加载 kubeconfig", "path", kubeconfigPath)

	restConfig, err := k8s.LoadKubeconfig(kubeconfigPath)
	if err != nil {
		log.Error("加载 kubeconfig 失败", "error", err)
		return nil, nil, "", "", fmt.Errorf("加载 kubeconfig 失败: %w", err)
	}
	log.Info("kubeconfig 加载成功")

	client, err := k8s.NewClient(restConfig)
	if err != nil {
		log.Error("创建 K8s 客户端失败", "error", err)
		return nil, nil, "", "", fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}
	log.Info("Kubernetes 客户端创建成功")

	defaultNamespace := ctx.String("namespace")
	if defaultNamespace == "" {
		defaultNamespace = "default"
	}

	clusterMgr := cluster.NewSingleClusterManager(client, kubeconfigPath, defaultNamespace)
	auditLogger := audit.NewLogger(logger.Info, os.Stdout)
	log.Info("审计日志器已初始化", "level", "info")

	return clusterMgr, auditLogger, defaultNamespace, "", nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.Debug
	case "warn":
		return logger.Warn
	case "error":
		return logger.Error
	default:
		return logger.Info
	}
}