package projectfile_test

import (
	"context"
	"database/sql"
	"iter"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/marcboeker/go-duckdb" // register the DuckDB driver for raw fixture databases
	"github.com/stretchr/testify/require"

	"github.com/adambrett/deckcheck/internal/dataset"
	"github.com/adambrett/deckcheck/internal/db"
	"github.com/adambrett/deckcheck/internal/project"
	"github.com/adambrett/deckcheck/internal/projectfile"
)

func TestCreateAndOpen(t *testing.T) {
	// Given
	dbPath := projectPath(t)

	// When
	created, err := projectfile.Create(context.Background(), dbPath, projectfile.CreateOptions{
		Name:        "Test Project",
		DatasetType: dataset.TypeCSVWithImage,
		ImageColumn: "image",
	})

	// Then
	require.NoError(t, err)
	require.NotZero(t, created.Info().ID)
	require.Equal(t, "Test Project", created.Info().Name)
	require.Equal(t, dataset.TypeCSVWithImage, created.Info().DatasetType)
	require.Equal(t, "image", created.Info().ImageColumn)
	require.NoError(t, created.Close())

	// When
	loaded, err := projectfile.Open(context.Background(), dbPath)

	// Then
	require.NoError(t, err)
	defer func() { _ = loaded.Close() }()
	require.Equal(t, created.Info().ID, loaded.Info().ID)
	require.Equal(t, created.Info().Name, loaded.Info().Name)
}

func TestCreateHonoursCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	created, err := projectfile.Create(ctx, projectPath(t), projectfile.CreateOptions{
		Name:        "Test",
		DatasetType: dataset.TypeCSV,
	})

	// Then
	require.Nil(t, created)
	require.ErrorIs(t, err, context.Canceled)
}

func TestOpenHonoursCanceledContext(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	opened, err := projectfile.Open(ctx, projectPath(t))

	// Then
	require.Nil(t, opened)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCreateReplacesExistingProjectFile(t *testing.T) {
	// Given
	dbPath := projectPath(t)
	first, err := projectfile.Create(context.Background(), dbPath, projectfile.CreateOptions{
		Name:        "First",
		DatasetType: dataset.TypeCSV,
		Source:      csvSource(t, "name\nAlice\n"),
	})
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When
	second, err := projectfile.Create(context.Background(), dbPath, projectfile.CreateOptions{
		Name:        "Second",
		DatasetType: dataset.TypeCSV,
		Source:      csvSource(t, "name\nBob\n"),
	})

	// Then
	require.NoError(t, err)
	require.NoError(t, second.Close())

	loaded, err := projectfile.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = loaded.Close() }()

	require.Equal(t, "Second", loaded.Info().Name)
	count, err := loaded.RecordCount(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
	record, err := loaded.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, "Bob", record.Data["name"])
}

func TestCreateImportsDatasetAndQuestions(t *testing.T) {
	// Given
	sourcePath := filepath.Join(t.TempDir(), "source.csv")
	require.NoError(t, os.WriteFile(sourcePath, []byte("name,value\nAlice,100\nBob,200\n"), 0o644))

	// When
	p := createProject(t, projectfile.CreateOptions{
		Name:        "Dataset Project",
		DatasetType: dataset.TypeCSV,
		Questions:   []project.QuestionDef{{Text: "Valid?", Answers: []string{"Yes", "No"}}},
		Source:      dataset.NewCSV(sourcePath),
	})

	// Then
	count, err := p.RecordCount(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)

	questions, err := p.Questions(context.Background())
	require.NoError(t, err)
	require.Len(t, questions, 1)
}

func TestCreateDoesNotMutateCallerQuestions(t *testing.T) {
	// Given a question definition that normalization would rewrite.
	questions := []project.QuestionDef{{Text: "  Valid?  ", Answers: []string{" Yes ", " No "}}}

	// When
	createProject(t, projectfile.CreateOptions{
		Name:        "Mutation Guard",
		DatasetType: dataset.TypeCSV,
		Questions:   questions,
	})

	// Then the caller's slice still holds the raw, untrimmed values.
	require.Equal(t, "  Valid?  ", questions[0].Text)
	require.Equal(t, []string{" Yes ", " No "}, questions[0].Answers)
}

func TestCreatePreservesOrderedColumnsAndAppendsExtras(t *testing.T) {
	// Given a source whose rows carry columns beyond the declared order.
	source := datasetSource{
		records: []dataset.RawRecord{
			{
				Columns: []string{"existing", "ordered"},
				Data: map[string]string{
					"existing": "1",
					"ordered":  "2",
					"z-extra":  "3",
					"a-extra":  "4",
				},
			},
		},
	}

	// When
	p := createProject(t, projectfile.CreateOptions{
		Name:        "Columns",
		DatasetType: dataset.TypeCSV,
		Source:      source,
	})

	// Then declared columns keep their order; extras follow, sorted.
	require.Equal(t, []string{"existing", "ordered", "a-extra", "z-extra"}, p.DataColumns())
}

func TestCreateRejectsInvalidProjectOptions(t *testing.T) {
	// Given
	cases := []struct {
		name string
		opts projectfile.CreateOptions
		want error
	}{
		{
			name: "blank name",
			opts: projectfile.CreateOptions{DatasetType: dataset.TypeCSV},
			want: projectfile.ErrInvalidProjectOptions,
		},
		{
			name: "unsupported dataset type",
			opts: projectfile.CreateOptions{Name: "Test", DatasetType: dataset.Type("unknown")},
			want: projectfile.ErrInvalidProjectOptions,
		},
		{
			name: "image column on plain csv",
			opts: projectfile.CreateOptions{Name: "Test", DatasetType: dataset.TypeCSV, ImageColumn: "image"},
			want: projectfile.ErrInvalidProjectOptions,
		},
		{
			name: "missing image column",
			opts: projectfile.CreateOptions{Name: "Test", DatasetType: dataset.TypeCSVWithImage},
			want: projectfile.ErrInvalidProjectOptions,
		},
		{
			name: "question with blank text",
			opts: projectfile.CreateOptions{
				Name:        "Test",
				DatasetType: dataset.TypeCSV,
				Questions:   []project.QuestionDef{{Text: " ", Answers: []string{"A", "B"}}},
			},
			want: projectfile.ErrInvalidQuestion,
		},
		{
			name: "question with too few answers",
			opts: projectfile.CreateOptions{
				Name:        "Test",
				DatasetType: dataset.TypeCSV,
				Questions:   []project.QuestionDef{{Text: "Valid?", Answers: []string{"Yes"}}},
			},
			want: projectfile.ErrInvalidQuestion,
		},
		{
			name: "question with blank answer",
			opts: projectfile.CreateOptions{
				Name:        "Test",
				DatasetType: dataset.TypeCSV,
				Questions:   []project.QuestionDef{{Text: "Valid?", Answers: []string{"Yes", ""}}},
			},
			want: projectfile.ErrInvalidQuestion,
		},
		{
			name: "question with duplicate answers",
			opts: projectfile.CreateOptions{
				Name:        "Test",
				DatasetType: dataset.TypeCSV,
				Questions:   []project.QuestionDef{{Text: "Valid?", Answers: []string{"A", "a"}}},
			},
			want: projectfile.ErrInvalidQuestion,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			created, err := projectfile.Create(context.Background(), projectPath(t), tc.opts)

			// Then
			require.Nil(t, created)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCreateLeavesValidFileOnImportFailure(t *testing.T) {
	// Given
	dbPath := projectPath(t)

	// When
	_, err := projectfile.Create(context.Background(), dbPath, projectfile.CreateOptions{
		Name:        "Broken",
		DatasetType: dataset.TypeCSV,
		Source:      dataset.NewCSV("/missing.csv"),
	})

	// Then the create fails, and no half-written project is installed.
	require.Error(t, err)
	if _, statErr := os.Stat(dbPath); statErr == nil {
		_, openErr := projectfile.Open(context.Background(), dbPath)
		require.Error(t, openErr)
	}
}

func TestOpenRejectsDatabaseWithoutProjectRow(t *testing.T) {
	// Given a schema-complete database with no project row.
	dbPath := projectPath(t)
	conn, err := db.Create(context.Background(), dbPath)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	// When
	opened, err := projectfile.Open(context.Background(), dbPath)

	// Then
	require.Nil(t, opened)
	require.ErrorIs(t, err, projectfile.ErrInvalidProjectFile)
}

func TestOpenRejectsNonDeckCheckDatabase(t *testing.T) {
	// Given a DuckDB file that is not a DeckCheck project.
	dbPath := projectPath(t)
	raw, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = raw.Exec("CREATE TABLE notes (body VARCHAR)")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// When
	opened, err := projectfile.Open(context.Background(), dbPath)

	// Then the db-layer rejection maps to the project-file sentinel.
	require.Nil(t, opened)
	require.ErrorIs(t, err, projectfile.ErrInvalidProjectFile)
}

func TestQuestionsRoundTrip(t *testing.T) {
	// Given
	p := createProject(t, projectfile.CreateOptions{
		Name:        "Questions",
		DatasetType: dataset.TypeCSV,
		Questions: []project.QuestionDef{
			{Text: "First?", Answers: []string{"A", "B"}},
			{Text: "Second?", Answers: []string{"C", "D"}},
		},
	})

	// When
	questions, err := p.Questions(context.Background())

	// Then
	require.NoError(t, err)
	require.Len(t, questions, 2)
	require.Equal(t, "First?", questions[0].Text)
	require.Len(t, questions[0].Answers, 2)
	require.Equal(t, "A", questions[0].Answers[0].Text)
	require.Equal(t, "Second?", questions[1].Text)
}

func TestRecordsAndClassifications(t *testing.T) {
	// Given
	p := createProjectWithData(t, "name,value\nAlice,100\nBob,200\n", singleQuestion())
	question := firstQuestion(t, p)

	count, err := p.RecordCount(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)

	record, err := p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, "Alice", record.Data["name"])

	// When
	require.NoError(t, p.SaveClassification(context.Background(), record.ID, question.ID, question.Answers[0].ID))

	// Then the answer reads back through Record and counts as progress.
	record, err = p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, question.Answers[0].ID, record.Answers[question.ID])

	classified, total, err := p.Progress(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, classified)
	require.Equal(t, 2, total)

	// When
	require.NoError(t, p.DeleteClassification(context.Background(), record.ID, question.ID))

	// Then
	record, err = p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Empty(t, record.Answers)
}

func TestProgressWithNoQuestions(t *testing.T) {
	// Given
	p := createProjectWithData(t, "name\nAlice\nBob\n", nil)

	// When
	classified, total, err := p.Progress(context.Background())

	// Then
	require.NoError(t, err)
	require.Equal(t, 0, classified)
	require.Equal(t, 2, total)
}

func TestRecordReturnsNotFoundForMissingIndex(t *testing.T) {
	// Given
	p := createProjectWithData(t, "name\nAlice\n", nil)

	// When
	record, err := p.Record(context.Background(), 10)

	// Then
	require.Nil(t, record)
	require.ErrorIs(t, err, projectfile.ErrRecordNotFound)
}

func TestDeleteMissingClassificationIsNoop(t *testing.T) {
	// Given
	p := createProject(t, projectfile.CreateOptions{Name: "Test", DatasetType: dataset.TypeCSV})

	// When
	err := p.DeleteClassification(context.Background(), 999, 999)

	// Then
	require.NoError(t, err)
}

func TestSaveClassificationIsUpsert(t *testing.T) {
	// Given
	p := createProjectWithData(t, "n\nhello\n", singleQuestion())
	question := firstQuestion(t, p)

	rec, err := p.Record(context.Background(), 0)
	require.NoError(t, err)

	// When the same (record, question) pair is saved twice
	require.NoError(t, p.SaveClassification(context.Background(), rec.ID, question.ID, question.Answers[0].ID))
	require.NoError(t, p.SaveClassification(context.Background(), rec.ID, question.ID, question.Answers[1].ID))

	// Then the second answer replaces the first.
	rec, err = p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, question.Answers[1].ID, rec.Answers[question.ID])
}

func TestSaveClassificationRejectsAnswerFromDifferentQuestion(t *testing.T) {
	// Given
	p := createProjectWithData(t, "n\nhello\n", []project.QuestionDef{
		{Text: "Q1", Answers: []string{"A", "B"}},
		{Text: "Q2", Answers: []string{"C", "D"}},
	})
	questions, err := p.Questions(context.Background())
	require.NoError(t, err)
	q1, q2 := questions[0], questions[1]

	rec, err := p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, p.SaveClassification(context.Background(), rec.ID, q1.ID, q1.Answers[0].ID))

	// When an answer belonging to q2 is saved against q1
	err = p.SaveClassification(context.Background(), rec.ID, q1.ID, q2.Answers[0].ID)

	// Then the save is rejected and the original answer is untouched.
	require.ErrorIs(t, err, projectfile.ErrInvalidClassification)
	rec, err = p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, q1.Answers[0].ID, rec.Answers[q1.ID])
}

func TestUnclassifiedNavigation(t *testing.T) {
	// Given three records with only the middle one classified.
	p := createProjectWithData(t, "a\n0\n1\n2\n", singleQuestion())
	question := firstQuestion(t, p)

	record, err := p.Record(context.Background(), 1)
	require.NoError(t, err)
	require.NoError(t, p.SaveClassification(context.Background(), record.ID, question.ID, question.Answers[0].ID))

	// When / Then scanning from the start finds the first record.
	index, found, err := p.NextUnclassified(context.Background(), 0)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 0, index)

	// When / Then scanning past the first skips the classified middle.
	index, found, err = p.NextUnclassified(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 2, index)

	// When / Then scanning backwards from the end skips it too.
	index, found, err = p.PreviousUnclassified(context.Background(), 2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 0, index)
}

func TestUnclassifiedNavigationReportsNoneRemaining(t *testing.T) {
	// Given every record classified.
	p := createProjectWithData(t, "a\n0\n1\n", singleQuestion())
	question := firstQuestion(t, p)

	for i := range 2 {
		rec, recErr := p.Record(context.Background(), i)
		require.NoError(t, recErr)
		require.NoError(t, p.SaveClassification(context.Background(), rec.ID, question.ID, question.Answers[0].ID))
	}

	// When
	_, found, err := p.NextUnclassified(context.Background(), 0)

	// Then exhaustion is a normal outcome, not an error.
	require.NoError(t, err)
	require.False(t, found)

	// When / Then the backwards scan agrees.
	_, found, err = p.PreviousUnclassified(context.Background(), 2)
	require.NoError(t, err)
	require.False(t, found)
}

func TestClassifiedRecordsAndDataColumns(t *testing.T) {
	// Given
	p := createProjectWithData(t, "text,value\nHello,100\nWorld,200\n", []project.QuestionDef{
		{Text: "Sentiment?", Answers: []string{"Positive", "Negative"}},
		{Text: "Category?", Answers: []string{"A", "B"}},
	})
	questions, err := p.Questions(context.Background())
	require.NoError(t, err)
	q1, q2 := questions[0], questions[1]

	record, err := p.Record(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, p.SaveClassification(context.Background(), record.ID, q1.ID, q1.Answers[0].ID))
	require.NoError(t, p.SaveClassification(context.Background(), record.ID, q2.ID, q2.Answers[1].ID))

	// When
	require.Equal(t, []string{"text", "value"}, p.DataColumns())

	var rows []project.ClassifiedRecord
	for rec, err := range p.ClassifiedRecords(context.Background(), questions) {
		require.NoError(t, err)
		rows = append(rows, rec)
	}

	// Then
	require.Len(t, rows, 2)
	require.Equal(t, "Positive", rows[0].Answers[q1.ID])
	require.Equal(t, "B", rows[0].Answers[q2.ID])
	require.Empty(t, rows[1].Answers)
}

func TestDataColumnsPreserveSourceOrder(t *testing.T) {
	// Given
	p := createProjectWithData(t, "z,a,m\nlast,first,middle\n", nil)

	// When / Then
	require.Equal(t, []string{"z", "a", "m"}, p.DataColumns())
}

func TestInfoDoesNotExposeMutableDataColumns(t *testing.T) {
	// Given
	p := createProjectWithData(t, "first,second\n1,2\n", nil)

	// When a caller mutates the slice Info returned
	info := p.Info()
	info.DataColumns[0] = "mutated"

	// Then the project's own copy is unaffected.
	require.Equal(t, []string{"first", "second"}, p.Info().DataColumns)
	require.Equal(t, []string{"first", "second"}, p.DataColumns())
}

func TestCloseIsIdempotent(t *testing.T) {
	// Given
	p := createProject(t, projectfile.CreateOptions{Name: "Test", DatasetType: dataset.TypeCSV})

	// When
	require.NoError(t, p.Close())
	require.NoError(t, p.Close())

	// Then
	count, err := p.RecordCount(context.Background())
	require.Zero(t, count)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)
}

func TestCloseNilProjectIsNoop(t *testing.T) {
	// Given
	var p *projectfile.Project

	// When / Then
	require.NoError(t, p.Close())
}

func TestClosedProjectMethodsReturnErrProjectClosed(t *testing.T) {
	// Given
	p := createProject(t, projectfile.CreateOptions{Name: "Test", DatasetType: dataset.TypeCSV})
	require.NoError(t, p.Close())

	// When / Then every database-touching method reports the closed state.
	_, err := p.Questions(context.Background())
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	_, err = p.Record(context.Background(), 0)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	err = p.SaveClassification(context.Background(), 1, 1, 1)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	err = p.DeleteClassification(context.Background(), 1, 1)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	_, _, err = p.Progress(context.Background())
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	_, _, err = p.NextUnclassified(context.Background(), 0)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	_, _, err = p.PreviousUnclassified(context.Background(), 0)
	require.ErrorIs(t, err, projectfile.ErrProjectClosed)

	var gotErr error
	for _, err := range p.ClassifiedRecords(context.Background(), nil) {
		gotErr = err
	}
	require.ErrorIs(t, gotErr, projectfile.ErrProjectClosed)
}

// projectPath returns a temp path that does not yet exist.
func projectPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "test.duckdb")
}

// createProject creates a project from opts and closes it when the test ends.
func createProject(t *testing.T, opts projectfile.CreateOptions) *projectfile.Project {
	t.Helper()

	p, err := projectfile.Create(context.Background(), projectPath(t), opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })

	return p
}

// createProjectWithData creates a project pre-loaded with csvContent rows
// and the given questions (nil for none).
func createProjectWithData(t *testing.T, csvContent string, questions []project.QuestionDef) *projectfile.Project {
	t.Helper()

	return createProject(t, projectfile.CreateOptions{
		Name:        "Test",
		DatasetType: dataset.TypeCSV,
		Questions:   questions,
		Source:      csvSource(t, csvContent),
	})
}

func singleQuestion() []project.QuestionDef {
	return []project.QuestionDef{{Text: "Q1", Answers: []string{"A", "B"}}}
}

func firstQuestion(t *testing.T, p *projectfile.Project) project.Question {
	t.Helper()

	questions, err := p.Questions(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, questions)

	return questions[0]
}

func csvSource(t *testing.T, content string) dataset.Source {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return dataset.NewCSV(path)
}

// datasetSource is an in-memory Source for exercising import column rules.
type datasetSource struct {
	records []dataset.RawRecord
	err     error
}

func (s datasetSource) Records(context.Context) iter.Seq2[dataset.RawRecord, error] {
	return func(yield func(dataset.RawRecord, error) bool) {
		if s.err != nil {
			yield(dataset.RawRecord{}, s.err)
			return
		}
		for _, rec := range s.records {
			if !yield(rec, nil) {
				return
			}
		}
	}
}
