-- Remove vpc_id column from access_points table
ALTER TABLE access_points DROP COLUMN IF EXISTS vpc_id;
