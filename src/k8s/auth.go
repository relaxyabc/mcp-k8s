package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// LoadKubeconfig 从 kubeconfig 文件加载 Kubernetes 配置
func LoadKubeconfig(kubeconfigPath string) (*rest.Config, error) {
	// 将 ~ 展开为用户主目录
	if strings.HasPrefix(kubeconfigPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfigPath = filepath.Join(homeDir, kubeconfigPath[1:])
	}

	// 使用 clientcmd 加载器加载 kubeconfig
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	loader.ExplicitPath = kubeconfigPath
	loadedConfig, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("加载 kubeconfig 文件失败: %w", err)
	}

	// 手动构建 rest 配置以处理 TLS 问题
	cluster := loadedConfig.Clusters[loadedConfig.Contexts[loadedConfig.CurrentContext].Cluster]
	config := &rest.Config{
		Host: cluster.Server,
	}

	// 获取用户认证信息
	user := loadedConfig.AuthInfos[loadedConfig.Contexts[loadedConfig.CurrentContext].AuthInfo]
	if user != nil {
		// 尝试加载客户端证书
		if len(user.ClientCertificateData) > 0 && len(user.ClientKeyData) > 0 {
			certData := user.ClientCertificateData
			keyData := user.ClientKeyData

			// 如需要，将 PKCS#1 RSA 私钥转换为 PKCS#8
			keyData = convertPKCS1ToPKCS8(keyData)

			// 尝试创建证书
			_, err := tls.X509KeyPair(certData, keyData)
			if err != nil {
				// TLS 证书解析失败，尝试不使用客户端证书认证
				// 使用不安全的 TLS
				config.TLSClientConfig = rest.TLSClientConfig{
					Insecure: true,
				}
			} else {
				// 使用客户端证书和不安全的 TLS
				config.TLSClientConfig = rest.TLSClientConfig{
					CertData: certData,
					KeyData:  keyData,
					Insecure: true,
				}
			}
		} else if user.ClientCertificate != "" && user.ClientKey != "" {
			// 从文件加载
			config.TLSClientConfig = rest.TLSClientConfig{
				CertFile: user.ClientCertificate,
				KeyFile:  user.ClientKey,
				Insecure: true,
			}
		} else {
			// 无客户端证书，仅设置不安全模式
			config.TLSClientConfig = rest.TLSClientConfig{
				Insecure: true,
			}
		}

		// 处理 bearer token
		if user.Token != "" {
			config.BearerToken = user.Token
		}
	}

	// 设置不安全的 TLS（使用 Insecure 时不设置 CAData）
	config.TLSClientConfig.Insecure = true

	// 添加 SPDY 支持所需的 Dial 配置
	// SPDY 需要能够建立原始 TCP 连接
	config.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return dialer.DialContext(ctx, network, addr)
	}

	return config, nil
}

// convertPKCS1ToPKCS8 将 PKCS#1 RSA 私钥转换为 PKCS#8 格式
// Go 的 TLS 库更倾向于使用 PKCS#8 格式的私钥
func convertPKCS1ToPKCS8(keyData []byte) []byte {
	// 解析 PEM 块
	block, _ := pem.Decode(keyData)
	if block == nil {
		return keyData // 非 PEM 格式，原样返回
	}

	// 检查是否为 PKCS#1 RSA 私钥
	if block.Type == "RSA PRIVATE KEY" {
		// 解析 RSA 私钥
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return keyData // 无法解析，原样返回
		}

		// 转换为 PKCS#8
		pkcs8Data, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return keyData // 无法转换，原样返回
		}

		// 创建新的 PKCS#8 格式 PEM 块
		newBlock := &pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: pkcs8Data,
		}

		return pem.EncodeToMemory(newBlock)
	}

	return keyData // 已是 PKCS#8 或其他格式，原样返回
}

// GetClientConfig 返回原始的 clientcmd 配置（用于需要原始配置的场景）
func GetClientConfig(kubeconfigPath string) (*api.Config, error) {
	if strings.HasPrefix(kubeconfigPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfigPath = filepath.Join(homeDir, kubeconfigPath[1:])
	}

	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	loader.ExplicitPath = kubeconfigPath
	return loader.Load()
}