package security

import (
	"strings"
)

// AllowedCommands Pod exec 操作允许的命令
var AllowedCommands = []string{"cat", "tail", "head", "grep", "ls"}

// ForbiddenCommands Pod exec 操作禁止的命令
var ForbiddenCommands = []string{"rm", "mv", "cp", "write", "edit", "chmod", "chown", "touch", "echo"}

// ForbiddenOutputOperators Shell 命令禁止的输出操作符
var ForbiddenOutputOperators = []string{">", ">>"}

// ValidateCommand 检查命令是否允许用于 Pod exec
func ValidateCommand(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	for _, allowed := range AllowedCommands {
		if cmd == allowed {
			return true
		}
	}
	return false
}

// getFirstWord 提取 shell 命令的第一个单词（实际执行的命令名）
func getFirstWord(shellCmd string) string {
	// 去掉前导空格
	shellCmd = strings.TrimSpace(shellCmd)
	// 找到第一个空格的位置
	idx := strings.Index(shellCmd, " ")
	if idx == -1 {
		return strings.ToLower(shellCmd)
	}
	return strings.ToLower(shellCmd[:idx])
}

// ValidateShellCommand 检查 shell 命令字符串是否安全
func ValidateShellCommand(shellCmd string) bool {
	// 只检查第一个单词（命令名）是否在禁止列表中
	firstWord := getFirstWord(shellCmd)
	for _, forbidden := range ForbiddenCommands {
		if firstWord == forbidden {
			return false
		}
	}
	// 检查管道后的命令是否安全
	if strings.Contains(shellCmd, "|") {
		parts := strings.Split(shellCmd, "|")
		for _, part := range parts {
			cmdInPipe := getFirstWord(part)
			for _, forbidden := range ForbiddenCommands {
				if cmdInPipe == forbidden {
					return false
				}
			}
		}
	}
	// 检查输出重定向
	for _, op := range ForbiddenOutputOperators {
		if strings.Contains(shellCmd, op) {
			return false
		}
	}
	return true
}

// IsReadOnlyVerb 检查 Kubernetes API 操作是否为只读
func IsReadOnlyVerb(verb string) bool {
	readOnlyVerbs := []string{"get", "list", "watch"}
	for _, v := range readOnlyVerbs {
		if verb == v {
			return true
		}
	}
	return false
}