package portstest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/audit"
	"github.com/paramah/denar/internal/usecase"
)

func newEntry(env Env, id, householdID domain.ID, actor domain.Actor, resourceType string) audit.Entry {
	return audit.Entry{
		ID: id, HouseholdID: householdID, At: now(env), Actor: actor, Action: resourceType + ".created",
		ResourceType: resourceType, ResourceID: "r-" + id, Changes: audit.Changes{}.Set("name", "Dom"),
	}
}

func auditFilter(householdID domain.ID) usecase.AuditFilter {
	return usecase.AuditFilter{HouseholdID: householdID}
}

func firstPage(limit int) usecase.Page { return usecase.Page{Limit: limit} }

func entryIDs(es []audit.Entry) []domain.ID {
	out := make([]domain.ID, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

// RunAuditSuite checks an AuditRepository. It needs Users and Households.
func RunAuditSuite(t *testing.T, factory Factory) {
	ctx := context.Background()
	user := domain.Actor{Kind: domain.ActorUser, ID: "u1"}
	bot := domain.Actor{Kind: domain.ActorBot, ID: "b1"}

	t.Run("Append then List returns the entry as stored", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		e := newEntry(env, "a1", "h1", user, audit.ResourceBot)
		e.Changes = audit.Changes{}.Diff("role", "member", "admin").Set("count", 3).Set("gone", nil)
		noError(t, env.Audit.Append(ctx, e))
		got, next, err := env.Audit.List(ctx, auditFilter("h1"), firstPage(10))
		noError(t, err)
		if next != "" || len(got) != 1 {
			t.Fatalf("List = %v, %q", got, next)
		}
		g := got[0]
		if !g.At.Equal(e.At) || g.Actor != e.Actor || g.Action != e.Action || g.ResourceType != e.ResourceType || g.ResourceID != e.ResourceID {
			t.Errorf("entry = %+v, want %+v", g, e)
		}
		gotJSON, _ := json.Marshal(g.Changes)
		wantJSON, _ := json.Marshal(e.Changes)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("changes = %s, want %s", gotJSON, wantJSON)
		}
	})

	t.Run("List is newest first, filtered and paged", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		noError(t, env.Households.Create(ctx, newHousehold(env, "h2", "u1")))
		for i := 1; i <= 7; i++ {
			actor, rt := user, audit.ResourceMembership
			if i%2 == 0 {
				actor, rt = bot, audit.ResourceBot
			}
			noError(t, env.Audit.Append(ctx, newEntry(env, domain.ID(fmt.Sprintf("a%02d", i)), "h1", actor, rt)))
		}
		noError(t, env.Audit.Append(ctx, newEntry(env, "a99", "h2", user, audit.ResourceBot)))

		var all []domain.ID
		cursor := ""
		for range 5 {
			page, next, err := env.Audit.List(ctx, auditFilter("h1"), usecase.Page{Cursor: cursor, Limit: 3})
			noError(t, err)
			all = append(all, entryIDs(page)...)
			if next == "" {
				break
			}
			cursor = next
		}
		sameOrder(t, "pages", all, []domain.ID{"a07", "a06", "a05", "a04", "a03", "a02", "a01"})

		exact, next, err := env.Audit.List(ctx, auditFilter("h1"), firstPage(7))
		noError(t, err)
		if len(exact) != 7 || next != "" {
			t.Errorf("a page that holds everything: %d items, next %q", len(exact), next)
		}

		byBot, _, err := env.Audit.List(ctx, usecase.AuditFilter{HouseholdID: "h1", ActorKind: domain.ActorBot}, firstPage(10))
		noError(t, err)
		sameOrder(t, "by actor kind", entryIDs(byBot), []domain.ID{"a06", "a04", "a02"})
		byActor, _, err := env.Audit.List(ctx, usecase.AuditFilter{HouseholdID: "h1", ActorKind: domain.ActorUser, ActorID: "u1"}, firstPage(10))
		noError(t, err)
		sameOrder(t, "by actor", entryIDs(byActor), []domain.ID{"a07", "a05", "a03", "a01"})
		byType, _, err := env.Audit.List(ctx, usecase.AuditFilter{HouseholdID: "h1", ResourceType: audit.ResourceBot}, firstPage(10))
		noError(t, err)
		sameOrder(t, "by resource type", entryIDs(byType), []domain.ID{"a06", "a04", "a02"})
	})

	t.Run("a cursor of another query or a malformed one is invalid", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		for i := 1; i <= 3; i++ {
			noError(t, env.Audit.Append(ctx, newEntry(env, domain.ID(fmt.Sprintf("a%d", i)), "h1", user, audit.ResourceBot)))
		}
		_, next, err := env.Audit.List(ctx, auditFilter("h1"), firstPage(1))
		noError(t, err)
		if next == "" {
			t.Fatal("no next cursor")
		}
		_, _, err = env.Audit.List(ctx, usecase.AuditFilter{HouseholdID: "h1", ResourceType: audit.ResourceHousehold}, usecase.Page{Cursor: next, Limit: 1})
		wantError(t, err, domain.ErrInvalid)
		_, _, err = env.Audit.List(ctx, auditFilter("h1"), usecase.Page{Cursor: "not a cursor", Limit: 1})
		wantError(t, err, domain.ErrInvalid)
	})

	t.Run("Append in a failed transaction leaves no trace", func(t *testing.T) {
		env := factory(t)
		seedHousehold(t, env)
		boom := errors.New("boom")
		err := env.Tx.WithinTx(ctx, func(ctx context.Context) error {
			noError(t, env.Audit.Append(ctx, newEntry(env, "a1", "h1", user, audit.ResourceBot)))
			return boom
		})
		wantError(t, err, boom)
		got, _, err := env.Audit.List(ctx, auditFilter("h1"), firstPage(10))
		noError(t, err)
		if len(got) != 0 {
			t.Errorf("rolled back entry is listed: %v", got)
		}
	})
}
