package cluster

import (
	"fmt"
	"sync"

	"github.com/relaxyabc/mcp-k8s/src/config"
	"github.com/relaxyabc/mcp-k8s/src/k8s"
	"github.com/relaxyabc/mcp-k8s/src/security"
)

// Manager manages multiple Kubernetes cluster connections
type Manager struct {
	mu           sync.RWMutex
	clusters     map[string]*config.LoadedCluster
	defaultName  string
}

// NewManager creates a new cluster manager from configuration
func NewManager(cfg *config.MCPConfig) (*Manager, error) {
	manager := &Manager{
		clusters:    make(map[string]*config.LoadedCluster),
		defaultName: cfg.DefaultCluster,
	}

	// Load all clusters
	for _, clusterCfg := range cfg.Clusters {
		restConfig, err := k8s.LoadKubeconfig(clusterCfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("加载集群 %s 的 kubeconfig 失败: %w", clusterCfg.Name, err)
		}

		k8sClient, err := k8s.NewClient(restConfig)
		if err != nil {
			return nil, fmt.Errorf("创建集群 %s 的客户端失败: %w", clusterCfg.Name, err)
		}

		manager.clusters[clusterCfg.Name] = &config.LoadedCluster{
			Config: clusterCfg,
			Client: k8sClient,
		}
	}

	return manager, nil
}

// NewSingleClusterManager creates a manager for single cluster mode (backward compatible)
func NewSingleClusterManager(client *k8s.Client, kubeconfigPath, defaultNamespace string) *Manager {
	return &Manager{
		clusters: map[string]*config.LoadedCluster{
			"default": &config.LoadedCluster{
				Config: config.ClusterConfig{
					Name:       "default",
					Kubeconfig: kubeconfigPath,
				},
				Client: client,
			},
		},
		defaultName: "default",
	}
}

// GetCluster returns a cluster by name, or the default cluster if name is empty
func (m *Manager) GetCluster(name string) (*config.LoadedCluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		name = m.defaultName
	}

	cluster, ok := m.clusters[name]
	if !ok {
		return nil, fmt.Errorf("集群 %q 不存在", name)
	}

	return cluster, nil
}

// IsNamespaceAllowed checks if a namespace is allowed for a cluster
// Empty allowedNamespaces means all namespaces are allowed
func (m *Manager) IsNamespaceAllowed(clusterName, namespace string) bool {
	cluster, err := m.GetCluster(clusterName)
	if err != nil {
		return false
	}

	// Empty allowedNamespaces means all namespaces are allowed
	if len(cluster.Config.AllowedNamespaces) == 0 {
		return true
	}

	for _, ns := range cluster.Config.AllowedNamespaces {
		if ns == namespace {
			return true
		}
	}

	return false
}

// ValidateNamespace checks if namespace access is allowed for a cluster
// Returns nil if allowed, or an error describing the restriction
func (m *Manager) ValidateNamespace(clusterName, namespace string) error {
	// 特权模式下跳过 namespace 检查
	if security.PrivilegedMode {
		return nil
	}

	cluster, err := m.GetCluster(clusterName)
	if err != nil {
		return err
	}

	// Empty namespace is used for cluster-wide resources (like listing namespaces)
	if namespace == "" {
		return nil
	}

	// Empty allowedNamespaces means all namespaces are allowed
	if len(cluster.Config.AllowedNamespaces) == 0 {
		return nil
	}

	for _, ns := range cluster.Config.AllowedNamespaces {
		if ns == namespace {
			return nil
		}
	}

	return fmt.Errorf("namespace %q 不在集群 %q 的允许列表中，允许的 namespace: %v",
		namespace, cluster.Config.Name, cluster.Config.AllowedNamespaces)
}

// ListClusters returns all available cluster names
func (m *Manager) ListClusters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

// DefaultCluster returns the default cluster name
func (m *Manager) DefaultCluster() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultName
}