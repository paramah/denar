# Tasks: Subskrypcje (004-subscriptions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-301 Domena: słownik i Cycle** — `vendor.go`, `cycle.go` (+testy tabelaryczne: 31 → krótkie miesiące, 29.02, co N dni/tygodni/miesięcy/lat, `OnOrAfter`, `NextDue`, `Between`, brak dryfu, `PerYear`). FR-301, FR-302.
- [x] **T-302 Domena: Subscription, PricePoint, Charge, Cost, MatchScore** — walidacja, `PriceOn`, `Plan`, `ReminderDue`, `CostOf`, `MatchScore` (+testy). FR-300, FR-303, FR-304, FR-307, FR-311, FR-315.
- [x] **T-303 Domena: zdarzenia** — `internal/domain/event` (typy, Event). FR-314.
- [x] **T-304 Porty, fakes, portstest** — `SubscriptionRepository`, `ChargeRepository`, `EventOutbox`; fakes z kaskadą; suity + kaskada gospodarstwa. FR-323.
- [x] **T-305 Use case'y subskrypcji** — CRUD, ceny, słownik, widoczność, regeneracja, audyt, zdarzenie `price_changed` (+testy). FR-300…FR-305, FR-314, FR-321.
- [x] **T-306 Use case'y obciążeń** — lista, sugestie w obie strony, potwierdzenie, odłączenie, zaksięgowanie, pominięcie, auto-dopasowanie (+testy). FR-307…FR-310.
- [x] **T-307 Podsumowanie i odnowienia** — `GetSubscriptionSummary`, `ListUpcomingRenewals` (+testy). FR-311…FR-313, FR-330.
- [x] **T-308 Joby** — generowanie, missed, przypomnienia, `RunSubscriptionJobs` (+testy idempotencji). FR-304, FR-306, FR-315, NFR-031.
- [x] **T-309 Integracje i macierz autoryzacji** — `DeleteEnvelope`, `DeleteTransaction`; `subscription_authz_test.go`. FR-320, FR-322.
- [x] **T-310 SQLite** — `00004_subscriptions.sql`, repozytoria; portstest zielone na SQLite. FR-323.
- [x] **T-311 REST** — `api/openapi.yaml`, `go generate`, handlery, mapowanie, konformancja.
- [x] **T-312 Konfiguracja, composition root i e2e** — `DENAR_CHARGE_HORIZON`, `DENAR_SUBSCRIPTION_INTERVAL`, wiring, job w tle, `TestSubscriptionsEndToEnd`.
- [x] **T-313 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.

## Wynik

- Zadania T-301…T-313 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (tabelaryczne `Cycle` — 31 → krótkie miesiące, 29.02 co rok, co N dni/tygodni/miesięcy/lat, `OnOrAfter`/`NextDue`, brak dryfu przez 48 terminów, porównanie z przejściem dzień po dniu przez 3 lata dla 5 cykli; `PerYear`, `PriceOn`, `Plan` dla wszystkich statusów, `NextCharge`, `ReminderDue`, `CostOf` z JPY i kursami, `MatchScore` na granicach tolerancji i okna, z wzorcem, bez kursu, z kwotami bliskimi int64; słownik dostawców); use case'y na fake'ach (tworzenie z obciążeniami i walidacją, regeneracja przy zmianie cyklu/pauzie/wznowieniu/anulowaniu z zachowaniem dopasowanych, historia cen ze zdarzeniem i przeliczeniem od dnia w przeszłości, joby z idempotencją, przypomnieniem raz, missed w strefie gospodarstwa i triale, sugestie w obie strony z przeliczeniem USD→PLN, potwierdzenie/odłączenie/pominięcie z konfliktami, zaksięgowanie z kursem, w walucie portfela, jawną kwotą i bez portfela, auto-dopasowanie jednoznaczne/niejednoznaczne po obu stronach, podsumowanie z brakiem kursu CHF, nadchodzące odnowienia, integracje `DeleteEnvelope`/`DeleteTransaction`); **macierz autoryzacji** `subscription_authz_test.go` (17 operacji × 3 rodzaje subskrypcji × 4 role × człowiek/bot × scope × lista portfeli + outsider, ~840 przypadków); suity `portstest` subscription/charge/event outbox i kaskada gospodarstwa na fake'ach i SQLite; konformancja REST 18 nowych operacji × każda zadeklarowana odpowiedź; e2e `TestSubscriptionsEndToEnd`.
