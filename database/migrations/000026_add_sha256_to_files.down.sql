-- Remove sha256 column from files table
ALTER TABLE files DROP COLUMN IF EXISTS sha256;
