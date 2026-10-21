package main

import (
	"testing"
)

type budgetEnvelope struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Assigned    money  `json:"assigned"`
	Activity    money  `json:"activity"`
	CarriedOver money  `json:"carried_over"`
	Available   money  `json:"available"`
	GoalStatus  *struct {
		Underfunded money `json:"underfunded"`
		Funded      bool  `json:"funded"`
	} `json:"goal_status"`
}

type budgetMonth struct {
	Month              string `json:"month"`
	ReadyToAssign      money  `json:"ready_to_assign"`
	AssignedInFuture   money  `json:"assigned_in_future"`
	OverspentLastMonth money  `json:"overspent_last_month"`
	Income             money  `json:"income"`
	Available          money  `json:"available"`
	Incomplete         bool   `json:"incomplete"`
	Groups             []struct {
		Name      string           `json:"name"`
		Envelopes []budgetEnvelope `json:"envelopes"`
	} `json:"groups"`
}

func (m budgetMonth) envelope(t *testing.T, name string) budgetEnvelope {
	t.Helper()
	for _, g := range m.Groups {
		for _, e := range g.Envelopes {
			if e.Name == name {
				return e
			}
		}
	}
	t.Fatalf("no envelope %s in %s", name, m.Month)
	return budgetEnvelope{}
}

// TestBudgetEndToEnd is the walk of stage 003: income, envelopes and
// goals, assignments, spending with overspending, the next month, a move,
// auto-assign, a transfer out of the budget, and the invariant against the
// wallet's balance.
func TestBudgetEndToEnd(t *testing.T) {
	setEnv(t, "DENAR_AUTO_MIGRATE", "true")
	e := startE2E(t)
	admin := e.browser()
	admin.must(200, "POST", "/api/auth/login", `{"email":"`+adminEmail+`","password":"`+adminPassword+`"}`, nil)
	var h household
	admin.must(201, "POST", "/api/households", `{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}`, &h)
	api := "/api/households/" + h.ID

	var konto, lokata wallet
	admin.must(201, "POST", api+"/wallets", `{"name":"Konto","type":"checking","currency":"PLN",
		"opening_balance":{"amount":"3000.00","currency":"PLN"},"opened_on":"2026-09-01"}`, &konto)
	admin.must(201, "POST", api+"/wallets", `{"name":"Lokata","type":"savings","currency":"PLN","on_budget":false}`, &lokata)
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+konto.ID+`","date":"2026-09-10","amount":{"amount":"2000.00","currency":"PLN"},"kind":"income"}`, nil)

	var daily, goals struct {
		ID string `json:"id"`
	}
	admin.must(201, "POST", api+"/budget/envelope-groups", `{"name":"Codzienne"}`, &daily)
	admin.must(201, "POST", api+"/budget/envelope-groups", `{"name":"Cele","position":1}`, &goals)
	envelope := func(body string) string {
		t.Helper()
		var out struct {
			ID string `json:"id"`
		}
		admin.must(201, "POST", api+"/budget/envelopes", body, &out)
		return out.ID
	}
	food := envelope(`{"group_id":"` + daily.ID + `","name":"Jedzenie","goal":{"type":"monthly","amount":{"amount":"800.00","currency":"PLN"}}}`)
	fun := envelope(`{"group_id":"` + daily.ID + `","name":"Rozrywka","position":1}`)
	trip := envelope(`{"group_id":"` + goals.ID + `","name":"Wakacje","goal":{"type":"target_by_date","amount":{"amount":"1200.00","currency":"PLN"},"target_date":"2027-02-28"}}`)

	months := api + "/budget/months/"
	admin.must(200, "PUT", months+"2026-09/assignments/"+food, `{"amount":{"amount":"800.00","currency":"PLN"}}`, nil)
	admin.must(200, "PUT", months+"2026-09/assignments/"+fun, `{"amount":{"amount":"200.00","currency":"PLN"}}`, nil)
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+konto.ID+`","date":"2026-09-15","amount":{"amount":"-950.00","currency":"PLN"},"envelope_id":"`+food+`"}`, nil)
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+konto.ID+`","date":"2026-09-16","amount":{"amount":"-50.00","currency":"PLN"},"envelope_id":"`+fun+`"}`, nil)
	// A payee's default envelope categorises an expense without one.
	admin.must(201, "POST", api+"/payees", `{"name":"Kino","default_envelope_id":"`+fun+`"}`, nil)
	var cinema txn
	admin.must(201, "POST", api+"/transactions", `{"wallet_id":"`+konto.ID+`","date":"2026-09-20","amount":{"amount":"-30.00","currency":"PLN"},"payee_name":"Kino"}`, &cinema)
	admin.must(422, "POST", api+"/transactions", `{"wallet_id":"`+konto.ID+`","amount":{"amount":"-1","currency":"PLN"},"envelope_id":"nope"}`, nil)

	var sep budgetMonth
	admin.must(200, "GET", months+"2026-09", "", &sep)
	if f := sep.envelope(t, "Jedzenie"); f.Available.Amount != "-150.00" || f.Activity.Amount != "-950.00" {
		t.Errorf("September food = %+v", f)
	}
	if f := sep.envelope(t, "Rozrywka"); f.Available.Amount != "120.00" || f.Activity.Amount != "-80.00" {
		t.Errorf("September fun = %+v", f)
	}
	if sep.ReadyToAssign.Amount != "4000.00" || sep.Income.Amount != "5000.00" || sep.ReadyToAssign.Currency != "PLN" {
		t.Errorf("September RTA = %+v, income %+v", sep.ReadyToAssign, sep.Income)
	}

	// October: food starts at zero and its overspending comes out of Ready
	// to Assign; fun carries its 120.
	var oct budgetMonth
	admin.must(200, "GET", months+"2026-10", "", &oct)
	if oct.OverspentLastMonth.Amount != "-150.00" || oct.ReadyToAssign.Amount != "3850.00" {
		t.Errorf("October RTA = %s, overspent %s", oct.ReadyToAssign.Amount, oct.OverspentLastMonth.Amount)
	}
	if f := oct.envelope(t, "Jedzenie"); f.Available.Amount != "0.00" || f.CarriedOver.Amount != "0.00" {
		t.Errorf("October food = %+v", f)
	}
	if f := oct.envelope(t, "Rozrywka"); f.CarriedOver.Amount != "120.00" {
		t.Errorf("October fun = %+v", f)
	}

	admin.must(200, "POST", months+"2026-10/moves", `{"from_envelope_id":"`+fun+`","to_envelope_id":"`+food+`","amount":{"amount":"20.00","currency":"PLN"}}`, &oct)
	if oct.envelope(t, "Jedzenie").Assigned.Amount != "20.00" || oct.envelope(t, "Rozrywka").Available.Amount != "100.00" {
		t.Errorf("after the move = %+v", oct)
	}

	// Auto-assign: food needs 780 more, the trip 1200 over five months.
	var auto struct {
		Assigned []struct {
			EnvelopeID string `json:"envelope_id"`
			Amount     money  `json:"amount"`
		} `json:"assigned"`
		Month budgetMonth `json:"month"`
	}
	admin.must(200, "POST", months+"2026-10/auto-assign", "", &auto)
	if len(auto.Assigned) != 2 || auto.Assigned[0].EnvelopeID != food || auto.Assigned[0].Amount.Amount != "780.00" ||
		auto.Assigned[1].EnvelopeID != trip || auto.Assigned[1].Amount.Amount != "240.00" {
		t.Errorf("auto-assign = %+v", auto.Assigned)
	}
	if auto.Month.ReadyToAssign.Amount != "2830.00" || !auto.Month.envelope(t, "Jedzenie").GoalStatus.Funded {
		t.Errorf("after auto-assign = %+v", auto.Month)
	}

	// Money moved out of the budget to savings, for the trip.
	admin.must(201, "POST", api+"/transfers", `{"from_wallet_id":"`+konto.ID+`","to_wallet_id":"`+lokata.ID+`","date":"2026-10-20",
		"amount":{"amount":"200.00","currency":"PLN"},"envelope_id":"`+trip+`"}`, nil)
	admin.must(200, "GET", months+"2026-10", "", &oct)
	if f := oct.envelope(t, "Wakacje"); f.Available.Amount != "40.00" || f.Activity.Amount != "-200.00" {
		t.Errorf("trip = %+v", f)
	}

	// The invariant: envelopes plus Ready to Assign plus later assignments
	// are the on-budget balance.
	var list struct {
		Items []wallet `json:"items"`
	}
	admin.must(200, "GET", api+"/wallets", "", &list)
	var onBudget int64
	for _, w := range list.Items {
		if w.ID == konto.ID {
			onBudget = w.BaseBalance.Minor
		}
	}
	if got := oct.Available.Minor + oct.ReadyToAssign.Minor + oct.AssignedInFuture.Minor; got != onBudget || onBudget != 377000 {
		t.Errorf("Σ available %d + RTA %d + future %d = %d, on-budget balance %d", oct.Available.Minor, oct.ReadyToAssign.Minor,
			oct.AssignedInFuture.Minor, got, onBudget)
	}

	var activity struct {
		Items []txn `json:"items"`
	}
	admin.must(200, "GET", months+"2026-09/envelopes/"+fun+"/transactions", "", &activity)
	if len(activity.Items) != 2 || activity.Items[0].ID != cinema.ID {
		t.Errorf("fun's September = %+v", activity.Items)
	}

	admin.must(409, "DELETE", api+"/budget/envelopes/"+food, "", nil)
	admin.must(409, "DELETE", api+"/budget/envelope-groups/"+daily.ID, "", nil)
	admin.must(200, "PATCH", api+"/budget/envelopes/"+food, `{"hidden":true}`, nil)

	// A bot that reads the budget sees it, but changes nothing and lists
	// no transactions without transactions:read.
	var created botWithToken
	admin.must(201, "POST", api+"/bots", `{"name":"Asystent","scopes":["budget:read"]}`, &created)
	bot := e.bot(created.Token)
	var seen budgetMonth
	bot.must(200, "GET", months+"2026-10", "", &seen)
	if seen.ReadyToAssign != oct.ReadyToAssign {
		t.Errorf("bot's RTA = %+v", seen.ReadyToAssign)
	}
	bot.must(403, "PUT", months+"2026-10/assignments/"+fun, `{"amount":{"amount":"1","currency":"PLN"}}`, nil)
	bot.must(403, "GET", months+"2026-10/envelopes/"+fun+"/transactions", "", nil)
}
