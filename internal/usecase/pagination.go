package usecase

import "git.eiac.dev/tools/denar/internal/domain"

const (
	// DefaultPageLimit is the page size used when the caller gives none.
	DefaultPageLimit = 50
	// MaxPageLimit is the largest page size a caller may ask for.
	MaxPageLimit = 200
)

// NewPage returns the page a repository expects. Without a limit it uses
// DefaultPageLimit; a limit outside 1..MaxPageLimit is rejected with a
// domain.ValidationError on the field "limit".
func NewPage(cursor string, limit domain.Optional[int]) (Page, error) {
	if !limit.Set {
		return Page{Cursor: cursor, Limit: DefaultPageLimit}, nil
	}
	if limit.Value < 1 || limit.Value > MaxPageLimit {
		return Page{}, domain.ValidationError{Field: "limit", Message: "must be between 1 and 200"}
	}
	return Page{Cursor: cursor, Limit: limit.Value}, nil
}

// cursorError puts a repository's rejection of a cursor on the field
// "cursor"; other errors pass through.
func cursorError(err error, cursor string) error {
	if cursor != "" && plainInvalid(err) {
		return domain.ValidationError{Field: "cursor", Message: "is not a cursor of this query"}
	}
	return err
}
