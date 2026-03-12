-- Add vpc_id column to access_points table
ALTER TABLE access_points ADD COLUMN IF NOT EXISTS vpc_id VARCHAR(255);
