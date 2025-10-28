package test

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"thechat/pkg/luarunner"
)

func TestAddLabelScript(t *testing.T) {
	scriptPath := "../examples/scripts/add-label.lua"

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
	}

	// Run the script
	logger := log.New(os.Stdout, "[script-test] ", log.LstdFlags)
	runner := luarunner.NewScriptRunner(logger)

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	inputJSON, _ := json.Marshal(inputObj)
	resultJSON, err := runner.RunScript("add-label.lua", string(scriptContent), inputJSON)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(resultJSON, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify labels were added
	metadata := resultObj["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})

	if labels["glua.maurice.fr/processed"] != "true" {
		t.Error("Expected 'processed' label to be 'true'")
	}

	if labels["glua.maurice.fr/timestamp"] == nil {
		t.Error("Expected 'timestamp' label to be set")
	}
}

func TestInjectSidecarScript(t *testing.T) {
	scriptPath := "../examples/scripts/inject-sidecar.lua"

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
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

	logger := log.New(os.Stdout, "[script-test] ", log.LstdFlags)
	runner := luarunner.NewScriptRunner(logger)

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	inputJSON, _ := json.Marshal(inputObj)
	resultJSON, err := runner.RunScript("inject-sidecar.lua", string(scriptContent), inputJSON)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(resultJSON, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify sidecar was added
	spec := resultObj["spec"].(map[string]interface{})
	containers := spec["containers"].([]interface{})

	if len(containers) != 2 {
		t.Fatalf("Expected 2 containers, got %d", len(containers))
	}

	sidecar := containers[1].(map[string]interface{})
	if sidecar["name"] != "log-collector" {
		t.Errorf("Expected sidecar name 'log-collector', got %v", sidecar["name"])
	}

	// Verify volume was added
	volumes := spec["volumes"].([]interface{})
	if len(volumes) == 0 {
		t.Error("Expected volume to be added")
	}
}

func TestValidateLabelsScript_Success(t *testing.T) {
	scriptPath := "../examples/scripts/validate-labels.lua"

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
			"labels": map[string]interface{}{
				"app": "myapp",
				"env": "production",
			},
		},
	}

	logger := log.New(os.Stdout, "[script-test] ", log.LstdFlags)
	runner := luarunner.NewScriptRunner(logger)

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	inputJSON, _ := json.Marshal(inputObj)
	_, err = runner.RunScript("validate-labels.lua", string(scriptContent), inputJSON)
	if err != nil {
		t.Errorf("Validation should pass but got error: %v", err)
	}
}

func TestValidateLabelsScript_Failure(t *testing.T) {
	scriptPath := "../examples/scripts/validate-labels.lua"

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
			// Missing required labels
		},
	}

	logger := log.New(os.Stdout, "[script-test] ", log.LstdFlags)
	runner := luarunner.NewScriptRunner(logger)

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	inputJSON, _ := json.Marshal(inputObj)
	_, err = runner.RunScript("validate-labels.lua", string(scriptContent), inputJSON)
	if err == nil {
		t.Error("Expected validation to fail but it passed")
	}
}

func TestAddAnnotationsScript(t *testing.T) {
	scriptPath := "../examples/scripts/add-annotations.lua"

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "test-cm",
		},
	}

	logger := log.New(os.Stdout, "[script-test] ", log.LstdFlags)
	runner := luarunner.NewScriptRunner(logger)

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	inputJSON, _ := json.Marshal(inputObj)
	resultJSON, err := runner.RunScript("add-annotations.lua", string(scriptContent), inputJSON)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(resultJSON, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify annotation was added
	metadata := resultObj["metadata"].(map[string]interface{})
	annotations := metadata["annotations"].(map[string]interface{})

	if annotations["glua.maurice.fr/mutation-info"] == nil {
		t.Error("Expected 'mutation-info' annotation to be set")
	}

	// Verify it's valid JSON
	mutationInfo := annotations["glua.maurice.fr/mutation-info"].(string)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(mutationInfo), &parsed); err != nil {
		t.Errorf("Mutation info should be valid JSON: %v", err)
	}
}
