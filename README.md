# K8s MCP Server

Kubernetes 只读 MCP (Model Context Protocol) 服务器，通过 stdio 与 MCP 客户端通信。

## 功能

- **list_resources**: 列出 Kubernetes 资源
- **get_resource**: 获取资源详情 (自动脱敏 secrets)
- **read_pod_logs**: 进入 Pod 读取日志文件 (支持 tail | grep 管道组合)

## 安全约束

- 所有操作严格只读，禁止 create/update/delete
- Secrets 数据自动脱敏
- 禁止访问敏感目录 (/etc/secrets, /root, ~/.ssh)

## 安装

```bash
# 构建
go build -o k8s-mcp ./cmd

# 运行
./k8s-mcp --kubeconfig ~/.kube/config
```

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| --kubeconfig, -k | ~/.kube/config | kubeconfig 文件路径 |
| --namespace, -n | (empty) | 默认 namespace |
| --log-level, -l | info | 日志级别: debug/info/warn/error |
| --log-file, -f | stdout | 审计日志输出路径 |

## MCP 客户端配置

### Claude Code

```json
{
  "mcpServers": {
    "k8s-readonly": {
      "command": "/path/to/k8s-mcp",
      "args": ["--kubeconfig", "/path/to/kubeconfig"]
    }
  }
}
```

## 使用示例

```
# 列出 pods
"列出 default namespace 下的所有 pods"

# 获取 pod 详情
"查看 pod nginx-pod 的详细信息"

# 读取日志文件
"进入 pod app-server，读取 /var/log/app/error.log 最后 100 行"

# 管道组合
"读取 pod app-server 的 info.log，过滤包含 ERROR 的行"

# 实时跟随
"实时跟随 pod app-server 的 info.log，过滤 Connection 关键词，持续 15 秒"
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

- Go v1.25
- urfave/cli v2.x
- k8s.io/client-go v0.32.0
- MCP stdio protocol (JSON-RPC 2.0)