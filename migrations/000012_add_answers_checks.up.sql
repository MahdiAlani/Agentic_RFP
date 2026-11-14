-- The deterministic quality checks a worker ran on an answer, and an md5 of the
-- text they were run on. Approval requires the checks to match the current text.
ALTER TABLE answers ADD COLUMN IF NOT EXISTS checks JSONB;
ALTER TABLE answers ADD COLUMN IF NOT EXISTS checks_text_md5 TEXT;
