package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"thechat/pkg/webhook"
)

var (
	port           = flag.Int("port", 8443, "Webhook server port")
	certFile       = flag.String("cert", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	keyFile        = flag.String("key", "/etc/webhook/certs/tls.key", "TLS key file")
	kubeconfig     = flag.String("kubeconfig", "", "Path to kubeconfig file (leave empty for in-cluster)")
	mutatingPath   = flag.String("mutating-path", "/mutate", "Path for mutating webhook")
	validatingPath = flag.String("validating-path", "/validate", "Path for validating webhook")
)

func main() {
	flag.Parse()

	// Set up logging
	logger := log.New(os.Stdout, "[glua-webhook] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting glua-webhook server")
	logger.Printf("Mutating webhook path: %s", *mutatingPath)
	logger.Printf("Validating webhook path: %s", *validatingPath)
	logger.Printf("Server port: %d", *port)

	// Create Kubernetes clientset
	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		logger.Printf("Using kubeconfig file: %s", *kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
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
	mux.Handle(*mutatingPath, mutatingHandler)
	mux.Handle(*validatingPath, validatingHandler)

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
	logger.Printf("  - %s (mutating webhook)", *mutatingPath)
	logger.Printf("  - %s (validating webhook)", *validatingPath)
	logger.Printf("  - /healthz (health check)")
	logger.Printf("  - /readyz (readiness check)")

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", *port),
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	logger.Printf("Starting HTTPS server on port %d", *port)
	logger.Printf("Using TLS certificate: %s", *certFile)
	logger.Printf("Using TLS key: %s", *keyFile)

	if err := server.ListenAndServeTLS(*certFile, *keyFile); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
