package budget

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/paramah/denar/internal/domain"
)

type op struct {
	kind   string // assign, act, in
	env    domain.ID
	month  string
	amount int64
}

func ledgerOf(ops ...op) *Ledger {
	l := NewLedger()
	for _, o := range ops {
		switch o.kind {
		case "assign":
			l.SetAssigned(o.env, month(o.month), o.amount)
		case "act":
			l.AddActivity(o.env, month(o.month), o.amount)
		case "in":
			l.AddInflow(month(o.month), o.amount)
		}
	}
	return l
}

func TestLedgerMonth(t *testing.T) {
	type want struct {
		envs                          map[domain.ID]EnvelopeMonth
		rta, future, overspent, inflo int64
	}
	tests := []struct {
		name  string
		ops   []op
		month string
		want  want
	}{
		{
			name:  "empty",
			month: "2026-10",
			want:  want{envs: map[domain.ID]EnvelopeMonth{}},
		},
		{
			name: "income assigned and spent",
			ops: []op{{"in", "", "2026-10", 500000}, {"assign", "food", "2026-10", 100000}, {"act", "food", "2026-10", -30000},
				{"assign", "rent", "2026-10", 250000}, {"act", "rent", "2026-10", -250000}},
			month: "2026-10",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"food": {Assigned: 100000, Activity: -30000, Available: 70000},
				"rent": {Assigned: 250000, Activity: -250000, Available: 0},
			}, rta: 150000, inflo: 500000},
		},
		{
			name: "carry over several months, gap included",
			ops: []op{{"in", "", "2026-08", 100000}, {"assign", "food", "2026-08", 50000}, {"act", "food", "2026-08", -10000},
				{"act", "food", "2026-10", -5000}},
			month: "2026-11",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"food": {CarriedIn: 35000, Available: 35000},
			}, rta: 50000},
		},
		{
			name: "overspending resets the envelope and lowers next month's RTA",
			ops: []op{{"in", "", "2026-10", 100000}, {"assign", "food", "2026-10", 20000}, {"act", "food", "2026-10", -35000},
				{"assign", "fun", "2026-10", 10000}},
			month: "2026-11",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"food": {CarriedIn: 0, Available: 0},
				"fun":  {CarriedIn: 10000, Available: 10000},
			}, rta: 100000 - 30000 - 15000, overspent: -15000},
		},
		{
			name:  "overspent month itself shows the negative envelope, RTA not yet lowered",
			ops:   []op{{"in", "", "2026-10", 100000}, {"assign", "food", "2026-10", 20000}, {"act", "food", "2026-10", -35000}},
			month: "2026-10",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"food": {Assigned: 20000, Activity: -35000, Available: -15000},
			}, rta: 80000, inflo: 100000},
		},
		{
			name:  "overspending two months back is not last month's",
			ops:   []op{{"act", "food", "2026-09", -1000}},
			month: "2026-11",
			want:  want{envs: map[domain.ID]EnvelopeMonth{"food": {}}, rta: -1000},
		},
		{
			name: "future assignments lower RTA now and count as assigned in future",
			ops: []op{{"in", "", "2026-10", 100000}, {"assign", "trip", "2026-10", 10000}, {"assign", "trip", "2026-12", 40000},
				{"assign", "food", "2027-01", 5000}},
			month: "2026-10",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"trip": {Assigned: 10000, Available: 10000},
				"food": {},
			}, rta: 100000 - 10000 - 45000, future: 45000, inflo: 100000},
		},
		{
			name:  "future month sees the future assignment as its own",
			ops:   []op{{"in", "", "2026-10", 100000}, {"assign", "trip", "2026-10", 10000}, {"assign", "trip", "2026-12", 40000}},
			month: "2026-12",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"trip": {Assigned: 40000, CarriedIn: 10000, Available: 50000},
			}, rta: 50000},
		},
		{
			name: "refund and negative assignment",
			ops: []op{{"in", "", "2026-10", 1000}, {"assign", "food", "2026-10", 5000}, {"act", "food", "2026-10", 2000},
				{"assign", "food", "2026-11", -6000}},
			month: "2026-11",
			want: want{envs: map[domain.ID]EnvelopeMonth{
				"food": {Assigned: -6000, CarriedIn: 7000, Available: 1000},
			}, rta: 1000 - 5000 + 6000},
		},
		{
			name:  "uncategorised spending lowers RTA directly",
			ops:   []op{{"in", "", "2026-10", 10000}, {"in", "", "2026-10", -2500}},
			month: "2026-10",
			want:  want{envs: map[domain.ID]EnvelopeMonth{}, rta: 7500, inflo: 7500},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ledgerOf(tt.ops...).Month(month(tt.month))
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Envelopes) != len(tt.want.envs) {
				t.Errorf("envelopes = %+v, want %+v", s.Envelopes, tt.want.envs)
			}
			var assigned, activity, available int64
			for id, w := range tt.want.envs {
				if s.Envelopes[id] != w {
					t.Errorf("%s = %+v, want %+v", id, s.Envelopes[id], w)
				}
				assigned, activity, available = assigned+w.Assigned, activity+w.Activity, available+w.Available
			}
			if s.ReadyToAssign != tt.want.rta || s.AssignedInFuture != tt.want.future || s.OverspentLastMonth != tt.want.overspent ||
				s.Inflow != tt.want.inflo {
				t.Errorf("rta %d future %d overspent %d inflow %d, want %+v", s.ReadyToAssign, s.AssignedInFuture,
					s.OverspentLastMonth, s.Inflow, tt.want)
			}
			if s.Assigned != assigned || s.Activity != activity || s.Available != available {
				t.Errorf("totals = %d %d %d", s.Assigned, s.Activity, s.Available)
			}
		})
	}
}

func TestLedgerOverflow(t *testing.T) {
	l := NewLedger()
	l.AddInflow(month("2026-10"), math.MaxInt64)
	l.AddInflow(month("2026-10"), 1)
	if _, err := l.Month(month("2026-10")); err != ErrOverflow {
		t.Errorf("err = %v", err)
	}
	l = NewLedger()
	l.AddInflow(month("2026-10"), math.MaxInt64)
	l.AddInflow(month("2026-11"), math.MaxInt64)
	if _, err := l.Month(month("2026-11")); err != ErrOverflow {
		t.Errorf("err = %v", err)
	}
}

// TestInvariantRandom checks, for random budgets, that the envelopes, Ready
// to Assign and the future assignments add up to all the money that came
// in up to the month.
func TestInvariantRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 7))
	envs := []domain.ID{"a", "b", "c", "d"}
	base := month("2026-01")
	for round := range 300 {
		l := NewLedger()
		total := map[Month]int64{}
		for range r.IntN(40) {
			m := NewMonth(2026, base.m+time.Month(r.IntN(12)))
			amount := r.Int64N(200000) - 100000
			switch r.IntN(3) {
			case 0:
				l.SetAssigned(envs[r.IntN(len(envs))], m, amount)
			case 1:
				l.AddActivity(envs[r.IntN(len(envs))], m, amount)
				total[m] += amount
			default:
				l.AddInflow(m, amount)
				total[m] += amount
			}
		}
		for i := range 13 {
			m := NewMonth(2026, time.Month(1+i))
			s, err := l.Month(m)
			if err != nil {
				t.Fatal(err)
			}
			var money int64
			for k, v := range total {
				if !k.After(m) {
					money += v
				}
			}
			var avail int64
			for _, e := range s.Envelopes {
				avail += e.Available
				if e.CarriedIn < 0 {
					t.Fatalf("round %d: negative carry %+v", round, e)
				}
			}
			if avail+s.ReadyToAssign+s.AssignedInFuture != money || avail != s.Available {
				t.Fatalf("round %d month %s: Σ available %d + RTA %d + future %d != %d", round, m, avail, s.ReadyToAssign,
					s.AssignedInFuture, money)
			}
		}
	}
}
