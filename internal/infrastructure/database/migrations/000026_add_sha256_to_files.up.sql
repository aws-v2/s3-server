-- Add sha256 column to files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS sha256 VARCHAR(64) DEFAULT 'not_calculated_yet';

-- Update existing records to have the default value (though column default handles it)
UPDATE files SET sha256 = 'not_calculated_yet' WHERE sha256 IS NULL;
