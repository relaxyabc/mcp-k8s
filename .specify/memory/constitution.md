<!--
Sync Impact Report:
- Version change: 1.0.0 → 2.0.0 (MAJOR - backward-incompatible principle redefinition)
- Modified principles:
  - Principle I: Read-Only Security → Read-Only Security with Privileged Mode Exception
  - Added Principle VI: Privileged Mode Management
- Added sections: Privileged Mode section in Security Requirements
- Removed sections: None
- Templates requiring updates:
  ✅ plan-template.md (reviewed - Constitution Check section will use updated principles)
  ✅ spec-template.md (reviewed - no changes needed)
  ✅ tasks-template.md (reviewed - no changes needed)
- Follow-up TODOs:
  - Implement privileged mode parameter validation in src/security/
  - Add configuration option for privileged mode enablement
  - Update RBAC requirements for privileged operations
  - Add audit logging for privileged mode activations
-->

# K8s MCP Server Constitution

## Core Principles

### I. Read-Only Security with Privileged Mode Exception

By default, all operations MUST be strictly read-only. The server MUST NOT support create, update, or delete operations on Kubernetes resources, unless explicitly operating in **privileged mode**.

**Default Behavior (Read-Only)**:
- All MCP tools validate operations against the read-only whitelist
- Any attempt to add write capabilities MUST be rejected at the security layer
- The `src/security/readonly.go` module is the authoritative source of allowed operations

**Privileged Mode Exception**:
- Privileged mode MUST be explicitly enabled via configuration parameter
- When privileged mode is enabled, the server MAY support create, update, or delete operations
- Privileged mode activation MUST be logged in audit trail
- Privileged mode operations MUST have additional security validations

**Rationale**: This MCP server provides safe, read-only access to Kubernetes clusters by default. Privileged mode provides controlled write access for operational scenarios where file uploads, configuration updates, or other modifications are necessary, while maintaining strong audit trails and security controls.

**Enforcement**:
- Default operation mode is read-only (no configuration required)
- Privileged mode requires explicit configuration parameter `privileged: true`
- All privileged operations MUST pass additional security checks
- The `src/security/privileged.go` module validates privileged mode permissions

### II. Multi-Cluster Configuration

The server MUST support multiple Kubernetes clusters through a centralized configuration file (YAML or JSON format). Each cluster configuration MUST include:

- Unique cluster name identifier
- Kubeconfig file path
- Optional namespace whitelist for access restriction
- Optional cluster description
- **Optional privileged mode flag** (per-cluster or global)

**Rationale**: Production environments typically manage multiple clusters (dev, staging, prod). A centralized configuration approach simplifies deployment and reduces configuration errors.

**Requirements**:
- Default cluster MUST be configurable via `defaultCluster` setting
- Cluster selection via optional `cluster` parameter in all MCP tool calls
- Namespace whitelisting MUST be enforced at the cluster manager level
- Privileged mode can be configured globally or per-cluster

### III. Security by Default

The server MUST enforce multiple layers of security protection:

1. **Secrets Auto-Redaction**: All secret data MUST be automatically redacted before being returned to clients
2. **Sensitive Directory Blocking**: Access to `/etc/secrets`, `/root`, and `~/.ssh` directories MUST be blocked in pod exec operations
3. **Command Whitelisting**: Pod exec MUST only allow: `cat`, `tail`, `head`, `grep`, `ls` commands
4. **No Shell Access**: Shell commands and subshells MUST be prohibited in pod exec
5. **Privileged Mode Validation**: When privileged mode is enabled, additional validations MUST be applied

**Rationale**: Defense-in-depth security prevents accidental or malicious access to sensitive information. AI agents should not have access to secrets, credentials, or shell capabilities.

**Enforcement**:
- The `src/security/` package contains all security validation logic
- Modifications to security rules MUST be reviewed and approved by human operators (⚠️ Human Review Required)
- Adding new allowed commands to the whitelist requires privileged mode AND explicit approval

### IV. Audit Trail & Observability

All operations MUST be logged for audit and debugging purposes:

- Audit logs MUST capture: tool name, parameters, cluster, namespace, timestamp, and user context
- Structured logging MUST be used for all operations
- Error conditions MUST be logged with appropriate severity levels
- Debug logs MUST be available for troubleshooting without exposing sensitive data
- **Privileged mode activations MUST be prominently logged with WARN level**
- **All privileged operations MUST include "PRIVILEGED" flag in audit logs**

**Rationale**: Audit trails are essential for security compliance, debugging production issues, and understanding agent behavior. Privileged mode operations require heightened visibility.

**Implementation**:
- The `src/audit/` package provides audit logging capabilities
- The `src/logger/` package provides development logging
- Log levels MUST be configurable via the configuration file
- Privileged mode log entries MUST be distinguishable from regular operations

### V. Namespace Isolation

Each cluster configuration MUST support namespace whitelisting to restrict access boundaries:

- An empty `allowedNamespaces` array indicates access to all namespaces
- Non-empty arrays MUST restrict tool operations to listed namespaces only
- Namespace validation MUST occur at the cluster manager level before any operation

**Rationale**: Namespace isolation prevents AI agents from accessing unauthorized namespaces, reducing the blast radius of potential issues and enforcing multi-tenancy boundaries.

**Enforcement**:
- The `src/cluster/manager.go` module enforces namespace validation
- Attempts to access non-whitelisted namespaces MUST return a clear error message

### VI. Privileged Mode Management

When privileged mode is enabled, the following controls MUST be enforced:

**Activation Requirements**:
- Privileged mode MUST require explicit configuration parameter (never default)
- Configuration file MUST contain `privileged: true` at global or cluster level
- Privileged mode status MUST be visible in server startup logs

**Operational Controls**:
- All privileged operations MUST validate target namespace against whitelist
- File operations MUST validate target directory against allowed paths (if configured)
- Backup operations MUST succeed before applying destructive changes
- Maximum file size limits MUST be enforced (default 100MB)
- Concurrent modification detection MUST be implemented for critical operations

**Audit Requirements**:
- Privileged mode activation MUST log WARN level message at server startup
- Each privileged operation MUST log with "PRIVILEGED" flag
- Operation failures MUST log full context for troubleshooting
- Audit logs MUST be retained per compliance requirements

**Rationale**: Privileged mode enables necessary operational capabilities while maintaining strong security controls, audit trails, and accountability.

**Enforcement**:
- `src/security/privileged.go` validates privileged mode configuration
- `src/audit/privileged.go` provides enhanced audit logging for privileged operations
- Configuration validation MUST fail fast if privileged mode is malformed

## Security Requirements

### NEVER Rules (Absolute Prohibition)

The following actions are absolutely prohibited without exception:

1. Committing secrets, tokens, or cryptographic keys to code or commit messages
2. Deleting or commenting out test cases to pass validation
3. Bypassing lint, hook, or CI validation (no `--no-verify` flags)
4. Refactoring code outside task scope, regardless of perceived elegance
5. Modifying files or directories not specified in the task
6. Printing sensitive information to logs
7. Modifying security package logic to relax restrictions (without privileged mode AND human review)
8. Adding new allowed commands to pod exec whitelist (only `cat`, `tail`, `head`, `grep`, `ls` allowed, even in privileged mode)
9. **Executing privileged operations without explicit configuration**
10. **Skipping audit logging for privileged mode operations**

### Ask First Rules

The following actions require explicit user confirmation before execution:

1. Installing new dependencies (must confirm existing dependencies cannot satisfy requirements)
2. Deleting files
3. Modifying RBAC permission configurations
4. Modifying MCP protocol definitions (`src/mcp/protocol.go`)
5. Pushing to remote repositories
6. **Enabling privileged mode in production environments**
7. **Modifying privileged mode security validations**

### Human Review Required

The following changes MUST be explicitly marked with ⚠️ in output:

1. Changes to permissions, authentication, or encryption logic (`src/security/`)
2. Changes to MCP tool interfaces (`cmd/main.go` tool registration)
3. Addition of new MCP tools
4. **Changes to privileged mode configuration or validation logic**
5. **RBAC changes that enable privileged operations**

## Development Workflow

### Working Principles

1. **Read First**: Understand context and requirements before making changes
2. **Search Before Implementing**: Look for existing implementations to avoid duplication
3. **Minimal Necessary Changes**: Make only required changes; avoid scope creep

### Tech Stack

- **Language**: Go 1.25.7
- **Framework**: urfave/cli v2.27.5
- **Package Manager**: Go modules
- **Dependencies**: k8s.io/client-go v0.32.0, gopkg.in/yaml.v3
- **Test Framework**: Go testing package
- **Build Tool**: Make

### Repository Structure

```
cmd/            # Executable entry point
src/api/        # API type definitions (request/response structures)
src/audit/      # Audit logger
src/cluster/    # Cluster manager (multi-cluster support)
src/config/     # Configuration file loader (YAML/JSON)
src/k8s/        # Kubernetes client, resource handlers, log handlers
src/logger/     # Development logger
src/mcp/        # MCP protocol implementation (server, registry, protocol)
src/security/   # Security validation (read-only check, privileged mode, command whitelist, redaction)
tests/          # Test suites (helpers, integration)
```

### Code Style Requirements

1. **Error Handling**: All errors MUST be handled or returned; no ignored errors (`_ = err`)
2. **Error Response Format**: Use `api.NewErrorResponse(api.ErrXxx, message)` for consistent error responses
3. **Parameter Parsing**: All MCP tool parameters MUST use `api.ParseParams[T]` for type-safe parsing

### Validation Requirements

| Change Type | Required Validation |
|------------|-------------------|
| Code logic (Go) | `go test ./src/[affected-package]/...` |
| Security-related code | `go test ./src/security/...` |
| Privileged mode code | `go test ./src/security/... -run Privileged` |
| Full validation | `go test ./...` |
| Build verification | `go build ./cmd` |
| Makefile changes | `make build` |

**Task Completion Criteria**:
- All relevant checks pass with no new error-level issues
- Output includes validation results
- All required files have been modified
- Privileged mode changes include audit log verification

### Output Format

Every task completion MUST include:

```
改动文件：[file list]
改动原因：[brief explanation]
验证结果：[commands run and pass/fail status]
风险 / 假设：[risks or assumptions; "无" if none]
需人工复核：[items requiring review with ⚠️; "无" if none]
```

## Governance

### Amendment Procedure

1. Proposed amendments MUST be documented with clear rationale
2. Amendments affecting security principles (I, III, VI) MUST undergo security review
3. All amendments MUST update the constitution version according to semantic versioning:
   - **MAJOR**: Backward-incompatible principle removals or redefinitions
   - **MINOR**: New principles added or materially expanded guidance
   - **PATCH**: Clarifications, wording improvements, non-semantic refinements
4. Constitution changes MUST be committed with message format: `docs: amend constitution to vX.Y.Z (summary)`

### Compliance Review

- All pull requests MUST verify compliance with constitution principles
- Security-sensitive changes MUST be marked for human review (⚠️)
- Complexity or deviations from principles MUST be justified in documentation
- Privileged mode usage MUST be justified in feature specifications

### Runtime Guidance

For runtime development guidance and coding standards, refer to `AGENTS.md` in the repository root.

**Version**: 2.0.0 | **Ratified**: 2026-07-24 | **Last Amended**: 2026-07-24