package monitor

import (
	v1 "k8s.io/api/core/v1"
)

// OOMKilledExitCode is the exit code for OOM killed processes
const OOMKilledExitCode int32 = 137

// IsOOMKilled checks if a pod was terminated due to OOM (Out of Memory)
func IsOOMKilled(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}

	// Check pod status for OOMKilled reason
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			// Exit code 137 = 128 + 9 = SIGKILL (typically OOM)
			if containerStatus.LastTerminationState.Terminated.ExitCode == OOMKilledExitCode {
				return true
			}
			// Check for OOMKilled in reason
			if containerStatus.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return true
			}
		}
	}

	// Check init containers too
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			if containerStatus.LastTerminationState.Terminated.ExitCode == OOMKilledExitCode {
				return true
			}
			if containerStatus.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return true
			}
		}
	}

	return false
}

// GetExitCode returns the exit code of a pod's main container
func GetExitCode(pod *v1.Pod) int32 {
	if pod == nil {
		return 0
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			return containerStatus.LastTerminationState.Terminated.ExitCode
		}
	}

	return 0
}

// GetTerminationMessage returns the termination message of a pod's main container
func GetTerminationMessage(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			return containerStatus.LastTerminationState.Terminated.Message
		}
	}

	return ""
}

// GetTerminationReason returns the termination reason of a pod's main container
func GetTerminationReason(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			return containerStatus.LastTerminationState.Terminated.Reason
		}
	}

	return ""
}

// PodIsRunning checks if all containers in a pod are in Running state
func PodIsRunning(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}

	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running == nil {
			return false
		}
	}

	return true
}

// PodIsTerminated checks if a pod has reached a terminal state
func PodIsTerminated(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}

	switch pod.Status.Phase {
	case v1.PodSucceeded, v1.PodFailed:
		return true
	}

	return false
}

// GetPodConditions returns the pod's conditions
func GetPodConditions(pod *v1.Pod) []v1.PodCondition {
	if pod == nil {
		return nil
	}
	return pod.Status.Conditions
}

// HasPodReadyCondition checks if pod has Ready condition
func HasPodReadyCondition(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}

	return false
}
