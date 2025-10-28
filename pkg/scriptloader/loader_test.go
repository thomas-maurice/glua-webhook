package scriptloader

import (
	"context"
	"log"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadScriptsFromAnnotations_Success(t *testing.T) {
	// Create fake clientset with ConfigMaps
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script1",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `print("script1")`,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script2",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"script.lua": `print("script2")`,
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		AnnotationScripts: "default/script1,kube-system/script2",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("Expected 2 scripts, got %d", len(scripts))
	}

	if scripts["default/script1"] != `print("script1")` {
		t.Errorf("Expected script1 content, got %s", scripts["default/script1"])
	}

	if scripts["kube-system/script2"] != `print("script2")` {
		t.Errorf("Expected script2 content, got %s", scripts["kube-system/script2"])
	}
}

func TestLoadScriptsFromAnnotations_NoAnnotation(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		"some-other-annotation": "value",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	if len(scripts) != 0 {
		t.Errorf("Expected nil or empty scripts, got %d", len(scripts))
	}
}

func TestLoadScriptsFromAnnotations_NilAnnotations(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), nil)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	if len(scripts) != 0 {
		t.Errorf("Expected nil or empty scripts, got %d", len(scripts))
	}
}

func TestLoadScriptsFromAnnotations_ConfigMapNotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		AnnotationScripts: "default/nonexistent",
	}

	_, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err == nil {
		t.Error("Expected error when ConfigMap not found, got nil")
	}
}

func TestLoadScriptsFromAnnotations_MissingScriptKey(t *testing.T) {
	// ConfigMap without script.lua key
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bad-script",
				Namespace: "default",
			},
			Data: map[string]string{
				"wrong-key": `print("wrong")`,
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		AnnotationScripts: "default/bad-script",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	// Should return empty scripts (warning logged but not error)
	if len(scripts) != 0 {
		t.Errorf("Expected 0 scripts when key missing, got %d", len(scripts))
	}
}

func TestLoadScriptsFromAnnotations_EmptyScript(t *testing.T) {
	// ConfigMap with empty script
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-script",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": "",
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		AnnotationScripts: "default/empty-script",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	// Should return empty scripts (warning logged)
	if len(scripts) != 0 {
		t.Errorf("Expected 0 scripts for empty content, got %d", len(scripts))
	}
}

func TestLoadScriptsFromAnnotations_InvalidFormat(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	// Invalid format (no namespace separator)
	annotations := map[string]string{
		AnnotationScripts: "invalid-format",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	// Should return empty scripts (warning logged)
	if len(scripts) != 0 {
		t.Errorf("Expected 0 scripts for invalid format, got %d", len(scripts))
	}
}

func TestLoadScriptsFromAnnotations_MultipleScriptsWithSpaces(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script1",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `print("script1")`,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script2",
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `print("script2")`,
			},
		},
	)

	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	// Annotation with extra spaces
	annotations := map[string]string{
		AnnotationScripts: " default/script1 , default/script2 ",
	}

	scripts, err := loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	if err != nil {
		t.Fatalf("LoadScriptsFromAnnotations failed: %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("Expected 2 scripts, got %d", len(scripts))
	}
}

func TestParseAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		expected   int
	}{
		{
			name:       "single script",
			annotation: "default/script1",
			expected:   1,
		},
		{
			name:       "multiple scripts",
			annotation: "default/script1,kube-system/script2",
			expected:   2,
		},
		{
			name:       "with spaces",
			annotation: " default/script1 , kube-system/script2 ",
			expected:   2,
		},
		{
			name:       "empty annotation",
			annotation: "",
			expected:   0,
		},
		{
			name:       "trailing comma",
			annotation: "default/script1,",
			expected:   1,
		},
		{
			name:       "invalid format",
			annotation: "invalid",
			expected:   0,
		},
		{
			name:       "mixed valid and invalid",
			annotation: "default/script1,invalid,kube-system/script2",
			expected:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAnnotation(tt.annotation)
			if len(result) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestParseAnnotation_Values(t *testing.T) {
	annotation := "default/script1,kube-system/script2"
	result := ParseAnnotation(annotation)

	if len(result) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(result))
	}

	if result[0].Namespace != "default" || result[0].Name != "script1" {
		t.Errorf("Expected default/script1, got %s/%s", result[0].Namespace, result[0].Name)
	}

	if result[1].Namespace != "kube-system" || result[1].Name != "script2" {
		t.Errorf("Expected kube-system/script2, got %s/%s", result[1].Namespace, result[1].Name)
	}
}

func TestNewScriptLoader(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	if loader == nil {
		t.Fatal("Expected non-nil loader")
	}

	if loader.clientset == nil {
		t.Error("Expected clientset to be set")
	}

	if loader.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestAnnotationConstants(t *testing.T) {
	if AnnotationPrefix != "glua.maurice.fr" {
		t.Errorf("Expected annotation prefix 'glua.maurice.fr', got %s", AnnotationPrefix)
	}

	if AnnotationScripts != "glua.maurice.fr/scripts" {
		t.Errorf("Expected annotation 'glua.maurice.fr/scripts', got %s", AnnotationScripts)
	}
}

// Benchmark for script loading
func BenchmarkLoadScriptsFromAnnotations(b *testing.B) {
	objects := []runtime.Object{}
	for i := 0; i < 10; i++ {
		objects = append(objects, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "script" + string(rune(i)),
				Namespace: "default",
			},
			Data: map[string]string{
				"script.lua": `print("test script")`,
			},
		})
	}

	clientset := fake.NewSimpleClientset(objects...)
	logger := log.New(os.Stdout, "[bench] ", log.LstdFlags)
	loader := NewScriptLoader(clientset, logger)

	annotations := map[string]string{
		AnnotationScripts: "default/script0,default/script1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = loader.LoadScriptsFromAnnotations(context.Background(), annotations)
	}
}
