package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceHandler Kubernetes 资源查询处理器
type ResourceHandler struct {
	client *Client
}

// NewResourceHandler 创建新的资源处理器
func NewResourceHandler(client *Client) *ResourceHandler {
	return &ResourceHandler{client: client}
}

// ListNamespaces 列出所有命名空间
func (h *ResourceHandler) ListNamespaces(ctx context.Context) ([]api.ResourceSummary, error) {
	namespaces, err := h.client.Clientset().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出命名空间失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(namespaces.Items))
	for i, ns := range namespaces.Items {
		result[i] = api.ResourceSummary{
			Name:      ns.Name,
			Status:    string(ns.Status.Phase),
			CreatedAt: ns.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// ListPods 列出指定命名空间的 Pod
func (h *ResourceHandler) ListPods(ctx context.Context, namespace string) ([]api.ResourceSummary, error) {
	pods, err := h.client.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出 Pod 失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(pods.Items))
	for i, pod := range pods.Items {
		result[i] = api.ResourceSummary{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			CreatedAt: pod.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// ListDeployments 列出指定命名空间的 Deployment
func (h *ResourceHandler) ListDeployments(ctx context.Context, namespace string) ([]api.ResourceSummary, error) {
	deployments, err := h.client.Clientset().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出 Deployment 失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(deployments.Items))
	for i, deploy := range deployments.Items {
		status := "Unknown"
		for _, cond := range deploy.Status.Conditions {
			if cond.Type == "Available" && cond.Status == "True" {
				status = "Available"
				break
			}
		}
		result[i] = api.ResourceSummary{
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
			Status:    status,
			CreatedAt: deploy.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// ListServices 列出指定命名空间的 Service
func (h *ResourceHandler) ListServices(ctx context.Context, namespace string) ([]api.ResourceSummary, error) {
	services, err := h.client.Clientset().CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出 Service 失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(services.Items))
	for i, svc := range services.Items {
		result[i] = api.ResourceSummary{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Status:    "Active",
			CreatedAt: svc.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// ListJobs 列出指定命名空间的 Job
func (h *ResourceHandler) ListJobs(ctx context.Context, namespace string) ([]api.ResourceSummary, error) {
	jobs, err := h.client.Clientset().BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出 Job 失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(jobs.Items))
	for i, job := range jobs.Items {
		status := "Pending"
		if job.Status.Succeeded > 0 {
			status = "Complete"
		} else if job.Status.Failed > 0 {
			status = "Failed"
		} else if job.Status.Active > 0 {
			status = "Running"
		}
		result[i] = api.ResourceSummary{
			Name:      job.Name,
			Namespace: job.Namespace,
			Status:    status,
			CreatedAt: job.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// ListConfigMaps 列出指定命名空间的 ConfigMap
func (h *ResourceHandler) ListConfigMaps(ctx context.Context, namespace string) ([]api.ResourceSummary, error) {
	configmaps, err := h.client.Clientset().CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出 ConfigMap 失败: %w", err)
	}

	result := make([]api.ResourceSummary, len(configmaps.Items))
	for i, cm := range configmaps.Items {
		result[i] = api.ResourceSummary{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Status:    "Active",
			CreatedAt: cm.CreationTimestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}

// GetPod 获取指定 Pod 详情
func (h *ResourceHandler) GetPod(ctx context.Context, namespace, name string) (*api.ResourceDetail, error) {
	pod, err := h.client.Clientset().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 Pod 失败: %w", err)
	}

	detail := &api.ResourceDetail{
		Metadata: map[string]interface{}{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"labels":    pod.Labels,
			"createdAt": pod.CreationTimestamp.Format(time.RFC3339),
		},
		Status: map[string]interface{}{
			"phase":     string(pod.Status.Phase),
			"podIP":     pod.Status.PodIP,
			"hostIP":    pod.Status.HostIP,
			"startTime": pod.Status.StartTime,
		},
	}

	// 添加容器信息
	containers := make([]map[string]interface{}, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		containers[i] = map[string]interface{}{
			"name":  c.Name,
			"image": c.Image,
		}
	}
	detail.Spec = map[string]interface{}{
		"containers": containers,
	}

	return detail, nil
}

// GetDeployment 获取指定 Deployment 详情
func (h *ResourceHandler) GetDeployment(ctx context.Context, namespace, name string) (*api.ResourceDetail, error) {
	deploy, err := h.client.Clientset().AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 Deployment 失败: %w", err)
	}

	detail := &api.ResourceDetail{
		Metadata: map[string]interface{}{
			"name":      deploy.Name,
			"namespace": deploy.Namespace,
			"labels":    deploy.Labels,
			"createdAt": deploy.CreationTimestamp.Format(time.RFC3339),
		},
		Spec: map[string]interface{}{
			"replicas": deploy.Spec.Replicas,
			"strategy": deploy.Spec.Strategy.Type,
		},
		Status: map[string]interface{}{
			"replicas":            deploy.Status.Replicas,
			"availableReplicas":   deploy.Status.AvailableReplicas,
			"unavailableReplicas": deploy.Status.UnavailableReplicas,
		},
	}

	return detail, nil
}

// GetService 获取指定 Service 详情
func (h *ResourceHandler) GetService(ctx context.Context, namespace, name string) (*api.ResourceDetail, error) {
	svc, err := h.client.Clientset().CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 Service 失败: %w", err)
	}

	detail := &api.ResourceDetail{
		Metadata: map[string]interface{}{
			"name":      svc.Name,
			"namespace": svc.Namespace,
			"labels":    svc.Labels,
			"createdAt": svc.CreationTimestamp.Format(time.RFC3339),
		},
		Spec: map[string]interface{}{
			"type":      svc.Spec.Type,
			"clusterIP": svc.Spec.ClusterIP,
			"ports":     svc.Spec.Ports,
		},
		Status: map[string]interface{}{},
	}

	return detail, nil
}

// GetJob 获取指定 Job 详情
func (h *ResourceHandler) GetJob(ctx context.Context, namespace, name string) (*api.ResourceDetail, error) {
	job, err := h.client.Clientset().BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 Job 失败: %w", err)
	}

	detail := &api.ResourceDetail{
		Metadata: map[string]interface{}{
			"name":      job.Name,
			"namespace": job.Namespace,
			"labels":    job.Labels,
			"createdAt": job.CreationTimestamp.Format(time.RFC3339),
		},
		Spec: map[string]interface{}{
			"parallelism": job.Spec.Parallelism,
			"completions": job.Spec.Completions,
		},
		Status: map[string]interface{}{
			"succeeded": job.Status.Succeeded,
			"failed":    job.Status.Failed,
			"active":    job.Status.Active,
		},
	}

	return detail, nil
}

// GetConfigMap 获取指定 ConfigMap
func (h *ResourceHandler) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	cm, err := h.client.Clientset().CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 ConfigMap 失败: %w", err)
	}
	return cm, nil
}

// GetSecret 获取指定 Secret（会被脱敏处理）
func (h *ResourceHandler) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret, err := h.client.Clientset().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取 Secret 失败: %w", err)
	}
	return secret, nil
}