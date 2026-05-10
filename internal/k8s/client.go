package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Constants for label keys
const (
	// AppLabelKey is the label key for application identification
	AppLabelKey = "app"
	// AppLabelValue is the label value for llm-eval app
	AppLabelValue = "llm-eval"

	// EvalIDLabelKey is the label key for evaluation ID
	EvalIDLabelKey = "eval-id"
	// ModelLabelKey is the label key for model name
	ModelLabelKey = "model"
	// DatasetLabelKey is the label key for dataset name
	DatasetLabelKey = "dataset"
)

// Default namespace for evaluations
const DefaultNamespace = "llm-eval"

// ClientConfig holds configuration for Kubernetes client
type ClientConfig struct {
	// Namespace is the target namespace for operations
	Namespace string
	// KubeconfigPath is the path to kubeconfig file (empty for in-cluster)
	KubeconfigPath string
	// Context is the kubeconfig context to use (empty for default)
	Context string
}

// ConfigOption is a functional option for ClientConfig
type ConfigOption func(*ClientConfig)

// WithNamespace sets the namespace
func WithNamespace(ns string) ConfigOption {
	return func(cfg *ClientConfig) {
		cfg.Namespace = ns
	}
}

// WithKubeconfig sets the kubeconfig path
func WithKubeconfig(path string) ConfigOption {
	return func(cfg *ClientConfig) {
		cfg.KubeconfigPath = path
	}
}

// WithContext sets the kubeconfig context
func WithContext(ctx string) ConfigOption {
	return func(cfg *ClientConfig) {
		cfg.Context = ctx
	}
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Namespace: DefaultNamespace,
	}
}

// NewClient creates a new Kubernetes client from config
// If kubeconfig path is empty, it tries in-cluster config first
func NewClient(cfg *ClientConfig) (*Client, error) {
	var config *rest.Config
	var err error

	if cfg.KubeconfigPath != "" {
		// Load from kubeconfig file
		config, err = loadKubeconfig(cfg.KubeconfigPath, cfg.Context)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", cfg.KubeconfigPath, err)
		}
	} else {
		// Try in-cluster config first, fall back to default kubeconfig
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to default kubeconfig location
			config, err = rest.InClusterConfig()
			if err != nil {
				// Try default kubeconfig path
				config, err = loadKubeconfig("", "")
				if err != nil {
					return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
				}
			}
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		namespace: cfg.Namespace,
		config:    config,
	}, nil
}

// loadKubeconfig loads kubeconfig from file or uses default
func loadKubeconfig(kubeconfigPath, context string) (*rest.Config, error) {
	loader, err := newConfigLoader(kubeconfigPath, context)
	if err != nil {
		return nil, err
	}
	return loader.Load()
}

// Client wraps the Kubernetes clientset with additional functionality
type Client struct {
	clientset kubernetes.Interface
	namespace string
	config    *rest.Config
}

// Namespace returns the configured namespace
func (c *Client) Namespace() string {
	return c.namespace
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// RESTConfig returns the REST config used by this client
func (c *Client) RESTConfig() *rest.Config {
	return c.config
}

// EnsureNamespace creates the namespace if it doesn't exist
func (c *Client) EnsureNamespace(ctx context.Context) error {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, c.namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace exists
		return nil
	}

	// Create namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
			Labels: map[string]string{
				AppLabelKey: AppLabelValue,
			},
		},
	}

	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", c.namespace, err)
	}

	return nil
}

// SetNamespace changes the client's namespace
func (c *Client) SetNamespace(ns string) {
	c.namespace = ns
}
