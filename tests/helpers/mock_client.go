package helpers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// MockK8sClient provides a mock Kubernetes client for testing
type MockK8sClient struct {
	fakeClient *fake.Clientset
}

// NewMockK8sClient creates a mock client with pre-populated resources
func NewMockK8sClient(objects ...runtime.Object) *MockK8sClient {
	return &MockK8sClient{
		fakeClient: fake.NewSimpleClientset(objects...),
	}
}

// Clientset returns the underlying fake clientset
func (m *MockK8sClient) Clientset() *fake.Clientset {
	return m.fakeClient
}

// Config returns a mock rest config
func (m *MockK8sClient) Config() *rest.Config {
	return &rest.Config{}
}

// AddNamespace adds a mock namespace
func (m *MockK8sClient) AddNamespace(name string, status corev1.NamespacePhase) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NamespaceStatus{
			Phase: status,
		},
	}
	m.fakeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
}

// AddPod adds a mock pod
func (m *MockK8sClient) AddPod(namespace, name string, phase corev1.PodPhase, containers []string) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: make([]corev1.Container, len(containers)),
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}
	for i, c := range containers {
		pod.Spec.Containers[i].Name = c
	}
	m.fakeClient.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
}

// AddSecret adds a mock secret
func (m *MockK8sClient) AddSecret(namespace, name string, data map[string]string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: make(map[string][]byte),
	}
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}
	m.fakeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
}

// AddConfigMap adds a mock configmap
func (m *MockK8sClient) AddConfigMap(namespace, name string, data map[string]string) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	m.fakeClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), cm, metav1.CreateOptions{})
}

// AddDeployment adds a mock deployment
func (m *MockK8sClient) AddDeployment(namespace, name string, replicas int32) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          replicas,
			AvailableReplicas: replicas,
		},
	}
	m.fakeClient.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
}

// Example usage:
// func TestSomething(t *testing.T) {
//     mock := helpers.NewMockK8sClient()
//     mock.AddNamespace("default", corev1.NamespaceActive)
//     mock.AddPod("default", "test-pod", corev1.PodRunning, []string{"main"})
//     // Use mock.Clientset() with your handlers
// }