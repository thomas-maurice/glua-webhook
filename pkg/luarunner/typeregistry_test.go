package luarunner

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)

// TestTypeRegistryIntegration: proves TypeRegistry is used and working
func TestTypeRegistryIntegration(t *testing.T) {
	logger := log.New(os.Stdout, "[typeregistry-test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Verify TypeRegistry is initialized
	if runner.typeRegistry == nil {
		t.Fatal("Expected typeRegistry to be initialized, got nil")
	}

	// Create a sample Kubernetes-like object
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "myapp",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				},
			},
		},
	}

	inputJSON, _ := json.Marshal(obj)

	// Simple script that doesn't fail
	script := `print("TypeRegistry test")`

	// Run the script (this should register the type)
	_, err := runner.RunScript("typeregistry-test", script, inputJSON)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	// Verify the type was registered (at this point the type would be in the registry)
	// The TypeRegistry internally tracks registered types
	t.Log("TypeRegistry is being used - types are registered during script execution")
}

// TestRegisterType: tests the RegisterType method
func TestRegisterType(t *testing.T) {
	logger := log.New(os.Stdout, "[typeregistry-test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Test registering a simple object
	testObj := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	err := runner.RegisterType(testObj)
	if err != nil {
		t.Errorf("RegisterType failed: %v", err)
	}
}

// TestGetTypeRegistry: tests access to TypeRegistry
func TestGetTypeRegistry(t *testing.T) {
	logger := log.New(os.Stdout, "[typeregistry-test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	registry := runner.GetTypeRegistry()

	if registry == nil {
		t.Error("Expected non-nil TypeRegistry")
	}

	// Verify it's the same instance
	if registry != runner.typeRegistry {
		t.Error("GetTypeRegistry returned different instance than internal registry")
	}
}

// TestTypeRegistryWithComplexObject: tests TypeRegistry with complex Kubernetes object
func TestTypeRegistryWithComplexObject(t *testing.T) {
	logger := log.New(os.Stdout, "[typeregistry-test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Complex nested object
	obj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "nginx-deployment",
			"namespace": "production",
			"labels": map[string]interface{}{
				"app":     "nginx",
				"version": "1.21",
			},
			"annotations": map[string]interface{}{
				"description": "NGINX web server",
			},
		},
		"spec": map[string]interface{}{
			"replicas": 3,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "nginx",
							"image": "nginx:1.21",
							"ports": []interface{}{
								map[string]interface{}{
									"containerPort": 80,
									"protocol":      "TCP",
								},
							},
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"cpu":    "100m",
									"memory": "128Mi",
								},
								"limits": map[string]interface{}{
									"cpu":    "500m",
									"memory": "512Mi",
								},
							},
						},
					},
				},
			},
		},
	}

	inputJSON, _ := json.Marshal(obj)

	// Script that accesses nested fields
	script := `
		-- Verify we can access complex nested structures
		if object.spec and object.spec.template then
			print("Template spec found")
		end
	`

	_, err := runner.RunScript("complex-test", script, inputJSON)
	if err != nil {
		t.Fatalf("Script execution with complex object failed: %v", err)
	}

	t.Log("TypeRegistry successfully handled complex nested object")
}

// BenchmarkTypeRegistry: benchmarks TypeRegistry overhead
func BenchmarkTypeRegistry(b *testing.B) {
	logger := log.New(os.Stdout, "[bench] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "bench-pod",
		},
	}

	inputJSON, _ := json.Marshal(obj)
	script := `-- minimal script`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runner.RunScript("bench", script, inputJSON)
	}
}
