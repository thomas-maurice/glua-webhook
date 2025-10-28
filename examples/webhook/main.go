// Package main implements a complete, production-ready example webhook using glua-webhook.
//
// This example demonstrates:
//   - Full webhook server setup with proper TLS configuration
//   - Comprehensive error handling and logging
//   - Script execution with all available glua modules
//   - Health checks and readiness probes
//   - Graceful shutdown
//   - Best practices for production deployment
//
// Usage:
//
//	go run examples/webhook/main.go \
//	  --port 8443 \
//	  --cert /path/to/tls.crt \
//	  --key /path/to/tls.key \
//	  --kubeconfig ~/.kube/config
//
// The webhook exposes:
//   - POST /mutate - Mutating admission webhook endpoint
//   - POST /validate - Validating admission webhook endpoint
//   - GET /healthz - Health check endpoint
//   - GET /readyz - Readiness check endpoint
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"thechat/pkg/webhook"
)

var (
	// Command-line flags
	port             int
	certFile         string
	keyFile          string
	kubeconfig       string
	mutatingPath     string
	validatingPath   string
	shutdownTimeout  int
	readTimeoutSec   int
	writeTimeoutSec  int
	maxHeaderBytes   int
	enableValidation bool
)

func init() {
	flag.IntVar(&port, "port", 8443, "HTTPS server port")
	flag.StringVar(&certFile, "cert", "/etc/webhook/certs/tls.crt", "TLS certificate file path")
	flag.StringVar(&keyFile, "key", "/etc/webhook/certs/tls.key", "TLS private key file path")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (leave empty for in-cluster config)")
	flag.StringVar(&mutatingPath, "mutating-path", "/mutate", "Path for mutating webhook endpoint")
	flag.StringVar(&validatingPath, "validating-path", "/validate", "Path for validating webhook endpoint")
	flag.IntVar(&shutdownTimeout, "shutdown-timeout", 30, "Graceful shutdown timeout in seconds")
	flag.IntVar(&readTimeoutSec, "read-timeout", 15, "HTTP read timeout in seconds")
	flag.IntVar(&writeTimeoutSec, "write-timeout", 15, "HTTP write timeout in seconds")
	flag.IntVar(&maxHeaderBytes, "max-header-bytes", 1048576, "Maximum HTTP header size in bytes (1MB default)")
	flag.BoolVar(&enableValidation, "enable-validation", true, "Enable validating webhook endpoint")
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "[webhook-example] ", log.LstdFlags)

	logger.Printf("Starting glua-webhook example server on port %d", port)
	logger.Printf("Mutating endpoint: %s", mutatingPath)
	if enableValidation {
		logger.Printf("Validating endpoint: %s", validatingPath)
	}

	// Create Kubernetes client
	clientset, err := createKubernetesClient()
	if err != nil {
		logger.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create webhook handlers
	mutatingHandler := webhook.NewWebhookHandler(clientset, logger, "mutating")
	var validatingHandler *webhook.WebhookHandler
	if enableValidation {
		validatingHandler = webhook.NewWebhookHandler(clientset, logger, "validating")
	}

	// Setup HTTP mux
	mux := http.NewServeMux()

	// Register webhook endpoints
	mux.Handle(mutatingPath, mutatingHandler)
	if enableValidation {
		mux.Handle(validatingPath, validatingHandler)
	}

	// Register health check endpoints
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", readyzHandler(clientset))

	// Configure TLS
	tlsConfig, err := loadTLSConfig()
	if err != nil {
		logger.Fatalf("Failed to load TLS configuration: %v", err)
	}

	// Create HTTPS server with production-ready settings
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        mux,
		TLSConfig:      tlsConfig,
		ReadTimeout:    time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout:   time.Duration(writeTimeoutSec) * time.Second,
		MaxHeaderBytes: maxHeaderBytes,
		ErrorLog:       logger,
	}

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		logger.Printf("Server listening on https://0.0.0.0:%d", port)
		logger.Printf("TLS certificate: %s", certFile)
		logger.Printf("TLS key: %s", keyFile)

		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	logger.Println("Shutting down server gracefully...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownTimeout)*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("Server forced to shutdown: %v", err)
	} else {
		logger.Println("Server stopped gracefully")
	}
}

// createKubernetesClient creates a Kubernetes clientset from either kubeconfig file or in-cluster config.
//
// Returns:
//   - *kubernetes.Clientset: Kubernetes client
//   - error: Error if client creation fails
func createKubernetesClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfig, err)
		}
	} else {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

// loadTLSConfig loads TLS configuration from certificate and key files.
//
// Returns:
//   - *tls.Config: TLS configuration with secure defaults
//   - error: Error if certificate loading fails
func loadTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}, nil
}

// healthzHandler handles liveness probe requests.
//
// Always returns 200 OK if the server is running.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// readyzHandler handles readiness probe requests.
//
// Checks if the Kubernetes API server is accessible.
// Returns 200 OK if ready, 503 Service Unavailable if not ready.
func readyzHandler(clientset kubernetes.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Try to list namespaces as a readiness check
		_, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(fmt.Sprintf("not ready: %v", err)))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
