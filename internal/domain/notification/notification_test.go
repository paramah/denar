package notification

import (
	"errors"
	"testing"
	"time"

	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/event"
)

func TestRender(t *testing.T) {
	sub := map[string]string{"subscription_id": "s1", "subscription_name": "ChatGPT Plus", "vendor": "openai", "vendor_name": "OpenAI"}
	with := func(extra map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range sub {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	tests := []struct {
		name string
		typ  event.Type
		p    map[string]string
		want Message
	}{
		{"renewal in 3 days", event.SubscriptionRenewalUpcoming, with(map[string]string{"due_date": "2026-10-31", "amount": "20.00", "currency": "USD", "days_before": "3"}),
			Message{"Odnowienie: ChatGPT Plus za 3 dni", "ChatGPT Plus (OpenAI) odnowi się 31.10.2026: 20.00 USD.", "/h/h1/subscriptions/s1"}},
		{"renewal today", event.SubscriptionRenewalUpcoming, with(map[string]string{"due_date": "2026-10-31", "amount": "20.00", "currency": "USD", "days_before": "0", "vendor": "other", "vendor_name": "Inne"}),
			Message{"Odnowienie: ChatGPT Plus dziś", "ChatGPT Plus odnowi się 31.10.2026: 20.00 USD.", "/h/h1/subscriptions/s1"}},
		{"trial tomorrow", event.SubscriptionTrialEnding, with(map[string]string{"trial_ends_at": "2026-10-22", "days_before": "1", "amount": "43.00", "currency": "PLN"}),
			Message{"Koniec okresu próbnego: ChatGPT Plus jutro", "Okres próbny ChatGPT Plus kończy się 22.10.2026. Potem 43.00 PLN za okres.", "/h/h1/subscriptions/s1"}},
		{"price with old", event.SubscriptionPriceChanged, with(map[string]string{"effective_from": "2026-11-01", "old_amount": "20.00", "old_currency": "USD", "new_amount": "25.00", "new_currency": "USD"}),
			Message{"Zmiana ceny: ChatGPT Plus", "Od 01.11.2026 ChatGPT Plus kosztuje 25.00 USD (było 20.00 USD).", "/h/h1/subscriptions/s1"}},
		{"price without old", event.SubscriptionPriceChanged, with(map[string]string{"effective_from": "2026-11-01", "new_amount": "25.00", "new_currency": "USD"}),
			Message{"Zmiana ceny: ChatGPT Plus", "Od 01.11.2026 ChatGPT Plus kosztuje 25.00 USD.", "/h/h1/subscriptions/s1"}},
		{"missed", event.SubscriptionChargeMissed, with(map[string]string{"due_date": "2026-10-20", "amount": "20.00", "currency": "USD"}),
			Message{"Brak płatności: ChatGPT Plus", "Nie znaleziono płatności ChatGPT Plus z 20.10.2026 (20.00 USD). Sprawdź dopasowania.", "/h/h1/subscriptions/charges"}},
		{"overspent", event.EnvelopeOverspent, map[string]string{"envelope_id": "e1", "envelope_name": "Jedzenie", "month": "2026-10", "available": "-12.34", "currency": "PLN"},
			Message{"Przekroczona koperta: Jedzenie", "Koperta Jedzenie jest na minusie w miesiącu 2026-10: -12.34 PLN.", "/h/h1/budget/2026-10"}},
		{"goal due", event.EnvelopeGoalDue, map[string]string{"envelope_id": "e1", "envelope_name": "Wakacje", "month": "2026-10", "target_date": "2026-10-28",
			"goal_amount": "3000.00", "missing": "500.00", "currency": "PLN", "days_left": "7"},
			Message{"Cel koperty Wakacje za 7 dni", "Termin celu 28.10.2026: brakuje 500.00 PLN z 3000.00 PLN.", "/h/h1/budget/2026-10"}},
		{"unknown", event.Type("x.y"), nil, Message{"x.y", "", "/h/h1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(event.Event{HouseholdID: "h1", Type: tt.typ, Payload: tt.p})
			if got != tt.want {
				t.Errorf("Render = %+v\nwant     %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeConfig(t *testing.T) {
	tests := []struct {
		name    string
		typ     ChannelType
		in      map[string]string
		want    map[string]string
		errWant string
	}{
		{"email default", Email, nil, map[string]string{"address": "me@example.com"}, ""},
		{"email own", Email, map[string]string{"address": " other@example.com "}, map[string]string{"address": "other@example.com"}, ""},
		{"email bad", Email, map[string]string{"address": "nope"}, nil, "config.address"},
		{"webhook", Webhook, map[string]string{"url": "https://hooks.example.com/x"}, map[string]string{"url": "https://hooks.example.com/x"}, ""},
		{"webhook missing", Webhook, nil, nil, "config.url"},
		{"webhook ftp", Webhook, map[string]string{"url": "ftp://x"}, nil, "config.url"},
		{"unknown key", Webhook, map[string]string{"url": "https://x", "secret": "s"}, nil, "config.secret"},
		{"ntfy default server", Ntfy, map[string]string{"topic": "denar-dom"}, map[string]string{"server": DefaultNtfyServer, "topic": "denar-dom"}, ""},
		{"ntfy own server", Ntfy, map[string]string{"server": "https://ntfy.example.com/", "topic": "a_b"}, map[string]string{"server": "https://ntfy.example.com", "topic": "a_b"}, ""},
		{"ntfy bad topic", Ntfy, map[string]string{"topic": "a b"}, nil, "config.topic"},
		{"telegram", Telegram, map[string]string{"chat_id": "-100123"}, map[string]string{"chat_id": "-100123"}, ""},
		{"telegram missing", Telegram, nil, nil, "config.chat_id"},
		{"matrix", Matrix, map[string]string{"homeserver": "https://matrix.org/", "room_id": "!abc:matrix.org"}, map[string]string{"homeserver": "https://matrix.org", "room_id": "!abc:matrix.org"}, ""},
		{"matrix alias", Matrix, map[string]string{"homeserver": "https://matrix.org", "room_id": "#denar:matrix.org"}, nil, "config.room_id"},
		{"unknown type", ChannelType("sms"), nil, nil, "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeConfig(tt.typ, tt.in, "me@example.com")
			if tt.errWant != "" {
				var ve domain.ValidationError
				if !errors.As(err, &ve) || ve.Field != tt.errWant {
					t.Fatalf("error = %v, want a validation error on %s", err, tt.errWant)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("config = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("config = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCheckSecrets(t *testing.T) {
	if _, err := CheckSecrets(Telegram, nil, true); err == nil {
		t.Error("telegram without a bot token passed")
	}
	if got, err := CheckSecrets(Telegram, map[string]string{"bot_token": " 1:x "}, true); err != nil || got["bot_token"] != "1:x" {
		t.Errorf("secrets = %v, %v", got, err)
	}
	if got, err := CheckSecrets(Webhook, map[string]string{"secret": ""}, true); err != nil || len(got) != 0 {
		t.Errorf("empty optional secret = %v, %v", got, err)
	}
	if _, err := CheckSecrets(Webhook, map[string]string{"token": "x"}, true); err == nil {
		t.Error("unknown secret passed")
	}
	if _, err := CheckSecrets(Matrix, map[string]string{}, false); err != nil {
		t.Errorf("incomplete update = %v", err)
	}
	if !Spec(Matrix).NeedsSecret() || Spec(Webhook).NeedsSecret() || Spec(Email).NeedsSecret() {
		t.Error("NeedsSecret is wrong")
	}
}

func TestBackoffAndFailure(t *testing.T) {
	want := []time.Duration{0, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 32 * time.Minute, time.Hour, time.Hour}
	for n, w := range want {
		if got := Backoff(n); got != w {
			t.Errorf("Backoff(%d) = %v, want %v", n, got, w)
		}
	}
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	d := Delivery{Status: Pending}
	for i := 1; i < MaxAttempts; i++ {
		d = d.Failure(now, "boom")
		if d.Status != Pending || d.Attempts != i || !d.NextAttemptAt.Equal(now.Add(Backoff(i))) || d.LastError != "boom" {
			t.Fatalf("after %d failures: %+v", i, d)
		}
	}
	d = d.Failure(now, "last")
	if d.Status != Failed || d.Attempts != MaxAttempts || d.LastError != "last" {
		t.Fatalf("after the last failure: %+v", d)
	}
	ok := Delivery{Status: Pending, LastError: "x"}.Succeeded(now)
	if ok.Status != Sent || !ok.SentAt.Equal(now) || ok.LastError != "" || ok.Attempts != 1 {
		t.Errorf("success = %+v", ok)
	}
	if a := (Delivery{Status: Pending}).Abandon(now, "off"); a.Status != Failed || a.LastError != "off" {
		t.Errorf("abandon = %+v", a)
	}
}

func TestPreferences(t *testing.T) {
	all := Merge([]Preference{{Type: event.EnvelopeOverspent, InApp: false, ChannelIDs: []domain.ID{"c1"}}})
	if len(all) != len(event.Types) {
		t.Fatalf("merged = %d", len(all))
	}
	for i, p := range all {
		if p.Type != event.Types[i] {
			t.Errorf("order: %v", all)
		}
		if p.Type == event.EnvelopeOverspent {
			if p.InApp || len(p.ChannelIDs) != 1 {
				t.Errorf("stored lost: %+v", p)
			}
		} else if !p.InApp || p.ChannelIDs == nil || len(p.ChannelIDs) != 0 {
			t.Errorf("default = %+v", p)
		}
	}
	if p := For(nil, event.SubscriptionChargeMissed); !p.InApp {
		t.Errorf("For default = %+v", p)
	}
	got, err := NormalizePreferences([]Preference{{Type: event.EnvelopeGoalDue, ChannelIDs: []domain.ID{"a", "a", "b"}},
		{Type: event.SubscriptionRenewalUpcoming, InApp: true}})
	if err != nil || got[0].Type != event.SubscriptionRenewalUpcoming || len(got[1].ChannelIDs) != 2 || got[0].ChannelIDs == nil {
		t.Errorf("normalized = %+v, %v", got, err)
	}
	if _, err := NormalizePreferences([]Preference{{Type: "nope"}}); err == nil {
		t.Error("unknown type passed")
	}
	if _, err := NormalizePreferences([]Preference{{Type: event.EnvelopeGoalDue}, {Type: event.EnvelopeGoalDue}}); err == nil {
		t.Error("duplicate type passed")
	}
}
