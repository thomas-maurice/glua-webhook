package luarunner

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
)

func TestRunScript_Success(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Simple script that adds a label
	script := `
		if object.metadata == nil then
			object.metadata = {}
		end
		if object.metadata.labels == nil then
			object.metadata.labels = {}
		end
		object.metadata.labels["added-by-lua"] = "true"
	`

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
	}

	inputJSON, _ := json.Marshal(inputObj)

	result, err := runner.RunScript("test-script", script, inputJSON)
	if err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify the label was added
	metadata := resultObj["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})

	if labels["added-by-lua"] != "true" {
		t.Errorf("Expected label 'added-by-lua' to be 'true', got %v", labels["added-by-lua"])
	}
}

func TestRunScript_InvalidLua(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Invalid Lua syntax
	script := `this is not valid lua code!@#$`

	inputObj := map[string]interface{}{"test": "data"}
	inputJSON, _ := json.Marshal(inputObj)

	_, err := runner.RunScript("invalid-script", script, inputJSON)
	if err == nil {
		t.Error("Expected error for invalid Lua script, got nil")
	}
}

func TestRunScript_InvalidJSON(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	script := `print("hello")`
	invalidJSON := []byte(`{invalid json}`)

	_, err := runner.RunScript("test-script", script, invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestRunScript_ModifyNestedFields(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	script := `
		-- Add a new container to the pod
		if object.spec == nil then
			object.spec = {}
		end
		if object.spec.containers == nil then
			object.spec.containers = {}
		end
		table.insert(object.spec.containers, {
			name = "sidecar",
			image = "busybox:latest"
		})
	`

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "test-pod",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "main",
					"image": "nginx:latest",
				},
			},
		},
	}

	inputJSON, _ := json.Marshal(inputObj)

	result, err := runner.RunScript("add-sidecar", script, inputJSON)
	if err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify the container was added
	spec := resultObj["spec"].(map[string]interface{})
	containers := spec["containers"].([]interface{})

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}

	sidecar := containers[1].(map[string]interface{})
	if sidecar["name"] != "sidecar" {
		t.Errorf("Expected sidecar name 'sidecar', got %v", sidecar["name"])
	}
}

func TestRunScriptsSequentially_Success(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	scripts := map[string]string{
		"z-third": `
			object.metadata.labels["order"] = "3"
		`,
		"a-first": `
			if object.metadata == nil then
				object.metadata = {}
			end
			if object.metadata.labels == nil then
				object.metadata.labels = {}
			end
			object.metadata.labels["order"] = "1"
		`,
		"m-second": `
			object.metadata.labels["order"] = "2"
		`,
	}

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
	}

	inputJSON, _ := json.Marshal(inputObj)

	result, err := runner.RunScriptsSequentially(scripts, inputJSON)
	if err != nil {
		t.Fatalf("RunScriptsSequentially failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Scripts should run in alphabetical order: a-first, m-second, z-third
	// So the final value should be "3"
	metadata := resultObj["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})

	if labels["order"] != "3" {
		t.Errorf("Expected order label to be '3' (z-third ran last), got %v", labels["order"])
	}
}

func TestRunScriptsSequentially_PartialFailure(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	scripts := map[string]string{
		"a-good": `
			if object.metadata == nil then
				object.metadata = {}
			end
			if object.metadata.labels == nil then
				object.metadata.labels = {}
			end
			object.metadata.labels["step1"] = "success"
		`,
		"b-bad": `this will fail!@#$`,
		"c-good": `
			object.metadata.labels["step3"] = "success"
		`,
	}

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
	}

	inputJSON, _ := json.Marshal(inputObj)

	// Should not return error even if one script fails
	result, err := runner.RunScriptsSequentially(scripts, inputJSON)
	if err != nil {
		t.Fatalf("RunScriptsSequentially should not fail on script errors: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check that good scripts ran
	metadata := resultObj["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})

	if labels["step1"] != "success" {
		t.Errorf("Expected step1 to be 'success', got %v", labels["step1"])
	}
	if labels["step3"] != "success" {
		t.Errorf("Expected step3 to be 'success', got %v", labels["step3"])
	}
}

func TestRunScriptsSequentially_EmptyScripts(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	scripts := map[string]string{}

	inputObj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
	}

	inputJSON, _ := json.Marshal(inputObj)

	result, err := runner.RunScriptsSequentially(scripts, inputJSON)
	if err != nil {
		t.Fatalf("RunScriptsSequentially failed: %v", err)
	}

	// Result should be unchanged
	if string(result) != string(inputJSON) {
		t.Error("Expected result to be unchanged when no scripts provided")
	}
}

func TestRunScript_GluaModulesAvailable(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	// Test that glua modules are loaded
	script := `
		-- Test that json module is available (from glua)
		local json = require("json")
		local encoded, err = json.stringify({test = "value"})
		if err then
			error("Failed to stringify: " .. err)
		end
		object.encoded = encoded
	`

	inputObj := map[string]interface{}{
		"test": "data",
	}

	inputJSON, _ := json.Marshal(inputObj)

	result, err := runner.RunScript("glua-test", script, inputJSON)
	if err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	var resultObj map[string]interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check that json encoding worked
	if resultObj["encoded"] == nil {
		t.Error("Expected 'encoded' field to be set by script")
	}

	encoded := resultObj["encoded"].(string)
	if !strings.Contains(encoded, "test") {
		t.Errorf("Expected encoded JSON to contain 'test', got %s", encoded)
	}
}

func TestNewScriptRunner(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	runner := NewScriptRunner(logger)

	if runner == nil {
		t.Error("Expected non-nil runner")
	}

	if runner.logger != logger {
		t.Error("Expected logger to be set")
	}
}
