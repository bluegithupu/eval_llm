-- Migration: Add concurrency control to evaluations table
-- This adds optimistic locking support to prevent race conditions
-- during status updates and handle concurrent modification conflicts

-- Add version column for optimistic locking
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 0;

-- Add lock_until column for pessimistic locking (optional, for long-running operations)
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS lock_until TIMESTAMP WITH TIME ZONE;

-- Add index for version column (used in optimistic locking)
CREATE INDEX IF NOT EXISTS idx_evaluations_version ON evaluations(version);

-- Add index for lock_until (for identifying locked evaluations)
CREATE INDEX IF NOT EXISTS idx_evaluations_lock_until ON evaluations(lock_until) WHERE lock_until IS NOT NULL;

-- Function to increment version on update
CREATE OR REPLACE FUNCTION increment_evaluation_version()
RETURNS TRIGGER AS $$
BEGIN
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-increment version on any update
DROP TRIGGER IF EXISTS increment_evaluation_version_on_update ON evaluations;
CREATE TRIGGER increment_evaluation_version_on_update
    BEFORE UPDATE ON evaluations
    FOR EACH ROW
    EXECUTE FUNCTION increment_evaluation_version();

-- Comments for documentation
COMMENT ON COLUMN evaluations.version IS 'Optimistic lock version - incremented on every update to detect concurrent modifications';
COMMENT ON COLUMN evaluations.lock_until IS 'Pessimistic lock expiry - set during long-running operations to prevent concurrent modifications';
