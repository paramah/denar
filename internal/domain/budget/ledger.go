package budget

import (
	"errors"
	"maps"
	"slices"

	"github.com/paramah/denar/internal/domain"
)

// ErrOverflow: a sum of the budget does not fit in int64 minor units.
var ErrOverflow = errors.New("budget: amount out of range")

// Ledger collects what a budget is computed from: the assignments of the
// envelopes, the activity of the envelopes (spending and refunds of money
// in envelopes) and the inflow to Ready to Assign (income, money spent
// without an envelope, transfer legs without one), all by budget month and
// in minor units of the base currency.
type Ledger struct {
	assigned map[domain.ID]map[Month]int64
	activity map[domain.ID]map[Month]int64
	inflow   map[Month]int64
	err      error
}

func NewLedger() *Ledger {
	return &Ledger{assigned: map[domain.ID]map[Month]int64{}, activity: map[domain.ID]map[Month]int64{}, inflow: map[Month]int64{}}
}

func (l *Ledger) add(dst *int64, v int64) {
	sum := *dst + v
	if (v > 0 && sum < *dst) || (v < 0 && sum > *dst) {
		l.err = ErrOverflow
		return
	}
	*dst = sum
}

func cell(m map[domain.ID]map[Month]int64, env domain.ID) map[Month]int64 {
	c, ok := m[env]
	if !ok {
		c = map[Month]int64{}
		m[env] = c
	}
	return c
}

// SetAssigned sets the assignment of the envelope for the month.
func (l *Ledger) SetAssigned(env domain.ID, m Month, amount int64) {
	cell(l.assigned, env)[m] = amount
	cell(l.activity, env) // the envelope is known
}

// AddActivity adds to the activity of the envelope in the month.
func (l *Ledger) AddActivity(env domain.ID, m Month, amount int64) {
	c := cell(l.activity, env)
	v := c[m]
	l.add(&v, amount)
	c[m] = v
}

// AddInflow adds to the inflow to Ready to Assign in the month.
func (l *Ledger) AddInflow(m Month, amount int64) {
	v := l.inflow[m]
	l.add(&v, amount)
	l.inflow[m] = v
}

// EnvelopeMonth is one envelope in one month.
type EnvelopeMonth struct {
	Assigned int64
	Activity int64
	// CarriedIn is what the envelope brings from the month before: its
	// available money, never less than zero (overspending is not carried).
	CarriedIn int64
	// Available = CarriedIn + Assigned + Activity; below zero the envelope
	// is overspent.
	Available int64
}

// MonthSummary is the budget of one month.
type MonthSummary struct {
	Month     Month
	Envelopes map[domain.ID]EnvelopeMonth
	// Inflow is what reached Ready to Assign in this month.
	Inflow int64
	// ReadyToAssign is the money not yet given to envelopes: all inflow up
	// to this month, less everything assigned up to this month, less the
	// overspending of earlier months, less what is assigned in later
	// months.
	ReadyToAssign    int64
	AssignedInFuture int64
	// OverspentLastMonth is the overspending of the month before (≤ 0),
	// which this month's Ready to Assign carries.
	OverspentLastMonth int64
	// Totals of the envelopes in this month.
	Assigned, Activity, Available int64
}

// Month computes the budget of month m. Overspending (an envelope whose
// available money is below zero at the end of a month) is not carried in
// the envelope: it starts the next month at zero and the next month's
// Ready to Assign is lower by as much (the YNAB rule).
//
// For every month the result keeps the invariant
//
//	Σ Available + ReadyToAssign + AssignedInFuture = Σ inflow and activity up to m
func (l *Ledger) Month(m Month) (MonthSummary, error) {
	if l.err != nil {
		return MonthSummary{}, l.err
	}
	set := map[Month]bool{m: true}
	for k := range l.inflow {
		set[k] = true
	}
	envs := slices.Sorted(maps.Keys(l.activity))
	for _, e := range envs {
		for k := range l.assigned[e] {
			set[k] = true
		}
		for k := range l.activity[e] {
			set[k] = true
		}
	}
	months := slices.SortedFunc(maps.Keys(set), Month.Compare)

	out := MonthSummary{Month: m, Envelopes: make(map[domain.ID]EnvelopeMonth, len(envs))}
	carry := map[domain.ID]int64{}
	var inflow, assigned, overspentBefore int64
	for _, k := range months {
		if k.After(m) {
			for _, e := range envs {
				l.add(&out.AssignedInFuture, l.assigned[e][k])
			}
			continue
		}
		l.add(&inflow, l.inflow[k])
		var overspent int64
		for _, e := range envs {
			a, hasA := l.assigned[e][k]
			act, hasAct := l.activity[e][k]
			if !hasA && !hasAct {
				continue // available stays the carry, which is never below zero
			}
			avail := carry[e]
			l.add(&avail, a)
			l.add(&avail, act)
			l.add(&assigned, a)
			if k == m {
				out.Envelopes[e] = EnvelopeMonth{Assigned: a, Activity: act, CarriedIn: carry[e], Available: avail}
			}
			if avail < 0 {
				l.add(&overspent, avail)
				carry[e] = 0
			} else {
				carry[e] = avail
			}
		}
		if k.Before(m) {
			l.add(&overspentBefore, overspent)
		}
		if k == m.Prev() {
			out.OverspentLastMonth = overspent
		}
	}
	for _, e := range envs {
		if _, ok := out.Envelopes[e]; !ok {
			out.Envelopes[e] = EnvelopeMonth{CarriedIn: carry[e], Available: carry[e]}
		}
		em := out.Envelopes[e]
		l.add(&out.Assigned, em.Assigned)
		l.add(&out.Activity, em.Activity)
		l.add(&out.Available, em.Available)
	}
	out.Inflow = l.inflow[m]
	rta := inflow
	l.add(&rta, -assigned)
	l.add(&rta, overspentBefore)
	l.add(&rta, -out.AssignedInFuture)
	out.ReadyToAssign = rta
	if l.err != nil {
		return MonthSummary{}, l.err
	}
	return out, nil
}
