package notification

import (
	"strconv"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
)

// Message is the text of a notification: a title, a body and the path of
// the web UI it links to.
type Message struct {
	Title string
	Body  string
	Link  string
}

// Render writes the notification of an event, in Polish.
func Render(e event.Event) Message {
	m := render(e)
	m.Title = strings.TrimSpace(m.Title)
	return m
}

func render(e event.Event) Message {
	p := e.Payload
	h := "/h/" + string(e.HouseholdID)
	sub := h + "/subscriptions/" + p["subscription_id"]
	budget := h + "/budget/" + p["month"]
	name := p["subscription_name"]
	switch e.Type {
	case event.SubscriptionRenewalUpcoming:
		vendor := ""
		if v := p["vendor_name"]; v != "" && p["vendor"] != "other" && !strings.EqualFold(v, name) {
			vendor = " (" + v + ")"
		}
		return Message{
			Title: "Odnowienie: " + name + " " + when(p["days_before"]),
			Body:  name + vendor + " odnowi się " + date(p["due_date"]) + ": " + money(p["amount"], p["currency"]) + ".",
			Link:  sub,
		}
	case event.SubscriptionTrialEnding:
		return Message{
			Title: "Koniec okresu próbnego: " + name + " " + when(p["days_before"]),
			Body: "Okres próbny " + name + " kończy się " + date(p["trial_ends_at"]) + ". Potem " +
				money(p["amount"], p["currency"]) + " za okres.",
			Link: sub,
		}
	case event.SubscriptionPriceChanged:
		body := "Od " + date(p["effective_from"]) + " " + name + " kosztuje " + money(p["new_amount"], p["new_currency"])
		if p["old_amount"] != "" {
			body += " (było " + money(p["old_amount"], p["old_currency"]) + ")"
		}
		return Message{Title: "Zmiana ceny: " + name, Body: body + ".", Link: sub}
	case event.SubscriptionChargeMissed:
		return Message{
			Title: "Brak płatności: " + name,
			Body: "Nie znaleziono płatności " + name + " z " + date(p["due_date"]) + " (" + money(p["amount"], p["currency"]) +
				"). Sprawdź dopasowania.",
			Link: h + "/subscriptions/charges",
		}
	case event.EnvelopeOverspent:
		env := p["envelope_name"]
		return Message{
			Title: "Przekroczona koperta: " + env,
			Body:  "Koperta " + env + " jest na minusie w miesiącu " + p["month"] + ": " + money(p["available"], p["currency"]) + ".",
			Link:  budget,
		}
	case event.EnvelopeGoalDue:
		env := p["envelope_name"]
		return Message{
			Title: "Cel koperty " + env + " " + when(p["days_left"]),
			Body: "Termin celu " + date(p["target_date"]) + ": brakuje " + money(p["missing"], p["currency"]) + " z " +
				money(p["goal_amount"], p["currency"]) + ".",
			Link: budget,
		}
	}
	return Message{Title: string(e.Type), Link: h}
}

// when writes a number of days ahead: dziś, jutro, za N dni.
func when(days string) string {
	switch n, err := strconv.Atoi(days); {
	case err != nil || n < 0:
		return ""
	case n == 0:
		return "dziś"
	case n == 1:
		return "jutro"
	default:
		return "za " + strconv.Itoa(n) + " dni"
	}
}

// date writes 2026-10-31 as 31.10.2026.
func date(s string) string {
	d, err := domain.ParseDate(s)
	if err != nil {
		return s
	}
	return d.Time().Format("02.01.2006")
}

func money(amount, currency string) string {
	return strings.TrimSpace(amount + " " + currency)
}
