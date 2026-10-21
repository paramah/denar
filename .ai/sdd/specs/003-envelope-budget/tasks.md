# Tasks: Budżet kopertowy (003-envelope-budget)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-201 Domena: Month** — `internal/domain/budget/month.go` (+testy tabelaryczne: parse, MonthOf dla start_day 1/10/28, Start/End, przełom roku, MonthsUntil). FR-200.
- [x] **T-202 Domena: Group, Envelope, Goal** — walidacja, normalizacja, sortowanie (+testy). FR-206, FR-210.
- [x] **T-203 Domena: Ledger, GoalStatus, AutoAssign** — przeniesienia, overspending → RTA następnego miesiąca, przydziały przyszłe, luki, przepełnienie; cele; auto-przydział; losowy test inwariantu. FR-203…FR-206, FR-213, NFR-020.
- [x] **T-204 Porty, fakes, portstest** — `EnvelopeGroupRepository`, `EnvelopeRepository`, `AssignmentRepository`, `TransactionRepository.BudgetFlows/CountByEnvelope`; fakes; suity + kaskada gospodarstwa. FR-232, NFR-021.
- [x] **T-205 Use case'y kopert i grup** — CRUD, `envelope_in_use`, `group_not_empty`, czyszczenie kontrahentów, audyt (+testy). FR-210, FR-231.
- [x] **T-206 Use case'y budżetu** — widok miesiąca, historia koperty, przydział, przeniesienie, auto-przydział (+testy, inwariant na fake'ach, macierz autoryzacji). FR-201, FR-202, FR-211…FR-215, FR-230.
- [x] **T-207 Integracja z 002** — walidacja `envelope_id` (transakcje, splity, przelewy, kontrahenci), domyślna koperta kontrahenta (+testy). FR-220, FR-221.
- [x] **T-208 SQLite** — `00003_envelope_budget.sql`, repozytoria, agregaty; portstest zielone na SQLite. FR-232, NFR-021.
- [x] **T-209 REST** — `api/openapi.yaml`, `go generate`, handlery, mapowanie, konformancja.
- [x] **T-210 Composition root i e2e** — wiring, `TestBudgetEndToEnd` (dochód → przydziały → wydatki → overspending → następny miesiąc → przeniesienie → auto-przydział → inwariant).
- [x] **T-211 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.

## Wynik

- Zadania T-201…T-211 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (tabelaryczne `Month` dla start_day 1/10/28 i przełomu roku, walidacja grup/kopert/celów, `Ledger` — przeniesienia przez wiele miesięcy z lukami, overspending i jego wpływ na RTA następnego miesiąca, przydziały przyszłe, zwroty i ujemne przydziały, przepełnienie; cele monthly/target_by_date przed, w trakcie i po terminie; `AutoAssign`; losowy test inwariantu 300 rund × 13 miesięcy); use case'y na fake'ach (CRUD grup i kopert z konfliktami, `group_not_empty`, `envelope_in_use`, czyszczenie domyślnej koperty kontrahentów, przydział/przeniesienie z audytem i atomowością, auto-przydział, widok miesiąca ze wszystkimi rodzajami pieniędzy — przychód, bez koperty, split, USD, przelewy on↔on z różnicą kursową, on→off z kopertą i bez, cudzy portfel prywatny, rate_pending → incomplete, start_day 10; historia koperty z filtrem widoczności i kursorem; walidacja `envelope_id` w transakcjach, splitach, przelewach i kontrahentach; `TestBudgetInvariant` — 12 losowych gospodarstw × 6 miesięcy), **macierz autoryzacji** `budget_authz_test.go` (14 operacji × 4 role × człowiek/bot × scope × lista portfeli + outsider); suity `portstest` envelope group/envelope/assignment, `BudgetFlows`, `CountByEnvelope` i kaskada gospodarstwa na fake'ach i SQLite; konformancja REST 14 nowych operacji × każda zadeklarowana odpowiedź; e2e `TestBudgetEndToEnd`.
