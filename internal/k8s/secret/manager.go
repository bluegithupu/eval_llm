package secret

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/eval_llm/backend/internal/k8s"
)

// APIKeys holds the API keys for different providers
type APIKeys struct {
	OpenAI string
	Claude string
	Qwen   string
}

// SecretData holds the data to be stored in the secret
type SecretData struct {
	EvalID    string
	ModelName string
	Dataset   string
	Keys      APIKeys
}

// SecretManager manages Kubernetes Secrets for API keys
type SecretManager struct {
	client kubernetes.Interface
}

// NewSecretManager creates a new SecretManager
func NewSecretManager(client kubernetes.Interface) *SecretManager {
	return &SecretManager{
		client: client,
	}
}

// CreateSecret creates a Kubernetes Secret with base64 encoded API keys
// The Secret will have owner reference to the Job (if jobName and jobUID provided)
// for garbage collection when the Job is deleted
func (m *SecretManager) CreateSecret(ctx context.Context, namespace string, data *SecretData, jobName, jobUID string) (*v1.Secret, error) {
	if err := validateSecretData(data); err != nil {
		return nil, fmt.Errorf("invalid secret data: %w", err)
	}

	// Create the Secret with base64 encoded keys
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(data.EvalID),
			Namespace: namespace,
			Labels:    k8s.Labels(data.EvalID, data.ModelName, data.Dataset),
		},
		Type: v1.SecretTypeOpaque,
		Data: encodeAPIKeys(data.Keys),
	}

	// Add owner reference to Job if provided
	if jobName != "" && jobUID != "" {
		ownerRef := k8s.JobOwnerReference(jobName, jobUID)
		secret.OwnerReferences = append(secret.OwnerReferences, *ownerRef)
	}

	result, err := m.client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret: %w", err)
	}

	return result, nil
}

// encodeAPIKeys encodes API keys to base64
func encodeAPIKeys(keys APIKeys) map[string][]byte {
	data := make(map[string][]byte)

	if keys.OpenAI != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(keys.OpenAI))
		data["openai-api-key"] = []byte(encoded)
	}
	if keys.Claude != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(keys.Claude))
		data["claude-api-key"] = []byte(encoded)
	}
	if keys.Qwen != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(keys.Qwen))
		data["qwen-api-key"] = []byte(encoded)
	}

	return data
}

// validateSecretData validates the secret data
func validateSecretData(data *SecretData) error {
	if data == nil {
		return fmt.Errorf("secret data is nil")
	}
	if data.EvalID == "" {
		return fmt.Errorf("evalID is required")
	}
	// API keys are optional - evaluations can proceed without real keys in test environments
	// Real API keys should be provided via environment variables in production
	return nil
}

// SecretName returns the standard name for an evaluation Secret
func SecretName(evalID string) string {
	return fmt.Sprintf("eval-secret-%s", evalID)
}

// GetSecret retrieves a Secret by evaluation ID
func (m *SecretManager) GetSecret(ctx context.Context, namespace, evalID string) (*v1.Secret, error) {
	return m.client.CoreV1().Secrets(namespace).Get(ctx, SecretName(evalID), metav1.GetOptions{})
}

// DeleteSecret deletes a Secret by evaluation ID
func (m *SecretManager) DeleteSecret(ctx context.Context, namespace, evalID string) error {
	return m.client.CoreV1().Secrets(namespace).Delete(ctx, SecretName(evalID), metav1.DeleteOptions{})
}

// HasAPIKeys checks if the secret contains any API keys
func HasAPIKeys(secret *v1.Secret) bool {
	if secret == nil || secret.Data == nil {
		return false
	}
	return secret.Data["openai-api-key"] != nil ||
		secret.Data["claude-api-key"] != nil ||
		secret.Data["qwen-api-key"] != nil
}

// GetKeyCount returns the number of API keys in the secret
func GetKeyCount(secret *v1.Secret) int {
	if secret == nil || secret.Data == nil {
		return 0
	}
	count := 0
	if secret.Data["openai-api-key"] != nil {
		count++
	}
	if secret.Data["claude-api-key"] != nil {
		count++
	}
	if secret.Data["qwen-api-key"] != nil {
		count++
	}
	return count
}

// VolumeMount returns the volume mount configuration for the Secret
// Keys are NOT exposed as environment variables to avoid accidental logging
// Instead, they are mounted as files and read by the application
func VolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      "api-keys",
		MountPath: "/etc/api-keys",
		ReadOnly:  true,
	}
}

// Volume returns the volume configuration for the Secret
func Volume(secretName string) v1.Volume {
	return v1.Volume{
		Name: "api-keys",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  secretName,
				Optional:    boolPtr(false),
				DefaultMode: int32Ptr(0400), // Read-only
			},
		},
	}
}

// VolumeWithItems returns the volume configuration with specific key-to-path mappings
func VolumeWithItems(secretName string, items []v1.KeyToPath) v1.Volume {
	return v1.Volume{
		Name: "api-keys",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  secretName,
				Optional:    boolPtr(false),
				Items:       items,
				DefaultMode: int32Ptr(0400),
			},
		},
	}
}

// KeyToPathItems returns the key-to-path mappings for the API keys
// This allows the keys to be mounted with specific filenames
func KeyToPathItems() []v1.KeyToPath {
	return []v1.KeyToPath{
		{Key: "openai-api-key", Path: "OPENAI_API_KEY", Mode: int32Ptr(0400)},
		{Key: "claude-api-key", Path: "ANTHROPIC_API_KEY", Mode: int32Ptr(0400)},
		{Key: "qwen-api-key", Path: "DASHSCOPE_API_KEY", Mode: int32Ptr(0400)},
	}
}

// IsKeyExposedInData checks if raw (non-base64) API key strings appear in the secret data
// This is a security check to ensure keys are properly encoded
func IsKeyExposedInData(data map[string][]byte) bool {
	if data == nil {
		return false
	}
	for _, value := range data {
		decoded := string(value)
		// Check for common API key patterns in decoded values
		if strings.Contains(decoded, "sk-") || // OpenAI key pattern
			strings.Contains(decoded, "anthropic") || // Claude key hint
			strings.Contains(decoded, "sk-") { // Qwen key pattern
			return true
		}
	}
	return false
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// int32Ptr returns a pointer to an int32 value
func int32Ptr(i int32) *int32 {
	return &i
}
