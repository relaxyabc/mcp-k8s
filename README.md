# K8s MCP Server

Kubernetes MCP (Model Context Protocol) 服务器，支持多集群配置、文件上传下载，通过 stdio 与 MCP 客户端通信。

## 功能

### 只读操作（默认模式）
- **多集群支持**: 通过配置文件管理多个 Kubernetes 集群
- **namespace 白名单**: 对每个集群限制可访问的 namespace
- **list_resources**: 列出 Kubernetes 资源 (pods, deployments, services, jobs, configmaps, namespaces)
- **get_resource**: 获取资源详情 (自动脱敏 secrets)
- **read_pod_logs**: 进入 Pod 读取日志文件 (支持 tail | grep 管道组合)
- **download_file**: 从 Pod 下载文件到本地（自动备份）

### 特权操作（需启用特权模式）
- **upload_file**: 上传本地文件到 Pod（自动备份）
- **exec_in_pod**: 在 Pod 容器中执行任意 shell 命令

## 安全约束

### 默认模式（只读）
- 所有操作严格只读，禁止 create/update/delete
- Secrets 数据自动脱敏
- 禁止访问敏感目录 (/etc/secrets, /root, ~/.ssh)
- Pod exec 仅允许：cat / tail / head / grep / ls
- 文件传输限制：最大 10MB
- **cluster 参数必需**：所有工具调用必须明确指定目标集群

### 特权模式
- 通过命令行参数 `--privileged` 启用
- 允许文件上传和任意命令执行
- 禁止上传到敏感目录 (/etc/secrets, /root, ~/.ssh, /etc/ssh, /var/run/secrets)
- 所有特权操作记录审计日志

## 安装

```bash
# 直接构建
go build -o k8s-mcp ./cmd

# 使用 Makefile（含版本信息）
make build

# 发布构建（linux + windows）
make release
```

## 使用方式

### 多集群模式（推荐）

通过配置文件管理多个集群：

```bash
# 启动服务器
k8s-mcp --config mcp-k8s-config.yaml

# 启动服务器（启用特权模式）
k8s-mcp --config mcp-k8s-config.yaml --privileged
```

配置文件格式 (YAML):

```yaml
# 版本标识（可选）
version: "1.0"

# 日志配置（可选）
logging:
  level: "info"            # debug|info|warn|error
  file: "/var/log/mcp-k8s/audit.log"  # 可选，默认 stdout

# 集群列表（必需）
clusters:
  - name: "dev-cluster"
    kubeconfig: "~/.kube/dev-config"
    description: "开发集群"
    allowedNamespaces:     # 可选，空数组或不设置表示允许所有 namespace
      - "default"
      - "dev"
      - "testing"

  - name: "prod-cluster"
    kubeconfig: "~/.kube/prod-config"
    description: "生产集群"
    allowedNamespaces:
      - "monitoring"
      - "logging"

  - name: "test-cluster"
    kubeconfig: "~/.kube/test-config"
    description: "测试集群"
    # 不设置 allowedNamespaces 表示允许所有 namespace
```

### 单集群模式（向后兼容）

```bash
k8s-mcp --kubeconfig ~/.kube/config --namespace default
```

**注意**：单集群模式下，集群名称为 `"default"`。

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| --config, -c | - | 配置文件路径 (YAML/JSON)，启用多集群模式 |
| --kubeconfig, -k | ~/.kube/config | kubeconfig 文件路径 (单集群模式) |
| --namespace, -n | default | 默认 namespace (单集群模式) |
| --privileged | false | 启用特权模式（允许上传文件和执行命令） |

## MCP 工具参数

### 通用说明

**重要**：所有工具的 `cluster` 参数均为**必需参数**，必须明确指定目标集群名称。

### list_resources

列出 Kubernetes 资源：

```json
{
  "cluster": "dev-cluster",      // 必需，集群名称
  "resourceType": "pods",        // 必需：pods|deployments|services|jobs|configmaps|namespaces
  "namespace": "default"         // 可选，namespace 资源类型时忽略
}
```

### get_resource

获取资源详情：

```json
{
  "cluster": "dev-cluster",      // 必需，集群名称
  "resourceType": "pod",         // 必需：pod|deployment|service|job|configmap|secret
  "namespace": "default",        // 必需
  "name": "nginx-pod"            // 必需，资源名称
}
```

**注意**：默认模式下 Secrets 数据自动脱敏；特权模式下显示完整数据。

### read_pod_logs

进入 Pod 读取日志文件：

```json
{
  "cluster": "dev-cluster",      // 必需，集群名称
  "namespace": "default",        // 必需
  "podName": "app-server",       // 必需
  "logDir": "/var/log/app",      // 必需，日志目录
  "logFile": "error.log",        // 必需，日志文件名
  "operation": "tail",           // 可选：tail|head|grep|cat|tail-grep|cat-grep|head-grep
  "lines": 100,                  // 可选，默认 100
  "pattern": "ERROR",            // grep 操作时必需
  "container": "main",           // 可选，默认第一个容器
  "follow": false,               // 可选，是否持续跟踪
  "followDuration": 10           // 可选，跟踪时长（秒）
}
```

### download_file（不需要特权模式）

从 Pod 下载文件到本地：

```json
{
  "cluster": "dev-cluster",       // 必需，源集群名称
  "podName": "app-server",        // 必需，源 Pod 名称
  "namespace": "default",         // 必需，源命名空间
  "remotePath": "/var/log/app/error.log",  // 必需，Pod 内文件路径（绝对路径）
  "localPath": "/local/error.log"         // 必需，本地目标路径（绝对路径）
}
```

**返回值：**
```json
{
  "success": true,
  "message": "文件下载成功",
  "localPath": "/local/error.log",
  "backupCreated": true,
  "backupPath": "/local/error.log.20260727-143022.bak",
  "fileSize": 2048,
  "duration": "0.8s",
  "privileged": false
}
```

### upload_file（需要特权模式）

上传本地文件到 Pod：

```json
{
  "cluster": "dev-cluster",       // 必需，目标集群名称
  "podName": "app-server",        // 必需，目标 Pod 名称
  "namespace": "default",         // 必需，目标命名空间
  "localPath": "/local/config.yaml",     // 必需，本地文件路径（绝对路径）
  "targetDir": "/tmp",            // 必需，Pod 内目标目录（绝对路径）
  "fileName": "config.yaml"       // 可选，目标文件名（默认使用本地文件名）
}
```

**返回值：**
```json
{
  "success": true,
  "message": "文件上传成功",
  "targetPath": "/tmp/config.yaml",
  "backupCreated": true,
  "backupPath": "/tmp/config.yaml.20260727-143022.bak",
  "fileSize": 1024,
  "duration": "1.2s",
  "privileged": true
}
```

**注意**：上传需要启动时添加 `--privileged` 参数。

### exec_in_pod（需要特权模式）

在 Pod 容器中执行任意 shell 命令：

```json
{
  "cluster": "dev-cluster",      // 必需，集群名称
  "namespace": "default",        // 必需
  "podName": "app-server",       // 必需
  "command": "ls -la /tmp",      // 必需，要执行的命令
  "container": "main"            // 可选，默认第一个容器
}
```

**返回值：**
```json
{
  "stdout": "total 8\ndrwxr-xr-x 2 root root 4096 ...",
  "stderr": "",
  "command": "ls -la /tmp",
  "podName": "app-server",
  "container": "main"
}
```

**注意**：执行命令需要启动时添加 `--privileged` 参数。

## MCP 客户端配置

### Claude Code

**多集群模式:**

```json
{
  "mcpServers": {
    "k8s-mcp": {
      "command": "/path/to/k8s-mcp",
      "args": ["--config", "/path/to/mcp-k8s-config.yaml"]
    }
  }
}
```

**多集群模式（启用特权模式）:**

```json
{
  "mcpServers": {
    "k8s-mcp": {
      "command": "/path/to/k8s-mcp",
      "args": ["--config", "/path/to/mcp-k8s-config.yaml", "--privileged"]
    }
  }
}
```

**单集群模式:**

```json
{
  "mcpServers": {
    "k8s-mcp": {
      "command": "/path/to/k8s-mcp",
      "args": ["--kubeconfig", "/path/to/kubeconfig", "--namespace", "default"]
    }
  }
}
```

## 使用示例

```
# 列出 pods（必须指定集群）
"列出 dev-cluster 集群 default namespace 下的所有 pods"

# 获取 pod 详情
"查看 dev-cluster 集群 pod nginx-pod 的详细信息"

# 读取日志文件
"进入 dev-cluster 集群的 pod app-server，读取 /var/log/app/error.log 最后 100 行"

# 管道组合
"读取 dev-cluster 集群 pod app-server 的 info.log，过滤包含 ERROR 的行"

# 下载文件（不需要特权模式）
"从 dev-cluster 集群 pod app-server 的 /var/log/app/error.log 下载到本地的 ./error.log"

# 上传文件（需要特权模式）
"上传本地的 ./config.yaml 到 dev-cluster 集群 pod app-server 的 /tmp 目录"

# 执行命令（需要特权模式）
"在 dev-cluster 集群 pod app-server 中执行 ls -la /tmp 命令"
```

## RBAC 要求

### 基础权限（只读操作）

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: mcp-readonly
rules:
- apiGroups: ["", "apps", "batch"]
  resources: ["pods", "deployments", "services", "jobs", "configmaps", "namespaces"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/exec", "pods/log"]
  verbs: ["get", "create"]
```

### 完整权限（包含文件上传下载）

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: mcp-full
rules:
- apiGroups: ["", "apps", "batch"]
  resources: ["pods", "deployments", "services", "jobs", "configmaps", "namespaces"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/exec", "pods/log"]
  verbs: ["get", "create"]  # exec 权限用于文件上传下载
```

## 技术栈

- Go 1.25.7
- urfave/cli v2.27.5
- k8s.io/client-go v0.32.0
- gopkg.in/yaml.v3
- MCP stdio protocol (JSON-RPC 2.0)

## 开发规范

详见 [AGENTS.md](./AGENTS.md)

## License

MIT