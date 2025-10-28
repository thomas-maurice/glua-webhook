package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"thechat/pkg/luarunner"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Test Lua scripts locally before deploying as webhooks",
	Long: `Execute a Lua script on a Kubernetes object (JSON format) and print the result.

This command is for testing scripts locally before creating ConfigMaps and
deploying them as webhooks. Use it to:
  - Test scripts before deployment
  - Debug script logic locally
  - Verify transformations work as expected

The script receives the object as a global 'object' variable and can modify
it in place. The modified object is printed to stdout.`,
	Example: `  # Test script on existing Pod
  kubectl get pod nginx -o json | glua-webhook exec --script add-label.lua

  # Test script on file
  glua-webhook exec --script inject-sidecar.lua --input pod.json --output modified.json

  # Test multiple scripts in sequence (simulating webhook chaining)
  kubectl get pod nginx -o json | \
    glua-webhook exec --script add-labels.lua | \
    glua-webhook exec --script inject-sidecar.lua`,
	Run: runExec,
}

// exec command flags
var (
	execScript  string
	execInput   string
	execOutput  string
	execVerbose bool
)

func init() {
	execCmd.Flags().StringVarP(&execScript, "script", "s", "", "Path to Lua script file (required)")
	execCmd.Flags().StringVarP(&execInput, "input", "i", "", "Path to input JSON file (default: stdin)")
	execCmd.Flags().StringVarP(&execOutput, "output", "o", "", "Path to output JSON file (default: stdout)")
	execCmd.Flags().BoolVarP(&execVerbose, "verbose", "v", false, "Verbose logging")
	if err := execCmd.MarkFlagRequired("script"); err != nil {
		panic(fmt.Sprintf("failed to mark script flag as required: %v", err))
	}
}

func runExec(cmd *cobra.Command, args []string) {
	// Set up logger
	logger := log.New(os.Stderr, "[glua-webhook] ", log.LstdFlags)
	if !execVerbose {
		logger.SetOutput(io.Discard)
	}

	// Read script file
	scriptContent, err := os.ReadFile(execScript)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script file %s: %v\n", execScript, err)
		os.Exit(1)
	}
	logger.Printf("Loaded script from %s (%d bytes)", execScript, len(scriptContent))

	// Read input (stdin or file)
	var inputData []byte
	if execInput == "" {
		logger.Printf("Reading input from stdin")
		inputData, err = io.ReadAll(os.Stdin)
	} else {
		logger.Printf("Reading input from %s", execInput)
		inputData, err = os.ReadFile(execInput)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Validate input is JSON
	var obj interface{}
	if err := json.Unmarshal(inputData, &obj); err != nil {
		fmt.Fprintf(os.Stderr, "Error: input is not valid JSON: %v\n", err)
		os.Exit(1)
	}
	logger.Printf("Validated input JSON (%d bytes)", len(inputData))

	// Create script runner
	runner := luarunner.NewScriptRunner(logger)

	// Execute script
	scripts := map[string]string{
		execScript: string(scriptContent),
	}

	logger.Printf("Executing script %s", execScript)
	outputData, err := runner.RunScriptsSequentially(scripts, inputData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
		os.Exit(1)
	}
	logger.Printf("Script execution completed successfully")

	// Write output (stdout or file)
	if execOutput == "" {
		fmt.Println(string(outputData))
	} else {
		if err := os.WriteFile(execOutput, outputData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output to %s: %v\n", execOutput, err)
			os.Exit(1)
		}
		logger.Printf("Output written to %s (%d bytes)", execOutput, len(outputData))
	}
}
