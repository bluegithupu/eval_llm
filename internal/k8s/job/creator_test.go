package job

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/eval_llm/backend/internal/k8s/configmap"
	"github.com/eval_llm/backend/internal/k8s/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobName(t *testing.T) {
	evalID := "test-eval-123"
	name := JobName(evalID)
	assert.Equal(t, "eval-job-test-eval-123", name)
}

func TestValidateJobSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *JobSpec
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: true,
			errMsg:  "job spec is nil",
		},
		{
			name: "empty evalID",
			spec: &JobSpec{
				EvalID:    "",
				ModelName: "gpt-4",
				Dataset:   "mmlu",
			},
			wantErr: true,
			errMsg:  "evalID is required",
		},
		{
			name: "empty modelName",
			spec: &JobSpec{
				EvalID:    "eval-123",
				ModelName: "",
				Dataset:   "mmlu",
			},
			wantErr: true,
			errMsg:  "modelName is required",
		},
		{
			name: "empty dataset",
			spec: &JobSpec{
				EvalID:    "eval-123",
				ModelName: "gpt-4",
				Dataset:   "",
			},
			wantErr: true,
			errMsg:  "dataset is required",
		},
		{
			name: "valid spec",
			spec: &JobSpec{
				EvalID:    "eval-123",
				ModelName: "gpt-4",
				Dataset:   "mmlu",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJobSpec(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateJob_Success(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:         "eval-123",
		ModelName:      "gpt-4",
		Dataset:        "mmlu",
		ContainerImage: "opencompass:latest",
		Command:        []string{"python", "run.py"},
		WorkingDir:     "/tmp/opencompass",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Verify Job name
	assert.Equal(t, JobName(spec.EvalID), job.Name)
	assert.Equal(t, namespace, job.Namespace)

	// Verify labels (VAL-K8S-002)
	assert.Equal(t, "llm-eval", job.Labels["app"])
	assert.Equal(t, "eval-123", job.Labels["eval-id"])
	assert.Equal(t, "gpt-4", job.Labels["model"])

	// Verify backoff limit (VAL-K8S-008)
	assert.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(3), *job.Spec.BackoffLimit)

	// Verify active deadline (VAL-K8S-008)
	assert.NotNil(t, job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(7200), *job.Spec.ActiveDeadlineSeconds)

	// Verify container image (VAL-K8S-003)
	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "opencompass:latest", container.Image)

	// Verify restart policy (VAL-K8S-007)
	assert.Equal(t, v1.RestartPolicyOnFailure, job.Spec.Template.Spec.RestartPolicy)

	// Verify resources (VAL-K8S-004)
	cpuRequest := container.Resources.Requests[v1.ResourceCPU]
	assert.Equal(t, "500m", cpuRequest.String())
	memRequest := container.Resources.Requests[v1.ResourceMemory]
	assert.Equal(t, "512Mi", memRequest.String())
	cpuLimit := container.Resources.Limits[v1.ResourceCPU]
	assert.Equal(t, "500m", cpuLimit.String())
	memLimit := container.Resources.Limits[v1.ResourceMemory]
	assert.Equal(t, "512Mi", memLimit.String())

	// Verify volumes exist (VAL-K8S-005, VAL-K8S-006)
	foundConfigVolume := false
	foundSecretVolume := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "config" && vol.ConfigMap != nil {
			foundConfigVolume = true
			assert.Equal(t, configmap.ConfigMapName(spec.EvalID), vol.ConfigMap.Name)
		}
		if vol.Name == "api-keys" && vol.Secret != nil {
			foundSecretVolume = true
			assert.Equal(t, secret.SecretName(spec.EvalID), vol.Secret.SecretName)
		}
	}
	assert.True(t, foundConfigVolume, "ConfigMap volume should be present")
	assert.True(t, foundSecretVolume, "Secret volume should be present")

	// Verify volume mounts
	foundConfigMount := false
	foundSecretMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == "config" && mount.MountPath == "/etc/config" {
			foundConfigMount = true
			assert.True(t, mount.ReadOnly)
		}
		if mount.Name == "api-keys" && mount.MountPath == "/etc/api-keys" {
			foundSecretMount = true
			assert.True(t, mount.ReadOnly)
		}
	}
	assert.True(t, foundConfigMount, "ConfigMap volume mount should be present")
	assert.True(t, foundSecretMount, "Secret volume mount should be present")
}

func TestCreateJob_DefaultValues(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	// Minimal spec without optional fields
	spec := &JobSpec{
		EvalID:    "eval-456",
		ModelName: "claude",
		Dataset:   "hellaswag",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Verify default image
	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "opencompass:latest", container.Image)

	// Verify default working dir
	assert.Equal(t, "/tmp/opencompass_runs", container.WorkingDir)

	// Verify default command runs the config.py from ConfigMap
	assert.Equal(t, []string{"sh", "-c", "cd /etc/config && python config.py"}, container.Command)
}

func TestGetJob(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"
	evalID := "eval-789"

	// First create a job
	spec := &JobSpec{
		EvalID:    evalID,
		ModelName: "qwen",
		Dataset:   "mmlu",
	}

	_, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	// Then retrieve it
	job, err := GetJob(ctx, client, namespace, evalID)
	require.NoError(t, err)
	assert.Equal(t, JobName(evalID), job.Name)
}

func TestGetJob_NotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	_, err := GetJob(ctx, client, namespace, "non-existent")
	assert.Error(t, err)
}

func TestDeleteJob(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"
	evalID := "eval-delete"

	// First create a job
	spec := &JobSpec{
		EvalID:    evalID,
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	_, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	// Delete the job
	err = DeleteJob(ctx, client, namespace, evalID)
	require.NoError(t, err)

	// Verify it's gone
	_, err = GetJob(ctx, client, namespace, evalID)
	assert.Error(t, err)
}

func TestJobIsRunning(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected bool
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: false,
		},
		{
			name: "job with active pods",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: true,
		},
		{
			name: "job with no active pods",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 0,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JobIsRunning(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJobIsCompleted(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected bool
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: false,
		},
		{
			name: "succeeded job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expected: true,
		},
		{
			name: "job not completed",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JobIsCompleted(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJobIsFailed(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected bool
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: false,
		},
		{
			name: "failed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Failed: 1,
				},
			},
			expected: true,
		},
		{
			name: "job not failed",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JobIsFailed(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetJobProgress(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected int32
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: 0,
		},
		{
			name: "completed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expected: 100,
		},
		{
			name: "running job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: 50,
		},
		{
			name: "pending job (no active, no failed)",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active:  0,
					Failed:  0,
					Succeeded: 0,
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetJobProgress(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateJob_Validation(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	tests := []struct {
		name    string
		spec    *JobSpec
		wantErr bool
	}{
		{
			name: "missing evalID",
			spec: &JobSpec{
				EvalID:    "",
				ModelName: "gpt-4",
				Dataset:   "mmlu",
			},
			wantErr: true,
		},
		{
			name: "missing modelName",
			spec: &JobSpec{
				EvalID:    "eval-123",
				ModelName: "",
				Dataset:   "mmlu",
			},
			wantErr: true,
		},
		{
			name: "missing dataset",
			spec: &JobSpec{
				EvalID:    "eval-123",
				ModelName: "gpt-4",
				Dataset:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateJob(ctx, client, namespace, tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateJob_MultipleJobs verifies that multiple jobs can be created with unique names
func TestCreateJob_MultipleJobs(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	evalIDs := []string{"eval-a", "eval-b", "eval-c"}

	for _, evalID := range evalIDs {
		spec := &JobSpec{
			EvalID:    evalID,
			ModelName: "gpt-4",
			Dataset:   "mmlu",
		}

		job, err := CreateJob(ctx, client, namespace, spec)
		require.NoError(t, err)
		assert.Equal(t, JobName(evalID), job.Name)

		// Verify labels are unique per job
		assert.Equal(t, evalID, job.Labels["eval-id"])
	}

	// Verify we can list all jobs
	jobList, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, jobList.Items, 3)
}

// TestCreateJob_NamespaceIsolation verifies jobs are created in correct namespace (VAL-K8S-001)
func TestCreateJob_NamespaceIsolation(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-ns-test",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)
	assert.Equal(t, namespace, job.Namespace)
	assert.Equal(t, "llm-eval", job.Namespace)
}

// TestCreateJob_ResourceLimits verifies resource limits match spec (VAL-K8S-004)
func TestCreateJob_ResourceLimits(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-resources",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	resources := container.Resources

	// Verify CPU request and limit
	cpuReq, ok := resources.Requests[v1.ResourceCPU]
	assert.True(t, ok)
	assert.Equal(t, resource.MustParse("500m"), cpuReq)

	cpuLim, ok := resources.Limits[v1.ResourceCPU]
	assert.True(t, ok)
	assert.Equal(t, resource.MustParse("500m"), cpuLim)

	// Verify memory request and limit
	memReq, ok := resources.Requests[v1.ResourceMemory]
	assert.True(t, ok)
	assert.Equal(t, resource.MustParse("512Mi"), memReq)

	memLim, ok := resources.Limits[v1.ResourceMemory]
	assert.True(t, ok)
	assert.Equal(t, resource.MustParse("512Mi"), memLim)
}

// TestCreateJob_Volumes verifies volumes are correctly configured (VAL-K8S-005, VAL-K8S-006)
func TestCreateJob_Volumes(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-volumes",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	volumes := job.Spec.Template.Spec.Volumes
	assert.Len(t, volumes, 2)

	// Find config volume
	var configVol *v1.Volume
	var secretVol *v1.Volume
	for _, v := range volumes {
		if v.Name == "config" {
			configVol = &v
		}
		if v.Name == "api-keys" {
			secretVol = &v
		}
	}

	require.NotNil(t, configVol, "config volume should exist")
	require.NotNil(t, secretVol, "secret volume should exist")

	// Verify ConfigMap volume
	assert.NotNil(t, configVol.ConfigMap)
	assert.Equal(t, "eval-config-eval-volumes", configVol.ConfigMap.Name)
	assert.NotNil(t, configVol.ConfigMap.DefaultMode)
	assert.Equal(t, int32(0444), *configVol.ConfigMap.DefaultMode)

	// Verify Secret volume
	assert.NotNil(t, secretVol.Secret)
	assert.Equal(t, "eval-secret-eval-volumes", secretVol.Secret.SecretName)
	assert.NotNil(t, secretVol.Secret.DefaultMode)
	assert.Equal(t, int32(0400), *secretVol.Secret.DefaultMode)
}

// TestCreateJob_RestartPolicy verifies restart policy is OnFailure (VAL-K8S-007)
func TestCreateJob_RestartPolicy(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-restart",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	// Verify restart policy is OnFailure
	assert.Equal(t, v1.RestartPolicyOnFailure, job.Spec.Template.Spec.RestartPolicy)
}

// TestCreateJob_BackoffLimit verifies backoff limit is 3 (VAL-K8S-008)
func TestCreateJob_BackoffLimit(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-backoff",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	// Verify backoff limit is 3
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(3), *job.Spec.BackoffLimit)
}

// TestCreateJob_ActiveDeadline verifies active deadline is 7200s (VAL-K8S-008)
func TestCreateJob_ActiveDeadline(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	spec := &JobSpec{
		EvalID:    "eval-deadline",
		ModelName: "gpt-4",
		Dataset:   "mmlu",
	}

	job, err := CreateJob(ctx, client, namespace, spec)
	require.NoError(t, err)

	// Verify active deadline is 7200 (2 hours)
	require.NotNil(t, job.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(7200), *job.Spec.ActiveDeadlineSeconds)
}

// TestCreateJob_Labels verifies job labels match expected behavior (VAL-K8S-002)
func TestCreateJob_Labels(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	namespace := "llm-eval"

	testCases := []struct {
		evalID    string
		modelName string
		dataset   string
	}{
		{"eval-001", "gpt-4", "mmlu"},
		{"eval-002", "claude", "hellaswag"},
		{"eval-003", "qwen", "mmlu"},
	}

	for _, tc := range testCases {
		spec := &JobSpec{
			EvalID:    tc.evalID,
			ModelName: tc.modelName,
			Dataset:   tc.dataset,
		}

		job, err := CreateJob(ctx, client, namespace, spec)
		require.NoError(t, err)

		// Verify all required labels
		assert.Equal(t, "llm-eval", job.Labels["app"])
		assert.Equal(t, tc.evalID, job.Labels["eval-id"])
		assert.Equal(t, tc.modelName, job.Labels["model"])
		assert.Equal(t, tc.dataset, job.Labels["dataset"])

		// Verify pod template also has same labels
		assert.Equal(t, "llm-eval", job.Spec.Template.Labels["app"])
		assert.Equal(t, tc.evalID, job.Spec.Template.Labels["eval-id"])
		assert.Equal(t, tc.modelName, job.Spec.Template.Labels["model"])
	}
}

// TestParseQuantity tests the quantity parsing helper
func TestParseQuantity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"500m", "500m", false},
		{"512Mi", "512Mi", false},
		{"1Gi", "1Gi", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			q, err := parseQuantity(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, q.String())
			}
		})
	}
}

// TestIntPtrHelpers tests the pointer helper functions
func TestIntPtrHelpers(t *testing.T) {
	i := int32Ptr(42)
	assert.Equal(t, int32(42), *i)

	l := int64Ptr(7200)
	assert.Equal(t, int64(7200), *l)

	b := boolPtr(true)
	assert.True(t, *b)
}
