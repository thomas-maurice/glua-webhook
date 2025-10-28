package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServeHTTP_InvalidMethod(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	req := httptest.NewRequest(http.MethodGet, "/mutate", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewBufferString("invalid json"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestServeHTTP_NoScripts(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	pod := corev1.Pod{
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

	podJSON, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Namespace: "default",
			Name:      "test-pod",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}

	admissionJSON, _ := json.Marshal(admissionReview)

	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewBuffer(admissionJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Expected request to be allowed")
	}

	if response.Response.Patch != nil {
		t.Error("Expected no patch when no scripts are provided")
	}
}

func TestServeHTTP_WithScripts_Mutating(t *testing.T) {
	// Create ConfigMap with Lua script
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "add-label-script",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `
					if object.metadata.labels == nil then
						object.metadata.labels = {}
					end
					object.metadata.labels["injected"] = "true"
				`,
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"glua.maurice.fr/scripts": "default/add-label-script",
			},
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

	podJSON, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Namespace: "default",
			Name:      "test-pod",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}

	admissionJSON, _ := json.Marshal(admissionReview)

	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewBuffer(admissionJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Response.Allowed {
		t.Error("Expected request to be allowed")
	}

	if response.Response.Patch == nil {
		t.Error("Expected patch to be present")
	}

	if response.Response.PatchType == nil || *response.Response.PatchType != admissionv1.PatchTypeJSONPatch {
		t.Error("Expected JSONPatch type")
	}
}

func TestServeHTTP_Validating(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "validate-script",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `
					-- Simple validation script
					if object.metadata.name == "invalid" then
						error("Invalid name")
					end
				`,
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "validating")

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"glua.maurice.fr/scripts": "default/validate-script",
			},
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

	podJSON, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Namespace: "default",
			Name:      "valid-pod",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}

	admissionJSON, _ := json.Marshal(admissionReview)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(admissionJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should be allowed even if validation logic runs
	if !response.Response.Allowed {
		t.Error("Expected request to be allowed (validation errors are ignored)")
	}

	// Validating webhooks should not have patches
	if response.Response.Patch != nil {
		t.Error("Expected no patch for validating webhook")
	}
}

func TestServeHTTP_ConfigMapNotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"glua.maurice.fr/scripts": "default/nonexistent",
			},
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

	podJSON, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Namespace: "default",
			Name:      "test-pod",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: podJSON,
			},
		},
	}

	admissionJSON, _ := json.Marshal(admissionReview)

	req := httptest.NewRequest(http.MethodPost, "/mutate", bytes.NewBuffer(admissionJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response admissionv1.AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should be rejected when ConfigMap is not found
	if response.Response.Allowed {
		t.Error("Expected request to be rejected when ConfigMap not found")
	}
}

func TestHandleAdmissionRequest_InvalidObjectJSON(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	handler := NewWebhookHandler(clientset, logger, "mutating")

	req := &admissionv1.AdmissionRequest{
		UID: "test-uid",
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		},
		Namespace: "default",
		Name:      "test-pod",
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: []byte(`{invalid json}`),
		},
	}

	response := handler.handleAdmissionRequest(context.Background(), req)

	if response.Allowed {
		t.Error("Expected request to be rejected for invalid JSON")
	}

	if response.Result == nil || response.Result.Message == "" {
		t.Error("Expected error message in response")
	}
}

func TestCreateJSONPatch(t *testing.T) {
	original := []byte(`{"name": "test", "value": 1}`)
	modified := []byte(`{"name": "test", "value": 2, "new": "field"}`)

	patch, err := createJSONPatch(original, modified)
	if err != nil {
		t.Fatalf("createJSONPatch failed: %v", err)
	}

	if patch == nil {
		t.Error("Expected non-nil patch")
	}

	// Verify patch is valid JSON
	var patchObj []map[string]interface{}
	if err := json.Unmarshal(patch, &patchObj); err != nil {
		t.Fatalf("Patch is not valid JSON: %v", err)
	}
}

func TestCreateJSONPatch_InvalidJSON(t *testing.T) {
	original := []byte(`{invalid}`)
	modified := []byte(`{"valid": "json"}`)

	_, err := createJSONPatch(original, modified)
	if err == nil {
		t.Error("Expected error for invalid original JSON")
	}

	original = []byte(`{"valid": "json"}`)
	modified = []byte(`{invalid}`)

	_, err = createJSONPatch(original, modified)
	if err == nil {
		t.Error("Expected error for invalid modified JSON")
	}
}

func TestNewWebhookHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	handler := NewWebhookHandler(clientset, logger, "mutating")

	if handler == nil {
		t.Error("Expected non-nil handler")
	}

	if handler.clientset == nil {
		t.Error("Expected clientset to be set")
	}

	if handler.logger != logger {
		t.Error("Expected logger to be set")
	}

	if handler.webhookType != "mutating" {
		t.Errorf("Expected webhook type 'mutating', got %s", handler.webhookType)
	}

	if handler.scriptLoader == nil {
		t.Error("Expected script loader to be initialized")
	}

	if handler.scriptRunner == nil {
		t.Error("Expected script runner to be initialized")
	}
}

func TestNewWebhookHandler_Validating(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	handler := NewWebhookHandler(clientset, logger, "validating")

	if handler.webhookType != "validating" {
		t.Errorf("Expected webhook type 'validating', got %s", handler.webhookType)
	}
}
