package usererror

import "errors"

// Operation-tag sentinels. Each names what the user was trying to do
// when the failure happened. Call sites wrap their domain error with
// one of these via [Wrap] so the routing in ForError can pick the
// right user-facing copy.
//
// Support codes DC01 and DC02 are retired and must not be reused.
// Active codes run from DC03 onward; uniqueness is enforced by the
// codes test in codes_test.go.
var (
	ErrCreateProject                  = errors.New("create project")
	ErrOpenProject                    = errors.New("open project")
	ErrLoadQuestions                  = errors.New("load project questions")
	ErrInitializeClassifier           = errors.New("initialize classifier")
	ErrReadCSVHeaders                 = errors.New("read csv headers")
	ErrFindFirstUnclassifiedRecord    = errors.New("find first unclassified record")
	ErrFindNextUnclassifiedRecord     = errors.New("find next unclassified record")
	ErrFindPreviousUnclassifiedRecord = errors.New("find previous unclassified record")
	ErrLoadRecord                     = errors.New("load record")
	ErrLoadProgress                   = errors.New("load progress")
	ErrSaveClassification             = errors.New("save classification")
	ErrDeleteClassification           = errors.New("delete classification")
	ErrExportCSV                      = errors.New("export csv")
)
