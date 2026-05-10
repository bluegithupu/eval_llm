package k8s

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Kubernetes client will be implemented in future features
// This placeholder imports client-go to ensure dependency is tracked

// JobManager interface for managing Kubernetes Jobs
type JobManager interface {
	CreateJob(ctx context.Context, evalID string, config map[string]any) error
	GetJobStatus(ctx context.Context, jobName string) (string, error)
	DeleteJob(ctx context.Context, jobName string) error
}

// K8sClient placeholder implementation
type K8sClient struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// Ensure k8s types are referenced for go mod
var _ kubernetes.Interface
