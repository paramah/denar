# Tasks: Budżet kopertowy (003-envelope-budget)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-201 Domena: Month** — `internal/domain/budget/month.go` (+testy tabelaryczne: parse, MonthOf dla start_day 1/10/28, Start/End, przełom roku, MonthsUntil). FR-200.
- [ ] **T-202 Domena: Group, Envelope, Goal** — walidacja, normalizacja, sortowanie (+testy). FR-206, FR-210.
- [ ] **T-203 Domena: Ledger, GoalStatus, AutoAssign** — przeniesienia, overspending → RTA następnego miesiąca, przydziały przyszłe, luki, przepełnienie; cele; auto-przydział; losowy test inwariantu. FR-203…FR-206, FR-213, NFR-020.
- [ ] **T-204 Porty, fakes, portstest** — `EnvelopeGroupRepository`, `EnvelopeRepository`, `AssignmentRepository`, `TransactionRepository.BudgetFlows/CountByEnvelope`; fakes; suity + kaskada gospodarstwa. FR-232, NFR-021.
- [ ] **T-205 Use case'y kopert i grup** — CRUD, `envelope_in_use`, `group_not_empty`, czyszczenie kontrahentów, audyt (+testy). FR-210, FR-231.
- [ ] **T-206 Use case'y budżetu** — widok miesiąca, historia koperty, przydział, przeniesienie, auto-przydział (+testy, inwariant na fake'ach, macierz autoryzacji). FR-201, FR-202, FR-211…FR-215, FR-230.
- [ ] **T-207 Integracja z 002** — walidacja `envelope_id` (transakcje, splity, przelewy, kontrahenci), domyślna koperta kontrahenta (+testy). FR-220, FR-221.
- [ ] **T-208 SQLite** — `00003_envelope_budget.sql`, repozytoria, agregaty; portstest zielone na SQLite. FR-232, NFR-021.
- [ ] **T-209 REST** — `api/openapi.yaml`, `go generate`, handlery, mapowanie, konformancja.
- [ ] **T-210 Composition root i e2e** — wiring, `TestBudgetEndToEnd` (dochód → przydziały → wydatki → overspending → następny miesiąc → przeniesienie → auto-przydział → inwariant).
- [ ] **T-211 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.
