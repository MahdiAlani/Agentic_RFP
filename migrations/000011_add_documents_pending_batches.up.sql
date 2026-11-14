-- Embedding batches still queued for a document; it is 'embedded' once this reaches 0.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS pending_batches INT NOT NULL DEFAULT 0;
