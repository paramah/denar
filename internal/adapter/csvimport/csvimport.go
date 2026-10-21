// Package csvimport reads bank statements exported as CSV (stage 008). It
// implements usecase.CSVParser: the options say how the file is written
// (encoding, delimiter, date and number formats, header) and which columns
// hold what; presets carry the defaults of known exports.
//
// The presets of Polish banks and Revolut follow what is known of their
// exports and are tested on synthetic samples in testdata; they need
// checking against real files, and every option can be overridden per
// request.
package csvimport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/imports"
	"github.com/paramah/denar/internal/usecase"
)

// Parser is the CSV reader. The zero value is ready.
type Parser struct{}

var _ usecase.CSVParser = Parser{}

// New returns a parser.
func New() Parser { return Parser{} }

const (
	encodingUTF8    = "utf-8"
	encodingCP1250  = "windows-1250"
	maxRowErrorsCSV = imports.MaxRows + 1
)

func invalid(field, msg string) error { return domain.ValidationError{Field: field, Message: msg} }

// resolved are the options of one parse, the preset's merged with the
// request's.
type resolved struct {
	preset       string
	delimiter    rune // 0: detect
	encoding     string
	dateLayout   string // "": detect
	decimal      byte   // 0: detect
	skipRows     int
	hasHeader    bool
	invert       bool
	acceptStates []string
	columns      usecase.CSVColumns
}

func pick(over, base string) string {
	if strings.TrimSpace(over) != "" {
		return strings.TrimSpace(over)
	}
	return base
}

func pickList(over, base []string) []string {
	var clean []string
	for _, s := range over {
		if s = strings.TrimSpace(s); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) > 0 {
		return clean
	}
	return base
}

// resolve merges the request's options into the preset's and checks them.
func resolve(opts usecase.CSVOptions) (resolved, error) {
	key := pick(opts.Preset, "generic")
	preset, ok := presetByKey(key)
	if !ok {
		return resolved{}, invalid("options.preset", "is not a known preset")
	}
	base := preset.Options
	r := resolved{preset: key, skipRows: base.SkipRows, hasHeader: true, acceptStates: pickList(opts.AcceptStates, base.AcceptStates)}
	var errs []error
	switch d := pick(opts.Delimiter, base.Delimiter); d {
	case "":
	case ";", ",", "|":
		r.delimiter = rune(d[0])
	case "tab", "\t", `\t`:
		r.delimiter = '\t'
	default:
		errs = append(errs, invalid("options.delimiter", "must be ;, , , | or tab"))
	}
	switch e := strings.ToLower(pick(opts.Encoding, base.Encoding)); e {
	case "":
	case "utf-8", "utf8":
		r.encoding = encodingUTF8
	case "windows-1250", "cp1250", "win1250":
		r.encoding = encodingCP1250
	default:
		errs = append(errs, invalid("options.encoding", "must be utf-8 or windows-1250"))
	}
	if f := pick(opts.DateFormat, base.DateFormat); f != "" {
		layout, err := dateLayout(f)
		if err != nil {
			errs = append(errs, invalid("options.date_format", err.Error()))
		}
		r.dateLayout = layout
	}
	switch d := pick(opts.Decimal, base.Decimal); d {
	case "":
	case ",", ".":
		r.decimal = d[0]
	default:
		errs = append(errs, invalid("options.decimal_separator", "must be , or ."))
	}
	if opts.SkipRows < 0 || opts.SkipRows > 1000 {
		errs = append(errs, invalid("options.skip_rows", "must be 0 to 1000"))
	} else if opts.SkipRows > 0 {
		r.skipRows = opts.SkipRows
	}
	if base.HasHeader != nil {
		r.hasHeader = *base.HasHeader
	}
	if opts.HasHeader != nil {
		r.hasHeader = *opts.HasHeader
	}
	if base.InvertAmount != nil {
		r.invert = *base.InvertAmount
	}
	if opts.InvertAmount != nil {
		r.invert = *opts.InvertAmount
	}
	c, b := opts.Columns, base.Columns
	r.columns = usecase.CSVColumns{
		Date: pickList(c.Date, b.Date), Amount: pickList(c.Amount, b.Amount), Debit: pickList(c.Debit, b.Debit),
		Credit: pickList(c.Credit, b.Credit), ExternalID: pickList(c.ExternalID, b.ExternalID),
		Currency: pickList(c.Currency, b.Currency), Fee: pickList(c.Fee, b.Fee), State: pickList(c.State, b.State),
		Payee: pickList(c.Payee, b.Payee), Memo: pickList(c.Memo, b.Memo),
	}
	if len(r.columns.Date) == 0 {
		errs = append(errs, invalid("options.columns.date", "is required"))
	}
	if len(r.columns.Amount) == 0 && len(r.columns.Debit) == 0 && len(r.columns.Credit) == 0 {
		errs = append(errs, invalid("options.columns.amount", "is required, or debit and credit"))
	}
	return r, errors.Join(errs...)
}

// decode turns the file into UTF-8 text.
func decode(data []byte, encoding string) (string, string) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if encoding == "" {
		encoding = encodingCP1250
		if utf8.Valid(data) {
			encoding = encodingUTF8
		}
	}
	if encoding == encodingCP1250 {
		out, _ := charmap.Windows1250.NewDecoder().Bytes(data) // every byte maps
		return string(out), encoding
	}
	return strings.ToValidUTF8(string(data), "�"), encoding
}

// detectDelimiter picks the most frequent of ; , and tab outside quotes in
// the first lines; ; wins a tie.
func detectDelimiter(text string) rune {
	counts := map[rune]int{}
	lines := 0
	for line := range strings.Lines(text) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if lines++; lines > 30 {
			break
		}
		quoted := false
		for _, r := range line {
			switch {
			case r == '"':
				quoted = !quoted
			case !quoted && (r == ';' || r == ',' || r == '\t'):
				counts[r]++
			}
		}
	}
	best := ';'
	for _, r := range []rune{',', '\t'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	return best
}

func delimiterName(r rune) string {
	if r == '\t' {
		return "tab"
	}
	return string(r)
}

// record is one CSV record and the line it starts on.
type record struct {
	line   int
	fields []string
}

func readRecords(text string, delimiter rune) ([]record, []imports.RowError) {
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma, cr.LazyQuotes, cr.FieldsPerRecord, cr.TrimLeadingSpace = delimiter, true, -1, false
	var (
		out  []record
		errs []imports.RowError
	)
	for {
		fields, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) && len(errs) < maxRowErrorsCSV {
				errs = append(errs, imports.RowError{Row: pe.StartLine, Field: "row", Message: "is not valid CSV"})
				continue
			}
			break
		}
		line, _ := cr.FieldPos(0)
		out = append(out, record{line: line, fields: fields})
	}
	return out, errs
}

// headerName normalises a header for comparison.
func headerName(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#")))
}

// columnIndex returns the index of the first alternative the file has, or
// -1.
func columnIndex(alts []string, headers []string) int {
	for _, a := range alts {
		if i := singleIndex(a, headers); i >= 0 {
			return i
		}
	}
	return -1
}

func singleIndex(a string, headers []string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(a)); err == nil {
		if n >= 1 {
			return n - 1
		}
		return -1
	}
	want := headerName(a)
	return slices.IndexFunc(headers, func(h string) bool { return headerName(h) == want })
}

// allIndexes returns the indexes of every alternative the file has, in
// the order given.
func allIndexes(alts []string, headers []string) []int {
	var out []int
	for _, a := range alts {
		if i := singleIndex(a, headers); i >= 0 && !slices.Contains(out, i) {
			out = append(out, i)
		}
	}
	return out
}

func hasNames(alts []string) bool {
	return slices.ContainsFunc(alts, func(a string) bool {
		_, err := strconv.Atoi(strings.TrimSpace(a))
		return err != nil
	})
}

// columns are the indexes of a file's fields; -1 for none.
type columns struct {
	date, amount, debit, credit, externalID, currency, fee, state int
	payee, memo                                                   []int
	headers                                                       []string
}

// squash collapses runs of white space, which exports pad texts with.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func cell(fields []string, i int) string {
	if i < 0 || i >= len(fields) {
		return ""
	}
	return strings.TrimSpace(fields[i])
}

func nonEmpty(fields []string) int {
	n := 0
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			n++
		}
	}
	return n
}

func (p Parser) Parse(data []byte, opts usecase.CSVOptions) (usecase.CSVParseResult, error) {
	r, err := resolve(opts)
	if err != nil {
		return usecase.CSVParseResult{}, err
	}
	text, encoding := decode(data, r.encoding)
	delimiter := r.delimiter
	if delimiter == 0 {
		delimiter = detectDelimiter(text)
	}
	records, rowErrs := readRecords(text, delimiter)
	out := usecase.CSVParseResult{Rows: []usecase.ImportedTransaction{}, Errors: rowErrs, Delimiter: delimiterName(delimiter),
		Encoding: encoding, Headers: []string{}}
	if out.Errors == nil {
		out.Errors = []imports.RowError{}
	}
	start := min(r.skipRows, len(records))
	var headers []string
	if r.hasHeader {
		found := -1
		if hasNames(r.columns.Date) {
			for i := start; i < len(records); i++ {
				if columnIndex(r.columns.Date, records[i].fields) >= 0 && !isNumberRef(r.columns.Date, records[i].fields) {
					found = i
					break
				}
			}
		} else if start < len(records) {
			found = start
		}
		if found < 0 {
			return usecase.CSVParseResult{}, invalid("options.columns.date", "no header row has the date column "+
				strconv.Quote(strings.Join(r.columns.Date, " / ")))
		}
		headers, out.HeaderLine, start = records[found].fields, records[found].line, found+1
		for _, h := range headers {
			out.Headers = append(out.Headers, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "#")))
		}
	}
	cols := columns{
		date: columnIndex(r.columns.Date, headers), amount: columnIndex(r.columns.Amount, headers),
		debit: columnIndex(r.columns.Debit, headers), credit: columnIndex(r.columns.Credit, headers),
		externalID: columnIndex(r.columns.ExternalID, headers), currency: columnIndex(r.columns.Currency, headers),
		fee: columnIndex(r.columns.Fee, headers), state: columnIndex(r.columns.State, headers),
		payee: allIndexes(r.columns.Payee, headers), memo: allIndexes(r.columns.Memo, headers), headers: headers,
	}
	if cols.date < 0 {
		return usecase.CSVParseResult{}, invalid("options.columns.date", "is not a column of the file")
	}
	if cols.amount < 0 && cols.debit < 0 && cols.credit < 0 {
		return usecase.CSVParseResult{}, invalid("options.columns.amount", "is not a column of the file, nor are debit and credit")
	}
	for _, rec := range records[start:] {
		f := rec.fields
		// Blank lines, notes and footers: no row of the statement (AD-813).
		if nonEmpty(f) < 2 || strings.HasPrefix(strings.TrimSpace(f[0]), "#") || cell(f, cols.date) == "" {
			continue
		}
		row, rowErr := r.row(rec, cols)
		if rowErr != nil {
			if len(out.Errors) < maxRowErrorsCSV {
				out.Errors = append(out.Errors, *rowErr)
			}
			continue
		}
		out.Rows = append(out.Rows, row)
		if len(out.Rows) > imports.MaxRows {
			break
		}
	}
	return out, nil
}

// isNumberRef reports whether only a numeric reference matched: a header
// row is found by name.
func isNumberRef(alts []string, fields []string) bool {
	for _, a := range alts {
		if _, err := strconv.Atoi(strings.TrimSpace(a)); err == nil {
			continue
		}
		if singleIndex(a, fields) >= 0 {
			return false
		}
	}
	return true
}

func (r resolved) row(rec record, cols columns) (usecase.ImportedTransaction, *imports.RowError) {
	f := rec.fields
	fail := func(field, msg string) (usecase.ImportedTransaction, *imports.RowError) {
		return usecase.ImportedTransaction{}, &imports.RowError{Row: rec.line, Field: field, Message: msg}
	}
	if cols.state >= 0 && len(r.acceptStates) > 0 {
		state := cell(f, cols.state)
		if !slices.ContainsFunc(r.acceptStates, func(s string) bool { return strings.EqualFold(s, state) }) {
			return fail("state", "is "+strconv.Quote(state)+"; only "+strings.Join(r.acceptStates, ", ")+" rows are imported")
		}
	}
	date, err := parseDate(cell(f, cols.date), r.dateLayout)
	if err != nil {
		return fail("date", "is not a date: "+strconv.Quote(cell(f, cols.date)))
	}
	amount, field, msg := r.amount(f, cols)
	if msg != "" {
		return fail(field, msg)
	}
	payee := ""
	for _, i := range cols.payee {
		if v := squash(cell(f, i)); v != "" {
			payee = v
			break
		}
	}
	var memo []string
	for _, i := range cols.memo {
		for j := i; j < len(f); j++ {
			if j > i && (j >= len(cols.headers) || strings.TrimSpace(cols.headers[j]) != "" || len(cols.headers) == 0) {
				break
			}
			if v := squash(cell(f, j)); v != "" && !strings.EqualFold(v, payee) && !slices.Contains(memo, v) {
				memo = append(memo, v)
			}
		}
	}
	return usecase.ImportedTransaction{Row: rec.line, ExternalID: cell(f, cols.externalID), Date: date, Amount: amount,
		Currency: cell(f, cols.currency), PayeeRaw: payee, Memo: strings.Join(memo, " · ")}, nil
}

// amount is the row's amount as a decimal with a point, or the field and
// message of what is wrong.
func (r resolved) amount(f []string, cols columns) (string, string, string) {
	var total *big.Rat
	scale := 0
	add := func(field, raw string, sign int) string {
		v, places, ok := parseNumber(raw, r.decimal)
		if !ok {
			return "is not a number: " + strconv.Quote(raw)
		}
		switch sign {
		case 1:
			v.Abs(v)
		case -1:
			v.Abs(v)
			v.Neg(v)
		}
		if total == nil {
			total = new(big.Rat)
		}
		total.Add(total, v)
		scale = max(scale, places)
		return ""
	}
	if cols.amount >= 0 {
		raw := cell(f, cols.amount)
		if raw == "" {
			return "", "amount", "is empty"
		}
		if msg := add("amount", raw, 0); msg != "" {
			return "", "amount", msg
		}
	} else {
		debit, credit := cell(f, cols.debit), cell(f, cols.credit)
		if debit == "" && credit == "" {
			return "", "amount", "is empty: neither debit nor credit is given"
		}
		if credit != "" {
			if msg := add("credit", credit, 1); msg != "" {
				return "", "credit", msg
			}
		}
		if debit != "" {
			if msg := add("debit", debit, -1); msg != "" {
				return "", "debit", msg
			}
		}
	}
	if fee := cell(f, cols.fee); cols.fee >= 0 && fee != "" {
		if msg := add("fee", fee, -1); msg != "" {
			return "", "fee", msg
		}
	}
	if r.invert {
		total.Neg(total)
	}
	return total.FloatString(scale), "", ""
}

// parseNumber reads an amount as banks write it: thousands separated by
// spaces, non-breaking spaces, apostrophes or the other separator, a comma
// or a point before the decimals, a currency code or symbol around, a
// minus before or after, or parentheses for a negative amount.
func parseNumber(raw string, decimal byte) (*big.Rat, int, bool) {
	s := strings.TrimSpace(raw)
	neg := strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c == ',', c == '.':
			b.WriteRune(c)
		case c == '-' || c == '−' || c == '–':
			neg = !neg
		case c == '+', unicode.IsSpace(c), c == '\'', c == ' ', c == '(', c == ')', unicode.IsLetter(c), unicode.IsSymbol(c):
		default:
			return nil, 0, false
		}
	}
	digits := b.String()
	if digits == "" {
		return nil, 0, false
	}
	if decimal == 0 {
		decimal = detectDecimal(digits)
	}
	thousands := ","
	if decimal == ',' {
		thousands = "."
	}
	digits = strings.ReplaceAll(digits, thousands, "")
	if decimal == ',' {
		digits = strings.Replace(digits, ",", ".", 1)
	}
	whole, frac, _ := strings.Cut(digits, ".")
	if whole == "" {
		whole = "0"
	}
	if strings.ContainsAny(frac, ".,") || strings.ContainsAny(whole, ".,") {
		return nil, 0, false
	}
	v, ok := new(big.Rat).SetString(whole + "." + cmpOr(frac, "0"))
	if !ok {
		return nil, 0, false
	}
	if neg {
		v.Neg(v)
	}
	return v, len(frac), true
}

func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// detectDecimal guesses the decimal separator: with both, the later one;
// with one that occurs once, that one; otherwise none (thousands only).
func detectDecimal(digits string) byte {
	comma, dot := strings.LastIndexByte(digits, ','), strings.LastIndexByte(digits, '.')
	switch {
	case comma >= 0 && dot >= 0:
		if comma > dot {
			return ','
		}
		return '.'
	case comma >= 0 && strings.Count(digits, ",") == 1:
		return ','
	case dot >= 0 && strings.Count(digits, ".") == 1:
		return '.'
	case comma >= 0:
		return '.' // commas are thousands
	}
	return ','
}

// dateLayout turns a format of tokens into a Go layout.
func dateLayout(format string) (string, error) {
	tokens := []struct{ token, layout string }{
		{"YYYY", "2006"}, {"YY", "06"}, {"MM", "01"}, {"DD", "02"}, {"HH", "15"}, {"mm", "04"}, {"ss", "05"}, {"M", "1"}, {"D", "2"},
	}
	var b strings.Builder
	seen := map[string]bool{}
	for i := 0; i < len(format); {
		matched := false
		for _, t := range tokens {
			if strings.HasPrefix(format[i:], t.token) {
				b.WriteString(t.layout)
				seen[t.token[:1]] = true
				i += len(t.token)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		c := format[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return "", errors.New("may hold only YYYY, YY, MM, M, DD, D, HH, mm, ss and separators")
		}
		b.WriteByte(c)
		i++
	}
	if !seen["Y"] || !seen["M"] || !seen["D"] {
		return "", errors.New("needs a year, a month and a day (YYYY, MM, DD)")
	}
	return b.String(), nil
}

// autoLayouts are tried, in order, when no date format is given.
var autoLayouts = []string{"2006-01-02", "02.01.2006", "02-01-2006", "02/01/2006", "2006/01/02", "2006.01.02", "2.1.2006"}

func parseDate(raw, layout string) (domain.Date, error) {
	layouts := autoLayouts
	if layout != "" {
		layouts = []string{layout}
	}
	for _, l := range layouts {
		v := raw
		// A time after the date is dropped when the layout has none.
		if !strings.ContainsAny(l, " T") {
			if i := strings.IndexAny(v, " T"); i > 0 {
				v = v[:i]
			}
		}
		if t, err := time.Parse(l, v); err == nil {
			return domain.NewDate(t.Year(), t.Month(), t.Day()), nil
		}
	}
	return domain.Date{}, errors.New("not a date")
}
