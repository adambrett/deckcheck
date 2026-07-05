-- Add image-grid questions and annotation-backed classifications.
--
-- Existing questions become multiple-choice questions. Grid questions store
-- their row/column count on questions and store their selected cells in the
-- classification row's annotation_value column. answer_id is nullable from
-- this point on because a grid annotation has no answer row.

ALTER TABLE questions ADD COLUMN question_kind VARCHAR;
ALTER TABLE questions ADD COLUMN grid_rows INTEGER;
ALTER TABLE questions ADD COLUMN grid_columns INTEGER;

UPDATE questions
SET question_kind = 'choice',
    grid_rows = 0,
    grid_columns = 0
WHERE question_kind IS NULL;

CREATE TABLE classifications_v3 (
    row_id            INTEGER NOT NULL,
    question_id       INTEGER NOT NULL,
    answer_id         INTEGER,
    annotation_value  VARCHAR NOT NULL DEFAULT '',
    classified_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (row_id, question_id),
    FOREIGN KEY (row_id) REFERENCES dataset_rows(id),
    FOREIGN KEY (question_id) REFERENCES questions(id)
);

INSERT INTO classifications_v3 (row_id, question_id, answer_id, annotation_value, classified_at)
SELECT row_id, question_id, answer_id, '', classified_at
FROM classifications;

DROP TABLE classifications;

ALTER TABLE classifications_v3 RENAME TO classifications;
