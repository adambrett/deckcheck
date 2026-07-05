package projectfile

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/adambrett/deckcheck/internal/project"
)

// insertQuestion writes a single question and its answers inside the
// given transaction. Questions are only ever created through [Create];
// there is deliberately no post-creation question API because changing
// the question set would invalidate existing classifications.
func insertQuestion(ctx context.Context, tx *sql.Tx, projectID int, def project.QuestionDef) (*project.Question, error) {
	var maxOrder int
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), 0) FROM questions WHERE project_id = ?", projectID,
	).Scan(&maxOrder); err != nil {
		return nil, fmt.Errorf("read sort_order: %w", err)
	}

	var questionID int
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO questions (
			project_id, question_text, question_kind, grid_rows, grid_columns, sort_order
		) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		projectID, def.Text, string(def.Kind), def.GridRows, def.GridColumns, maxOrder+1,
	).Scan(&questionID); err != nil {
		return nil, fmt.Errorf("insert question: %w", err)
	}

	out := &project.Question{
		ID:          questionID,
		Kind:        def.Kind,
		Text:        def.Text,
		GridRows:    def.GridRows,
		GridColumns: def.GridColumns,
		Answers:     make([]project.Answer, 0, len(def.Answers)),
	}
	for i, answerText := range def.Answers {
		var answerID int
		if err := tx.QueryRowContext(ctx,
			"INSERT INTO answers (question_id, answer_text, sort_order) VALUES (?, ?, ?) RETURNING id",
			questionID, answerText, i+1,
		).Scan(&answerID); err != nil {
			return nil, fmt.Errorf("insert answer: %w", err)
		}
		out.Answers = append(out.Answers, project.Answer{ID: answerID, Text: answerText})
	}

	return out, nil
}

// Questions returns every question in the project together with its
// answers in a single SQL roundtrip. Questions are ordered by the
// position the user defined them in; answers within each question are
// in their original definition order.
func (p *Project) Questions(ctx context.Context) ([]project.Question, error) {
	conn, err := p.conn()
	if err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT q.id, q.question_text, q.question_kind, q.grid_rows, q.grid_columns, q.sort_order,
		       a.id, a.answer_text, a.sort_order
		FROM questions q
		LEFT JOIN answers a ON a.question_id = q.id
		WHERE q.project_id = ?
		ORDER BY q.sort_order, a.sort_order
	`, p.info.ID)
	if err != nil {
		return nil, fmt.Errorf("query questions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var questions []project.Question
	indexByID := make(map[int]int)
	for rows.Next() {
		var (
			qID, qRows, qColumns, qSort int
			qText, qKind                string
			aID                         sql.NullInt64
			aText                       sql.NullString
			aSort                       sql.NullInt64
		)
		if err := rows.Scan(&qID, &qText, &qKind, &qRows, &qColumns, &qSort, &aID, &aText, &aSort); err != nil {
			return nil, fmt.Errorf("scan question row: %w", err)
		}

		idx, ok := indexByID[qID]
		if !ok {
			idx = len(questions)
			indexByID[qID] = idx
			questions = append(questions, project.Question{
				ID:          qID,
				Kind:        project.QuestionKind(qKind),
				Text:        qText,
				GridRows:    qRows,
				GridColumns: qColumns,
			})
		}
		if aID.Valid {
			questions[idx].Answers = append(questions[idx].Answers, project.Answer{
				ID:   int(aID.Int64),
				Text: aText.String,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate questions: %w", err)
	}

	return questions, nil
}

// questionCount returns the number of questions defined on this project.
// It is used internally by the navigation helpers so callers no longer
// have to thread the count in by hand.
func (p *Project) questionCount(ctx context.Context) (int, error) {
	conn, err := p.conn()
	if err != nil {
		return 0, err
	}

	var count int
	if err := conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM questions WHERE project_id = ?", p.info.ID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count questions: %w", err)
	}
	return count, nil
}
