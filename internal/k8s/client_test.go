package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultNamespace(t *testing.T) {
	assert.Equal(t, "llm-eval", DefaultNamespace)
}

func TestClientNamespace(t *testing.T) {
	// Test that we can create a mock client setup
	cfg := DefaultClientConfig()
	client := &Client{namespace: cfg.Namespace}
	
	assert.Equal(t, "llm-eval", client.Namespace())
}

func TestClientSetNamespace(t *testing.T) {
	cfg := DefaultClientConfig()
	client := &Client{namespace: cfg.Namespace}
	
	assert.Equal(t, "llm-eval", client.Namespace())
	
	client.SetNamespace("custom-ns")
	assert.Equal(t, "custom-ns", client.Namespace())
}

func TestClientConfigOptions(t *testing.T) {
	// Test all config options
	cfg := &ClientConfig{}
	
	WithNamespace("test-ns")(cfg)
	WithKubeconfig("/path/kubeconfig")(cfg)
	WithContext("test-context")(cfg)
	
	assert.Equal(t, "test-ns", cfg.Namespace)
	assert.Equal(t, "/path/kubeconfig", cfg.KubeconfigPath)
	assert.Equal(t, "test-context", cfg.Context)
}

func TestDefaultClientConfigValues(t *testing.T) {
	cfg := DefaultClientConfig()
	
	assert.Equal(t, DefaultNamespace, cfg.Namespace)
	assert.Empty(t, cfg.KubeconfigPath)
	assert.Empty(t, cfg.Context)
}

func TestClientConfigNamespaceOverride(t *testing.T) {
	cfg := &ClientConfig{Namespace: "default"}
	WithNamespace("override")(cfg)
	
	assert.Equal(t, "override", cfg.Namespace)
}
