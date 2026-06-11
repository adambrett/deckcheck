package projectfile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/project"
)

// importDataset writes every record from source into dataset_rows inside
// the given transaction and returns the union of column names observed
// across all rows in first-seen source order.
func importDataset(ctx context.Context, tx *sql.Tx, projectID int, source dataset.Source) ([]string, error) {
	columns := newColumnSet()
	rowIndex := 0
	for rec, err := range source.Records(ctx) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			return nil, err
		}

		jsonData, err := json.Marshal(rec.Data)
		if err != nil {
			return nil, fmt.Errorf("marshal row data: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO dataset_rows (project_id, row_index, original_data, image_path) VALUES (?, ?, ?::JSON, ?)",
			projectID, rowIndex, string(jsonData), rec.ImagePath,
		); err != nil {
			return nil, fmt.Errorf("insert row %d: %w", rowIndex, err)
		}

		columns.add(rec.Columns, rec.Data)
		rowIndex++
	}

	return columns.slice(), nil
}

// columnSet accumulates the union of column names across imported rows,
// preserving first-seen order for columns the source declares and
// appending any extra map-only columns in sorted order after them.
type columnSet struct {
	seen  map[string]struct{}
	names []string
}

func newColumnSet() *columnSet {
	return &columnSet{seen: make(map[string]struct{})}
}

func (s *columnSet) add(ordered []string, data map[string]string) {
	for _, column := range ordered {
		s.append(column)
	}

	extra := make([]string, 0, len(data))
	for column := range data {
		if _, ok := s.seen[column]; ok {
			continue
		}
		extra = append(extra, column)
	}
	sort.Strings(extra)

	for _, column := range extra {
		s.append(column)
	}
}

func (s *columnSet) append(column string) {
	if _, ok := s.seen[column]; ok {
		return
	}

	s.seen[column] = struct{}{}
	s.names = append(s.names, column)
}

func (s *columnSet) slice() []string {
	if s.names == nil {
		return []string{}
	}

	return s.names
}

// RecordCount returns the number of dataset rows imported for this project.
func (p *Project) RecordCount(ctx context.Context) (int, error) {
	conn, err := p.conn()
	if err != nil {
		return 0, err
	}

	var count int
	if err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dataset_rows WHERE project_id = ?", p.info.ID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count records: %w", err)
	}

	return count, nil
}

// Record returns the dataset record at the given zero-based index along
// with any classifications already recorded against it. The row and its
// classifications are fetched in a single query. [ErrRecordNotFound] is
// returned when no row exists at index.
func (p *Project) Record(ctx context.Context, index int) (*project.Record, error) {
	conn, err := p.conn()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT dr.id, dr.row_index, dr.original_data::VARCHAR, dr.image_path,
		       c.question_id, c.answer_id
		FROM dataset_rows dr
		LEFT JOIN classifications c ON c.row_id = dr.id
		WHERE dr.project_id = ? AND dr.row_index = ?
	`, p.info.ID, index)
	if err != nil {
		return nil, fmt.Errorf("query record: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		rowID     int
		rowIdx    int
		jsonData  string
		imagePath *string
		answers   = make(map[int]int)
		found     bool
	)
	for rows.Next() {
		var (
			qID sql.NullInt64
			aID sql.NullInt64
		)
		if err := rows.Scan(&rowID, &rowIdx, &jsonData, &imagePath, &qID, &aID); err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		found = true
		if qID.Valid && aID.Valid {
			answers[int(qID.Int64)] = int(aID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate record: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("%w: index %d", ErrRecordNotFound, index)
	}

	data := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("decode record data: %w", err)
	}

	path := ""
	if imagePath != nil {
		path = *imagePath
	}

	return &project.Record{
		ID:        rowID,
		Index:     rowIdx,
		Data:      data,
		ImagePath: path,
		Answers:   answers,
	}, nil
}
