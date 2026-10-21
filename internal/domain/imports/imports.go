// Package imports describes batches of transactions imported from outside
// the app: a CSV file today, bank and billing connectors later.
package imports

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Source is where a batch came from.
type Source string

const (
	SourceCSV        Source = "csv"
	SourceConnection Source = "connection"
)

// Valid reports whether s is a known source.
func (s Source) Valid() bool { return s == SourceCSV || s == SourceConnection }

// Status is the state of a batch.
type Status string

const (
	Completed Status = "completed"
	Reverted  Status = "reverted"
)

// Limits of one batch.
const (
	// MaxRows is the most rows a batch may have.
	MaxRows = 10000
	// MaxStoredErrors is the most row errors a batch keeps; its counter
	// stays exact.
	MaxStoredErrors = 100
	// MaxExternalID is the longest external id, as for transactions.
	MaxExternalID = 200
)

// RowError says why one row was not imported. Row is the line of the
// file (1-based), or the position in the batch for other sources; 0 is the
// batch as a whole.
type RowError struct {
	Row     int
	Field   string
	Message string
}

// Batch is one import into one wallet.
type Batch struct {
	ID          domain.ID
	HouseholdID domain.ID
	WalletID    domain.ID
	Source      Source
	// SourceRef is the file name or the connection id.
	SourceRef string
	// Preset is the CSV preset used; "" for other sources.
	Preset string
	Status Status
	// Rows counts every row of the input; Created, Duplicates and Failed
	// split them. Matched counts the new transactions matched to
	// subscription charges; Reverted those a revert deleted.
	Rows, Created, Duplicates, Failed, Matched, Reverted int
	// Errors are the first MaxStoredErrors row errors.
	Errors     []RowError
	CreatedBy  domain.Actor
	CreatedAt  time.Time
	RevertedBy domain.Actor
	RevertedAt time.Time
}

// KeepErrors returns at most MaxStoredErrors of errs.
func KeepErrors(errs []RowError) []RowError {
	if len(errs) > MaxStoredErrors {
		errs = errs[:MaxStoredErrors]
	}
	return append([]RowError{}, errs...)
}

// NormalizeText trims the text, lowers its case and collapses runs of
// white space, so that the same description written twice compares equal.
func NormalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// hashVersion starts every hashed id; another algorithm gets another.
const hashVersion = "v1"

// HashExternalID is the external id of a row that has none: a hash of the
// date, the amount in minor units, the normalised description and the
// row's occurrence among rows of the batch with the same three values
// (1, 2, ...). The same row of the same file always gets the same id.
func HashExternalID(date domain.Date, amount int64, text string, occurrence int) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{hashVersion, date.String(), strconv.FormatInt(amount, 10),
		NormalizeText(text), strconv.Itoa(occurrence)}, "|")))
	return "h:" + hex.EncodeToString(sum[:16])
}
