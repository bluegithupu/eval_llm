package secret

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretName(t *testing.T) {
	evalID := "550e8400-e29b-41d4-a716-446655440000"
	name := SecretName(evalID)
	assert.Equal(t, "eval-secret-550e8400-e29b-41d4-a716-446655440000", name)
}

func TestEncodeAPIKeys_AllKeys(t *testing.T) {
	keys := APIKeys{
		OpenAI: "sk-test-openai-key",
		Claude: "sk-ant-test-claude-key",
		Qwen:   "sk-qwen-test-key",
	}

	data := encodeAPIKeys(keys)

	assert.Len(t, data, 3)
	assert.NotNil(t, data["openai-api-key"])
	assert.NotNil(t, data["claude-api-key"])
	assert.NotNil(t, data["qwen-api-key"])

	// Verify base64 encoding
	decodedOpenAI, err := base64.StdEncoding.DecodeString(string(data["openai-api-key"]))
	require.NoError(t, err)
	assert.Equal(t, "sk-test-openai-key", string(decodedOpenAI))

	decodedClaude, err := base64.StdEncoding.DecodeString(string(data["claude-api-key"]))
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-test-claude-key", string(decodedClaude))

	decodedQwen, err := base64.StdEncoding.DecodeString(string(data["qwen-api-key"]))
	require.NoError(t, err)
	assert.Equal(t, "sk-qwen-test-key", string(decodedQwen))
}

func TestEncodeAPIKeys_SomeKeys(t *testing.T) {
	keys := APIKeys{
		OpenAI: "sk-openai-only",
		Claude: "",
		Qwen:   "",
	}

	data := encodeAPIKeys(keys)

	assert.Len(t, data, 1)
	assert.NotNil(t, data["openai-api-key"])
	_, ok := data["claude-api-key"]
	assert.False(t, ok)
	_, ok = data["qwen-api-key"]
	assert.False(t, ok)
}

func TestEncodeAPIKeys_OnlyClaude(t *testing.T) {
	keys := APIKeys{
		OpenAI: "",
		Claude: "sk-ant-claude-key",
		Qwen:   "",
	}

	data := encodeAPIKeys(keys)

	assert.Len(t, data, 1)
	_, ok := data["openai-api-key"]
	assert.False(t, ok)
	assert.NotNil(t, data["claude-api-key"])
}

func TestEncodeAPIKeys_OnlyQwen(t *testing.T) {
	keys := APIKeys{
		OpenAI: "",
		Claude: "",
		Qwen:   "sk-qwen-key",
	}

	data := encodeAPIKeys(keys)

	assert.Len(t, data, 1)
	_, ok := data["openai-api-key"]
	assert.False(t, ok)
	_, ok = data["claude-api-key"]
	assert.False(t, ok)
	assert.NotNil(t, data["qwen-api-key"])
}

func TestValidateSecretData_Valid(t *testing.T) {
	data := &SecretData{
		EvalID: "test-eval-123",
		Keys:   APIKeys{OpenAI: "sk-test"},
	}
	err := validateSecretData(data)
	assert.NoError(t, err)
}

func TestValidateSecretData_Nil(t *testing.T) {
	err := validateSecretData(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret data is nil")
}

func TestValidateSecretData_MissingEvalID(t *testing.T) {
	data := &SecretData{
		EvalID: "",
		Keys:   APIKeys{OpenAI: "sk-test"},
	}
	err := validateSecretData(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evalID is required")
}

func TestValidateSecretData_NoAPIKeys(t *testing.T) {
	data := &SecretData{
		EvalID: "test-eval-123",
		Keys:   APIKeys{OpenAI: "", Claude: "", Qwen: ""},
	}
	err := validateSecretData(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one API key is required")
}

func TestHasAPIKeys_WithKeys(t *testing.T) {
	secret := &v1.Secret{
		Data: map[string][]byte{
			"openai-api-key": []byte("test"),
		},
	}
	assert.True(t, HasAPIKeys(secret))
}

func TestHasAPIKeys_EmptyData(t *testing.T) {
	secret := &v1.Secret{
		Data: map[string][]byte{},
	}
	assert.False(t, HasAPIKeys(secret))
}

func TestHasAPIKeys_NilSecret(t *testing.T) {
	assert.False(t, HasAPIKeys(nil))
}

func TestHasAPIKeys_NilData(t *testing.T) {
	secret := &v1.Secret{}
	assert.False(t, HasAPIKeys(secret))
}

func TestGetKeyCount_AllKeys(t *testing.T) {
	secret := &v1.Secret{
		Data: map[string][]byte{
			"openai-api-key": []byte("test"),
			"claude-api-key": []byte("test"),
			"qwen-api-key":   []byte("test"),
		},
	}
	assert.Equal(t, 3, GetKeyCount(secret))
}

func TestGetKeyCount_TwoKeys(t *testing.T) {
	secret := &v1.Secret{
		Data: map[string][]byte{
			"openai-api-key": []byte("test"),
			"claude-api-key": []byte("test"),
		},
	}
	assert.Equal(t, 2, GetKeyCount(secret))
}

func TestGetKeyCount_NoKeys(t *testing.T) {
	secret := &v1.Secret{
		Data: map[string][]byte{},
	}
	assert.Equal(t, 0, GetKeyCount(secret))
}

func TestGetKeyCount_NilSecret(t *testing.T) {
	assert.Equal(t, 0, GetKeyCount(nil))
}

func TestVolumeMount(t *testing.T) {
	mount := VolumeMount()

	assert.Equal(t, "api-keys", mount.Name)
	assert.Equal(t, "/etc/api-keys", mount.MountPath)
	assert.True(t, mount.ReadOnly)
}

func TestVolume(t *testing.T) {
	vol := Volume("test-secret")

	assert.Equal(t, "api-keys", vol.Name)
	assert.NotNil(t, vol.Secret)
	assert.Equal(t, "test-secret", vol.Secret.SecretName)
	assert.NotNil(t, vol.Secret.Optional)
	assert.False(t, *vol.Secret.Optional)
}

func TestVolumeWithItems(t *testing.T) {
	items := KeyToPathItems()
	vol := VolumeWithItems("my-secret", items)

	assert.Equal(t, "api-keys", vol.Name)
	assert.NotNil(t, vol.Secret)
	assert.Equal(t, "my-secret", vol.Secret.SecretName)
	assert.Len(t, vol.Secret.Items, 3)
}

func TestKeyToPathItems(t *testing.T) {
	items := KeyToPathItems()

	assert.Len(t, items, 3)

	// Check OpenAI key mapping
	assert.Equal(t, "openai-api-key", items[0].Key)
	assert.Equal(t, "OPENAI_API_KEY", items[0].Path)
	assert.NotNil(t, items[0].Mode)
	assert.Equal(t, int32(0400), *items[0].Mode)

	// Check Claude key mapping
	assert.Equal(t, "claude-api-key", items[1].Key)
	assert.Equal(t, "ANTHROPIC_API_KEY", items[1].Path)

	// Check Qwen key mapping
	assert.Equal(t, "qwen-api-key", items[2].Key)
	assert.Equal(t, "DASHSCOPE_API_KEY", items[2].Path)
}

func TestIsKeyExposedInData_NotExposed(t *testing.T) {
	// Valid base64 encoded data - the actual base64 string of "sk-valid-key"
	validBase64 := []byte("c2stdmFsaWQta2V5") // base64 of "sk-valid-key"
	data := map[string][]byte{
		"openai-api-key": validBase64,
	}
	assert.False(t, IsKeyExposedInData(data))
}

func TestIsKeyExposedInData_Exposed(t *testing.T) {
	// Raw key data (simulating improperly stored keys)
	data := map[string][]byte{
		"openai-api-key": []byte("sk-raw-key-exposed"),
	}
	assert.True(t, IsKeyExposedInData(data))
}

func TestIsKeyExposedInData_NilData(t *testing.T) {
	assert.False(t, IsKeyExposedInData(nil))
}

func TestIsKeyExposedInData_AnthropicHint(t *testing.T) {
	data := map[string][]byte{
		"claude-api-key": []byte("anthropic_sk_test"),
	}
	assert.True(t, IsKeyExposedInData(data))
}

func TestCreateSecret_ValidData(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	data := &SecretData{
		EvalID:    "test-eval-123",
		ModelName: "gpt-4",
		Dataset:   "MMLU",
		Keys: APIKeys{
			OpenAI: "sk-test-key",
		},
	}

	ctx := context.Background()
	secret, err := manager.CreateSecret(ctx, "llm-eval", data, "", "")

	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "eval-secret-test-eval-123", secret.Name)
	assert.Equal(t, "llm-eval", secret.Namespace)
	assert.Equal(t, v1.SecretTypeOpaque, secret.Type)

	// Verify labels
	assert.Equal(t, "llm-eval", secret.Labels["app"])
	assert.Equal(t, "test-eval-123", secret.Labels["eval-id"])
	assert.Equal(t, "gpt-4", secret.Labels["model"])
	assert.Equal(t, "MMLU", secret.Labels["dataset"])

	// Verify data is base64 encoded
	assert.NotNil(t, secret.Data["openai-api-key"])
	decoded, _ := base64.StdEncoding.DecodeString(string(secret.Data["openai-api-key"]))
	assert.Equal(t, "sk-test-key", string(decoded))
}

func TestCreateSecret_WithOwnerReference(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	data := &SecretData{
		EvalID:    "test-eval-456",
		ModelName: "claude-3-opus",
		Dataset:   "HellaSwag",
		Keys: APIKeys{
			Claude: "sk-ant-test",
		},
	}

	ctx := context.Background()
	secret, err := manager.CreateSecret(ctx, "llm-eval", data, "eval-job-456", "12345-uid")

	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, "batch/v1", secret.OwnerReferences[0].APIVersion)
	assert.Equal(t, "Job", secret.OwnerReferences[0].Kind)
	assert.Equal(t, "eval-job-456", secret.OwnerReferences[0].Name)
	assert.Equal(t, types.UID("12345-uid"), secret.OwnerReferences[0].UID)
	assert.NotNil(t, secret.OwnerReferences[0].Controller)
	assert.True(t, *secret.OwnerReferences[0].Controller)
	assert.NotNil(t, secret.OwnerReferences[0].BlockOwnerDeletion)
	assert.True(t, *secret.OwnerReferences[0].BlockOwnerDeletion)
}

func TestCreateSecret_AllAPIKeys(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	data := &SecretData{
		EvalID:    "test-eval-all-keys",
		ModelName: "gpt-4",
		Dataset:   "MMLU",
		Keys: APIKeys{
			OpenAI: "sk-openai",
			Claude: "sk-claude",
			Qwen:   "sk-qwen",
		},
	}

	ctx := context.Background()
	secret, err := manager.CreateSecret(ctx, "llm-eval", data, "", "")

	require.NoError(t, err)
	assert.Len(t, secret.Data, 3)
	assert.NotNil(t, secret.Data["openai-api-key"])
	assert.NotNil(t, secret.Data["claude-api-key"])
	assert.NotNil(t, secret.Data["qwen-api-key"])
}

func TestCreateSecret_InvalidData(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	// Missing evalID
	data := &SecretData{
		EvalID: "",
		Keys:   APIKeys{OpenAI: "sk-test"},
	}

	ctx := context.Background()
	_, err := manager.CreateSecret(ctx, "llm-eval", data, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evalID is required")
}

func TestGetSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	ctx := context.Background()

	// First create a secret
	data := &SecretData{
		EvalID:    "get-test-eval",
		ModelName: "gpt-4",
		Dataset:   "MMLU",
		Keys:      APIKeys{OpenAI: "sk-get-test"},
	}

	_, err := manager.CreateSecret(ctx, "llm-eval", data, "", "")
	require.NoError(t, err)

	// Now retrieve it
	secret, err := manager.GetSecret(ctx, "llm-eval", "get-test-eval")

	require.NoError(t, err)
	assert.Equal(t, "eval-secret-get-test-eval", secret.Name)
}

func TestDeleteSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	manager := NewSecretManager(client)

	ctx := context.Background()

	// First create a secret
	data := &SecretData{
		EvalID:    "delete-test-eval",
		ModelName: "gpt-4",
		Dataset:   "MMLU",
		Keys:      APIKeys{OpenAI: "sk-delete-test"},
	}

	_, err := manager.CreateSecret(ctx, "llm-eval", data, "", "")
	require.NoError(t, err)

	// Now delete it
	err = manager.DeleteSecret(ctx, "llm-eval", "delete-test-eval")
	require.NoError(t, err)

	// Verify it's deleted
	_, err = manager.GetSecret(ctx, "llm-eval", "delete-test-eval")
	assert.Error(t, err)
}
