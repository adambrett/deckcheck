package projectfile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/adambrett/deckcheck/internal/project"
)

// DataColumns returns the union of column names captured at import
// time. The list is cached on the project row, so this is a constant-
// time accessor that issues no SQL.
func (p *Project) DataColumns() []string {
	out := make([]string, len(p.info.DataColumns))
	copy(out, p.info.DataColumns)
	return out
}

// ClassifiedRecords streams every record in the project together with
// the answer text for each classified question. The implementation
// reads a single LEFT JOIN in (row, question_id) order and groups
// consecutive rows that share a row id, so a 10k-record project costs
// one query rather than 10k+1.
//
// Iteration stops on the first error. The underlying query is opened
// on demand and closed when the iterator returns.
func (p *Project) ClassifiedRecords(ctx context.Context, questions []project.Question) iter.Seq2[project.ClassifiedRecord, error] {
	answerText := make(map[int]string)
	for _, q := range questions {
		for _, a := range q.Answers {
			answerText[a.ID] = a.Text
		}
	}

	return func(yield func(project.ClassifiedRecord, error) bool) {
		conn, err := p.conn()
		if err != nil {
			yield(project.ClassifiedRecord{}, err)
			return
		}

		rows, err := conn.QueryContext(ctx, `
			SELECT dr.id, dr.original_data::VARCHAR, dr.image_path,
			       c.question_id, c.answer_id, c.annotation_value
			FROM dataset_rows dr
			LEFT JOIN classifications c ON c.row_id = dr.id
			WHERE dr.project_id = ?
			ORDER BY dr.row_index, c.question_id
		`, p.info.ID)
		if err != nil {
			yield(project.ClassifiedRecord{}, fmt.Errorf("query records: %w", err))
			return
		}
		defer func() { _ = rows.Close() }()

		var (
			currentID   int
			current     project.ClassifiedRecord
			haveCurrent bool
		)

		for rows.Next() {
			var (
				rowID      int
				jsonData   string
				imagePath  *string
				qID        sql.NullInt64
				aID        sql.NullInt64
				annotation sql.NullString
			)
			if err := rows.Scan(&rowID, &jsonData, &imagePath, &qID, &aID, &annotation); err != nil {
				yield(project.ClassifiedRecord{}, fmt.Errorf("scan record: %w", err))
				return
			}

			if !haveCurrent || rowID != currentID {
				if haveCurrent && !yield(current, nil) {
					return
				}
				data := make(map[string]string)
				if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
					yield(project.ClassifiedRecord{}, fmt.Errorf("decode record data: %w", err))
					return
				}
				current = project.ClassifiedRecord{
					Data:            data,
					Answers:         make(map[int]string),
					GridAnnotations: make(map[int]string),
				}
				if imagePath != nil {
					current.ImagePath = *imagePath
				}
				currentID = rowID
				haveCurrent = true
			}

			if qID.Valid && aID.Valid {
				if text, ok := answerText[int(aID.Int64)]; ok {
					current.Answers[int(qID.Int64)] = text
				}
			} else if qID.Valid {
				current.GridAnnotations[int(qID.Int64)] = annotation.String
			}
		}
		if err := rows.Err(); err != nil {
			yield(project.ClassifiedRecord{}, fmt.Errorf("iterate dataset rows: %w", err))
			return
		}
		if haveCurrent {
			yield(current, nil)
		}
	}
}
