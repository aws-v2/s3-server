-- Create users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Create index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Create index on is_active for filtering active users
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

-- Add foreign key constraint to buckets table (owner_id references users.id)
ALTER TABLE buckets DROP CONSTRAINT IF EXISTS fk_buckets_owner;
ALTER TABLE buckets
ADD CONSTRAINT fk_buckets_owner
FOREIGN KEY (owner_id) REFERENCES users(id)
ON DELETE CASCADE;
