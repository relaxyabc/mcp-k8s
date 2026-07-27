package config

// ClusterConfig represents a single cluster configuration
type ClusterConfig struct {
	Name              string   `json:"name" yaml:"name"`
	Kubeconfig        string   `json:"kubeconfig" yaml:"kubeconfig"`
	Description       string   `json:"description,omitempty" yaml:"description,omitempty"`
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty" yaml:"allowedNamespaces,omitempty"`
	Context           string   `json:"context,omitempty" yaml:"context,omitempty"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
	File  string `json:"file,omitempty" yaml:"file,omitempty"`
}

// MCPConfig represents the complete MCP-K8s configuration
type MCPConfig struct {
	Version  string        `json:"version,omitempty" yaml:"version,omitempty"`
	Logging  LoggingConfig `json:"logging" yaml:"logging"`
	Clusters []ClusterConfig `json:"clusters" yaml:"clusters"`
}

// LoadedCluster represents a cluster with its loaded Kubernetes client
type LoadedCluster struct {
	Config ClusterConfig
	Client any // Will be *k8s.Client after initialization
}