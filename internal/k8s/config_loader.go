package k8s

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ConfigLoader handles loading Kubernetes configuration
type ConfigLoader struct {
	kubeconfig string
	context    string
}

// newConfigLoader creates a new config loader
func newConfigLoader(kubeconfig, context string) (*ConfigLoader, error) {
	return &ConfigLoader{
		kubeconfig: kubeconfig,
		context:    context,
	}, nil
}

// Load loads the Kubernetes configuration
func (l *ConfigLoader) Load() (*rest.Config, error) {
	// First, try in-cluster config if no kubeconfig specified
	if l.kubeconfig == "" {
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}
	}

	// Load from kubeconfig file
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if l.kubeconfig != "" {
		loadingRules.ExplicitPath = l.kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if l.context != "" {
		configOverrides.CurrentContext = l.context
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	return kubeConfig.ClientConfig()
}

// LoadFromKubeconfig loads configuration from a specific kubeconfig file
func LoadFromKubeconfig(kubeconfigPath string) (*rest.Config, error) {
	loader := &ConfigLoader{kubeconfig: kubeconfigPath}
	return loader.Load()
}

// LoadInCluster loads configuration from in-cluster service account
func LoadInCluster() (*rest.Config, error) {
	return rest.InClusterConfig()
}

// LoadDefault loads configuration with fallback: in-cluster -> default kubeconfig
func LoadDefault() (*rest.Config, error) {
	// Try in-cluster first
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fall back to default kubeconfig
	loader := &ConfigLoader{}
	return loader.Load()
}
