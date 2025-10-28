package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"thechat/pkg/webhook"
)

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
  glua-webhook webhook --port 8443 --cert /certs/tls.crt --key /certs/tls.key`,
	Run: runWebhook,
}

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
	webhookCmd.Flags().IntVar(&webhookPort, "port", 8443, "Webhook server port")
	webhookCmd.Flags().StringVar(&webhookCert, "cert", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	webhookCmd.Flags().StringVar(&webhookKey, "key", "/etc/webhook/certs/tls.key", "TLS key file")
	webhookCmd.Flags().StringVar(&webhookKubeconfig, "kubeconfig", "", "Path to kubeconfig file (leave empty for in-cluster)")
	webhookCmd.Flags().StringVar(&webhookMutatingPath, "mutating-path", "/mutate", "Path for mutating webhook")
	webhookCmd.Flags().StringVar(&webhookValidatingPath, "validating-path", "/validate", "Path for validating webhook")
}

func runWebhook(cmd *cobra.Command, args []string) {
	// Set up logging
	logger := log.New(os.Stdout, "[glua-webhook] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting glua-webhook in webhook mode")
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
		_, _ = fmt.Fprintf(w, "ok")
	})

	// Readiness check endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "ready")
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
