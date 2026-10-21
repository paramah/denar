# Tasks: Serwer MCP (006-mcp)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-601 Domena i dostęp** — `ForbiddenError.Scope`, `requireScope` podaje scope; pusty miesiąc = bieżący (`GetBudgetMonth`, `SetAssignment`, `MoveBetweenEnvelopes`) (+testy). FR-612, AD-605, AD-606.
- [ ] **T-602 Use case `Resolve`** — `resolve.go` (wallet/envelope/payee/subscription; scope'y najpierw; id → nazwa → podciąg; niejednoznaczne → invalid; brak → `RefNotFound`) (+testy, w tym bot z listą portfeli i portfel prywatny). FR-614, AD-601.
- [ ] **T-603 Use case `Whoami`** — `whoami.go` (+testy). FR-620, AD-602.
- [ ] **T-604 Use case `SpendingReport`** — `spending_report.go` (+testy grupowań, zwrotów, splitów, pending, zakresów, truncated; macierz autoryzacji). FR-623, AD-603, AD-604.
- [ ] **T-605 Adapter MCP: szkielet** — go-sdk v1.8.0, `doc.go`, `server.go`, `errors.go`, `auth.go`, `middleware.go`; `sdk_probe_test.go`, `errors_test.go`, `auth_test.go`. FR-600…FR-604, FR-611, FR-615.
- [ ] **T-606 Narzędzia odczytu** — `tools_read.go`, `tools_subscriptions.go`, `types.go` (+testy na fake'ach). FR-620.
- [ ] **T-607 Narzędzia zapisu** — `tools_write.go` (+testy: brak scope'u, portfel spoza listy, viewer-twórca, `source=bot`, audyt, sugestie). FR-621, FR-622, FR-624.
- [ ] **T-608 Kontrakt** — `api/mcp-tools.md`; `contract_test.go` (lista narzędzi = kontrakt). NFR-060.
- [ ] **T-609 Composition root i e2e** — `/mcp` w `rootHandler`, `e2e_mcp_test.go` klientem go-sdk. FR-600, FR-601.
- [ ] **T-610 Zamknięcie** — README (sekcja MCP), `task verify`, statusy, INDEX.md, PLAN.md.
