package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/adapter/sqlite"
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/platform/config"
	"git.eiac.dev/tools/denar/internal/usecase"
)

const tokenUsage = `usage: denar token create --household <id> --name <name> --scopes <scope,scope> [--as <email>] [--wallets <id,id>] [--expires <RFC 3339 time>]

Creates a bot in the household and prints its token once. The bot acts for
the member given with --as, or for the oldest owner, and never has more
rights than that member.`

// tokenCommand runs "denar token create" (FR-036). The token goes to
// stdout alone, so a script can capture it.
func tokenCommand(ctx context.Context, cfg config.Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "create" {
		return errors.New(tokenUsage)
	}
	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, tokenUsage) }
	household := fs.String("household", "", "household id")
	name := fs.String("name", "", "bot name")
	scopes := fs.String("scopes", "", "comma-separated scopes")
	as := fs.String("as", "", "email of the member the bot acts for")
	wallets := fs.String("wallets", "", "comma-separated wallet ids")
	expires := fs.String("expires", "", "expiry, RFC 3339")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *household == "" || *name == "" || *scopes == "" {
		return errors.New(tokenUsage)
	}
	var expiresAt time.Time
	if *expires != "" {
		t, err := time.Parse(time.RFC3339, *expires)
		if err != nil {
			return fmt.Errorf("--expires must be an RFC 3339 time such as 2027-01-01T00:00:00Z")
		}
		expiresAt = t
	}

	db, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := sqlite.CheckSchemaCurrent(ctx, db, sqlite.Migrations()); err != nil {
		return err
	}
	d := useCaseDeps(db, slog.New(slog.NewJSONHandler(stderr, nil)), nil, nil)
	created, err := usecase.NewCreateBotCLI(d).Do(ctx, usecase.CreateBotCLIInput{
		HouseholdID:  domain.ID(*household),
		CreatorEmail: *as,
		CreateBotInput: usecase.CreateBotInput{
			Name: *name, Scopes: splitList(*scopes), WalletIDs: splitList(*wallets), ExpiresAt: expiresAt,
		},
	})
	if err != nil {
		return describe(err)
	}
	fmt.Fprintf(stderr, "bot %s (%s) created for %s with scopes %s\n", created.Bot.ID, created.Bot.Name, created.Bot.CreatedBy,
		strings.Join(scopeNames(created.EffectiveScopes), ","))
	fmt.Fprintln(stdout, created.Token)
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func scopeNames[S ~string](scopes []S) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

// describe spells out validation errors for a person at a terminal.
func describe(err error) error {
	var ve domain.ValidationError
	if errors.As(err, &ve) || errors.Is(err, domain.ErrInvalid) {
		return fmt.Errorf("invalid input: %w", err)
	}
	return err
}
