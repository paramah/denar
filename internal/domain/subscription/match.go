package subscription

import (
	"math/big"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
)

// Weights of MatchScore; they add up to 100.
const (
	WeightAmount = 40
	WeightDate   = 30
	WeightPayee  = 20
	WeightWallet = 10
)

// AutoMatchThreshold is the score from which a match may be made without a
// person (an import).
const AutoMatchThreshold = 80

// Candidate is a transaction as MatchScore sees it.
type Candidate struct {
	WalletID domain.ID
	Date     domain.Date
	// Amount is what was paid, > 0, in the transaction's currency.
	Amount int64
	// Expected is the charge's amount in the transaction's currency; zero
	// when unknown (no exchange rate).
	Expected int64
	// Text is the payee's name and the note.
	Text string
}

// MatchScore rates how well the candidate pays the charge, 0 to 100: the
// amount within the tolerance (40, falling to 20 at its edge), the date
// within the window (30, falling to 15 at its edge), the payee pattern
// (20) and the subscription's wallet (10). ok is false when the amount is
// outside the tolerance or the date outside the window: not a candidate.
func MatchScore(s Subscription, c Charge, x Candidate) (int, bool) {
	if x.Amount <= 0 {
		return 0, false
	}
	score := 0
	if x.Expected > 0 {
		points, ok := amountPoints(x.Amount, x.Expected, s.Match.AmountTolerancePct)
		if !ok {
			return 0, false
		}
		score += points
	}
	days := daysApart(x.Date, c.DueDate)
	window := s.Match.DateWindowDays
	if days > window {
		return 0, false
	}
	if window == 0 {
		score += WeightDate
	} else {
		score += WeightDate - WeightDate/2*days/window
	}
	if payeeMatches(s, x.Text) {
		score += WeightPayee
	}
	if s.WalletID != "" && s.WalletID == x.WalletID {
		score += WeightWallet
	}
	return score, true
}

// amountPoints compares in basis points with big integers, as amounts
// times 10000 may not fit in int64.
func amountPoints(actual, expected int64, tolerancePct int) (int, bool) {
	diff := new(big.Int).Sub(big.NewInt(actual), big.NewInt(expected))
	diff.Abs(diff)
	if diff.Sign() == 0 {
		return WeightAmount, true
	}
	if tolerancePct == 0 {
		return 0, false
	}
	// diff / expected ≤ tol / 100  ⇔  diff·100 ≤ tol·expected
	lhs := new(big.Int).Mul(diff, big.NewInt(100))
	rhs := new(big.Int).Mul(big.NewInt(int64(tolerancePct)), big.NewInt(expected))
	if lhs.Cmp(rhs) > 0 {
		return 0, false
	}
	// lost = ⌊20 · diff·100 / (tol·expected)⌋
	lost := new(big.Int).Mul(lhs, big.NewInt(WeightAmount/2))
	lost.Quo(lost, rhs)
	return WeightAmount - int(lost.Int64()), true
}

func daysApart(a, b domain.Date) int {
	d := int(a.Time().Sub(b.Time()) / (24 * time.Hour))
	if d < 0 {
		d = -d
	}
	return d
}

// patterns are what a matching payee or note contains: the rule's
// pattern, or else the vendor's and the subscription's names.
func (s Subscription) patterns() []string {
	if s.Match.PayeePattern != "" {
		return []string{s.Match.PayeePattern}
	}
	out := []string{s.Name}
	if s.Vendor != VendorOther {
		out = append(out, VendorName(s.Vendor))
	}
	return out
}

func payeeMatches(s Subscription, text string) bool {
	text = strings.ToLower(text)
	for _, p := range s.patterns() {
		if p != "" && strings.Contains(text, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
