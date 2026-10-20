-- Citations are numbered to match the inline [n] markers in the answer text.
ALTER TABLE answer_citations ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0;
