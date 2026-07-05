package projectfile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/db"
	"github.com/adambrett/deckcheck/internal/project"
)

// Project is a single open DeckCheck project. The caller owns the
// lifecycle: every successful return from [Open] or [Create] must
// eventually be paired with a [Project.Close] call.
//
// Close may be called concurrently with other methods: calls that start
// after Close observe the project as closed, while calls already running
// finish against the closing connection pool.
type Project struct {
	database  atomic.Pointer[sql.DB]
	info      project.Info
	closeOnce sync.Once
	closeErr  error
}

// newProject wraps an open connection and its metadata in a Project.
func newProject(conn *sql.DB, info project.Info) *Project {
	p := &Project{info: info}
	p.database.Store(conn)

	return p
}

// Open reads an existing DeckCheck project from path. The underlying
// file is migrated forward to the current schema if it is older; see
// [db.ErrSchemaTooNew] if it is newer than this binary knows about.
func Open(ctx context.Context, path string) (*Project, error) {
	conn, err := db.Open(ctx, path)
	if err != nil {
		if errors.Is(err, db.ErrNotDeckCheckProject) {
			return nil, fmt.Errorf("%w: %w", ErrInvalidProjectFile, err)
		}
		return nil, err
	}

	info, err := loadInfo(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return newProject(conn, info), nil
}

// CreateOptions describes the contents of a new project. ImageColumn is
// only consulted when DatasetType is [dataset.TypeCSVWithImage]. Source
// may be nil; if supplied, every record from it is imported into the
// project before Create returns.
type CreateOptions struct {
	Name        string
	DatasetType dataset.Type
	ImageColumn string
	Questions   []project.QuestionDef
	Source      dataset.Source
}

// Create initialises a new DeckCheck project at path and populates it
// from opts. The project is built in a temporary database first and then
// moved into place, so a failed import never leaves behind a half-created
// project or appends a second project row to an existing file.
func Create(ctx context.Context, path string, opts CreateOptions) (*Project, error) {
	opts, err := normalizeCreateOptions(opts)
	if err != nil {
		return nil, err
	}

	tempPath, err := tempProjectPath(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	conn, err := db.Create(ctx, tempPath)
	if err != nil {
		return nil, err
	}
	info, err := createInTx(ctx, conn, opts)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if closeErr := conn.Close(); closeErr != nil {
		return nil, fmt.Errorf("close temporary project: %w", closeErr)
	}
	if replaceErr := atomicReplaceProjectFile(tempPath, path); replaceErr != nil {
		return nil, fmt.Errorf("install project file: %w", replaceErr)
	}

	conn, err = db.Open(ctx, path)
	if err != nil {
		return nil, err
	}

	return newProject(conn, info), nil
}

func tempProjectPath(path string) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	file, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temporary project file: %w", err)
	}
	tempPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close temporary project file: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return "", fmt.Errorf("prepare temporary project file: %w", err)
	}

	return tempPath, nil
}

type projectFileOps struct {
	goos     string
	rename   func(string, string) error
	remove   func(string) error
	tempPath func(string) (string, error)
}

// atomicReplaceProjectFile renames tempPath onto path, falling back to a
// three-step swap on Windows (where os.Rename refuses to overwrite an
// existing file). The Windows branch deliberately reports the backup
// location in its error message so a user whose project is stranded at
// a temp filename can recover it manually.
func atomicReplaceProjectFile(tempPath, path string) error {
	return atomicReplaceProjectFileWithOps(tempPath, path, projectFileOps{
		goos:     runtime.GOOS,
		rename:   os.Rename,
		remove:   os.Remove,
		tempPath: tempProjectPath,
	})
}

func atomicReplaceProjectFileWithOps(tempPath, path string, ops projectFileOps) error {
	if err := ops.rename(tempPath, path); err == nil {
		return nil
	} else if ops.goos != "windows" || !errors.Is(err, fs.ErrExist) {
		return err
	}

	backupPath, err := ops.tempPath(path)
	if err != nil {
		return err
	}

	if err := ops.rename(path, backupPath); err != nil {
		return err
	}

	if err := ops.rename(tempPath, path); err != nil {
		if restoreErr := ops.rename(backupPath, path); restoreErr != nil {
			// Both the install and the restore failed: the user's
			// original project now lives at backupPath. Surface that
			// path verbatim so they can move it back themselves.
			return fmt.Errorf("install project file: %w; restore previous project: %w; original project is preserved at %s", err, restoreErr, backupPath)
		}
		return err
	}

	_ = ops.remove(backupPath)
	return nil
}

// createInTx runs the entire project bootstrap inside a single
// transaction so a partial failure leaves the project tables empty.
func createInTx(ctx context.Context, conn *sql.DB, opts CreateOptions) (project.Info, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return project.Info{}, fmt.Errorf("begin project create: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	info, err := insertProject(ctx, tx, opts.Name, opts.DatasetType, opts.ImageColumn)
	if err != nil {
		return project.Info{}, err
	}

	for _, q := range opts.Questions {
		if _, err := insertQuestion(ctx, tx, info.ID, q); err != nil {
			return project.Info{}, fmt.Errorf("add question: %w", err)
		}
	}

	if opts.Source != nil {
		columns, err := importDataset(ctx, tx, info.ID, opts.Source)
		if err != nil {
			return project.Info{}, fmt.Errorf("import dataset: %w", err)
		}
		if err := setDataColumns(ctx, tx, info.ID, columns); err != nil {
			return project.Info{}, fmt.Errorf("record data columns: %w", err)
		}
		info.DataColumns = columns
	}

	if err := tx.Commit(); err != nil {
		return project.Info{}, fmt.Errorf("commit project create: %w", err)
	}

	return info, nil
}

// loadInfo reads the single project row out of conn. The query fetches
// up to two rows so a malformed file with duplicate project rows is
// reported as ErrInvalidProjectFile rather than silently using the
// first row; the schema-level singleton index added in migration 0003
// keeps fresh files honest, but older files predate that guard.
func loadInfo(ctx context.Context, conn *sql.DB) (project.Info, error) {
	var (
		info        project.Info
		imageColumn *string
		datasetType string
		dataColumns string
	)
	rows, err := conn.QueryContext(ctx,
		"SELECT id, name, dataset_type, image_column, data_columns::VARCHAR, created_at FROM project ORDER BY id LIMIT 2",
	)
	if err != nil {
		return project.Info{}, fmt.Errorf("query project row: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return project.Info{}, fmt.Errorf("iterate project row: %w", err)
		}
		return project.Info{}, fmt.Errorf("%w: missing project row", ErrInvalidProjectFile)
	}
	if err := rows.Scan(&info.ID, &info.Name, &datasetType, &imageColumn, &dataColumns, &info.CreatedAt); err != nil {
		return project.Info{}, fmt.Errorf("scan project row: %w", err)
	}
	if rows.Next() {
		return project.Info{}, fmt.Errorf("%w: multiple project rows", ErrInvalidProjectFile)
	}
	if err := rows.Err(); err != nil {
		return project.Info{}, fmt.Errorf("iterate project row: %w", err)
	}

	info.DatasetType = dataset.Type(datasetType)
	if imageColumn != nil {
		info.ImageColumn = *imageColumn
	}
	if dataColumns != "" {
		if err := json.Unmarshal([]byte(dataColumns), &info.DataColumns); err != nil {
			return project.Info{}, fmt.Errorf("decode data columns: %w", err)
		}
	}

	return info, nil
}

// insertProject inserts the project row and returns the populated Info
// (sans DataColumns, which are filled after dataset import).
func insertProject(ctx context.Context, tx *sql.Tx, name string, datasetType dataset.Type, imageColumn string) (project.Info, error) {
	now := time.Now()
	var id int
	err := tx.QueryRowContext(ctx,
		// singleton_guard is always 1; the unique index on it enforces
		// the one-project-per-file invariant. See migration 0003.
		"INSERT INTO project (name, dataset_type, image_column, created_at, singleton_guard) VALUES (?, ?, ?, ?, 1) RETURNING id",
		name, string(datasetType), imageColumn, now,
	).Scan(&id)
	if err != nil {
		return project.Info{}, fmt.Errorf("insert project row: %w", err)
	}

	return project.Info{
		ID:          id,
		Name:        name,
		DatasetType: datasetType,
		ImageColumn: imageColumn,
		CreatedAt:   now,
	}, nil
}

// setDataColumns persists the union of column names from the imported
// dataset onto the project row, so the exporter can read it cheaply at
// export time rather than re-scanning every dataset row.
func setDataColumns(ctx context.Context, tx *sql.Tx, projectID int, columns []string) error {
	encoded, err := json.Marshal(columns)
	if err != nil {
		return fmt.Errorf("marshal data columns: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE project SET data_columns = ?::JSON WHERE id = ?", string(encoded), projectID,
	); err != nil {
		return fmt.Errorf("update data columns: %w", err)
	}
	return nil
}

// Info returns the project's metadata.
func (p *Project) Info() project.Info {
	info := p.info
	info.DataColumns = p.DataColumns()

	return info
}

func (p *Project) conn() (*sql.DB, error) {
	if p == nil {
		return nil, ErrProjectClosed
	}

	database := p.database.Load()
	if database == nil {
		return nil, ErrProjectClosed
	}

	return database, nil
}

// Close releases the underlying database connection. Calls that start
// after Close fail with [ErrProjectClosed]; Close on an already-closed
// Project is a no-op and returns the same error reported by the first
// Close.
func (p *Project) Close() error {
	if p == nil {
		return nil
	}

	p.closeOnce.Do(func() {
		if database := p.database.Swap(nil); database != nil {
			p.closeErr = database.Close()
		}
	})

	return p.closeErr
}
