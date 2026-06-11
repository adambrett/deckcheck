-- Add the singleton-guard column that enforces the single-project-row
-- invariant at the schema level, and backfill rows that predate it.
--
-- A DeckCheck project file describes exactly one project; loadInfo
-- already rejects files with more than one project row, but that guard
-- runs after the file is in the user's hands. The guard column gets a
-- unique index in migration 0004 so any attempt to insert a second
-- row fails at the database boundary.
--
-- DuckDB's ALTER TABLE does not currently accept NOT NULL / DEFAULT
-- clauses, so the column is added nullable and backfilled here. The
-- backfill must run BEFORE the unique index exists: DuckDB updates an
-- indexed column by deleting and re-inserting the whole row, which
-- falsely trips the primary-key check against the row's own previous
-- version ("Duplicate key ... violates primary key constraint"; see
-- the index-limitations section of the DuckDB docs). The application
-- layer always inserts singleton_guard = 1 (see the projectfile
-- package).

ALTER TABLE project ADD COLUMN singleton_guard INTEGER;

UPDATE project SET singleton_guard = 1 WHERE singleton_guard IS NULL;
