package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/api"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

// WriteHandler Kubernetes 写操作处理器（特权模式专用）
type WriteHandler struct {
	client *Client
	dyn    dynamic.Interface
	mapper meta.RESTMapper
}

// NewWriteHandler 创建新的写操作处理器
func NewWriteHandler(client *Client) (*WriteHandler, error) {
	// 创建 discovery client
	dc, err := discovery.NewDiscoveryClientForConfig(client.Config())
	if err != nil {
		return nil, fmt.Errorf("创建 discovery client 失败: %w", err)
	}

	// 获取 API group resources
	grs, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, fmt.Errorf("获取 API group resources 失败: %w", err)
	}

	// 创建 RESTMapper
	mapper := restmapper.NewDiscoveryRESTMapper(grs)

	// 创建 dynamic client
	dyn, err := dynamic.NewForConfig(client.Config())
	if err != nil {
		return nil, fmt.Errorf("创建 dynamic client 失败: %w", err)
	}

	return &WriteHandler{
		client: client,
		dyn:    dyn,
		mapper: mapper,
	}, nil
}

// commonGVR 常用资源类型的 GVR 映射
var commonGVR = map[string]schema.GroupVersionResource{
	"pod":         {Group: "", Version: "v1", Resource: "pods"},
	"pods":        {Group: "", Version: "v1", Resource: "pods"},
	"service":     {Group: "", Version: "v1", Resource: "services"},
	"services":    {Group: "", Version: "v1", Resource: "services"},
	"configmap":   {Group: "", Version: "v1", Resource: "configmaps"},
	"configmaps":  {Group: "", Version: "v1", Resource: "configmaps"},
	"secret":      {Group: "", Version: "v1", Resource: "secrets"},
	"secrets":     {Group: "", Version: "v1", Resource: "secrets"},
	"deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"deployments": {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulset": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonset":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"replicaset":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"job":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjob":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"ingress":     {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"namespace":   {Group: "", Version: "v1", Resource: "namespaces"},
	"namespaces":  {Group: "", Version: "v1", Resource: "namespaces"},
}

// resolveGVR 解析资源类型到 GroupVersionResource
func (h *WriteHandler) resolveGVR(ctx context.Context, resourceType string) (schema.GroupVersionResource, error) {
	// 先尝试静态映射
	lowerType := strings.ToLower(strings.TrimSpace(resourceType))
	if gvr, ok := commonGVR[lowerType]; ok {
		return gvr, nil
	}

	// 使用 discovery 解析
	gv, err := schema.ParseGroupVersion(lowerType)
	if err != nil {
		// 如果无法解析为 group/version，尝试直接作为 kind 查找
		mapping, mapErr := h.mapper.RESTMapping(schema.GroupKind{Kind: strings.Title(lowerType)})
		if mapErr != nil {
			return schema.GroupVersionResource{}, fmt.Errorf("无法解析资源类型 %q: %w", resourceType, mapErr)
		}
		return mapping.Resource, nil
	}
	mapping, err := h.mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: strings.Title(lowerType)})
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("无法解析资源类型 %q: %w", resourceType, err)
	}
	return mapping.Resource, nil
}

// resolveGVRFromUnstructured 从 Unstructured 对象解析 GVR
func (h *WriteHandler) resolveGVRFromUnstructured(obj *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	gvk := obj.GroupVersionKind()

	// 使用 RESTMapper 解析
	mapping, err := h.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("无法解析 GVK %s: %w", gvk, err)
	}
	return mapping.Resource, nil
}

// ApplyYAML 应用 YAML/JSON 清单（Server-Side Apply）
func (h *WriteHandler) ApplyYAML(ctx context.Context, namespace, fieldManager string, manifest []byte) (*api.ApplyResult, error) {
	// 解析 YAML/JSON
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(manifest, obj); err != nil {
		return nil, fmt.Errorf("解析清单失败: %w", err)
	}

	// 获取资源信息
	kind := obj.GetKind()
	name := obj.GetName()
	objNamespace := obj.GetNamespace()

	// 如果清单中没有 namespace，使用传入的 namespace
	if objNamespace == "" && namespace != "" {
		obj.SetNamespace(namespace)
		objNamespace = namespace
	}

	// 解析 GVR
	gvr, err := h.resolveGVRFromUnstructured(obj)
	if err != nil {
		return nil, err
	}

	// 设置默认 fieldManager
	if fieldManager == "" {
		fieldManager = "k8s-mcp"
	}

	// 检查资源是否已存在（判断是创建还是更新）
	var dr dynamic.ResourceInterface
	if objNamespace != "" {
		dr = h.dyn.Resource(gvr).Namespace(objNamespace)
	} else {
		dr = h.dyn.Resource(gvr)
	}

	// 尝试获取现有资源以判断操作类型
	existing, err := dr.Get(ctx, name, metav1.GetOptions{})
	operation := "created"
	if err == nil && existing != nil {
		operation = "updated"
	}

	// Server-Side Apply
	_, err = dr.Apply(ctx, name, obj, metav1.ApplyOptions{
		FieldManager: fieldManager,
		Force:        false,
	})
	if err != nil {
		return nil, fmt.Errorf("Apply 失败: %w", err)
	}

	return &api.ApplyResult{
		Kind:      kind,
		Name:      name,
		Namespace: objNamespace,
		Operation: operation,
	}, nil
}

// PatchResource 补丁资源
func (h *WriteHandler) PatchResource(ctx context.Context, namespace, resourceType, name, patchType string, patch []byte) (*api.PatchResult, error) {
	// 解析 GVR
	gvr, err := h.resolveGVR(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	// 确定 patch 类型
	var pt types.PatchType
	switch strings.ToLower(patchType) {
	case "json":
		pt = types.JSONPatchType
	case "merge", "":
		pt = types.MergePatchType
	case "strategic":
		pt = types.StrategicMergePatchType
	default:
		pt = types.MergePatchType
	}

	// 获取资源接口
	dr := h.dyn.Resource(gvr).Namespace(namespace)

	// 执行 patch
	_, err = dr.Patch(ctx, name, pt, patch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("Patch 失败: %w", err)
	}

	return &api.PatchResult{
		Kind:      resourceType,
		Name:      name,
		Namespace: namespace,
	}, nil
}

// DeleteResource 删除资源
func (h *WriteHandler) DeleteResource(ctx context.Context, namespace, resourceType, name string) (*api.DeleteResult, error) {
	// 解析 GVR
	gvr, err := h.resolveGVR(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	// 获取资源接口
	dr := h.dyn.Resource(gvr).Namespace(namespace)

	// 执行删除
	err = dr.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return nil, fmt.Errorf("Delete 失败: %w", err)
	}

	return &api.DeleteResult{
		Kind:      resourceType,
		Name:      name,
		Namespace: namespace,
	}, nil
}

// RolloutRestart 滚动重启 Deployment/StatefulSet/DaemonSet
func (h *WriteHandler) RolloutRestart(ctx context.Context, namespace, resourceType, name string) (*api.RestartResult, error) {
	// 解析 GVR
	gvr, err := h.resolveGVR(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	// 验证资源类型支持重启
	lowerType := strings.ToLower(resourceType)
	if lowerType != "deployment" && lowerType != "deployments" &&
		lowerType != "statefulset" && lowerType != "statefulsets" &&
		lowerType != "daemonset" && lowerType != "daemonsets" {
		return nil, fmt.Errorf("资源类型 %q 不支持滚动重启，仅支持 deployment/statefulset/daemonset", resourceType)
	}

	// 获取资源接口
	dr := h.dyn.Resource(gvr).Namespace(namespace)

	// 验证资源存在
	_, err = dr.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取资源失败: %w", err)
	}

	// 准备重启时间戳
	now := time.Now().UTC().Format(time.RFC3339)

	// 构造 patch：添加/更新 spec.template.metadata.annotations
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/restartedAt": now,
					},
				},
			},
		},
	}

	patchBytes, err := yaml.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("构造 patch 失败: %w", err)
	}

	// 执行 patch
	_, err = dr.Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("重启 patch 失败: %w", err)
	}

	return &api.RestartResult{
		Kind:        resourceType,
		Name:        name,
		Namespace:   namespace,
		RestartedAt: now,
	}, nil
}