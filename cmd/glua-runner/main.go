package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"thechat/pkg/luarunner"
	"thechat/pkg/webhook"
)

var rootCmd = &cobra.Command{
	Use:   "glua-runner",
	Short: "Run Lua scripts as Kubernetes admission webhooks",
	Long: `glua-runner executes Lua scripts as Kubernetes admission webhooks.

The primary use case is running as a webhook server that processes admission
requests from the Kubernetes API server. Scripts are stored in ConfigMaps and
referenced via annotations on resources.

The 'exec' command allows testing scripts locally before deploying them.`,
}

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
  kubectl get pod nginx -o json | glua-runner exec --script add-label.lua

  # Test script on file
  glua-runner exec --script inject-sidecar.lua --input pod.json --output modified.json

  # Test multiple scripts in sequence (simulating webhook chaining)
  kubectl get pod nginx -o json | \
    glua-runner exec --script add-labels.lua | \
    glua-runner exec --script inject-sidecar.lua`,
	Run: runExec,
}

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Run as Kubernetes admission webhook server",
	Long: `Run as a Kubernetes admission webhook server.

This mode starts an HTTPS server that receives AdmissionReview requests
from the Kubernetes API server, executes Lua scripts from ConfigMaps,
and returns modified or validated resources.

Scripts are referenced via the 'glua.maurice.fr/scripts' annotation on
resources, pointing to ConfigMaps containing Lua code.`,
	Example: `  # Run webhook server
  glua-runner webhook --port 8443 --cert /certs/tls.crt --key /certs/tls.key`,
	Run: runWebhook,
}

// exec command flags
var (
	execScript  string
	execInput   string
	execOutput  string
	execVerbose bool
)

// webhook command flags
var (
	webhookPort           int
	webhookCert           string
	webhookKey            string
	webhookKubeconfig     string
	webhookMutatingPath   string
	webhookValidatingPath string
)

func init() {
	// exec command flags
	execCmd.Flags().StringVarP(&execScript, "script", "s", "", "Path to Lua script file (required)")
	execCmd.Flags().StringVarP(&execInput, "input", "i", "", "Path to input JSON file (default: stdin)")
	execCmd.Flags().StringVarP(&execOutput, "output", "o", "", "Path to output JSON file (default: stdout)")
	execCmd.Flags().BoolVarP(&execVerbose, "verbose", "v", false, "Verbose logging")
	execCmd.MarkFlagRequired("script")

	// webhook command flags
	webhookCmd.Flags().IntVar(&webhookPort, "port", 8443, "Webhook server port")
	webhookCmd.Flags().StringVar(&webhookCert, "cert", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	webhookCmd.Flags().StringVar(&webhookKey, "key", "/etc/webhook/certs/tls.key", "TLS key file")
	webhookCmd.Flags().StringVar(&webhookKubeconfig, "kubeconfig", "", "Path to kubeconfig file (leave empty for in-cluster)")
	webhookCmd.Flags().StringVar(&webhookMutatingPath, "mutating-path", "/mutate", "Path for mutating webhook")
	webhookCmd.Flags().StringVar(&webhookValidatingPath, "validating-path", "/validate", "Path for validating webhook")

	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(webhookCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runExec(cmd *cobra.Command, args []string) {
	// Set up logger
	logger := log.New(os.Stderr, "[glua-runner] ", log.LstdFlags)
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

func runWebhook(cmd *cobra.Command, args []string) {
	// Set up logging
	logger := log.New(os.Stdout, "[glua-webhook] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting glua-runner in webhook mode")
	logger.Printf("Mutating webhook path: %s", webhookMutatingPath)
	logger.Printf("Validating webhook path: %s", webhookValidatingPath)
	logger.Printf("Server port: %d", webhookPort)

	// Create Kubernetes clientset
	var config *rest.Config
	var err error

	if webhookKubeconfig != "" {
		logger.Printf("Using kubeconfig file: %s", webhookKubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", webhookKubeconfig)
	} else {
		logger.Printf("Using in-cluster configuration")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		logger.Fatalf("Failed to create Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	logger.Printf("Successfully connected to Kubernetes API")

	// Create webhook handlers
	mutatingHandler := webhook.NewWebhookHandler(clientset, logger, "mutating")
	validatingHandler := webhook.NewWebhookHandler(clientset, logger, "validating")

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.Handle(webhookMutatingPath, mutatingHandler)
	mux.Handle(webhookValidatingPath, validatingHandler)

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	})

	// Readiness check endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ready")
	})

	logger.Printf("Registered handlers:")
	logger.Printf("  - %s (mutating webhook)", webhookMutatingPath)
	logger.Printf("  - %s (validating webhook)", webhookValidatingPath)
	logger.Printf("  - /healthz (health check)")
	logger.Printf("  - /readyz (readiness check)")

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", webhookPort),
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	logger.Printf("Starting HTTPS server on port %d", webhookPort)
	logger.Printf("Using TLS certificate: %s", webhookCert)
	logger.Printf("Using TLS key: %s", webhookKey)

	if err := server.ListenAndServeTLS(webhookCert, webhookKey); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
