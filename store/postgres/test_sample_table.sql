-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create test sample table
CREATE TABLE IF NOT EXISTS test_samples (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'active',
    count INTEGER DEFAULT 0,
    amount DECIMAL(10,2),
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create index on name
CREATE INDEX IF NOT EXISTS idx_test_samples_name ON test_samples(name);

-- Create index on status
CREATE INDEX IF NOT EXISTS idx_test_samples_status ON test_samples(status);

-- Create index on deleted_at for soft deletes
CREATE INDEX IF NOT EXISTS idx_test_samples_deleted_at ON test_samples(deleted_at);

-- Create updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_test_samples_updated_at 
    BEFORE UPDATE ON test_samples 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();