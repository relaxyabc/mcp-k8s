package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a file (auto-detects JSON/YAML by extension)
func Load(path string) (*MCPConfig, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("获取用户主目录失败: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &MCPConfig{}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析 YAML 配置失败: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析 JSON 配置失败: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, cfg); err != nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("解析配置文件失败 (尝试 YAML 和 JSON): %w", err)
			}
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *MCPConfig) Validate() error {
	if len(c.Clusters) == 0 {
		return fmt.Errorf("至少需要定义一个集群")
	}

	seenNames := make(map[string]bool)
	for _, cluster := range c.Clusters {
		if cluster.Name == "" {
			return fmt.Errorf("集群名称不能为空")
		}
		if seenNames[cluster.Name] {
			return fmt.Errorf("集群名称重复: %s", cluster.Name)
		}
		if cluster.Kubeconfig == "" {
			return fmt.Errorf("集群 %s 的 kubeconfig 路径不能为空", cluster.Name)
		}
		seenNames[cluster.Name] = true
	}

	// Validate logging level if specified
	if c.Logging.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[strings.ToLower(c.Logging.Level)] {
			return fmt.Errorf("无效的日志级别: %s (可选值: debug, info, warn, error)", c.Logging.Level)
		}
	}

	return nil
}

// GetLogLevel returns the logging level, defaulting to "info"
func (c *MCPConfig) GetLogLevel() string {
	if c.Logging.Level == "" {
		return "info"
	}
	return strings.ToLower(c.Logging.Level)
}

// GetLogFile returns the logging file path, empty means stdout
func (c *MCPConfig) GetLogFile() string {
	return c.Logging.File
}