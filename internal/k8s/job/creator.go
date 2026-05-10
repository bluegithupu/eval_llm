package job

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/k8s/configmap"
	"github.com/eval_llm/backend/internal/k8s/secret"
)

// JobSpec holds the specification for creating a Job
type JobSpec struct {
	EvalID         string
	ModelName      string
	Dataset        string
	ContainerImage string
	Command        []string
	WorkingDir     string
}

// CreateJob creates a Kubernetes Job with the evaluation configuration
// It creates the ConfigMap and Secret volumes, mounts them, and sets resource limits
func CreateJob(ctx context.Context, client kubernetes.Interface, namespace string, spec *JobSpec) (*batchv1.Job, error) {
	if err := validateJobSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid job spec: %w", err)
	}

	jobName := JobName(spec.EvalID)
	labels := k8s.Labels(spec.EvalID, spec.ModelName, spec.Dataset)

	// Default command if not provided
	command := spec.Command
	if len(command) == 0 {
		command = []string{"python", "-c", "print('OpenCompass evaluation')"}
	}

	// Use default image if not provided
	image := spec.ContainerImage
	if image == "" {
		image = "opencompass:latest"
	}

	// Default working dir
	workingDir := spec.WorkingDir
	if workingDir == "" {
		workingDir = "/tmp/opencompass_runs"
	}

	// Build the Job spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          int32Ptr(3),
			ActiveDeadlineSeconds: int64Ptr(7200), // 2 hours
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					RestartPolicy:      v1.RestartPolicyOnFailure,
					ServiceAccountName: "default", // Use default service account
					Containers: []v1.Container{
						{
							Name:  "opencompass",
							Image: image,
							Command: []string{
								"sh", "-c",
								"cd /etc/config && python config.py",
							},
							WorkingDir: workingDir,
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    mustParseQuantity("500m"),
									v1.ResourceMemory: mustParseQuantity("512Mi"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    mustParseQuantity("500m"),
									v1.ResourceMemory: mustParseQuantity("512Mi"),
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/config",
									ReadOnly:  true,
								},
								{
									Name:      "api-keys",
									MountPath: "/etc/api-keys",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: configmap.ConfigMapName(spec.EvalID),
									},
									Optional:    boolPtr(false),
									DefaultMode: int32Ptr(0444),
								},
							},
						},
						{
							Name: "api-keys",
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName:  secret.SecretName(spec.EvalID),
									Optional:    boolPtr(false),
									DefaultMode: int32Ptr(0400),
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Job: %w", err)
	}

	return result, nil
}

// validateJobSpec validates the job specification
func validateJobSpec(spec *JobSpec) error {
	if spec == nil {
		return fmt.Errorf("job spec is nil")
	}
	if spec.EvalID == "" {
		return fmt.Errorf("evalID is required")
	}
	if spec.ModelName == "" {
		return fmt.Errorf("modelName is required")
	}
	if spec.Dataset == "" {
		return fmt.Errorf("dataset is required")
	}
	return nil
}

// JobName returns the standard name for an evaluation Job
func JobName(evalID string) string {
	return fmt.Sprintf("eval-job-%s", evalID)
}

// GetJob retrieves a Job by evaluation ID
func GetJob(ctx context.Context, client kubernetes.Interface, namespace, evalID string) (*batchv1.Job, error) {
	return client.BatchV1().Jobs(namespace).Get(ctx, JobName(evalID), metav1.GetOptions{})
}

// DeleteJob deletes a Job by evaluation ID
func DeleteJob(ctx context.Context, client kubernetes.Interface, namespace, evalID string) error {
	return client.BatchV1().Jobs(namespace).Delete(ctx, JobName(evalID), metav1.DeleteOptions{})
}

// UpdateJobStatus updates a Job with a new status (used for status tracking)
func UpdateJobStatus(ctx context.Context, client kubernetes.Interface, namespace, evalID string, status *batchv1.JobStatus) (*batchv1.Job, error) {
	job, err := GetJob(ctx, client, namespace, evalID)
	if err != nil {
		return nil, err
	}

	job.Status = *status
	return client.BatchV1().Jobs(namespace).UpdateStatus(ctx, job, metav1.UpdateOptions{})
}

// JobIsRunning checks if a Job is currently running (has active pods)
func JobIsRunning(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return job.Status.Active > 0
}

// JobIsCompleted checks if a Job has completed successfully
func JobIsCompleted(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return job.Status.Succeeded > 0
}

// JobIsFailed checks if a Job has failed (reached backoff limit)
func JobIsFailed(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	return job.Status.Failed > 0
}

// GetJobProgress calculates the progress of a Job based on its status
// Returns a value between 0 and 100
func GetJobProgress(job *batchv1.Job) int32 {
	if job == nil {
		return 0
	}

	// If completed, return 100
	if job.Status.Succeeded > 0 {
		return 100
	}

	// If we have no active or failed pods, it's pending
	if job.Status.Active == 0 && job.Status.Failed == 0 {
		return 0
	}

	// For running jobs, we estimate progress
	// In a real scenario, this would be determined by the evaluation progress
	// For now, we return 50 as a placeholder for running jobs
	if job.Status.Active > 0 {
		return 50
	}

	// If failed, return a special value (could be -1 or 0)
	return 0
}

// Helper functions for pointer types

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func mustParseQuantity(s string) resource.Quantity {
	q, err := parseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

func parseQuantity(s string) (resource.Quantity, error) {
	return resource.ParseQuantity(s)
}
