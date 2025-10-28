package luarunner

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/thomas-maurice/glua/pkg/glua"
	"github.com/thomas-maurice/glua/pkg/modules/base64"
	"github.com/thomas-maurice/glua/pkg/modules/fs"
	"github.com/thomas-maurice/glua/pkg/modules/hash"
	"github.com/thomas-maurice/glua/pkg/modules/hex"
	"github.com/thomas-maurice/glua/pkg/modules/http"
	gluajson "github.com/thomas-maurice/glua/pkg/modules/json"
	glualog "github.com/thomas-maurice/glua/pkg/modules/log"
	"github.com/thomas-maurice/glua/pkg/modules/spew"
	"github.com/thomas-maurice/glua/pkg/modules/template"
	"github.com/thomas-maurice/glua/pkg/modules/time"
	"github.com/thomas-maurice/glua/pkg/modules/yaml"
	lua "github.com/yuin/gopher-lua"
)

// ScriptRunner: executes Lua scripts against Kubernetes objects with isolated VM instances
type ScriptRunner struct {
	logger       *log.Logger
	translator   *glua.Translator
	typeRegistry *glua.TypeRegistry
}

// NewScriptRunner: creates a new Lua script runner with logging
func NewScriptRunner(logger *log.Logger) *ScriptRunner {
	registry := glua.NewTypeRegistry()

	// Register common Kubernetes types for stub generation
	// This enables LSP autocompletion and type checking in IDEs
	logger.Printf("Initializing TypeRegistry for Kubernetes types")

	return &ScriptRunner{
		logger:       logger,
		translator:   glua.NewTranslator(),
		typeRegistry: registry,
	}
}

// RegisterType: registers a Kubernetes type with the TypeRegistry for stub generation
// This is used to enable IDE support and type checking for Lua scripts
func (r *ScriptRunner) RegisterType(obj interface{}) error {
	r.logger.Printf("Registering type: %T", obj)
	return r.typeRegistry.Register(obj)
}

// GetTypeRegistry: returns the TypeRegistry for external use (e.g., stub generation)
func (r *ScriptRunner) GetTypeRegistry() *glua.TypeRegistry {
	return r.typeRegistry
}

// loadModules: preloads ALL available glua modules into the Lua state
// This includes: json, yaml, base64, hex, hash, http, log, spew, template, time, fs
// Note: k8sclient and kubernetes modules require rest.Config and are not loaded here
// The webhook provides access to K8s resources through the object global variable
func (r *ScriptRunner) loadModules(L *lua.LState) {
	// Data encoding/decoding
	L.PreloadModule("json", gluajson.Loader)
	L.PreloadModule("yaml", yaml.Loader)
	L.PreloadModule("base64", base64.Loader)
	L.PreloadModule("hex", hex.Loader)

	// Cryptography and hashing
	L.PreloadModule("hash", hash.Loader)

	// Network and HTTP
	L.PreloadModule("http", http.Loader)

	// Utilities
	L.PreloadModule("log", glualog.Loader)
	L.PreloadModule("spew", spew.Loader)
	L.PreloadModule("template", template.Loader)
	L.PreloadModule("time", time.Loader)

	// File system operations
	L.PreloadModule("fs", fs.Loader)

	r.logger.Printf("Loaded glua modules: json, yaml, base64, hex, hash, http, log, spew, template, time, fs")
}

// RunScript: executes a single Lua script against a Kubernetes object
// Each invocation creates a fresh gopher-lua VM instance
// Returns the modified object as JSON bytes and any error
func (r *ScriptRunner) RunScript(scriptName, scriptContent string, objectJSON []byte) ([]byte, error) {
	r.logger.Printf("Running script %s (length: %d bytes) against object (length: %d bytes)",
		scriptName, len(scriptContent), len(objectJSON))

	// Create a new Lua VM instance for this script
	L := lua.NewState()
	defer L.Close()

	// Load glua modules
	r.loadModules(L)
	r.logger.Printf("Loaded glua modules for script %s", scriptName)

	// Parse the input JSON into a Go value
	var obj interface{}
	if err := json.Unmarshal(objectJSON, &obj); err != nil {
		r.logger.Printf("ERROR: Failed to unmarshal JSON for script %s: %v", scriptName, err)
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Register the type for stub generation (best-effort, ignore errors)
	// This helps build LSP type information for IDE support
	if err := r.typeRegistry.Register(obj); err != nil {
		r.logger.Printf("DEBUG: Could not register type for stub generation: %v", err)
	}

	// Convert Go object to Lua value using glua translator
	luaValue, err := r.translator.ToLua(L, obj)
	if err != nil {
		r.logger.Printf("ERROR: Failed to convert object to Lua for script %s: %v", scriptName, err)
		return nil, fmt.Errorf("failed to convert to Lua: %w", err)
	}

	L.SetGlobal("object", luaValue)
	r.logger.Printf("Set global 'object' for script %s", scriptName)

	// Execute the script
	r.logger.Printf("Executing Lua script %s", scriptName)
	if err := L.DoString(scriptContent); err != nil {
		r.logger.Printf("ERROR: Script %s execution failed: %v", scriptName, err)
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// Retrieve the modified object
	modifiedObj := L.GetGlobal("object")

	// Convert back to Go value using glua translator
	var goObj interface{}
	if err := r.translator.FromLua(L, modifiedObj, &goObj); err != nil {
		r.logger.Printf("ERROR: Failed to convert Lua value back to Go for script %s: %v", scriptName, err)
		return nil, fmt.Errorf("failed to convert from Lua: %w", err)
	}

	// Convert back to JSON
	resultJSON, err := json.Marshal(goObj)
	if err != nil {
		r.logger.Printf("ERROR: Failed to marshal result for script %s: %v", scriptName, err)
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	r.logger.Printf("Script %s completed successfully, result length: %d bytes", scriptName, len(resultJSON))
	return resultJSON, nil
}

// RunScriptsSequentially: executes multiple scripts in sequence, each with its own VM
// Scripts are executed in alphabetical order
// If a script fails, it logs the error and continues with remaining scripts
func (r *ScriptRunner) RunScriptsSequentially(scripts map[string]string, objectJSON []byte) ([]byte, error) {
	r.logger.Printf("Running %d scripts sequentially against object", len(scripts))

	// Sort script names alphabetically
	sortedNames := make([]string, 0, len(scripts))
	for name := range scripts {
		sortedNames = append(sortedNames, name)
	}
	// Simple bubble sort for alphabetical order
	for i := 0; i < len(sortedNames); i++ {
		for j := i + 1; j < len(sortedNames); j++ {
			if sortedNames[i] > sortedNames[j] {
				sortedNames[i], sortedNames[j] = sortedNames[j], sortedNames[i]
			}
		}
	}

	currentJSON := objectJSON
	successCount := 0
	failCount := 0

	for _, name := range sortedNames {
		scriptContent := scripts[name]
		r.logger.Printf("Executing script %d/%d: %s", successCount+failCount+1, len(scripts), name)

		result, err := r.RunScript(name, scriptContent, currentJSON)
		if err != nil {
			r.logger.Printf("WARNING: Script %s failed (ignoring): %v", name, err)
			failCount++
			// Continue with remaining scripts using the current state
			continue
		}

		currentJSON = result
		successCount++
		r.logger.Printf("Script %s succeeded, continuing to next script", name)
	}

	r.logger.Printf("Script execution complete: %d succeeded, %d failed", successCount, failCount)
	return currentJSON, nil
}
