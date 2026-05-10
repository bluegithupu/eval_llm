# Kubernetes Job Integration Assertions

This document enumerates behavioral assertions for the LLM evaluation backend's Kubernetes Job integration.

---

## Job Creation

### VAL-K8S-001: Job Created with Correct Namespace
Job is created in the designated evaluation namespace and not in default namespace.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.metadata.namespace}'`

### VAL-K8S-002: Job Contains Required Labels
Job has required labels for identification: `app=llm-eval`, `eval-id=<eval-id>`, `model=<model-name>`.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.metadata.labels}'`

### VAL-K8S-003: Job Uses Correct Container Image
Job spec references the configured evaluation container image with correct tag.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].image}'`

### VAL-K8S-004: Job Has Resource Limits Defined
Job container has CPU and memory requests/limits specified.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].resources}'`

### VAL-K8S-005: Job Mounts ConfigMap Volume
Job spec includes volume mount for ConfigMap containing evaluation configuration.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.volumes}'`

### VAL-K8S-006: Job Mounts Secret Volume
Job spec includes volume mount for Secret containing API keys.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].volumeMounts}'`

### VAL-K8S-007: Job Sets Correct RestartPolicy
Job template has `RestartPolicy: OnFailure` or `Never` (not `Always`).
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.restartPolicy}'`

### VAL-K8S-008: Job Sets Backoff Limit
Job has backoff limit configured to control retry attempts on failure.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.backoffLimit}'`

### VAL-K8S-009: Job Has Active Deadline Seconds
Job has timeout configured via activeDeadlineSeconds to prevent hung jobs.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.activeDeadlineSeconds}'`

### VAL-K8S-010: Job Environment Variables Include Eval Context
Job container has environment variables for evaluation ID, model name, and callback URL.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].env}'`

---

## ConfigMap Generation

### VAL-K8S-011: ConfigMap Created with Valid MMEngine Format
ConfigMap data contains valid MMEngine configuration format (Python config file).
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.data}'`

### VAL-K8S-012: ConfigMap Contains Model Configuration
ConfigMap includes model configuration with correct model type, path, and API settings.
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.data.config\.py}'`

### VAL-K8S-013: ConfigMap Contains Dataset Configuration
ConfigMap includes dataset configuration specifying evaluation datasets and prompts.
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.data}' | grep -i dataset`

### VAL-K8S-014: ConfigMap Has Matching Labels
ConfigMap has same labels as the Job for resource association.
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.metadata.labels}'`

### VAL-K8S-015: ConfigMap Mounted at Correct Path
ConfigMap is mounted at the expected path inside container (e.g., `/config/`).
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].volumeMounts}' && kubectl exec <pod-name> -- ls /config/`

---

## Secret Management

### VAL-K8S-016: Secret Created for API Keys
Secret is created containing API keys for OpenAI, Claude, and Qwen.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.data}'`

### VAL-K8S-017: Secret Keys Base64 Encoded
Secret data values are base64 encoded as per Kubernetes requirements.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.data.openai-api-key}' | base64 -d`

### VAL-K8S-018: Secret Mounted as Volume
Secret is mounted as volume, not exposed as plaintext environment variables.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.volumes[?(@.secret)]}'`

### VAL-K8S-019: Secret Not Exposed in Pod Logs
API keys are not visible in pod logs or stdout.
Tool: kubectl
Evidence: `kubectl logs <pod-name> -n <namespace> | grep -i "api-key\|sk-\|key=" || echo "No key exposure found"`

### VAL-K8S-020: Secret Has Owner Reference for Cleanup
Secret has owner reference to Job for automatic cleanup on Job deletion.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.metadata.ownerReferences}'`

---

## Status Monitoring

### VAL-K8S-021: Job Status Correctly Reflects Running State
When Job Pod is running, API returns status "running" with progress information.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.active}'`

### VAL-K8S-022: Job Status Correctly Reflects Completed State
When Job completes successfully, API returns status "completed" and `.status.succeeded` is set.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.succeeded}'`

### VAL-K8S-023: Job Status Correctly Reflects Failed State
When Job fails, API returns status "failed" and `.status.failed` is set.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.failed}'`

### VAL-K8S-024: Job Events Captured for Monitoring
Job events (Started, Completed, Failed) are captured and stored.
Tool: kubectl
Evidence: `kubectl get events --field-selector involvedObject.name=<job-name> -n <namespace>`

### VAL-K8S-025: Pod Status Synced to API
Pod phase (Pending, Running, Succeeded, Failed) is reflected in API status endpoint.
Tool: kubectl
Evidence: `kubectl get pods -l job-name=<job-name> -n <namespace> -o jsonpath='{.items[0].status.phase}'`

---

## Result Collection

### VAL-K8S-026: Results Retrieved from Shared Volume
After Job completion, results are successfully copied from shared volume.
Tool: kubectl
Evidence: `kubectl exec <pod-name> -- ls /results/` or check shared volume contents

### VAL-K8S-027: Results Contain Evaluation Metrics
Collected results include evaluation metrics (accuracy, F1, etc.).
Tool: kubectl
Evidence: `kubectl exec <pod-name> -- cat /results/metrics.json` or check stored results file

### VAL-K8S-028: Results Contain Model Predictions
Collected results include model predictions for each evaluation sample.
Tool: kubectl
Evidence: `kubectl exec <pod-name> -- cat /results/predictions.json` or check stored results file

### VAL-K8S-029: Results Stored in Persistent Storage
Results are persisted to database or object storage after collection.
Tool: kubectl/API
Evidence: Query API for results or check object storage bucket for result files

### VAL-K8S-030: Callback URL Notified on Completion
Job sends completion callback to configured URL with results.
Tool: kubectl
Evidence: `kubectl logs <pod-name> -n <namespace> | grep -i callback` or API logs

---

## Cleanup

### VAL-K8S-031: Job Deleted After Completion
Completed Job is deleted after results are collected.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace>` should return "not found" after cleanup

### VAL-K8S-032: ConfigMap Deleted After Job Completion
ConfigMap associated with Job is deleted after Job completion.
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace>` should return "not found" after cleanup

### VAL-K8S-033: Secret Deleted After Job Completion
Secret containing API keys is deleted after Job completion.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace>` should return "not found" after cleanup

### VAL-K8S-034: Pods Deleted After Job Completion
Pods created by Job are deleted after Job completion.
Tool: kubectl
Evidence: `kubectl get pods -l job-name=<job-name> -n <namespace>` should return empty list after cleanup

### VAL-K8S-035: Cleanup Respects TTL Seconds After Finished
Job cleanup respects TTL configuration for completed jobs.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.ttlSecondsAfterFinished}'`

---

## Error Handling

### VAL-K8S-036: Job Failure Triggers Retry Logic
Failed Job triggers retry with exponential backoff up to max retries.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.failed}'` and retry job creation logs

### VAL-K8S-037: Job Failure Captures Error Message
Job failure error message is captured and stored for debugging.
Tool: kubectl
Evidence: `kubectl logs <pod-name> -n <namespace> --previous` for failed container logs

### VAL-K8S-038: Job Failure Updates API Status
API status reflects failure with error details when Job fails.
Tool: kubectl/API
Evidence: Query API status endpoint for failed job

### VAL-K8S-039: OOM Killed Pods Detected and Reported
Out-of-memory killed pods are detected and reported as resource exhaustion errors.
Tool: kubectl
Evidence: `kubectl describe pod <pod-name> -n <namespace> | grep -i OOMKilled`

### VAL-K8S-040: Image Pull Errors Handled Gracefully
Image pull errors are detected and reported with actionable error message.
Tool: kubectl
Evidence: `kubectl describe pod <pod-name> -n <namespace> | grep -i "ImagePullBackOff\|ErrImagePull"`

### VAL-K8S-041: Timeout Creates Failed Status
Job exceeding activeDeadlineSeconds is marked as failed in API.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.conditions[?(@.type=="Failed")].reason}'`

### VAL-K8S-042: Cleanup on Permanent Failure
Permanently failed jobs (exceeding backoff limit) are cleaned up with resources deleted.
Tool: kubectl
Evidence: Check for Job, ConfigMap, Secret, Pod absence after permanent failure

---

## Security Considerations

### VAL-K8S-043: Secret Immutable Flag Set
Secret has immutable flag set to prevent accidental modification.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.immutable}'`

### VAL-K8S-044: Job Runs as Non-Root User
Job container runs as non-root user for security.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].securityContext.runAsNonRoot}'`

### VAL-K8S-045: Service Account Configured
Job uses dedicated service account with minimal permissions.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.serviceAccountName}'`

### VAL-K8S-046: Network Policy Restricts Egress
Job namespace has network policy restricting egress to only required endpoints.
Tool: kubectl
Evidence: `kubectl get networkpolicy -n <namespace>`

---

## Integration Verification

### VAL-K8S-047: End-to-End Job Execution Completes
A complete evaluation job executes from creation to cleanup without manual intervention.
Tool: kubectl
Evidence: Full job lifecycle logs and final result storage verification

### VAL-K8S-048: Concurrent Jobs Supported
Multiple evaluation jobs can run concurrently without resource conflicts.
Tool: kubectl
Evidence: `kubectl get jobs -n <namespace> -l app=llm-eval` showing multiple active jobs

### VAL-K8S-049: Job Status Polling Works
Status polling endpoint returns correct state for each Job phase.
Tool: kubectl/API
Evidence: Poll status API during job execution and verify matches kubectl job status

### VAL-K8S-050: Cleanup on Job Deletion Cascades
Deleting Job cascades to delete ConfigMap and Secret via owner references.
Tool: kubectl
Evidence: `kubectl delete job <job-name> -n <namespace>` then verify ConfigMap and Secret are deleted
