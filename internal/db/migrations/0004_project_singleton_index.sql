-- Enforce the single-project-row invariant with a unique index over
-- the guard column added and backfilled by migration 0003.
--
-- The index creation runs as its own migration because DuckDB cannot
-- create an index on a table with outstanding updates in the same
-- transaction, and each migration runs in exactly one transaction.
-- IF NOT EXISTS keeps this a no-op for files created while the index
-- still lived in migration 0003.

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_singleton ON project(singleton_guard);
