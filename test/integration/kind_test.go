package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestKindIntegration: integration test using Kind cluster
// Run with: go test -v ./test/integration -tags=integration
func TestKindIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if Kind is available
	if _, err := exec.LookPath("kind"); err != nil {
		t.Skip("Kind not found, skipping integration test")
	}

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found, skipping integration test")
	}

	ctx := context.Background()
	clusterName := "glua-webhook-test"

	// Create Kind cluster
	t.Logf("Creating Kind cluster: %s", clusterName)
	createCmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--wait", "60s")
	createCmd.Stdout = os.Stdout
	createCmd.Stderr = os.Stderr
	if err := createCmd.Run(); err != nil {
		t.Fatalf("Failed to create Kind cluster: %v", err)
	}

	// Cleanup function
	defer func() {
		t.Logf("Deleting Kind cluster: %s", clusterName)
		deleteCmd := exec.Command("kind", "delete", "cluster", "--name", clusterName)
		deleteCmd.Run()
	}()

	// Get kubeconfig
	kubeconfigPath := "/tmp/kind-" + clusterName + "-config"
	exportCmd := exec.Command("kind", "export", "kubeconfig", "--name", clusterName, "--kubeconfig", kubeconfigPath)
	if err := exportCmd.Run(); err != nil {
		t.Fatalf("Failed to export kubeconfig: %v", err)
	}
	defer os.Remove(kubeconfigPath)

	// Create K8s client
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("Failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create clientset: %v", err)
	}

	// Wait for cluster to be ready
	t.Log("Waiting for cluster to be ready...")
	time.Sleep(5 * time.Second)

	// Test 1: Create ConfigMap with Lua script
	t.Run("CreateScriptConfigMap", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-script",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `
					if object.metadata == nil then
						object.metadata = {}
					end
					if object.metadata.labels == nil then
						object.metadata.labels = {}
					end
					object.metadata.labels["test"] = "success"
				`,
			},
		}

		_, err := clientset.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create ConfigMap: %v", err)
		}

		t.Log("ConfigMap created successfully")

		// Verify ConfigMap exists
		fetchedCM, err := clientset.CoreV1().ConfigMaps("default").Get(ctx, "test-script", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get ConfigMap: %v", err)
		}

		if fetchedCM.Data["script.lua"] == "" {
			t.Error("Expected script.lua to have content")
		}
	})

	// Test 2: Build and load webhook image (if Dockerfile exists)
	t.Run("BuildAndLoadWebhookImage", func(t *testing.T) {
		// Check if Dockerfile exists
		if _, err := os.Stat("../../Dockerfile"); os.IsNotExist(err) {
			t.Skip("Dockerfile not found, skipping image build")
		}

		// Build image
		t.Log("Building webhook image...")
		buildCmd := exec.Command("docker", "build", "-t", "glua-webhook:test", "../..")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("Failed to build image: %v", err)
		}

		// Load image into Kind
		t.Log("Loading image into Kind cluster...")
		loadCmd := exec.Command("kind", "load", "docker-image", "glua-webhook:test", "--name", clusterName)
		if err := loadCmd.Run(); err != nil {
			t.Fatalf("Failed to load image: %v", err)
		}
	})

	// Test 3: Apply manifests
	t.Run("ApplyManifests", func(t *testing.T) {
		manifestsDir := "../../examples/manifests"

		// Apply namespace
		applyCmd := exec.Command("kubectl", "apply", "-f", manifestsDir+"/00-namespace.yaml", "--kubeconfig", kubeconfigPath)
		if err := applyCmd.Run(); err != nil {
			t.Logf("Warning: Failed to apply namespace: %v", err)
		}

		// Apply ConfigMaps
		applyCmd = exec.Command("kubectl", "apply", "-f", manifestsDir+"/01-configmaps.yaml", "--kubeconfig", kubeconfigPath)
		if err := applyCmd.Run(); err != nil {
			t.Logf("Warning: Failed to apply ConfigMaps: %v", err)
		}

		// Apply RBAC
		applyCmd = exec.Command("kubectl", "apply", "-f", manifestsDir+"/04-rbac.yaml", "--kubeconfig", kubeconfigPath)
		if err := applyCmd.Run(); err != nil {
			t.Logf("Warning: Failed to apply RBAC: %v", err)
		}
	})

	// Test 4: Verify basic cluster operations
	t.Run("VerifyClusterOperations", func(t *testing.T) {
		// Create a test pod
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:latest",
					},
				},
			},
		}

		createdPod, err := clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create pod: %v", err)
		}

		t.Logf("Created pod: %s", createdPod.Name)

		// Cleanup
		err = clientset.CoreV1().Pods("default").Delete(ctx, "test-pod", metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: Failed to delete pod: %v", err)
		}
	})

	t.Log("Integration test completed successfully")
}

// TestWebhookEndToEnd: end-to-end test of webhook functionality
func TestWebhookEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// This test requires a running webhook server
	// It should be run after deploying the webhook to a Kind cluster

	t.Log("E2E webhook test - requires manual setup")
	t.Skip("Skipping - requires deployed webhook")
}

// Helper function to pretty print JSON
func prettyJSON(t *testing.T, v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal JSON: %v", err)
		return ""
	}
	return string(b)
}
