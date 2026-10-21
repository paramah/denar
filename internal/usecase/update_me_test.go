package usecase_test

import (
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/bot"
	"github.com/paramah/denar/internal/usecase"
)

func strPtr(s string) *string { return &s }

func TestUpdateMe(t *testing.T) {
	f := newFamily(t)
	uc := usecase.NewUpdateMe(f.deps)

	got, err := uc.Do(ctx, human(f.member), usecase.UpdateMeInput{Locale: strPtr("en")})
	noErr(t, err)
	if got.Locale == nil || *got.Locale != "en" || got.PasswordHash != "" || got.ID != "member" {
		t.Fatalf("user = %+v", got)
	}
	stored, err := f.Users.ByID(ctx, "member")
	noErr(t, err)
	if stored.EffectiveLocale() != "en" || stored.PasswordHash == "" || stored.Name != f.member.Name {
		t.Errorf("stored = %+v", stored)
	}

	// null goes back to automatic.
	got, err = uc.Do(ctx, human(f.member), usecase.UpdateMeInput{Locale: nil})
	noErr(t, err)
	if got.Locale != nil {
		t.Errorf("cleared locale = %v", *got.Locale)
	}
	stored, _ = f.Users.ByID(ctx, "member")
	if stored.Locale != nil {
		t.Error("cleared locale is still stored")
	}

	// An unknown language is rejected and nothing changes.
	_, err = uc.Do(ctx, human(f.owner), usecase.UpdateMeInput{Locale: strPtr("de")})
	wantField(t, err, "locale")
	stored, _ = f.Users.ByID(ctx, "owner")
	if stored.Locale != nil {
		t.Error("rejected locale stored")
	}

	// The use case reads the stored account, so a stale principal does not
	// undo other changes.
	renamed := stored
	renamed.Name = "Renamed"
	f.must(f.Users.Update(ctx, renamed))
	_, err = uc.Do(ctx, human(f.owner), usecase.UpdateMeInput{Locale: strPtr("pl")})
	noErr(t, err)
	stored, _ = f.Users.ByID(ctx, "owner")
	if stored.Name != "Renamed" || stored.EffectiveLocale() != "pl" || stored.Locale == nil {
		t.Errorf("stored = %+v", stored)
	}

	// Bots have no profile.
	b := f.addBot("b1", "h1", f.owner, bot.ScopeTransactionsRead)
	_, err = uc.Do(ctx, asBot(b), usecase.UpdateMeInput{Locale: strPtr("en")})
	wantForbidden(t, err, domain.ForbiddenHumanOnly)

	// A personal setting: no household audit entry (AD-1304).
	if len(f.actions()) != 0 {
		t.Errorf("audit = %v", f.actions())
	}
}
