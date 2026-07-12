CREATE TABLE answers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id   UUID REFERENCES projects(id) ON DELETE CASCADE,
    document_id  UUID REFERENCES documents(id) ON DELETE SET NULL,
    question     TEXT NOT NULL,
    answer_text  TEXT,
    status       TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_answers_workspace_id ON answers(workspace_id);
CREATE INDEX idx_answers_document_id ON answers(document_id);
