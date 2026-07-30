-- Extracted RFP questions keep the order they appeared in the source document.
ALTER TABLE answers ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0;

-- The RFP editor lists answers per project, so index that lookup.
CREATE INDEX IF NOT EXISTS idx_answers_project_id ON answers(project_id);

-- status vocabulary: 'new' (extracted, no answer yet) -> 'pending' (queued with
-- a worker) -> 'done', or 'failed'.
