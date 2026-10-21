# Tasks: Subskrypcje (004-subscriptions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-301 Domena: słownik i Cycle** — `vendor.go`, `cycle.go` (+testy tabelaryczne: 31 → krótkie miesiące, 29.02, co N dni/tygodni/miesięcy/lat, `OnOrAfter`, `NextDue`, `Between`, brak dryfu, `PerYear`). FR-301, FR-302.
- [ ] **T-302 Domena: Subscription, PricePoint, Charge, Cost, MatchScore** — walidacja, `PriceOn`, `Plan`, `ReminderDue`, `CostOf`, `MatchScore` (+testy). FR-300, FR-303, FR-304, FR-307, FR-311, FR-315.
- [ ] **T-303 Domena: zdarzenia** — `internal/domain/event` (typy, Event). FR-314.
- [ ] **T-304 Porty, fakes, portstest** — `SubscriptionRepository`, `ChargeRepository`, `EventOutbox`; fakes z kaskadą; suity + kaskada gospodarstwa. FR-323.
- [ ] **T-305 Use case'y subskrypcji** — CRUD, ceny, słownik, widoczność, regeneracja, audyt, zdarzenie `price_changed` (+testy). FR-300…FR-305, FR-314, FR-321.
- [ ] **T-306 Use case'y obciążeń** — lista, sugestie w obie strony, potwierdzenie, odłączenie, zaksięgowanie, pominięcie, auto-dopasowanie (+testy). FR-307…FR-310.
- [ ] **T-307 Podsumowanie i odnowienia** — `GetSubscriptionSummary`, `ListUpcomingRenewals` (+testy). FR-311…FR-313, FR-330.
- [ ] **T-308 Joby** — generowanie, missed, przypomnienia, `RunSubscriptionJobs` (+testy idempotencji). FR-304, FR-306, FR-315, NFR-031.
- [ ] **T-309 Integracje i macierz autoryzacji** — `DeleteEnvelope`, `DeleteTransaction`; `subscription_authz_test.go`. FR-320, FR-322.
- [ ] **T-310 SQLite** — `00004_subscriptions.sql`, repozytoria; portstest zielone na SQLite. FR-323.
- [ ] **T-311 REST** — `api/openapi.yaml`, `go generate`, handlery, mapowanie, konformancja.
- [ ] **T-312 Konfiguracja, composition root i e2e** — `DENAR_CHARGE_HORIZON`, `DENAR_SUBSCRIPTION_INTERVAL`, wiring, job w tle, `TestSubscriptionsEndToEnd`.
- [ ] **T-313 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.
