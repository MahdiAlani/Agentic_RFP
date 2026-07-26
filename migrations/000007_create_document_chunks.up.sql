CREATE EXTENSION IF NOT EXISTS vector;

-- Embedded chunks of uploaded documents, searched by the generate worker.
-- NOTE: embedding dimension (1536) assumes an OpenAI text-embedding-3-small
-- style model. Change vector(N) to match whatever embedding model you use.
CREATE TABLE document_chunks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    chunk_index  INT NOT NULL,
    content      TEXT NOT NULL,
    embedding    vector(1536),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_document_chunks_document_id ON document_chunks(document_id);
CREATE INDEX idx_document_chunks_workspace_id ON document_chunks(workspace_id);

-- Approximate-nearest-neighbour index for cosine similarity search.
CREATE INDEX idx_document_chunks_embedding
    ON document_chunks USING hnsw (embedding vector_cosine_ops);
