package usecase_test

import (
	"slices"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/budget"
	"github.com/paramah/denar/internal/domain/wallet"
	"github.com/paramah/denar/internal/usecase"
)

// budgetFamily is fxFamily with a group "Codzienne" holding envelopes
// food "Jedzenie" and fun "Rozrywka", by the member.
func budgetFamily(t *testing.T) *family {
	f := fxFamily(t)
	g, err := usecase.NewCreateEnvelopeGroup(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "Codzienne"})
	f.must(err)
	f.vars["group"] = g.ID
	for _, e := range []struct{ key, name string }{{"food", "Jedzenie"}, {"fun", "Rozrywka"}} {
		env, err := usecase.NewCreateEnvelope(f.deps).Do(ctx, human(f.member), "h1",
			usecase.CreateEnvelopeInput{GroupID: string(g.ID), Name: e.name, Position: len(f.vars)})
		f.must(err)
		f.vars[e.key] = env.ID
	}
	return f
}

func ids[T any](items []T, id func(T) domain.ID) []domain.ID {
	out := []domain.ID{}
	for _, it := range items {
		out = append(out, id(it))
	}
	return out
}

func TestEnvelopeGroups(t *testing.T) {
	f := fxFamily(t)
	create := usecase.NewCreateEnvelopeGroup(f.deps)
	g, err := create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: " Rachunki ", Position: 2})
	noErr(t, err)
	if g.Name != "Rachunki" || g.Position != 2 || g.HouseholdID != "h1" {
		t.Errorf("group = %+v", g)
	}
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "rachunki"})
	wantConflict(t, err, domain.ConflictDuplicateName)
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "", Position: -1})
	wantField(t, err, "name")
	wantField(t, err, "position")
	other, err := create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "Cele"})
	noErr(t, err)

	list, err := usecase.NewListEnvelopeGroups(f.deps).Do(ctx, human(f.viewer), "h1")
	noErr(t, err)
	if !slices.Equal(ids(list, func(g budget.Group) domain.ID { return g.ID }), []domain.ID{other.ID, "g-h1", g.ID}) {
		t.Errorf("list = %+v", list)
	}

	update := usecase.NewUpdateEnvelopeGroup(f.deps)
	up, err := update.Do(ctx, human(f.member), "h1", g.ID, usecase.UpdateEnvelopeGroupInput{Name: domain.Some("Opłaty"), Position: domain.Some(0)})
	noErr(t, err)
	if up.Name != "Opłaty" || up.Position != 0 {
		t.Errorf("updated = %+v", up)
	}
	_, err = update.Do(ctx, human(f.member), "h1", g.ID, usecase.UpdateEnvelopeGroupInput{Name: domain.Some("CELE")})
	wantConflict(t, err, domain.ConflictDuplicateName)
	_, err = update.Do(ctx, human(f.outsider), "h1", g.ID, usecase.UpdateEnvelopeGroupInput{})
	wantErr(t, err, domain.ErrNotFound)
	_, err = update.Do(ctx, human(f.member), "h1", "nope", usecase.UpdateEnvelopeGroupInput{})
	wantErr(t, err, domain.ErrNotFound)
	_, err = update.Do(ctx, human(f.viewer), "h1", g.ID, usecase.UpdateEnvelopeGroupInput{Name: domain.Some("X")})
	wantForbidden(t, err, domain.ForbiddenRole)

	_, err = usecase.NewCreateEnvelope(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: string(g.ID), Name: "Prąd"})
	noErr(t, err)
	del := usecase.NewDeleteEnvelopeGroup(f.deps)
	wantConflict(t, del.Do(ctx, human(f.member), "h1", g.ID), domain.ConflictGroupNotEmpty)
	noErr(t, del.Do(ctx, human(f.member), "h1", other.ID))
	wantErr(t, del.Do(ctx, human(f.member), "h1", other.ID), domain.ErrNotFound)
	want := []string{"envelope_group.created", "envelope_group.created", "envelope_group.updated", "envelope.created", "envelope_group.deleted"}
	if got := f.actions(); !slices.Equal(got[len(got)-len(want):], want) {
		t.Errorf("audit = %v", got)
	}
	// An update that changes nothing records nothing.
	before := len(f.actions())
	_, err = update.Do(ctx, human(f.member), "h1", g.ID, usecase.UpdateEnvelopeGroupInput{Name: domain.Some("Opłaty")})
	noErr(t, err)
	if len(f.actions()) != before {
		t.Error("a no-op update was recorded")
	}
}

func TestEnvelopes(t *testing.T) {
	f := budgetFamily(t)
	group := string(f.vars["group"])
	create := usecase.NewCreateEnvelope(f.deps)
	e, err := create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: group, Name: " Wakacje ", Position: 5, Hidden: true,
		Goal: &usecase.GoalInput{Type: "target_by_date", Amount: pln("1200.00"), TargetDate: day("2027-03-15")}})
	noErr(t, err)
	if e.Name != "Wakacje" || !e.Hidden || e.Goal != (budget.Goal{Type: budget.GoalTargetByDate, Amount: 120000, TargetDate: day("2027-03-15")}) {
		t.Errorf("envelope = %+v", e)
	}
	for _, tt := range []struct {
		in    usecase.CreateEnvelopeInput
		field string
	}{
		{usecase.CreateEnvelopeInput{GroupID: group}, "name"},
		{usecase.CreateEnvelopeInput{Name: "X"}, "group_id"},
		{usecase.CreateEnvelopeInput{GroupID: "nope", Name: "X"}, "group_id"},
		{usecase.CreateEnvelopeInput{GroupID: group, Name: "X", Goal: &usecase.GoalInput{Type: "monthly", Amount: usd("1")}}, "goal.amount.currency"},
		{usecase.CreateEnvelopeInput{GroupID: group, Name: "X", Goal: &usecase.GoalInput{Type: "monthly", Amount: pln("0")}}, "goal.amount.amount"},
		{usecase.CreateEnvelopeInput{GroupID: group, Name: "X", Goal: &usecase.GoalInput{Type: "target_by_date", Amount: pln("1")}}, "goal.target_date"},
		{usecase.CreateEnvelopeInput{GroupID: group, Name: "X", Goal: &usecase.GoalInput{Type: "", Amount: pln("1")}}, "goal.type"},
	} {
		_, err := create.Do(ctx, human(f.member), "h1", tt.in)
		wantField(t, err, tt.field)
	}
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: group, Name: "JEDZENIE"})
	wantConflict(t, err, domain.ConflictDuplicateName)
	// A group of another household is not one of this one.
	f.addEnvelopes("h2", "x")
	_, err = create.Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: "g-h2", Name: "X"})
	wantField(t, err, "group_id")

	list := usecase.NewListEnvelopes(f.deps)
	visible, err := list.Do(ctx, human(f.viewer), "h1", usecase.ListEnvelopesInput{})
	noErr(t, err)
	all, err := list.Do(ctx, human(f.viewer), "h1", usecase.ListEnvelopesInput{IncludeHidden: true})
	noErr(t, err)
	if len(visible) != 7 || len(all) != 8 {
		t.Errorf("visible %d, all %d", len(visible), len(all))
	}
	got, err := usecase.NewGetEnvelope(f.deps).Do(ctx, human(f.viewer), "h1", e.ID)
	noErr(t, err)
	if got.ID != e.ID {
		t.Errorf("get = %+v", got)
	}
	_, err = usecase.NewGetEnvelope(f.deps).Do(ctx, human(f.viewer), "h1", "x")
	wantErr(t, err, domain.ErrNotFound)

	update := usecase.NewUpdateEnvelope(f.deps)
	g2, err := usecase.NewCreateEnvelopeGroup(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeGroupInput{Name: "Cele"})
	noErr(t, err)
	up, err := update.Do(ctx, human(f.member), "h1", e.ID, usecase.UpdateEnvelopeInput{GroupID: domain.Some(string(g2.ID)),
		Hidden: domain.Some(false), Goal: domain.Some(&usecase.GoalInput{Type: "monthly", Amount: pln("100")})})
	noErr(t, err)
	if up.GroupID != g2.ID || up.Hidden || up.Goal != (budget.Goal{Type: budget.GoalMonthly, Amount: 10000}) {
		t.Errorf("updated = %+v", up)
	}
	up, err = update.Do(ctx, human(f.member), "h1", e.ID, usecase.UpdateEnvelopeInput{Goal: domain.Some[*usecase.GoalInput](nil)})
	noErr(t, err)
	if !up.Goal.IsZero() {
		t.Errorf("goal not removed: %+v", up.Goal)
	}
	_, err = update.Do(ctx, human(f.member), "h1", e.ID, usecase.UpdateEnvelopeInput{GroupID: domain.Some("nope")})
	wantField(t, err, "group_id")
	_, err = update.Do(ctx, human(f.member), "h1", "x", usecase.UpdateEnvelopeInput{})
	wantErr(t, err, domain.ErrNotFound)
	audit := f.Audit.All()
	last := audit[len(audit)-1]
	if last.Action != "envelope.updated" || last.Changes["goal"].From != "monthly 100.00 PLN" || last.Changes["goal"].To != "" {
		t.Errorf("last audit = %+v", last)
	}
}

func TestDeleteEnvelope(t *testing.T) {
	f := budgetFamily(t)
	food, fun := f.vars["food"], f.vars["fun"]
	del := usecase.NewDeleteEnvelope(f.deps)

	// Used by a transaction: only hiding is left.
	_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-10"), EnvelopeID: string(food)})
	noErr(t, err)
	wantConflict(t, del.Do(ctx, human(f.member), "h1", food), domain.ConflictEnvelopeInUse)
	// So is one in a split line only.
	_, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-10"),
		Splits: []usecase.SplitInput{{EnvelopeID: string(fun), Amount: pln("-4")}, {Amount: pln("-6")}}})
	noErr(t, err)
	wantConflict(t, del.Do(ctx, human(f.member), "h1", fun), domain.ConflictEnvelopeInUse)

	free, err := usecase.NewCreateEnvelope(f.deps).Do(ctx, human(f.member), "h1", usecase.CreateEnvelopeInput{GroupID: string(f.vars["group"]), Name: "Nowa"})
	noErr(t, err)
	_, err = usecase.NewSetAssignment(f.deps).Do(ctx, human(f.member), "h1", free.ID, usecase.SetAssignmentInput{Month: "2026-10", Amount: pln("50")})
	noErr(t, err)
	pe, err := usecase.NewCreatePayee(f.deps).Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: "Kino", DefaultEnvelopeID: string(free.ID)})
	noErr(t, err)

	wantForbidden(t, del.Do(ctx, human(f.viewer), "h1", free.ID), domain.ForbiddenRole)
	noErr(t, del.Do(ctx, human(f.member), "h1", free.ID))
	wantErr(t, del.Do(ctx, human(f.member), "h1", free.ID), domain.ErrNotFound)
	got, err := f.Payees.ByID(ctx, "h1", pe.ID)
	noErr(t, err)
	if got.DefaultEnvelopeID != "" {
		t.Errorf("payee keeps the envelope: %+v", got)
	}
	left, _ := f.Assignments.ListByHousehold(ctx, "h1")
	if len(left) != 0 {
		t.Errorf("assignments left: %+v", left)
	}
	audit := f.Audit.All()
	if a := audit[len(audit)-1]; a.Action != "envelope.deleted" || a.Changes["assignments"].From != "2026-10:50.00 PLN" {
		t.Errorf("audit = %+v", a)
	}
	if a := audit[len(audit)-2]; a.Action != "payee.updated" || a.ResourceID != pe.ID {
		t.Errorf("audit = %+v", a)
	}
}

func TestEnvelopeChecksInTransactions(t *testing.T) {
	f := budgetFamily(t)
	food := string(f.vars["food"])
	f.addEnvelopes("h2", "foreign")
	// Unknown and foreign envelopes are refused everywhere.
	for _, env := range []string{"nope", "foreign"} {
		_, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1"), EnvelopeID: env})
		wantField(t, err, "envelope_id")
		_, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-2"),
			Splits: []usecase.SplitInput{{EnvelopeID: food, Amount: pln("-1")}, {EnvelopeID: env, Amount: pln("-1")}}})
		wantField(t, err, "splits[1].envelope_id")
		_, err = usecase.NewCreatePayee(f.deps).Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: "X " + env, DefaultEnvelopeID: env})
		wantField(t, err, "default_envelope_id")
	}
	off := f.addWallet("wo", "h1", f.member, wallet.Shared)
	off.OnBudget = false
	f.must(f.Wallets.Update(ctx, off))
	_, err := transfer(f, human(f.member), usecase.CreateTransferInput{FromWalletID: "w1", ToWalletID: "wo", Amount: pln("5"), EnvelopeID: "nope"})
	wantField(t, err, "envelope_id")

	v, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-1")})
	noErr(t, err)
	update := usecase.NewUpdateTransaction(f.deps)
	_, err = update.Do(ctx, human(f.member), "h1", v.Transaction.ID, usecase.UpdateTransactionInput{EnvelopeID: domain.Some("nope")})
	wantField(t, err, "envelope_id")
	_, err = update.Do(ctx, human(f.member), "h1", v.Transaction.ID, usecase.UpdateTransactionInput{
		Splits: domain.Some([]usecase.SplitInput{{EnvelopeID: "nope", Amount: pln("-1")}})})
	wantField(t, err, "splits[0].envelope_id")
	pe, err := usecase.NewCreatePayee(f.deps).Do(ctx, human(f.member), "h1", usecase.CreatePayeeInput{Name: "Lidl"})
	noErr(t, err)
	_, err = usecase.NewUpdatePayee(f.deps).Do(ctx, human(f.member), "h1", pe.ID, usecase.UpdatePayeeInput{DefaultEnvelopeID: domain.Some("nope")})
	wantField(t, err, "default_envelope_id")

	// A hidden envelope is still an envelope.
	_, err = usecase.NewUpdateEnvelope(f.deps).Do(ctx, human(f.member), "h1", f.vars["food"], usecase.UpdateEnvelopeInput{Hidden: domain.Some(true)})
	noErr(t, err)
	_, err = update.Do(ctx, human(f.member), "h1", v.Transaction.ID, usecase.UpdateTransactionInput{EnvelopeID: domain.Some(food)})
	noErr(t, err)

	// A transaction of an envelope from before stage 003 stays editable.
	legacy := v.Transaction
	legacy.EnvelopeID = "legacy"
	f.must(f.Transactions.Update(ctx, legacy))
	_, err = update.Do(ctx, human(f.member), "h1", legacy.ID, usecase.UpdateTransactionInput{Note: domain.Some("ok")})
	noErr(t, err)

	// The payee's default envelope fills an expense without one, while it
	// exists.
	_, err = usecase.NewUpdatePayee(f.deps).Do(ctx, human(f.member), "h1", pe.ID, usecase.UpdatePayeeInput{DefaultEnvelopeID: domain.Some(food)})
	noErr(t, err)
	got, err := record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-3"), PayeeID: string(pe.ID)})
	noErr(t, err)
	if got.Transaction.EnvelopeID != f.vars["food"] {
		t.Errorf("default envelope = %q", got.Transaction.EnvelopeID)
	}
	stale, _ := f.Payees.ByID(ctx, "h1", pe.ID)
	stale.DefaultEnvelopeID = "gone"
	f.must(f.Payees.Update(ctx, stale))
	got, err = record(f, human(f.member), usecase.RecordTransactionInput{WalletID: "w1", Amount: pln("-3"), PayeeID: string(pe.ID)})
	noErr(t, err)
	if got.Transaction.EnvelopeID != "" {
		t.Errorf("a missing default envelope was used: %q", got.Transaction.EnvelopeID)
	}
}
