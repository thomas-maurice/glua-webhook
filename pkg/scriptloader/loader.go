package scriptloader

import (
	"context"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// AnnotationPrefix: prefix for all glua-webhook annotations
	AnnotationPrefix = "glua.maurice.fr"
	// AnnotationScripts: annotation key for specifying ConfigMap scripts
	// Format: "namespace/configmap-name,namespace/configmap-name2"
	AnnotationScripts = AnnotationPrefix + "/scripts"
)

// ScriptLoader: loads Lua scripts from Kubernetes ConfigMaps
type ScriptLoader struct {
	clientset kubernetes.Interface
	logger    *log.Logger
}

// NewScriptLoader: creates a new script loader with K8s client
func NewScriptLoader(clientset kubernetes.Interface, logger *log.Logger) *ScriptLoader {
	return &ScriptLoader{
		clientset: clientset,
		logger:    logger,
	}
}

// LoadScriptsFromAnnotations: loads Lua scripts from ConfigMaps specified in object annotations
// Annotation format: glua.maurice.fr/scripts: "namespace/configmap1,namespace/configmap2"
// Each ConfigMap should contain a single Lua script in a key named "script.lua"
// Returns a map of scriptName -> scriptContent
func (l *ScriptLoader) LoadScriptsFromAnnotations(ctx context.Context, annotations map[string]string) (map[string]string, error) {
	if annotations == nil {
		l.logger.Printf("No annotations found on object")
		return nil, nil
	}

	scriptsAnnotation, exists := annotations[AnnotationScripts]
	if !exists {
		l.logger.Printf("No %s annotation found", AnnotationScripts)
		return nil, nil
	}

	l.logger.Printf("Found scripts annotation: %s", scriptsAnnotation)

	// Parse the annotation: "namespace/configmap1,namespace/configmap2"
	configMapRefs := strings.Split(scriptsAnnotation, ",")
	scripts := make(map[string]string)

	for _, ref := range configMapRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}

		// Parse namespace/name
		parts := strings.Split(ref, "/")
		if len(parts) != 2 {
			l.logger.Printf("WARNING: Invalid ConfigMap reference format: %s (expected namespace/name)", ref)
			continue
		}

		namespace := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])

		l.logger.Printf("Loading script from ConfigMap %s/%s", namespace, name)

		// Fetch the ConfigMap
		cm, err := l.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			l.logger.Printf("ERROR: Failed to fetch ConfigMap %s/%s: %v", namespace, name, err)
			return nil, fmt.Errorf("failed to fetch ConfigMap %s/%s: %w", namespace, name, err)
		}

		// Extract the script from the ConfigMap
		// Look for "script.lua" key
		scriptContent, exists := cm.Data["script.lua"]
		if !exists {
			l.logger.Printf("WARNING: ConfigMap %s/%s does not contain 'script.lua' key", namespace, name)
			continue
		}

		if scriptContent == "" {
			l.logger.Printf("WARNING: ConfigMap %s/%s has empty 'script.lua' content", namespace, name)
			continue
		}

		// Use namespace/name as the script identifier
		scriptName := fmt.Sprintf("%s/%s", namespace, name)
		scripts[scriptName] = scriptContent
		l.logger.Printf("Loaded script %s (length: %d bytes)", scriptName, len(scriptContent))
	}

	l.logger.Printf("Successfully loaded %d scripts from ConfigMaps", len(scripts))
	return scripts, nil
}

// ParseAnnotation: helper to parse the scripts annotation into namespace/name pairs
func ParseAnnotation(annotation string) []struct{ Namespace, Name string } {
	var result []struct{ Namespace, Name string }

	refs := strings.Split(annotation, ",")
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}

		parts := strings.Split(ref, "/")
		if len(parts) != 2 {
			continue
		}

		result = append(result, struct{ Namespace, Name string }{
			Namespace: strings.TrimSpace(parts[0]),
			Name:      strings.TrimSpace(parts[1]),
		})
	}

	return result
}
