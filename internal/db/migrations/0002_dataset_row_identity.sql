-- Protect the one-row-index-per-project invariant. Re-importing a dataset
-- into an existing project is rejected in code; this index keeps older project
-- files from drifting into ambiguous duplicate row indexes.

CREATE UNIQUE INDEX IF NOT EXISTS idx_dataset_rows_project_row_index_unique
    ON dataset_rows(project_id, row_index);

-- DuckDB cannot reliably update a column that carries a foreign-key
-- constraint, which makes classification upserts fragile. Keep the row and
-- question constraints in the database; answer ownership is validated in the
-- project package before every save.

CREATE TABLE classifications_v2 (
    row_id         INTEGER NOT NULL,
    question_id    INTEGER NOT NULL,
    answer_id      INTEGER NOT NULL,
    classified_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (row_id, question_id),
    FOREIGN KEY (row_id) REFERENCES dataset_rows(id),
    FOREIGN KEY (question_id) REFERENCES questions(id)
);

INSERT INTO classifications_v2 (row_id, question_id, answer_id, classified_at)
SELECT row_id, question_id, answer_id, classified_at
FROM classifications;

DROP TABLE classifications;

ALTER TABLE classifications_v2 RENAME TO classifications;
