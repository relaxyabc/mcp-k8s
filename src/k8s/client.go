package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client Kubernetes 客户端包装器
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewClient 创建新的 Kubernetes 客户端包装器
func NewClient(config *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{
			clientset: clientset,
			config:    config,
		}, nil
}

// Clientset 返回底层 Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// Config 返回底层 REST 配置
func (c *Client) Config() *rest.Config {
	return c.config
}