DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='buckets' AND column_name='policy') THEN
        ALTER TABLE buckets ADD COLUMN policy JSONB;
    END IF;
END $$;