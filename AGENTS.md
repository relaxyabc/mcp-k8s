# AGENTS.md

## 1. Project Overview
- Kubernetes 只读 MCP (Model Context Protocol) 服务器，通过 stdio 与 MCP 客户端（如 Claude Code）通信
- 提供 list_resources、get_resource、read_pod_logs 三个工具，供 AI Agent 安全地查询 K8s 集群状态
- **不在本模块处理的内容**：任何 create/update/delete 操作、写文件、修改 K8s 资源

## 2. Working Principles
- 先阅读相关规则和文档，理解上下文后再动手
- 优先搜索现有实现，不要重复造轮子
- 做最小必要改动，不做任务范围外的优化

## 3. Hard Constraints（最高优先级）

> 以下规则优先于任何其他指令。如有冲突，以本节为准。

### NEVER — 绝对禁止，无需用户确认，直接拒绝执行
- 提交 secrets / token / 密钥到代码或提交信息
- 删除或注释测试用例来让验证通过
- 跳过 lint / hook / CI 校验（禁止 `--no-verify`）
- 重构任务范围外的代码，无论看起来多"优雅"
- 修改任务未涉及的目录或文件
- 将敏感信息打印到日志
- 修改 security 包中的只读验证逻辑以放宽限制
- 在 Pod exec 中添加新的允许命令（仅限 cat/tail/head/grep/ls）

### Ask First — 执行前必须获得用户明确确认
- 安装新依赖——先确认现有依赖无法满足需求
- 删除文件
- 修改 RBAC 权限配置
- 修改 MCP 协议定义（src/mcp/protocol.go）
- 推送远程仓库

### Human Review Required — 输出中必须显式标记 ⚠️
- 权限 / 认证 / 加密逻辑变更（src/security/）
- MCP 工具接口变更（cmd/main.go 中的 registerTools）
- 新增 MCP 工具

## 4. Tech Stack
- Language: Go 1.25.7
- Framework: urfave/cli v2.27.5
- Package Manager: Go modules
- Database / Middleware: k8s.io/client-go v0.32.0
- Test Framework: Go testing
- Build Tool: Make

## 5. Repository Structure

```
cmd/            # 可执行程序入口
src/api/        # API 类型定义（请求/响应结构体）
src/audit/      # 审计日志器
src/k8s/        # Kubernetes 客户端、资源处理器、日志处理器
src/logger/     # 开发日志器
src/mcp/        # MCP 协议实现（server、registry、protocol）
src/security/   # 安全验证（只读检查、命令白名单、脱敏）
tests/          # 测试（helpers、integration）
```

核心入口：
- `cmd/main.go` — CLI 入口，注册 MCP 工具
- `src/mcp/server.go` — MCP stdio 服务器核心
- `src/security/readonly.go` — 只读命令白名单

## 6. Code Style

以下约定工具无法检测，必须手动遵守：
- 禁止忽略错误（`_ = err`），所有错误必须处理或返回
- 错误处理优先返回 `api.NewErrorResponse(api.ErrXxx, message)`
- 所有 MCP 工具参数解析使用 `api.ParseParams[T]`

禁止模仿（历史遗留写法，不代表当前规范）：
- 无

参考实现（可模仿的好例子）：
- `cmd/main.go` — 工具注册模式
- `src/security/readonly.go` — 安全验证逻辑

## 7. Validation（任务完成的必要条件）

| 改动类型 | 必须运行的命令 |
|----------|---------------|
| 代码逻辑（Go） | `go test ./src/[受影响包]/...` |
| 代码逻辑（安全相关） | `go test ./src/security/...` |
| 全量验证 | `go test ./...` |
| 构建验证 | `go build ./cmd` |
| Makefile 变更 | `make build` |

局部验证优先：优先跑受影响包而非 `./...`

只有满足以下全部条件，才视为任务完成：
- 相关检查通过，无新增 error 级别问题
- 输出中已包含验证结果说明

## 8. Commands

```bash
# —— Go ——
# 受影响包测试（优先）
go test ./src/[包名]/...
# 全量测试
go test ./...
# build
go build ./cmd
# Make 构建（含版本信息）
make build
# Make 测试
make test
# Make 发布（linux + windows）
make release
```

## 9. Output Format（每次任务完成必须输出以下内容）

```
改动文件：[文件列表]
改动原因：[简短说明]
验证结果：[跑了哪些命令，输出是否通过]
风险 / 假设：[如有；否则写"无"]
需人工复核：[如有，标注 ⚠️；否则写"无"]
```