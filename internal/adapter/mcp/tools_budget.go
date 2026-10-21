package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/usecase"
)

type listEnvelopesIn struct {
	IncludeHidden bool `json:"include_hidden,omitempty" jsonschema:"also list hidden envelopes; false by default"`
}

type envelopesOut struct {
	Currency  string        `json:"currency"`
	Envelopes []envelopeOut `json:"envelopes"`
}

type budgetOverviewIn struct {
	Month string `json:"month,omitempty" jsonschema:"budget month, YYYY-MM such as 2026-10; the current one by default"`
}

func (s *server) addBudgetTools(m *sdk.Server) {
	addTool(s, m, "list_envelopes",
		"List the budget envelopes (categories) in their groups, with their goals. Amounts are in the household's base currency. "+
			"Needs budget:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in listEnvelopesIn) (any, error) {
			groups, err := s.uc.listEnvelopeGroups.Do(ctx, p, hid)
			if err != nil {
				return nil, err
			}
			es, err := s.uc.listEnvelopes.Do(ctx, p, hid, usecase.ListEnvelopesInput{IncludeHidden: in.IncludeHidden})
			if err != nil {
				return nil, err
			}
			c, err := s.uc.whoami.Do(ctx, p)
			if err != nil {
				return nil, err
			}
			names := map[domain.ID]string{}
			for _, g := range groups {
				names[g.ID] = g.Name
			}
			base := c.Household.BaseCurrency
			out := envelopesOut{Currency: base, Envelopes: []envelopeOut{}}
			for _, e := range es {
				out.Envelopes = append(out.Envelopes, envelopeOut{ID: string(e.ID), Name: e.Name, GroupID: string(e.GroupID),
					GroupName: names[e.GroupID], Hidden: e.Hidden, Goal: goalView(e.Goal, base)})
			}
			return out, nil
		})

	addTool(s, m, "budget_overview",
		"Show the envelope budget of a month: Ready to Assign, and for every envelope what was assigned, spent (activity, negative) and "+
			"is available, with goal status. A negative available means the envelope is overspent. Needs budget:read.",
		true, func(ctx context.Context, p usecase.Principal, hid domain.ID, in budgetOverviewIn) (any, error) {
			v, err := s.uc.getBudgetMonth.Do(ctx, p, hid, in.Month)
			if err != nil {
				return nil, err
			}
			return budgetMonthView(v), nil
		})
}
