# K8s MCP Server

Kubernetes 只读 MCP (Model Context Protocol) 服务器，支持多集群配置，通过 stdio 与 MCP 客户端通信。

## 功能

- **多集群支持**: 通过配置文件管理多个 Kubernetes 集群
- **namespace 白名单**: 对每个集群限制可访问的 namespace
- **list_resources**: 列出 Kubernetes 资源 (pods, deployments, services, jobs, configmaps, namespaces)
- **get_resource**: 获取资源详情 (自动脱敏 secrets)
- **read_pod_logs**: 进入 Pod 读取日志文件 (支持 tail | grep 管道组合)

## 安全约束

- 所有操作严格只读，禁止 create/update/delete
- Secrets 数据自动脱敏
- 禁止访问敏感目录 (/etc/secrets, /root, ~/.ssh)
- Pod exec 仅允许：cat / tail / head / grep / ls

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
k8s-mcp --config mcp-k8s-config.yaml
```

配置文件格式 (YAML):

```yaml
# 默认集群
defaultCluster: "dev-cluster"

# 日志配置
logging:
  level: "info"            # debug|info|warn|error
  file: "/var/log/mcp-k8s/audit.log"  # 可选，默认 stdout

# 集群列表
clusters:
  - name: "dev-cluster"
    kubeconfig: "~/.kube/dev-config"
    description: "开发集群"
    allowedNamespaces:     # 可选，空数组表示允许所有 namespace
      - "default"
      - "dev"

  - name: "prod-cluster"
    kubeconfig: "~/.kube/prod-config"
    description: "生产集群"
    allowedNamespaces:
      - "monitoring"
```

### 单集群模式（向后兼容）

```bash
k8s-mcp --kubeconfig ~/.kube/config --namespace default
```

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| --config, -c | - | 配置文件路径 (YAML/JSON)，启用多集群模式 |
| --kubeconfig, -k | ~/.kube/config | kubeconfig 文件路径 (单集群模式) |
| --namespace, -n | default | 默认 namespace (单集群模式) |

## MCP 工具参数

所有工具新增可选 `cluster` 参数，用于指定目标集群：

```json
{
  "cluster": "dev-cluster",      // 可选，未指定时使用 defaultCluster
  "resourceType": "pods",
  "namespace": "default"
}
```

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
# 列出 pods (默认集群)
"列出 default namespace 下的所有 pods"

# 指定集群
"列出 dev-cluster 集群的 pods"

# 获取 pod 详情
"查看 pod nginx-pod 的详细信息"

# 读取日志文件
"进入 pod app-server，读取 /var/log/app/error.log 最后 100 行"

# 管道组合
"读取 pod app-server 的 info.log，过滤包含 ERROR 的行"
```

## RBAC 要求

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

## 技术栈

- Go 1.25.7
- urfave/cli v2.27.5
- k8s.io/client-go v0.32.0
- gopkg.in/yaml.v3
- MCP stdio protocol (JSON-RPC 2.0)

## 开发规范

详见 [AGENTS.md](./AGENTS.md)