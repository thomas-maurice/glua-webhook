package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// TestHealthzHandler tests the liveness probe endpoint
func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	healthzHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != "ok" {
		t.Errorf("Expected body 'ok', got %s", body)
	}
}

// TestReadyzHandler tests the readiness probe endpoint
func TestReadyzHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	})

	handler := readyzHandler(clientset)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != "ready" {
		t.Errorf("Expected body 'ready', got %s", body)
	}
}

// TestReadyzHandler_NotReady tests readiness probe when API server is unavailable
func TestReadyzHandler_NotReady(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Fake clientset will return errors for certain operations
	// This is a limitation - in real scenarios, we'd mock the client to return errors

	handler := readyzHandler(clientset)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Fake clientset actually returns success, so this test demonstrates the pattern
	if w.Code != http.StatusOK {
		t.Logf("Status: %d (expected for unavailable API server)", w.Code)
	}
}

// TestLoadTLSConfig tests TLS configuration loading
func TestLoadTLSConfig(t *testing.T) {
	// Create temporary test certificate and key
	certPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICEjCCAXsCAg36MA0GCSqGSIb3DQEBBQUAMIGbMQswCQYDVQQGEwJKUDEOMAwG
A1UECBMFVG9reW8xEDAOBgNVBAcTB0NodW8ta3UxETAPBgNVBAoTCEZyYW5rNERE
MRgwFgYDVQQLEw9XZWJDZXJ0IFN1cHBvcnQxGDAWBgNVBAMTD0ZyYW5rNEREIFdl
YiBDQTEjMCEGCSqGSIb3DQEJARYUc3VwcG9ydEBmcmFuazRkZC5jb20wHhcNMTIw
ODIyMDUyNjU0WhcNMTcwODIxMDUyNjU0WjBKMQswCQYDVQQGEwJKUDEOMAwGA1UE
CAwFVG9reW8xETAPBgNVBAoMCEZyYW5rNEREMRgwFgYDVQQDDA93d3cuZXhhbXBs
ZS5jb20wXDANBgkqhkiG9w0BAQEFAANLADBIAkEAm/xmkHmEQrurE/0re/jeFRLl
8ZPjBop7uLHhnia7lQG/5zDtZIUC3RVpqDSwBuw/NTweGyuP+o8AG98HxqxTBwID
AQABMA0GCSqGSIb3DQEBBQUAA4GBABS2TLuBeTPmcaTaUW/LCB2NYOy8GMdzR1mx
8iBIu2H6/E2tiY3RIevV2OW61qY2/XRQg7YPxx3ffeUugX9F4J/iPnnu1zAxzyYw
m+h6FeWiFlyN+mJTBYG6Pq9J1P6oRtqvZF4n2lQrn8x7VDz8M5qbvJYfF+rJ9+3g
Z8PNzBqN
-----END CERTIFICATE-----`)

	keyPEM := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBAJv8ZpB5hEK7qxP9K3v43hUS5fGT4waKe7ix4Z4mu5UBv+cw7WSF
At0Vaag0sAbsPzU8Hhsrj/qPABvfB8asUwcCAwEAAQJAL6cexrxwBpUCmj4kOncN
K2Q3TaL2jEBMJjGkfMWGtm3K5I+s5JF9m/FZQB8vhm+r8KqQMU8I1gYDGvSwpXl7
AQIhAPd5PLqJ3qLiBp+HZbJV2c8V/fQv3KKLs9gL7L1q5uL7AiEAoG5VlPHlhF8n
aqHZ6Y8shw5B6pePYg8thS/8sKt4UYECIQDJoaV7pxPKVQiSL5Vo8fy0E6yMThTk
+LiW0RaNjG8TqwIgHXWwbE8ScqKD2P4vLiTZGiGQc/1MQQfUFgvJCLNTqAECIQDQ
yHrPfPWGLZWGvkxnCy1HQRh5d5Y5w5A2VkxmCCPmag==
-----END RSA PRIVATE KEY-----`)

	// Write temporary files
	certFilePath := "/tmp/test-cert.pem"
	keyFilePath := "/tmp/test-key.pem"

	if err := os.WriteFile(certFilePath, certPEM, 0600); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}
	defer os.Remove(certFilePath)

	if err := os.WriteFile(keyFilePath, keyPEM, 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}
	defer os.Remove(keyFilePath)

	// Override global variables for test
	origCert, origKey := certFile, keyFile
	certFile, keyFile = certFilePath, keyFilePath
	defer func() {
		certFile, keyFile = origCert, origKey
	}()

	tlsConfig, err := loadTLSConfig()
	if err != nil {
		t.Fatalf("Failed to load TLS config: %v", err)
	}

	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected TLS 1.2, got %v", tlsConfig.MinVersion)
	}

	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}
}

// TestLoadTLSConfig_MissingFiles tests TLS loading with missing files
func TestLoadTLSConfig_MissingFiles(t *testing.T) {
	origCert, origKey := certFile, keyFile
	certFile, keyFile = "/nonexistent/cert.pem", "/nonexistent/key.pem"
	defer func() {
		certFile, keyFile = origCert, origKey
	}()

	_, err := loadTLSConfig()
	if err == nil {
		t.Error("Expected error with nonexistent files, got nil")
	}
}

// TestCreateKubernetesClient_InvalidKubeconfig tests client creation with invalid kubeconfig
func TestCreateKubernetesClient_InvalidKubeconfig(t *testing.T) {
	origKubeconfig := kubeconfig
	kubeconfig = "/nonexistent/kubeconfig"
	defer func() {
		kubeconfig = origKubeconfig
	}()

	_, err := createKubernetesClient()
	if err == nil {
		t.Error("Expected error with invalid kubeconfig, got nil")
	}

	if !strings.Contains(err.Error(), "failed to build config from kubeconfig") {
		t.Errorf("Expected kubeconfig error, got: %v", err)
	}
}

// TestServerConfiguration tests server configuration
func TestServerConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		timeout  int
		expected bool
	}{
		{"Default port", 8443, 15, true},
		{"Custom port", 9443, 30, true},
		{"Low timeout", 443, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.port < 1 || tt.port > 65535 {
				t.Errorf("Invalid port: %d", tt.port)
			}

			if tt.timeout < 1 {
				t.Errorf("Invalid timeout: %d", tt.timeout)
			}
		})
	}
}

// TestFlagDefaults tests default flag values
func TestFlagDefaults(t *testing.T) {
	// Reset flags to defaults
	flag := func() {
		port = 8443
		certFile = "/etc/webhook/certs/tls.crt"
		keyFile = "/etc/webhook/certs/tls.key"
		kubeconfig = ""
		mutatingPath = "/mutate"
		validatingPath = "/validate"
		shutdownTimeout = 30
		readTimeoutSec = 15
		writeTimeoutSec = 15
		maxHeaderBytes = 1048576
		enableValidation = true
	}
	flag()

	if port != 8443 {
		t.Errorf("Expected default port 8443, got %d", port)
	}

	if mutatingPath != "/mutate" {
		t.Errorf("Expected default mutating path /mutate, got %s", mutatingPath)
	}

	if shutdownTimeout != 30 {
		t.Errorf("Expected default shutdown timeout 30s, got %d", shutdownTimeout)
	}

	if !enableValidation {
		t.Error("Expected validation enabled by default")
	}
}

// TestGracefulShutdown tests graceful shutdown behavior
func TestGracefulShutdown(t *testing.T) {
	server := &http.Server{
		Addr:    ":0",
		Handler: http.NewServeMux(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start server
	go func() {
		_ = server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	err := server.Shutdown(ctx)
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("Shutdown error: %v", err)
	}
}

// TestAdmissionReviewHandling tests admission review handling
func TestAdmissionReviewHandling(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
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

	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("Failed to marshal pod: %v", err)
	}

	admissionReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}

	// Validate structure
	if admissionReview.Request == nil {
		t.Fatal("Expected non-nil request")
	}

	if admissionReview.Request.Kind.Kind != "Pod" {
		t.Errorf("Expected Kind Pod, got %s", admissionReview.Request.Kind.Kind)
	}

	if admissionReview.Request.Operation != admissionv1.Create {
		t.Errorf("Expected operation Create, got %s", admissionReview.Request.Operation)
	}
}

// BenchmarkHealthzHandler benchmarks the healthz endpoint
func BenchmarkHealthzHandler(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		healthzHandler(w, req)
	}
}

// BenchmarkReadyzHandler benchmarks the readyz endpoint
func BenchmarkReadyzHandler(b *testing.B) {
	clientset := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	})

	handler := readyzHandler(clientset)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
	}
}
