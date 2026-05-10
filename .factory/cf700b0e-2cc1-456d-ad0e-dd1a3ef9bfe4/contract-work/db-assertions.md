# PostgreSQL Database Behavioral Assertions

## Schema Integrity

### VAL-DB-001: Evaluations Table Schema
The `evaluations` table exists with columns: id (UUID/serial), model_id (UUID/integer), dataset_ids (array/JSONB), config (JSONB), status (varchar/enum), created_at (timestamp), updated_at (timestamp), started_at (timestamp), completed_at (timestamp). All timestamp columns accept NULL except created_at.
- Tool: psql
- Evidence: `\d evaluations` shows all columns with correct types and nullable flags

### VAL-DB-002: Models Table Schema
The `models` table exists with columns: id (UUID/serial), name (varchar NOT NULL), type (varchar CHECK constraint for 'api'/'local'), provider (varchar CHECK constraint for 'openai'/'claude'/'qwen'), api_key_ref (varchar). Name is unique.
- Tool: psql
- Evidence: `\d models` shows columns, constraints including CHECK constraints and unique index on name

### VAL-DB-003: Datasets Table Schema
The `datasets` table exists with columns: id (UUID/serial), name (varchar NOT NULL UNIQUE), description (text), config_template (JSONB). Name must be unique.
- Tool: psql
- Evidence: `\d datasets` shows columns with NOT NULL and UNIQUE constraints

### VAL-DB-004: Results Table Schema
The `results` table exists with columns: evaluation_id (UUID/integer), accuracy (decimal/float), metrics (JSONB), summary (text). evaluation_id is a foreign key referencing evaluations(id).
- Tool: psql
- Evidence: `\d results` shows columns and foreign key constraint to evaluations

### VAL-DB-005: Predictions Table Schema
The `predictions` table exists with columns: evaluation_id (UUID/integer), question (text NOT NULL), prediction (text NOT NULL), answer (text NOT NULL), correct (boolean NOT NULL). evaluation_id references evaluations(id).
- Tool: psql
- Evidence: `\d predictions` shows columns with NOT NULL constraints and foreign key

### VAL-DB-006: Logs Table Schema
The `logs` table exists with columns: evaluation_id (UUID/integer), timestamp (timestamp NOT NULL), level (varchar CHECK for 'debug'/'info'/'warn'/'error'), message (text NOT NULL). evaluation_id references evaluations(id).
- Tool: psql
- Evidence: `\d logs` shows columns with constraints and foreign key

---

## CRUD Operations

### VAL-DB-007: Create Evaluation
INSERT into evaluations with valid model_id and dataset_ids succeeds. Status defaults to 'pending' and created_at is auto-populated with current timestamp.
- Tool: psql
- Evidence: Execute `INSERT INTO evaluations (model_id, dataset_ids, config, status) VALUES (...) RETURNING *;` returns new row with generated id and created_at

### VAL-DB-008: Update Evaluation Status
UPDATE evaluation status from 'pending' to 'running' sets started_at timestamp. UPDATE from 'running' to 'completed' sets completed_at timestamp.
- Tool: psql
- Evidence: Execute status transitions and verify started_at, completed_at, updated_at are set correctly

### VAL-DB-009: Insert Results with JSONB Metrics
INSERT into results with evaluation_id, accuracy, and metrics as JSONB object succeeds. Metrics column accepts nested JSON structures.
- Tool: psql
- Evidence: `INSERT INTO results (evaluation_id, accuracy, metrics, summary) VALUES (..., '{"precision": 0.85, "recall": 0.90, "f1": 0.87}') RETURNING metrics->>'precision';` returns '0.85'

### VAL-DB-010: Batch Insert Predictions
INSERT multiple predictions for an evaluation succeeds. All rows are created with correct evaluation_id association.
- Tool: psql
- Evidence: `INSERT INTO predictions (evaluation_id, question, prediction, answer, correct) VALUES ..., (...) RETURNING count(*);` returns inserted count

### VAL-DB-011: Delete Evaluation Cascades to Related Data
DELETE an evaluation removes all associated results, predictions, and logs due to foreign key cascade or triggers. No orphan records remain.
- Tool: psql
- Evidence: Delete evaluation, then query `SELECT count(*) FROM predictions WHERE evaluation_id = <deleted_id>;` returns 0

---

## Query Behavior

### VAL-DB-012: Paginated Evaluation List
SELECT evaluations with LIMIT and OFFSET returns correct page of results ordered by created_at DESC. Total count can be retrieved separately.
- Tool: psql
- Evidence: `SELECT * FROM evaluations ORDER BY created_at DESC LIMIT 10 OFFSET 0;` returns first 10 evaluations. `SELECT count(*) FROM evaluations;` returns total count.

### VAL-DB-013: Filter Evaluations by Status
SELECT evaluations WHERE status = 'completed' returns only completed evaluations. Multiple status filter with IN clause works correctly.
- Tool: psql
- Evidence: `SELECT * FROM evaluations WHERE status = 'completed';` returns only completed records. Verify status values match filter.

### VAL-DB-014: Filter Evaluations by Date Range
SELECT evaluations WHERE created_at BETWEEN start AND end returns evaluations within date range. Supports ISO timestamp format.
- Tool: psql
- Evidence: `SELECT * FROM evaluations WHERE created_at BETWEEN '2024-01-01' AND '2024-12-31';` returns correct filtered results

### VAL-DB-015: Order Evaluations by Multiple Columns
SELECT evaluations ORDER BY status ASC, created_at DESC returns results sorted by status first, then by creation date within each status group.
- Tool: psql
- Evidence: `SELECT id, status, created_at FROM evaluations ORDER BY status ASC, created_at DESC;` shows correct sort order

---

## Data Integrity

### VAL-DB-016: Foreign Key Enforcement - Results
INSERT into results with non-existent evaluation_id fails with foreign key violation error. Database prevents orphan result records.
- Tool: psql
- Evidence: `INSERT INTO results (evaluation_id, accuracy) VALUES ('non-existent-uuid', 0.5);` returns ERROR: foreign key violation

### VAL-DB-017: Foreign Key Enforcement - Predictions
INSERT into predictions with non-existent evaluation_id fails. NOT NULL columns (question, prediction, answer, correct) reject NULL values.
- Tool: psql
- Evidence: Attempt INSERT with NULL question returns ERROR: null value in column "question" violates not-null constraint

### VAL-DB-018: Model Name Uniqueness
INSERT into models with duplicate name fails with unique constraint violation. Each model name must be unique.
- Tool: psql
- Evidence: `INSERT INTO models (name, type, provider) VALUES ('existing-model', 'api', 'openai');` on duplicate name returns ERROR: duplicate key value

---

## JSONB Operations

### VAL-DB-019: Query JSONB Metrics Fields
SELECT results where metrics JSONB contains specific key-value pair. Using `->>` operator extracts text value for comparison.
- Tool: psql
- Evidence: `SELECT * FROM results WHERE metrics->>'f1' = '0.87';` returns results with matching f1 score. Alternative: `SELECT * FROM results WHERE metrics @> '{"f1": 0.87}';`

### VAL-DB-020: Update JSONB Config Nested Field
UPDATE evaluations config JSONB with jsonb_set() modifies nested fields while preserving other config. Partial updates do not overwrite entire JSONB.
- Tool: psql
- Evidence: `UPDATE evaluations SET config = jsonb_set(config, '{temperature}', '0.8') WHERE id = '...';` updates only temperature field

### VAL-DB-021: JSONB Array Operations
SELECT evaluations where dataset_ids JSONB array contains specific dataset_id. Using `?` operator or `@>` for array containment.
- Tool: psql
- Evidence: `SELECT * FROM evaluations WHERE dataset_ids @> '["dataset-uuid-123"]';` returns evaluations containing that dataset

### VAL-DB-022: Aggregate JSONB Metrics
SELECT with aggregation on JSONB metrics column calculates average accuracy across evaluations. Can extract and aggregate JSONB numeric values.
- Tool: psql
- Evidence: `SELECT avg((metrics->>'f1')::float) FROM results;` returns average f1 score as numeric

---

## Indexes

### VAL-DB-023: Index on Evaluations Status
EXPLAIN ANALYZE on SELECT evaluations WHERE status = 'completed' shows index scan using status index, not sequential scan.
- Tool: psql
- Evidence: `EXPLAIN ANALYZE SELECT * FROM evaluations WHERE status = 'completed';` output shows "Index Scan" and index name

### VAL-DB-024: Index on Evaluations Created At
EXPLAIN ANALYZE on SELECT evaluations ORDER BY created_at DESC shows index scan on created_at index for efficient sorting.
- Tool: psql
- Evidence: `EXPLAIN ANALYZE SELECT * FROM evaluations ORDER BY created_at DESC LIMIT 10;` shows "Index Scan Backward"

### VAL-DB-025: Composite Index Usage
EXPLAIN ANALYZE on SELECT evaluations WHERE status = 'completed' ORDER BY created_at DESC shows composite index usage (if exists) or both indexes.
- Tool: psql
- Evidence: `EXPLAIN ANALYZE SELECT * FROM evaluations WHERE status = 'completed' ORDER BY created_at DESC LIMIT 10;` shows efficient query plan

### VAL-DB-026: Foreign Key Index on Predictions
EXPLAIN ANALYZE on SELECT predictions WHERE evaluation_id = '...' shows index scan on evaluation_id foreign key index.
- Tool: psql
- Evidence: `EXPLAIN ANALYZE SELECT * FROM predictions WHERE evaluation_id = 'some-uuid';` shows "Index Scan" on evaluation_id

---

## Connection Handling

### VAL-DB-027: Connection Pool Max Connections
Application respects max connection pool limit. Opening connections beyond pool limit blocks until a connection is released, not rejected with error.
- Tool: psql
- Evidence: Query `SELECT count(*) FROM pg_stat_activity WHERE datname = 'database_name';` before and during load test shows bounded connections

### VAL-DB-028: Connection Timeout Handling
Query exceeding statement timeout returns error with timeout message. Long-running queries are terminated after configured timeout period.
- Tool: psql
- Evidence: `SET statement_timeout = '1s'; SELECT pg_sleep(2);` returns ERROR: canceling statement due to statement timeout

### VAL-DB-029: Connection Pool Idle Timeout
Idle connections in pool are closed after configured idle timeout period. New requests establish fresh connections.
- Tool: psql
- Evidence: Monitor `pg_stat_activity` for pool connections before and after idle timeout period, verify connections are cleaned up

### VAL-DB-030: Transaction Rollback on Error
Failed transaction within connection is properly rolled back. Subsequent queries on same connection do not see partial data from failed transaction.
- Tool: psql
- Evidence: Start transaction, INSERT, cause error, ROLLBACK, verify data not present with SELECT

---

## Summary

Total Assertions: 30

| Category | Count |
|----------|-------|
| Schema Integrity | 6 |
| CRUD Operations | 5 |
| Query Behavior | 4 |
| Data Integrity | 3 |
| JSONB Operations | 4 |
| Indexes | 4 |
| Connection Handling | 4 |

Each assertion includes:
- Clear behavioral description
- Pass/fail condition
- Tool specification (psql)
- Evidence collection method (queries, schema inspection, connection monitoring)
