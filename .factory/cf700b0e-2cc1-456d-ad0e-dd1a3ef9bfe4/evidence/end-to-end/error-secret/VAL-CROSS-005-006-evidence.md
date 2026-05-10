# Evidence for VAL-CROSS-005 and VAL-CROSS-006 Testing

## VAL-CROSS-005: OpenCompass Failure Propagation

### DB Query: Check for Failed Evaluations
```sql
SELECT id, status, error_message FROM evaluations WHERE status = 'failed';
-- Result: 0 rows
```

### DB Schema: Error Message Column
```sql
\d evaluations
-- Column: error_message (text)
```

### K8s Check: No Jobs Found
```
kubectl get jobs -n llm-eval
-- Result: No resources found in llm-eval namespace
```

## VAL-CROSS-006: API Key Security

### DB: Models Table (API Key References)
```sql
SELECT id, name, api_key_ref FROM models;
-- Results:
-- openai-api-key, anthropic-api-key, dashscope-api-key
-- Only references, no actual keys
```

### DB: Evaluations Config
```sql
SELECT config FROM evaluations LIMIT 5;
-- Result: {} for all - no API keys stored
```

### API Response: Evaluation Details
```json
{
  "id": "8bb55603-42f1-4056-b073-6f805fcd5114",
  "model": "gpt-4",
  "dataset": "mmlu",
  "status": "completed",
  "config": {},
  "progress": 0
}
-- No API keys exposed
```

### API Response: Results Endpoint
```json
{
  "results": [...],
  "predictions": {...}
}
-- No API keys exposed in results or predictions
```

### K8s Secrets
```
kubectl get secret -n llm-eval
-- Result: No secrets found (only kube-root-ca.crt ConfigMap)
```

### K8s Pods
```
kubectl get pods -n llm-eval
-- Result: No pods running - no active jobs to verify
```

### Postgres Logs Check
```bash
docker logs eval-postgres | grep -i "key\|sk-\|api-key\|password"
-- Result: Only FK constraint ERROR messages, no key exposure
```
