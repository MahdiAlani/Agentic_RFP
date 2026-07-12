CREATE TABLE answer_citations (
    answer_id   UUID NOT NULL REFERENCES answers(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_ref   TEXT NOT NULL,
    PRIMARY KEY (answer_id, document_id, chunk_ref)
);

CREATE INDEX idx_answer_citations_answer_id ON answer_citations(answer_id);
