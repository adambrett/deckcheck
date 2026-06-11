-- Initial DeckCheck project schema.
--
-- Adds the five aggregate tables (project / questions / answers /
-- dataset_rows / classifications), their identity sequences, indexes for
-- the hot lookups, and a denormalised data_columns blob on project so
-- exports do not have to re-derive the column union at write time.

CREATE SEQUENCE IF NOT EXISTS project_seq START 1;
CREATE SEQUENCE IF NOT EXISTS questions_seq START 1;
CREATE SEQUENCE IF NOT EXISTS answers_seq START 1;
CREATE SEQUENCE IF NOT EXISTS dataset_rows_seq START 1;

-- The project table holds exactly one row per .deckcheck file. The
-- singleton_guard column added by migration 0003 carries the unique
-- index that enforces the invariant at the database boundary.
CREATE TABLE IF NOT EXISTS project (
    id            INTEGER PRIMARY KEY DEFAULT nextval('project_seq'),
    name          VARCHAR NOT NULL,
    dataset_type  VARCHAR NOT NULL,
    image_column  VARCHAR,
    data_columns  JSON    NOT NULL DEFAULT '[]',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS questions (
    id            INTEGER PRIMARY KEY DEFAULT nextval('questions_seq'),
    project_id    INTEGER NOT NULL,
    question_text VARCHAR NOT NULL,
    sort_order    INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES project(id)
);
CREATE INDEX IF NOT EXISTS idx_questions_project ON questions(project_id, sort_order);

CREATE TABLE IF NOT EXISTS answers (
    id           INTEGER PRIMARY KEY DEFAULT nextval('answers_seq'),
    question_id  INTEGER NOT NULL,
    answer_text  VARCHAR NOT NULL,
    sort_order   INTEGER NOT NULL,
    FOREIGN KEY (question_id) REFERENCES questions(id)
);
CREATE INDEX IF NOT EXISTS idx_answers_question ON answers(question_id, sort_order);

CREATE TABLE IF NOT EXISTS dataset_rows (
    id             INTEGER PRIMARY KEY DEFAULT nextval('dataset_rows_seq'),
    project_id     INTEGER NOT NULL,
    row_index      INTEGER NOT NULL,
    original_data  JSON,
    image_path     VARCHAR,
    FOREIGN KEY (project_id) REFERENCES project(id)
);
CREATE INDEX IF NOT EXISTS idx_dataset_rows_project ON dataset_rows(project_id, row_index);

-- classifications has no surrogate id: (row_id, question_id) is naturally
-- unique and never referenced elsewhere, so it doubles as the primary key.
-- DuckDB's index handling rejects updates of foreign-key columns when an
-- equivalent column also carries a surrogate PK; this shape sidesteps it.
CREATE TABLE IF NOT EXISTS classifications (
    row_id         INTEGER NOT NULL,
    question_id    INTEGER NOT NULL,
    answer_id      INTEGER NOT NULL,
    classified_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (row_id, question_id),
    FOREIGN KEY (row_id) REFERENCES dataset_rows(id),
    FOREIGN KEY (question_id) REFERENCES questions(id),
    FOREIGN KEY (answer_id) REFERENCES answers(id)
);
