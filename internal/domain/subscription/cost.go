package subscription

import (
	"math/big"

	"git.eiac.dev/tools/denar/internal/domain/money"
)

// Cost is what a subscription costs per month and per year on average, in
// minor units of one currency.
type Cost struct {
	Monthly int64
	Yearly  int64
}

// CostOf normalises a price of a cycle into the currency to, at the rate
// (one unit of the price's currency is rate units of to): yearly = amount ×
// terms per year × rate, monthly = yearly / 12, each rounded once, half to
// even.
func CostOf(p PricePoint, c Cycle, rate *big.Rat, to money.Currency) (Cost, error) {
	from, ok := money.LookupCurrency(p.Currency)
	if !ok {
		return Cost{}, money.ErrCurrencyMismatch
	}
	yearly := new(big.Rat).SetInt64(p.Amount)
	yearly.Mul(yearly, c.PerYear())
	yearly.Mul(yearly, rate)
	exp := to.Decimals - from.Decimals
	pow := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(exp))), nil))
	if exp >= 0 {
		yearly.Mul(yearly, pow)
	} else {
		yearly.Quo(yearly, pow)
	}
	y, err := money.RoundHalfEven(yearly)
	if err != nil {
		return Cost{}, err
	}
	m, err := money.RoundHalfEven(new(big.Rat).Quo(yearly, big.NewRat(12, 1)))
	if err != nil {
		return Cost{}, err
	}
	return Cost{Monthly: m, Yearly: y}, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
