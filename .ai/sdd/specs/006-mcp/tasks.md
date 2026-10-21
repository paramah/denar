# Tasks: Serwer MCP (006-mcp)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-601 Domena i dostęp** — `ForbiddenError.Scope`, `requireScope` podaje scope; pusty miesiąc = bieżący (`GetBudgetMonth`, `SetAssignment`, `MoveBetweenEnvelopes`) (+testy). FR-612, AD-605, AD-606.
- [x] **T-602 Use case `Resolve`** — `resolve.go` (wallet/envelope/payee/subscription; scope'y najpierw; id → nazwa → podciąg; niejednoznaczne → invalid; brak → `RefNotFound`) (+testy, w tym bot z listą portfeli i portfel prywatny). FR-614, AD-601.
- [x] **T-603 Use case `Whoami`** — `whoami.go` (+testy). FR-620, AD-602.
- [x] **T-604 Use case `SpendingReport`** — `spending_report.go` (+testy grupowań, zwrotów, splitów, pending, zakresów, truncated; macierz autoryzacji). FR-623, AD-603, AD-604.
- [x] **T-605 Adapter MCP: szkielet** — go-sdk v1.8.0, `doc.go`, `server.go`, `errors.go`, `auth.go`, `middleware.go`; `sdk_probe_test.go`, `errors_test.go`, `auth_test.go`. FR-600…FR-604, FR-611, FR-615.
- [x] **T-606 Narzędzia odczytu** — `tools_read.go`, `tools_subscriptions.go`, `types.go` (+testy na fake'ach). FR-620.
- [x] **T-607 Narzędzia zapisu** — `tools_write.go` (+testy: brak scope'u, portfel spoza listy, viewer-twórca, `source=bot`, audyt, sugestie). FR-621, FR-622, FR-624.
- [x] **T-608 Kontrakt** — `api/mcp-tools.md`; `contract_test.go` (lista narzędzi = kontrakt). NFR-060.
- [x] **T-609 Composition root i e2e** — `/mcp` w `rootHandler`, `e2e_mcp_test.go` klientem go-sdk. FR-600, FR-601.
- [x] **T-610 Zamknięcie** — README (sekcja MCP), `task verify`, statusy, INDEX.md, PLAN.md.

## Wynik

- Zadania T-601…T-610 zakończone; `task verify` zielone (2026-10-21).
- Testy: use case'y na fake'ach — `Resolve` (id, nazwa bez wielkości liter, podciąg, niejednoznaczność z kandydatami, portfel prywatny, bot z listą portfeli, scope przed nazwami), `Whoami` (scope'y efektywne, viewer, lista portfeli), `SpendingReport` (4 grupowania, zwroty, splity, przelewy/przychody pominięte, brak kursu → incomplete, zakresy domyślne i błędne, truncated) z macierzą autoryzacji (4 role × 2 zestawy scope'ów × lista portfeli × portfel prywatny + outsider), `ForbiddenError.Scope`, pusty miesiąc = bieżący (dzień startu, strefa); adapter MCP — probe go-sdk v1.8.0 (10 testów), mapowanie błędów, 401 jednakowe dla 8 przypadków, 429 adresu i bota, transport (405/400/413/403, nosniff, log bez tokenu), każde z 25 narzędzi klientem SDK przez HTTP (brak scope'u z nazwą, portfel spoza listy → `not_found`, viewer-twórca → brak zapisu i audytu, `source=bot` i audyt bota, sugestie po `record_expense`, niejednoznaczna nazwa, błędy schematu → `invalid`), zgodność listy narzędzi i scope'ów z `api/mcp-tools.md`; e2e `TestMCPEndToEnd` (REST → token → `whoami` → `record_expense` → `upcoming_renewals` → `subscription_costs` → `forbidden`).
