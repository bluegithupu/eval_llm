package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestIsOOMKilled(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "pod not OOM killed",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 0,
									Reason:   "Completed",
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod OOM killed with exit code 137",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 137,
									Reason:   "OOMKilled",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod OOM killed with reason OOMKilled",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 1,
									Reason:   "OOMKilled",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with init container OOM killed",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					InitContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 137,
									Reason:   "OOMKilled",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod running",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOOMKilled(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected int32
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: 0,
		},
		{
			name: "pod with exit code 137",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 137,
								},
							},
						},
					},
				},
			},
			expected: 137,
		},
		{
			name: "pod with exit code 0",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 0,
								},
							},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "pod with no terminated state",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExitCode(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTerminationMessage(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected string
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: "",
		},
		{
			name: "pod with termination message",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									Message: "OOMKilled: container exceeded memory limit",
								},
							},
						},
					},
				},
			},
			expected: "OOMKilled: container exceeded memory limit",
		},
		{
			name: "pod with no message",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									ExitCode: 0,
								},
							},
						},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTerminationMessage(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTerminationReason(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected string
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: "",
		},
		{
			name: "pod with OOMKilled reason",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
					},
				},
			},
			expected: "OOMKilled",
		},
		{
			name: "pod with Completed reason",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{
									Reason: "Completed",
								},
							},
						},
					},
				},
			},
			expected: "Completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTerminationReason(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPodIsRunning(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "pod running with all containers running",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod not running",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			expected: false,
		},
		{
			name: "pod running but container not running",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					ContainerStatuses: []v1.ContainerStatus{
						{
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PodIsRunning(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPodIsTerminated(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "pod succeeded",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
				},
			},
			expected: true,
		},
		{
			name: "pod failed",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
				},
			},
			expected: true,
		},
		{
			name: "pod running",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
			expected: false,
		},
		{
			name: "pod pending",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PodIsTerminated(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasPodReadyCondition(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "pod with Ready condition true",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with Ready condition false",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with no conditions",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPodReadyCondition(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}
