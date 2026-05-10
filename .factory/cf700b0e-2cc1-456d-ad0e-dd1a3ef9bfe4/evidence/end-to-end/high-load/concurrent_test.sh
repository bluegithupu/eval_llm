#!/bin/bash
# Concurrent evaluation test script

API_URL="http://localhost:3100/api/v1/evaluations"
EVIDENCE_DIR="/Users/mac/.factory/missions/cf700b0e-2cc1-456d-ad0e-dd1a3ef9bfe4/evidence/end-to-end/high-load"
RESULTS_FILE="${EVIDENCE_DIR}/concurrent_test_results.txt"

# Payload for creating evaluation
PAYLOAD='{"model":"gpt-4","dataset":"mmlu"}'

echo "=== High Concurrency Load Test (VAL-CROSS-010) ===" | tee "$RESULTS_FILE"
echo "Start time: $(date -Iseconds)" | tee -a "$RESULTS_FILE"

# Record start time
START_TIME=$(date +%s.%N)

# Counter for successful/failed responses
SUCCESS_COUNT=0
FAIL_COUNT=0
RESPONSE_CODES=""

# Run 20 concurrent curl requests
for i in {1..20}; do
  (
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL" \
      -H "Content-Type: application/json" \
      -d "$PAYLOAD" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | sed '$d')
    
    echo "Request $i: HTTP $HTTP_CODE, Body: $BODY" >> "${EVIDENCE_DIR}/request_$i.txt"
  ) &
done

# Wait for all background jobs to complete
wait

# Calculate elapsed time
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "$END_TIME - $START_TIME" | bc)

echo "" | tee -a "$RESULTS_FILE"
echo "All requests completed." | tee -a "$RESULTS_FILE"
echo "End time: $(date -Iseconds)" | tee -a "$RESULTS_FILE"
echo "Elapsed time: ${ELAPSED}s" | tee -a "$RESULTS_FILE"

# Now collect results from individual files
for i in {1..20}; do
  if [ -f "${EVIDENCE_DIR}/request_$i.txt" ]; then
    CODE=$(grep -oP '\d+$' "${EVIDENCE_DIR}/request_$i.txt" | tail -1)
    if [ "$CODE" = "202" ]; then
      SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    else
      FAIL_COUNT=$((FAIL_COUNT + 1))
      RESPONSE_CODES="${RESPONSE_CODES} $CODE"
    fi
  fi
done

echo "" | tee -a "$RESULTS_FILE"
echo "=== Results Summary ===" | tee -a "$RESULTS_FILE"
echo "Total requests: 20" | tee -a "$RESULTS_FILE"
echo "Successful (202): $SUCCESS_COUNT" | tee -a "$RESULTS_FILE"
echo "Failed: $FAIL_COUNT" | tee -a "$RESULTS_FILE"
echo "Failure codes:${RESPONSE_CODES:- (none)}" | tee -a "$RESULTS_FILE"

# Check if all 20 returned 202
if [ $SUCCESS_COUNT -eq 20 ]; then
  echo "STATUS: PASS - All 20 requests returned 202" | tee -a "$RESULTS_FILE"
else
  echo "STATUS: FAIL - Only $SUCCESS_COUNT/20 returned 202" | tee -a "$RESULTS_FILE"
fi

echo "" | tee -a "$RESULTS_FILE"
echo "=== Checking K8s Jobs ===" | tee -a "$RESULTS_FILE"
kubectl get jobs -n llm-eval 2>&1 | tee -a "$RESULTS_FILE"

JOB_COUNT=$(kubectl get jobs -n llm-eval --no-headers 2>/dev/null | wc -l)
echo "" | tee -a "$RESULTS_FILE"
echo "Job count: $JOB_COUNT" | tee -a "$RESULTS_FILE"

echo "" | tee -a "$RESULTS_FILE"
echo "=== Checking evaluations in DB ===" | tee -a "$RESULTS_FILE"
PGPASSWORD=eval_pass psql -h localhost -p 3105 -U eval_user -d evaluations -c "SELECT COUNT(*) FROM evaluations;" 2>&1 | tee -a "$RESULTS_FILE"

echo "" | tee -a "$RESULTS_FILE"
echo "=== Test Complete ===" | tee -a "$RESULTS_FILE"
