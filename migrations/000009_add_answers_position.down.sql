DROP INDEX IF EXISTS idx_answers_project_id;
ALTER TABLE answers DROP COLUMN IF EXISTS position;
