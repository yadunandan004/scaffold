-- Create test samples tracker table for audit logging
CREATE TABLE IF NOT EXISTS test_samples_tracker (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    entity JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on created_at
CREATE INDEX IF NOT EXISTS idx_test_samples_tracker_created_at ON test_samples_tracker(created_at);