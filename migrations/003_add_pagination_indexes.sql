-- Migration: Add composite index for efficient prediction pagination
-- This creates a composite index on (evaluation_id, question_index) for efficient
-- LIMIT/OFFSET pagination on large result sets (10,000+ predictions)
-- 
-- The composite index allows the query:
--   SELECT * FROM predictions 
--   WHERE evaluation_id = $1 
--   ORDER BY question_index ASC 
--   LIMIT $2 OFFSET $3
-- to use index-only scan or efficient index scan

-- Create composite index for efficient pagination queries
CREATE INDEX IF NOT EXISTS idx_predictions_evaluation_id_question_index 
ON predictions(evaluation_id, question_index);

-- Create index on question_index alone for any queries that filter by question index
CREATE INDEX IF NOT EXISTS idx_predictions_question_index ON predictions(question_index);

-- Analyze the predictions table to update statistics for query planner
ANALYZE predictions;

-- Comments for documentation
COMMENT ON INDEX idx_predictions_evaluation_id_question_index IS 'Composite index for efficient pagination on large prediction sets. Supports ORDER BY question_index with evaluation_id filter.';
