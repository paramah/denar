package csvimport

import "github.com/paramah/denar/internal/usecase"

func list(s ...string) []string { return s }

// presets are the known exports. They need checking against real files:
// banks rename columns now and then.
var presets = []usecase.CSVPreset{
	{
		Key: "generic", Name: "Dowolny CSV",
		Description: "Wykrywa kodowanie, separator, format daty i liczb; kolumny po typowych nazwach (data, kwota, kontrahent, opis) lub wskazane ręcznie.",
		Options: usecase.CSVOptions{Preset: "generic", Columns: usecase.CSVColumns{
			Date:       list("date", "data", "data operacji", "data transakcji", "data księgowania", "booking date", "transaction date"),
			Amount:     list("amount", "kwota", "kwota operacji", "kwota transakcji", "value", "wartość"),
			Payee:      list("payee", "kontrahent", "nadawca/odbiorca", "odbiorca", "nadawca", "counterparty", "description", "opis"),
			Memo:       list("memo", "tytuł", "title", "opis operacji", "notes", "notatka"),
			ExternalID: list("id", "external_id", "transaction id", "nr transakcji", "reference"),
			Currency:   list("currency", "waluta"),
		}},
	},
	{
		Key: "mbank", Name: "mBank",
		Description: "Eksport historii operacji mBanku (CSV, Windows-1250, średnik); preambuła i stopka pomijane.",
		Options: usecase.CSVOptions{Preset: "mbank", Delimiter: ";", Encoding: "windows-1250", DateFormat: "YYYY-MM-DD", Decimal: ",",
			Columns: usecase.CSVColumns{
				Date:   list("Data operacji", "Data księgowania"),
				Amount: list("Kwota"),
				Payee:  list("Nadawca/Odbiorca", "Opis operacji"),
				Memo:   list("Tytuł", "Opis operacji"),
			}},
	},
	{
		Key: "ing", Name: "ING Bank Śląski",
		Description: "Eksport historii ING (CSV, Windows-1250, średnik); kwota w walucie rachunku, numer transakcji jako ID.",
		Options: usecase.CSVOptions{Preset: "ing", Delimiter: ";", Encoding: "windows-1250", DateFormat: "YYYY-MM-DD", Decimal: ",",
			Columns: usecase.CSVColumns{
				Date:       list("Data transakcji", "Data księgowania"),
				Amount:     list("Kwota transakcji (waluta rachunku)"),
				Payee:      list("Dane kontrahenta"),
				Memo:       list("Tytuł", "Szczegóły"),
				ExternalID: list("Nr transakcji"),
				Currency:   list("Waluta"),
			}},
	},
	{
		Key: "pko", Name: "PKO BP",
		Description: "Eksport historii PKO BP (CSV, Windows-1250, przecinek, kropka dziesiętna); opis z kolumn bez nagłówka.",
		Options: usecase.CSVOptions{Preset: "pko", Delimiter: ",", Encoding: "windows-1250", DateFormat: "YYYY-MM-DD", Decimal: ".",
			Columns: usecase.CSVColumns{
				Date:     list("Data operacji", "Data waluty"),
				Amount:   list("Kwota"),
				Currency: list("Waluta"),
				Memo:     list("Typ transakcji", "Opis transakcji"),
			}},
	},
	{
		Key: "revolut", Name: "Revolut",
		Description: "Wyciąg Revolut (CSV, UTF-8, przecinek); kwota minus opłata, tylko operacje COMPLETED.",
		Options: usecase.CSVOptions{Preset: "revolut", Delimiter: ",", Encoding: "utf-8", DateFormat: "YYYY-MM-DD", Decimal: ".",
			AcceptStates: list("COMPLETED"),
			Columns: usecase.CSVColumns{
				Date:     list("Completed Date", "Started Date"),
				Amount:   list("Amount"),
				Fee:      list("Fee"),
				Currency: list("Currency"),
				State:    list("State"),
				Payee:    list("Description"),
				Memo:     list("Type"),
			}},
	},
}

func presetByKey(key string) (usecase.CSVPreset, bool) {
	for _, p := range presets {
		if p.Key == key {
			return p, true
		}
	}
	return usecase.CSVPreset{}, false
}

// Presets returns the presets, generic first.
func (Parser) Presets() []usecase.CSVPreset {
	out := make([]usecase.CSVPreset, len(presets))
	copy(out, presets)
	return out
}
