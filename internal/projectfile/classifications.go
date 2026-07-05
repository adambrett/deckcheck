package projectfile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/adambrett/deckcheck/internal/project"
)

// SaveClassification stores the user's answer for a (record, question)
// pair. Any existing answer for the same pair is replaced.
//
// DuckDB rejects ON CONFLICT updates of foreign-key columns, so the
// portable form is update-first, insert-if-missing inside a transaction.
// The row, question, and answer relationship is validated first so callers
// cannot save an answer against the wrong question.
func (p *Project) SaveClassification(ctx context.Context, rowID, questionID, answerID int) error {
	conn, err := p.conn()
	if err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save classification: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	valid, err := validClassification(ctx, tx, p.info.ID, rowID, questionID, answerID)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidClassification
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE classifications
		 SET answer_id = ?, annotation_value = '', classified_at = CURRENT_TIMESTAMP
		 WHERE row_id = ? AND question_id = ?`,
		answerID, rowID, questionID,
	)
	if err != nil {
		return fmt.Errorf("update classification: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated classifications: %w", err)
	}
	if affected == 0 {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO classifications (row_id, question_id, answer_id, annotation_value, classified_at)
			 VALUES (?, ?, ?, '', CURRENT_TIMESTAMP)`,
			rowID, questionID, answerID,
		); err != nil {
			return fmt.Errorf("insert classification: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit classification: %w", err)
	}

	return nil
}

func validClassification(ctx context.Context, tx *sql.Tx, projectID, rowID, questionID, answerID int) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM dataset_rows dr
		JOIN questions q ON q.project_id = dr.project_id
		JOIN answers a ON a.question_id = q.id
		WHERE dr.project_id = ?
		  AND dr.id = ?
		  AND q.id = ?
		  AND q.question_kind = ?
		  AND a.id = ?
	`, projectID, rowID, questionID, string(project.QuestionKindChoice), answerID).Scan(&count); err != nil {
		return false, fmt.Errorf("validate classification: %w", err)
	}

	return count == 1, nil
}

// SaveGridAnnotation stores the user's selected grid cells for a
// (record, image-grid question) pair. Any existing classification for
// the same pair is replaced. value is normalised before storage.
func (p *Project) SaveGridAnnotation(ctx context.Context, rowID, questionID int, value string) error {
	conn, err := p.conn()
	if err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save grid annotation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, columns, valid, err := validGridAnnotation(ctx, tx, p.info.ID, rowID, questionID)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidClassification
	}

	normalized, err := project.NormalizeGridSelection(value, rows, columns)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidClassification, err)
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE classifications
		 SET answer_id = NULL, annotation_value = ?, classified_at = CURRENT_TIMESTAMP
		 WHERE row_id = ? AND question_id = ?`,
		normalized, rowID, questionID,
	)
	if err != nil {
		return fmt.Errorf("update grid annotation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated grid annotations: %w", err)
	}
	if affected == 0 {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO classifications (row_id, question_id, answer_id, annotation_value, classified_at)
			 VALUES (?, ?, NULL, ?, CURRENT_TIMESTAMP)`,
			rowID, questionID, normalized,
		); err != nil {
			return fmt.Errorf("insert grid annotation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit grid annotation: %w", err)
	}

	return nil
}

func validGridAnnotation(ctx context.Context, tx *sql.Tx, projectID, rowID, questionID int) (rows, columns int, valid bool, err error) {
	queryErr := tx.QueryRowContext(ctx, `
		SELECT q.grid_rows, q.grid_columns
		FROM dataset_rows dr
		JOIN questions q ON q.project_id = dr.project_id
		WHERE dr.project_id = ?
		  AND dr.id = ?
		  AND q.id = ?
		  AND q.question_kind = ?
	`, projectID, rowID, questionID, string(project.QuestionKindImageGrid)).Scan(&rows, &columns)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	if queryErr != nil {
		return 0, 0, false, fmt.Errorf("validate grid annotation: %w", queryErr)
	}

	return rows, columns, true, nil
}

// DeleteClassification removes the answer recorded for a (record,
// question) pair, if any.
func (p *Project) DeleteClassification(ctx context.Context, rowID, questionID int) error {
	conn, err := p.conn()
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx,
		"DELETE FROM classifications WHERE row_id = ? AND question_id = ?",
		rowID, questionID,
	); err != nil {
		return fmt.Errorf("delete classification: %w", err)
	}
	return nil
}

// Progress returns the number of fully-classified records and the total
// record count. A record is fully classified when it has an answer for
// every question in the project.
func (p *Project) Progress(ctx context.Context) (int, int, error) {
	conn, err := p.conn()
	if err != nil {
		return 0, 0, err
	}

	var total int
	if queryErr := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dataset_rows WHERE project_id = ?", p.info.ID,
	).Scan(&total); queryErr != nil {
		return 0, 0, fmt.Errorf("count records: %w", queryErr)
	}

	questions, err := p.questionCount(ctx)
	if err != nil {
		return 0, 0, err
	}
	if questions == 0 {
		return 0, total, nil
	}

	var classified int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT dr.id
			FROM dataset_rows dr
			LEFT JOIN classifications c ON c.row_id = dr.id
			WHERE dr.project_id = ?
			GROUP BY dr.id
			HAVING COUNT(c.question_id) = ?
		) t
	`, p.info.ID, questions).Scan(&classified); err != nil {
		return 0, 0, fmt.Errorf("count classified: %w", err)
	}
	return classified, total, nil
}

// NextUnclassified returns the row_index of the first unclassified
// record at or after fromIndex. Pass fromIndex == 0 to scan from the
// start. ok is false when no unclassified record remains at or after
// fromIndex: a normal outcome, not an error.
func (p *Project) NextUnclassified(ctx context.Context, fromIndex int) (index int, ok bool, err error) {
	return p.findUnclassified(ctx, "AND dr.row_index >= ?", "ORDER BY dr.row_index ASC", fromIndex)
}

// PreviousUnclassified returns the row_index of the closest unclassified
// record strictly before beforeIndex. ok is false when no unclassified
// record exists before beforeIndex: a normal outcome, not an error.
func (p *Project) PreviousUnclassified(ctx context.Context, beforeIndex int) (index int, ok bool, err error) {
	return p.findUnclassified(ctx, "AND dr.row_index < ?", "ORDER BY dr.row_index DESC", beforeIndex)
}

// findUnclassified shares the body of the unclassified-index queries.
// positionPredicate is an extra AND clause; orderClause is the ORDER BY
// direction; refIndex is bound into the predicate.
func (p *Project) findUnclassified(ctx context.Context, positionPredicate, orderClause string, refIndex int) (int, bool, error) {
	conn, err := p.conn()
	if err != nil {
		return 0, false, err
	}

	questions, err := p.questionCount(ctx)
	if err != nil {
		return 0, false, err
	}

	//nolint:gosec // positionPredicate and orderClause are package-internal constants, not user input
	q := `
		SELECT dr.row_index
		FROM dataset_rows dr
		LEFT JOIN classifications c ON c.row_id = dr.id
		WHERE dr.project_id = ?
		` + positionPredicate + `
		GROUP BY dr.row_index
		HAVING COUNT(c.question_id) < ?
		` + orderClause + `
		LIMIT 1
	`

	var index int
	err = conn.QueryRowContext(ctx, q, p.info.ID, refIndex, questions).Scan(&index)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find unclassified record: %w", err)
	}

	return index, true, nil
}
