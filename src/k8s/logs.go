package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"github.com/relaxyabc/mcp-k8s/src/security"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
)

// LogHandler Pod 日志文件读取处理器
type LogHandler struct {
	client *Client
}

// NewLogHandler 创建新的日志处理器
func NewLogHandler(client *Client) *LogHandler {
	return &LogHandler{client: client}
}

// buildExecURL 构建完整的 exec URL，确保参数正确传递
func (h *LogHandler) buildExecURL(namespace, podName, container string, command []string) (*url.URL, error) {
	restClient := h.client.Clientset().CoreV1().RESTClient()

	// 获取基础 URL
	baseURL := restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		URL()

	// 手动构建查询参数（确保参数正确传递）
	queryParams := url.Values{}
	queryParams.Set("container", container)
	for _, cmd := range command {
		queryParams.Add("command", cmd)
	}
	queryParams.Set("stdout", "true")
	queryParams.Set("stderr", "true")
	queryParams.Set("stdin", "false")
	queryParams.Set("tty", "false")

	// 将参数绑定到 URL
	baseURL.RawQuery = queryParams.Encode()

	return baseURL, nil
}

// execShellCommand 在 Pod 中执行 shell 命令
func (h *LogHandler) execShellCommand(ctx context.Context, namespace, podName, container, shellCmd string) (string, error) {
	// 构建 exec URL（确保参数正确传递）
	execURL, err := h.buildExecURL(namespace, podName, container, []string{"sh", "-c", shellCmd})
	if err != nil {
		return "", err
	}

	// 创建 SPDY executor
	executor, err := remotecommand.NewSPDYExecutor(h.client.Config(), "POST", execURL)
	if err != nil {
		return "", fmt.Errorf("创建 SPDY 执行器失败: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// 只要 stdout 有数据就返回
	if stdout.Len() > 0 {
		return stdout.String(), nil
	}

	if err != nil {
		return "", fmt.Errorf("执行失败: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ReadLogFile 通过 exec 从 Pod 读取日志文件
func (h *LogHandler) ReadLogFile(ctx context.Context, namespace, podName, container, logPath string) (string, error) {
	if !security.ValidatePath(logPath) {
		return "", fmt.Errorf("路径 '%s' 因安全原因被禁止访问", logPath)
	}
	return h.execShellCommand(ctx, namespace, podName, container, "cat "+logPath)
}

// ReadLogWithOperation 使用指定操作读取日志文件
func (h *LogHandler) ReadLogWithOperation(ctx context.Context, namespace, podName, container, logPath, operation string, lines int, pattern string) (*api.LogContent, error) {
	if !security.ValidatePath(logPath) {
		return nil, fmt.Errorf("路径 '%s' 因安全原因被禁止访问", logPath)
	}

	cmd := buildCommand(logPath, operation, lines, pattern)

	if !security.ValidateShellCommand(cmd) {
		return nil, fmt.Errorf("命令 '%s' 包含禁止的操作", cmd)
	}

	output, err := h.execShellCommand(ctx, namespace, podName, container, cmd)
	if err != nil {
		return nil, err
	}

	linesSlice := strings.Split(output, "\n")
	if len(linesSlice) > 0 && linesSlice[len(linesSlice)-1] == "" {
		linesSlice = linesSlice[:len(linesSlice)-1]
	}

	return &api.LogContent{
		Lines:      linesSlice,
		TotalCount: len(linesSlice),
		Operation:  operation,
		Command:    cmd,
		FilePath:   logPath,
	}, nil
}

// ReadLogFollow 跟踪日志文件（tail -f），带超时
func (h *LogHandler) ReadLogFollow(ctx context.Context, namespace, podName, container, logPath string, durationSeconds int, pattern string) (*api.LogContent, error) {
	if !security.ValidatePath(logPath) {
		return nil, fmt.Errorf("路径 '%s' 因安全原因被禁止访问", logPath)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSeconds)*time.Second)
	defer cancel()

	var cmd string
	if pattern != "" {
		cmd = fmt.Sprintf("timeout %d tail -f %s | grep --line-buffered '%s'", durationSeconds, logPath, pattern)
	} else {
		cmd = fmt.Sprintf("timeout %d tail -f %s", durationSeconds, logPath)
	}

	if !security.ValidateShellCommand(cmd) {
		return nil, fmt.Errorf("命令 '%s' 包含禁止的操作", cmd)
	}

	output, err := h.execShellCommand(timeoutCtx, namespace, podName, container, cmd)

	if err != nil {
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) ||
			strings.Contains(err.Error(), "command terminated with exit code 124") ||
			strings.Contains(err.Error(), "exit code 124") {
			// 超时是预期行为
		} else {
			return nil, fmt.Errorf("Follow 日志流失败: %w", err)
		}
	}

	linesSlice := strings.Split(output, "\n")
	if len(linesSlice) > 0 && linesSlice[len(linesSlice)-1] == "" {
		linesSlice = linesSlice[:len(linesSlice)-1]
	}

	operation := "tail"
	if pattern != "" {
		operation = "tail-grep"
	}

	return &api.LogContent{
		Lines:      linesSlice,
		TotalCount: len(linesSlice),
		Operation:  operation,
		Command:    cmd,
		FilePath:   logPath,
		Truncated:  true,
		Followed:   true,
	}, nil
}

// GetDefaultContainer 获取 Pod 的默认容器名称
func (h *LogHandler) GetDefaultContainer(ctx context.Context, namespace, podName string) (string, error) {
	pod, err := h.client.Clientset().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("获取 Pod 失败: %w", err)
	}

	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("Pod 没有容器")
	}

	return pod.Spec.Containers[0].Name, nil
}

// buildCommand 根据操作类型构建 shell 命令
func buildCommand(logPath, operation string, lines int, pattern string) string {
	switch operation {
	case "tail":
		return fmt.Sprintf("tail -%d %s", lines, logPath)
	case "head":
		return fmt.Sprintf("head -%d %s", lines, logPath)
	case "cat":
		return fmt.Sprintf("cat %s", logPath)
	case "grep":
		return fmt.Sprintf("grep '%s' %s", pattern, logPath)
	case "tail-grep":
		return fmt.Sprintf("tail -%d %s | grep --line-buffered '%s'", lines, logPath, pattern)
	case "cat-grep":
		return fmt.Sprintf("cat %s | grep '%s'", logPath, pattern)
	case "head-grep":
		return fmt.Sprintf("head -%d %s | grep '%s'", lines, logPath, pattern)
	default:
		return fmt.Sprintf("tail -%d %s", lines, logPath)
	}
}