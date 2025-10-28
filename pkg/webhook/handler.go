package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"thechat/pkg/luarunner"
	"thechat/pkg/scriptloader"
)

// WebhookHandler: handles admission webhook requests (both mutating and validating)
type WebhookHandler struct {
	clientset    kubernetes.Interface
	scriptLoader *scriptloader.ScriptLoader
	scriptRunner *luarunner.ScriptRunner
	logger       *log.Logger
	webhookType  string // "mutating" or "validating"
}

// NewWebhookHandler: creates a new webhook handler
func NewWebhookHandler(clientset kubernetes.Interface, logger *log.Logger, webhookType string) *WebhookHandler {
	return &WebhookHandler{
		clientset:    clientset,
		scriptLoader: scriptloader.NewScriptLoader(clientset, logger),
		scriptRunner: luarunner.NewScriptRunner(logger),
		logger:       logger,
		webhookType:  webhookType,
	}
}

// ServeHTTP: implements http.Handler interface for webhook requests
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Printf("Received %s webhook request from %s", h.webhookType, r.RemoteAddr)

	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.logger.Printf("ERROR: Invalid method %s, only POST allowed", r.Method)
		http.Error(w, "only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode the admission review request
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		h.logger.Printf("ERROR: Failed to decode admission review: %v", err)
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// Process the request
	response := h.handleAdmissionRequest(r.Context(), admissionReview.Request)

	// Construct the response
	admissionReview.Response = response
	admissionReview.Response.UID = admissionReview.Request.UID

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(admissionReview); err != nil {
		h.logger.Printf("ERROR: Failed to encode response: %v", err)
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Printf("Successfully sent %s webhook response (allowed: %v)", h.webhookType, response.Allowed)
}

// handleAdmissionRequest: processes an admission request and returns a response
func (h *WebhookHandler) handleAdmissionRequest(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	h.logger.Printf("Processing %s admission request: Kind=%s, Namespace=%s, Name=%s, Operation=%s",
		h.webhookType, req.Kind.Kind, req.Namespace, req.Name, req.Operation)

	// Default response: allow with no changes
	response := &admissionv1.AdmissionResponse{
		Allowed: true,
	}

	// Extract object metadata to get annotations
	var metadata struct {
		Metadata metav1.ObjectMeta `json:"metadata"`
	}

	if err := json.Unmarshal(req.Object.Raw, &metadata); err != nil {
		h.logger.Printf("ERROR: Failed to unmarshal object metadata: %v", err)
		response.Allowed = false
		response.Result = &metav1.Status{
			Message: fmt.Sprintf("failed to parse object metadata: %v", err),
		}
		return response
	}

	h.logger.Printf("Object annotations: %v", metadata.Metadata.Annotations)

	// Load scripts from ConfigMaps based on annotations
	scripts, err := h.scriptLoader.LoadScriptsFromAnnotations(ctx, metadata.Metadata.Annotations)
	if err != nil {
		h.logger.Printf("ERROR: Failed to load scripts: %v", err)
		response.Allowed = false
		response.Result = &metav1.Status{
			Message: fmt.Sprintf("failed to load scripts: %v", err),
		}
		return response
	}

	// If no scripts found, allow the request as-is
	if len(scripts) == 0 {
		h.logger.Printf("No scripts to execute, allowing request as-is")
		return response
	}

	// For validating webhooks, we don't modify the object
	if h.webhookType == "validating" {
		h.logger.Printf("Validating webhook: executing %d scripts for validation", len(scripts))
		// Run scripts to validate (errors are logged but ignored per requirements)
		_, err := h.scriptRunner.RunScriptsSequentially(scripts, req.Object.Raw)
		if err != nil {
			h.logger.Printf("WARNING: Validation scripts encountered errors (ignoring): %v", err)
		}
		// Always allow for now (per requirements: ignore script failures)
		response.Allowed = true
		return response
	}

	// For mutating webhooks, execute scripts and return patches
	h.logger.Printf("Mutating webhook: executing %d scripts", len(scripts))
	modifiedJSON, err := h.scriptRunner.RunScriptsSequentially(scripts, req.Object.Raw)
	if err != nil {
		h.logger.Printf("ERROR: Failed to execute scripts: %v", err)
		response.Allowed = false
		response.Result = &metav1.Status{
			Message: fmt.Sprintf("failed to execute scripts: %v", err),
		}
		return response
	}

	// Check if the object was modified
	if string(modifiedJSON) != string(req.Object.Raw) {
		h.logger.Printf("Object was modified by scripts, creating JSON patch")

		// Create a JSON patch
		patchType := admissionv1.PatchTypeJSONPatch
		response.PatchType = &patchType

		// Generate JSON patch
		patch, err := createJSONPatch(req.Object.Raw, modifiedJSON)
		if err != nil {
			h.logger.Printf("ERROR: Failed to create JSON patch: %v", err)
			response.Allowed = false
			response.Result = &metav1.Status{
				Message: fmt.Sprintf("failed to create patch: %v", err),
			}
			return response
		}

		response.Patch = patch
		h.logger.Printf("Applied patch of length %d bytes", len(patch))
	} else {
		h.logger.Printf("Object was not modified by scripts")
	}

	return response
}

// createJSONPatch: creates a JSON patch between original and modified objects
func createJSONPatch(original, modified []byte) ([]byte, error) {
	// For simplicity, we'll use a replace operation on the entire object
	// A more sophisticated implementation could use a proper JSON patch library
	var originalObj, modifiedObj interface{}

	if err := json.Unmarshal(original, &originalObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal original: %w", err)
	}

	if err := json.Unmarshal(modified, &modifiedObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal modified: %w", err)
	}

	// Create a simple patch that replaces specific fields
	// This is a simplified approach - in production you'd want to use a proper JSON patch library
	patch := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/",
			"value": modifiedObj,
		},
	}

	return json.Marshal(patch)
}
