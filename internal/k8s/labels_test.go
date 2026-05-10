package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()
	assert.Equal(t, DefaultNamespace, cfg.Namespace)
	assert.Empty(t, cfg.KubeconfigPath)
	assert.Empty(t, cfg.Context)
}

func TestConfigOptions(t *testing.T) {
	cfg := &ClientConfig{}
	
	// Test WithNamespace
	WithNamespace("test-ns")(cfg)
	assert.Equal(t, "test-ns", cfg.Namespace)
	
	// Test WithKubeconfig
	WithKubeconfig("/path/to/kubeconfig")(cfg)
	assert.Equal(t, "/path/to/kubeconfig", cfg.KubeconfigPath)
	
	// Test WithContext
	WithContext("my-context")(cfg)
	assert.Equal(t, "my-context", cfg.Context)
}

func TestLabels(t *testing.T) {
	labels := Labels("eval-123", "gpt-4", "MMLU")
	
	assert.Equal(t, AppLabelValue, labels[AppLabelKey])
	assert.Equal(t, "eval-123", labels[EvalIDLabelKey])
	assert.Equal(t, "gpt-4", labels[ModelLabelKey])
	assert.Equal(t, "MMLU", labels[DatasetLabelKey])
}

func TestAppLabels(t *testing.T) {
	labels := AppLabels()
	assert.Equal(t, AppLabelValue, labels[AppLabelKey])
	assert.Len(t, labels, 1)
}

func TestEvalIDLabels(t *testing.T) {
	labels := EvalIDLabels("eval-456")
	assert.Equal(t, AppLabelValue, labels[AppLabelKey])
	assert.Equal(t, "eval-456", labels[EvalIDLabelKey])
	assert.Len(t, labels, 2)
}

func TestModelLabels(t *testing.T) {
	labels := ModelLabels("claude")
	assert.Equal(t, AppLabelValue, labels[AppLabelKey])
	assert.Equal(t, "claude", labels[ModelLabelKey])
	assert.Len(t, labels, 2)
}

func TestMergeLabels(t *testing.T) {
	map1 := map[string]string{"key1": "value1", "key2": "value2"}
	map2 := map[string]string{"key2": "overridden", "key3": "value3"}
	
	result := MergeLabels(map1, map2)
	
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, "overridden", result["key2"])
	assert.Equal(t, "value3", result["key3"])
}

func TestLabelSelector(t *testing.T) {
	labels := map[string]string{
		"app":     "llm-eval",
		"eval-id": "test-123",
	}
	
	selector := LabelSelector(labels)
	assert.Contains(t, selector, "app=llm-eval")
	assert.Contains(t, selector, "eval-id=test-123")
}

func TestAppSelector(t *testing.T) {
	selector := AppSelector()
	assert.Equal(t, "app=llm-eval", selector)
}

func TestEvalIDSelector(t *testing.T) {
	selector := EvalIDSelector("eval-789")
	assert.Contains(t, selector, "app=llm-eval")
	assert.Contains(t, selector, "eval-id=eval-789")
}

func TestModelSelector(t *testing.T) {
	selector := ModelSelector("qwen")
	assert.Contains(t, selector, "app=llm-eval")
	assert.Contains(t, selector, "model=qwen")
}

func TestHasLabel(t *testing.T) {
	labels := map[string]string{"app": "llm-eval", "eval-id": "test"}
	
	assert.True(t, HasLabel(labels, "app", "llm-eval"))
	assert.False(t, HasLabel(labels, "app", "other"))
	assert.False(t, HasLabel(labels, "nonexistent", "value"))
}

func TestHasAppLabel(t *testing.T) {
	validLabels := map[string]string{"app": "llm-eval"}
	invalidLabels := map[string]string{"app": "other"}
	
	assert.True(t, HasAppLabel(validLabels))
	assert.False(t, HasAppLabel(invalidLabels))
	assert.False(t, HasAppLabel(nil))
}

func TestHasEvalIDLabel(t *testing.T) {
	labels := map[string]string{"app": "llm-eval", "eval-id": "eval-123"}
	
	assert.True(t, HasEvalIDLabel(labels, "eval-123"))
	assert.False(t, HasEvalIDLabel(labels, "eval-456"))
}

func TestGetLabelValue(t *testing.T) {
	labels := map[string]string{"app": "llm-eval", "eval-id": "test"}
	
	value, ok := GetLabelValue(labels, "app")
	assert.True(t, ok)
	assert.Equal(t, "llm-eval", value)
	
	value, ok = GetLabelValue(labels, "nonexistent")
	assert.False(t, ok)
	assert.Empty(t, value)
}

func TestOwnerReference(t *testing.T) {
	ref := OwnerReference("batch/v1", "Job", "my-job", "uid-123")
	
	assert.Equal(t, "batch/v1", ref.APIVersion)
	assert.Equal(t, "Job", ref.Kind)
	assert.Equal(t, "my-job", ref.Name)
	assert.Equal(t, "uid-123", string(ref.UID))
	assert.NotNil(t, ref.BlockOwnerDeletion)
	assert.True(t, *ref.BlockOwnerDeletion)
	assert.NotNil(t, ref.Controller)
	assert.True(t, *ref.Controller)
}

func TestJobOwnerReference(t *testing.T) {
	ref := JobOwnerReference("eval-job-1", "uid-abc")
	
	assert.Equal(t, "batch/v1", ref.APIVersion)
	assert.Equal(t, "Job", ref.Kind)
	assert.Equal(t, "eval-job-1", ref.Name)
	assert.Equal(t, "uid-abc", string(ref.UID))
}

func TestConfigMapOwnerReference(t *testing.T) {
	ref := ConfigMapOwnerReference("my-config", "uid-def")
	
	assert.Equal(t, "v1", ref.APIVersion)
	assert.Equal(t, "ConfigMap", ref.Kind)
	assert.Equal(t, "my-config", ref.Name)
}

func TestSecretOwnerReference(t *testing.T) {
	ref := SecretOwnerReference("my-secret", "uid-ghi")
	
	assert.Equal(t, "v1", ref.APIVersion)
	assert.Equal(t, "Secret", ref.Kind)
	assert.Equal(t, "my-secret", ref.Name)
}

func TestAddOwnerReference(t *testing.T) {
	obj := &metav1.ObjectMeta{}
	owner := &metav1.OwnerReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       "my-job",
	}
	
	AddOwnerReference(obj, owner)
	
	require.Len(t, obj.OwnerReferences, 1)
	assert.Equal(t, "my-job", obj.OwnerReferences[0].Name)
}

func TestSetController(t *testing.T) {
	ref := &metav1.OwnerReference{}
	
	SetController(ref, true)
	require.NotNil(t, ref.Controller)
	assert.True(t, *ref.Controller)
	
	SetController(ref, false)
	require.NotNil(t, ref.Controller)
	assert.False(t, *ref.Controller)
}

func TestIsControlledBy(t *testing.T) {
	uid := types.UID("owner-uid")
	obj := &metav1.ObjectMeta{
		OwnerReferences: []metav1.OwnerReference{
			{
				UID: uid,
			},
		},
	}
	
	assert.True(t, IsControlledBy(obj, uid))
	assert.False(t, IsControlledBy(obj, types.UID("other-uid")))
}

func TestDefaultResourceRequirements(t *testing.T) {
	reqs := DefaultResourceRequirements()
	
	assert.Equal(t, "500m", reqs.CPURequest)
	assert.Equal(t, "1000m", reqs.CPULimit)
	assert.Equal(t, "512Mi", reqs.MemoryRequest)
	assert.Equal(t, "1Gi", reqs.MemoryLimit)
}

func TestResourceRequirementsToK8s(t *testing.T) {
	reqs := &ResourceRequirements{
		CPURequest:    "250m",
		CPULimit:      "500m",
		MemoryRequest: "256Mi",
		MemoryLimit:   "512Mi",
	}
	
	k8sReqs := reqs.ToK8sResources()
	
	// Check requests
	cpuReq := k8sReqs.Requests[v1.ResourceCPU]
	assert.Equal(t, "250m", cpuReq.String())
	
	memReq := k8sReqs.Requests[v1.ResourceMemory]
	assert.Equal(t, "256Mi", memReq.String())
	
	// Check limits
	cpuLim := k8sReqs.Limits[v1.ResourceCPU]
	assert.Equal(t, "500m", cpuLim.String())
	
	memLim := k8sReqs.Limits[v1.ResourceMemory]
	assert.Equal(t, "512Mi", memLim.String())
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "app", AppLabelKey)
	assert.Equal(t, "llm-eval", AppLabelValue)
	assert.Equal(t, "eval-id", EvalIDLabelKey)
	assert.Equal(t, "model", ModelLabelKey)
	assert.Equal(t, "dataset", DatasetLabelKey)
	assert.Equal(t, "llm-eval", DefaultNamespace)
}
