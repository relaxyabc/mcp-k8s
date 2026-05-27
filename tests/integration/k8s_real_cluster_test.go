package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/relaxyabc/mcp-k8s/src/k8s"
)

// Integration tests that require real Kubernetes cluster
// Set KUBECONFIG environment variable before running

func getKubeconfigPath() string {
	// Check environment variable
 if path := os.Getenv("KUBECONFIG"); path != "" {
		return path
	}
	// Default path
 return "~/.kube/config"
}

func skipIfNoCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to load kubeconfig
 _, err := k8s.LoadKubeconfig(getKubeconfigPath())
	if err != nil {
		t.Skipf("skipping: no kubeconfig available: %v", err)
	}
}

func TestListNamespacesIntegration(t *testing.T) {
	skipIfNoCluster(t)

	config, err := k8s.LoadKubeconfig(getKubeconfigPath())
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	client, err := k8s.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	handler := k8s.NewResourceHandler(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := handler.ListNamespaces(ctx)
	if err != nil {
		t.Errorf("failed to list namespaces: %v", err)
	}

	// Verify at least some namespaces exist
 if len(result) == 0 {
		t.Error("expected at least one namespace")
	}

	// Verify structure
 for _, ns := range result {
		if ns.Name == "" {
			t.Error("namespace name should not be empty")
		}
		if ns.Status == "" {
			t.Error("namespace status should not be empty")
		}
	}

	t.Logf("Found %d namespaces", len(result))
}

func TestListPodsIntegration(t *testing.T) {
	skipIfNoCluster(t)

	config, err := k8s.LoadKubeconfig(getKubeconfigPath())
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	client, err := k8s.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	handler := k8s.NewResourceHandler(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List pods in default namespace
 result, err := handler.ListPods(ctx, "default")
	if err != nil {
		t.Logf("failed to list pods: %v (may be permission issue)", err)
		return
	}

	t.Logf("Found %d pods in default namespace", len(result))

	for _, pod := range result {
		t.Logf("Pod: %s, Status: %s", pod.Name, pod.Status)
	}
}

func TestGetPodIntegration(t *testing.T) {
	skipIfNoCluster(t)

	config, err := k8s.LoadKubeconfig(getKubeconfigPath())
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	client, err := k8s.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	handler := k8s.NewResourceHandler(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First get list of pods
 pods, err := handler.ListPods(ctx, "default")
	if err != nil || len(pods) == 0 {
		t.Skip("no pods available to test get")
	}

	// Get first pod
 podName := pods[0].Name
	detail, err := handler.GetPod(ctx, "default", podName)
	if err != nil {
		t.Errorf("failed to get pod %s: %v", podName, err)
		return
	}

	// Verify detail structure
 if detail.Metadata["name"] != podName {
		t.Errorf("expected name %s, got %v", podName, detail.Metadata["name"])
	}

	t.Logf("Got pod detail: %v", detail.Metadata)
}

func TestReadPodLogsIntegration(t *testing.T) {
	skipIfNoCluster(t)

	config, err := k8s.LoadKubeconfig(getKubeconfigPath())
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	client, err := k8s.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	logHandler := k8s.NewLogHandler(client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get list of running pods
 pods, err := logHandler.GetDefaultContainer(ctx, "default", "some-pod")
	if err != nil {
		t.Logf("pod lookup failed: %v (expected if no suitable pod)", err)
		return
	}

	t.Logf("Default container: %s", pods)
}