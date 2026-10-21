package notification

import (
	"strconv"
	"strings"

	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/event"
	"github.com/paramah/denar/internal/domain/user"
)

// Message is the text of a notification: a title, a body and the path of
// the web UI it links to.
type Message struct {
	Title string
	Body  string
	Link  string
}

// Render writes the notification of an event in the language of the
// recipient; an unsupported language falls back to Polish.
func Render(locale user.Locale, e event.Event) Message {
	var m Message
	if user.EffectiveLocale(&locale) == user.LocaleEN {
		m = renderEN(e)
	} else {
		m = renderPL(e)
	}
	m.Link = link(e)
	m.Title = strings.TrimSpace(m.Title)
	return m
}

// link is the path of the web UI an event leads to; it does not depend on
// the language.
func link(e event.Event) string {
	h := "/h/" + string(e.HouseholdID)
	switch e.Type {
	case event.SubscriptionRenewalUpcoming, event.SubscriptionTrialEnding, event.SubscriptionPriceChanged:
		return h + "/subscriptions/" + e.Payload["subscription_id"]
	case event.SubscriptionChargeMissed:
		return h + "/subscriptions/charges"
	case event.EnvelopeOverspent, event.EnvelopeGoalDue:
		return h + "/budget/" + e.Payload["month"]
	}
	return h
}

// vendorOf is the vendor shown next to a subscription name, or "" when it
// would say nothing new.
func vendorOf(p map[string]string) string {
	if v := p["vendor_name"]; v != "" && p["vendor"] != "other" && !strings.EqualFold(v, p["subscription_name"]) {
		return " (" + v + ")"
	}
	return ""
}

func renderPL(e event.Event) Message {
	p := e.Payload
	name := p["subscription_name"]
	switch e.Type {
	case event.SubscriptionRenewalUpcoming:
		return Message{
			Title: "Odnowienie: " + name + " " + whenPL(p["days_before"]),
			Body:  name + vendorOf(p) + " odnowi się " + datePL(p["due_date"]) + ": " + money(p["amount"], p["currency"]) + ".",
		}
	case event.SubscriptionTrialEnding:
		return Message{
			Title: "Koniec okresu próbnego: " + name + " " + whenPL(p["days_before"]),
			Body: "Okres próbny " + name + " kończy się " + datePL(p["trial_ends_at"]) + ". Potem " +
				money(p["amount"], p["currency"]) + " za okres.",
		}
	case event.SubscriptionPriceChanged:
		body := "Od " + datePL(p["effective_from"]) + " " + name + " kosztuje " + money(p["new_amount"], p["new_currency"])
		if p["old_amount"] != "" {
			body += " (było " + money(p["old_amount"], p["old_currency"]) + ")"
		}
		return Message{Title: "Zmiana ceny: " + name, Body: body + "."}
	case event.SubscriptionChargeMissed:
		return Message{
			Title: "Brak płatności: " + name,
			Body: "Nie znaleziono płatności " + name + " z " + datePL(p["due_date"]) + " (" + money(p["amount"], p["currency"]) +
				"). Sprawdź dopasowania.",
		}
	case event.EnvelopeOverspent:
		env := p["envelope_name"]
		return Message{
			Title: "Przekroczona koperta: " + env,
			Body:  "Koperta " + env + " jest na minusie w miesiącu " + p["month"] + ": " + money(p["available"], p["currency"]) + ".",
		}
	case event.EnvelopeGoalDue:
		env := p["envelope_name"]
		return Message{
			Title: "Cel koperty " + env + " " + whenPL(p["days_left"]),
			Body: "Termin celu " + datePL(p["target_date"]) + ": brakuje " + money(p["missing"], p["currency"]) + " z " +
				money(p["goal_amount"], p["currency"]) + ".",
		}
	}
	return Message{Title: string(e.Type)}
}

func renderEN(e event.Event) Message {
	p := e.Payload
	name := p["subscription_name"]
	switch e.Type {
	case event.SubscriptionRenewalUpcoming:
		return Message{
			Title: "Renewal: " + name + " " + whenEN(p["days_before"]),
			Body:  name + vendorOf(p) + " renews on " + dateEN(p["due_date"]) + ": " + money(p["amount"], p["currency"]) + ".",
		}
	case event.SubscriptionTrialEnding:
		return Message{
			Title: "Trial ending: " + name + " " + whenEN(p["days_before"]),
			Body: "The " + name + " trial ends on " + dateEN(p["trial_ends_at"]) + ". After that it costs " +
				money(p["amount"], p["currency"]) + " per period.",
		}
	case event.SubscriptionPriceChanged:
		body := "From " + dateEN(p["effective_from"]) + " " + name + " costs " + money(p["new_amount"], p["new_currency"])
		if p["old_amount"] != "" {
			body += " (was " + money(p["old_amount"], p["old_currency"]) + ")"
		}
		return Message{Title: "Price change: " + name, Body: body + "."}
	case event.SubscriptionChargeMissed:
		return Message{
			Title: "Missed payment: " + name,
			Body: "No payment for " + name + " due " + dateEN(p["due_date"]) + " (" + money(p["amount"], p["currency"]) +
				") was found. Check the matches.",
		}
	case event.EnvelopeOverspent:
		env := p["envelope_name"]
		return Message{
			Title: "Envelope overspent: " + env,
			Body:  "Envelope " + env + " is negative in " + p["month"] + ": " + money(p["available"], p["currency"]) + ".",
		}
	case event.EnvelopeGoalDue:
		env := p["envelope_name"]
		title := "Goal for envelope " + env
		if w := whenEN(p["days_left"]); w != "" {
			title += " due " + w
		}
		return Message{
			Title: title,
			Body: "Goal due " + dateEN(p["target_date"]) + ": " + money(p["missing"], p["currency"]) + " short of " +
				money(p["goal_amount"], p["currency"]) + ".",
		}
	}
	return Message{Title: string(e.Type)}
}

func days(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

// whenPL writes a number of days ahead: dziś, jutro, za N dni.
func whenPL(s string) string {
	switch n, ok := days(s); {
	case !ok:
		return ""
	case n == 0:
		return "dziś"
	case n == 1:
		return "jutro"
	default:
		return "za " + strconv.Itoa(n) + " dni"
	}
}

// whenEN writes a number of days ahead: today, tomorrow, in N days.
func whenEN(s string) string {
	switch n, ok := days(s); {
	case !ok:
		return ""
	case n == 0:
		return "today"
	case n == 1:
		return "tomorrow"
	default:
		return "in " + strconv.Itoa(n) + " days"
	}
}

// datePL writes 2026-10-31 as 31.10.2026.
func datePL(s string) string { return formatDate(s, "02.01.2006") }

// dateEN keeps the unambiguous ISO form 2026-10-31.
func dateEN(s string) string { return formatDate(s, "2006-01-02") }

func formatDate(s, layout string) string {
	d, err := domain.ParseDate(s)
	if err != nil {
		return s
	}
	return d.Time().Format(layout)
}

// money writes an amount as the event carries it (a decimal string) with
// its currency code, the same in every language.
func money(amount, currency string) string {
	return strings.TrimSpace(amount + " " + currency)
}
