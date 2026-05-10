package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

// Labels returns a complete label map for a job
func Labels(evalID, modelName, datasetName string) map[string]string {
	labels := map[string]string{
		AppLabelKey:     AppLabelValue,
		EvalIDLabelKey:  evalID,
		ModelLabelKey:   modelName,
		DatasetLabelKey: datasetName,
	}
	return labels
}

// AppLabels returns labels with only app key
func AppLabels() map[string]string {
	return map[string]string{
		AppLabelKey: AppLabelValue,
	}
}

// EvalIDLabels returns labels for a specific evaluation ID
func EvalIDLabels(evalID string) map[string]string {
	return map[string]string{
		AppLabelKey:    AppLabelValue,
		EvalIDLabelKey: evalID,
	}
}

// ModelLabels returns labels for a specific model
func ModelLabels(modelName string) map[string]string {
	return map[string]string{
		AppLabelKey:  AppLabelValue,
		ModelLabelKey: modelName,
	}
}

// MergeLabels merges multiple label maps into one
// Later maps override earlier ones
func MergeLabels(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// LabelSelector returns a label selector string for the given labels
func LabelSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	selectorParts := make([]string, 0, len(labels))
	for key, value := range labels {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", key, value))
	}
	return joinSelectors(selectorParts)
}

// AppSelector returns a selector for app=llm-eval
func AppSelector() string {
	return AppLabelKey + "=" + AppLabelValue
}

// EvalIDSelector returns a selector for a specific evaluation ID
func EvalIDSelector(evalID string) string {
	return fmt.Sprintf("%s=%s,%s=%s", AppLabelKey, AppLabelValue, EvalIDLabelKey, evalID)
}

// ModelSelector returns a selector for a specific model
func ModelSelector(modelName string) string {
	return fmt.Sprintf("%s=%s,%s=%s", AppLabelKey, AppLabelValue, ModelLabelKey, modelName)
}

// StatusSelector returns a selector for evaluation status
// Note: status is not a label on the job, this is for reference
func StatusSelector(status string) string {
	return fmt.Sprintf("%s=%s", "status", status)
}

// joinSelectors joins multiple selector parts with comma
func joinSelectors(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ","
		}
		result += part
	}
	return result
}

// HasLabel checks if a label map contains a specific key-value pair
func HasLabel(labels map[string]string, key, value string) bool {
	v, ok := labels[key]
	return ok && v == value
}

// HasAppLabel checks if labels contain app=llm-eval
func HasAppLabel(labels map[string]string) bool {
	return HasLabel(labels, AppLabelKey, AppLabelValue)
}

// HasEvalIDLabel checks if labels contain eval-id=<evalID>
func HasEvalIDLabel(labels map[string]string, evalID string) bool {
	return HasLabel(labels, EvalIDLabelKey, evalID)
}

// GetLabelValue retrieves a label value from a label map
func GetLabelValue(labels map[string]string, key string) (string, bool) {
	v, ok := labels[key]
	return v, ok
}

// OwnerReference creates an owner reference for a resource
func OwnerReference(apiVersion, kind, name, uid string) *metav1.OwnerReference {
	blockOwnerDeletion := true
	controller := true
	return &metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               name,
		UID:                types.UID(uid),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &controller,
	}
}

// JobOwnerReference creates an owner reference for a Job resource
func JobOwnerReference(jobName, jobUID string) *metav1.OwnerReference {
	return OwnerReference("batch/v1", "Job", jobName, jobUID)
}

// ConfigMapOwnerReference creates an owner reference for a ConfigMap
func ConfigMapOwnerReference(cmName, cmUID string) *metav1.OwnerReference {
	return OwnerReference("v1", "ConfigMap", cmName, cmUID)
}

// SecretOwnerReference creates an owner reference for a Secret
func SecretOwnerReference(secretName, secretUID string) *metav1.OwnerReference {
	return OwnerReference("v1", "Secret", secretName, secretUID)
}

// AddOwnerReference adds an owner reference to an object
func AddOwnerReference(obj metav1.Object, owner *metav1.OwnerReference) {
	refs := obj.GetOwnerReferences()
	refs = append(refs, *owner)
	obj.SetOwnerReferences(refs)
}

// SetController sets the controller flag on an owner reference
func SetController(ref *metav1.OwnerReference, controller bool) {
	ref.Controller = &controller
}

// IsControlledBy checks if an object is controlled by a given UID
func IsControlledBy(obj metav1.Object, uid types.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == uid {
			return true
		}
	}
	return false
}

// ResourceRequirements defines CPU and memory resource limits
type ResourceRequirements struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// DefaultResourceRequirements returns default resource requirements for evaluation jobs
func DefaultResourceRequirements() *ResourceRequirements {
	return &ResourceRequirements{
		CPURequest:    "500m",
		CPULimit:      "1000m",
		MemoryRequest: "512Mi",
		MemoryLimit:   "1Gi",
	}
}

// ToK8sResources converts ResourceRequirements to k8s ResourceRequirements
func (r *ResourceRequirements) ToK8sResources() v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    mustParseQuantity(r.CPURequest),
			v1.ResourceMemory: mustParseQuantity(r.MemoryRequest),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    mustParseQuantity(r.CPULimit),
			v1.ResourceMemory: mustParseQuantity(r.MemoryLimit),
		},
	}
}

// mustParseQuantity parses a quantity string, panics on error
func mustParseQuantity(s string) resource.Quantity {
	q, err := parseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

// parseQuantity parses a quantity string
func parseQuantity(s string) (resource.Quantity, error) {
	return resource.ParseQuantity(s)
}
