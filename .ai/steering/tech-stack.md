# Tech Stack Steering

> Status: Draft
> Last Updated: 2026-10-21

## Runtime / Platform

- **Runtime:** Go 1.27 — jeden binarny monolit (`denar serve|migrate|token`).
- **Package manager:** Go modules; npm dla `web/`.
- **Deployment target:** Self-hosted, pojedyncze binarium z osadzoną SPA (`embed`), obraz Docker (non-root, wolumen `/data`).
- **Monorepo:** Tak (backend + `web/`).

## Frontend

- **Framework:** React 19 + Vite + TypeScript 7 (SPA osadzona w binarium).
- **Lint:** ESLint 10, `typescript-eslint`, `@eslint-react`, `eslint-plugin-react-hooks`, `eslint-plugin-jsx-a11y-x`.
- **State/data:** `@tanstack/react-query`, `openapi-fetch`; typy z `api/openapi.yaml` przez `openapi-typescript`.
- **UI/Styling:** własne komponenty „Mennica” (007-webui).
- **i18n:** `i18next` + `react-i18next`, zasoby `web/src/i18n/{pl,en}/*.json` (013-i18n).

## Backend

- **HTTP:** `net/http` + `oapi-codegen` (std-http-server, strict-server).
- **Database:** SQLite (`modernc.org/sqlite`) albo PostgreSQL 16+ (`jackc/pgx/v5`), oba bez cgo, wybór przez `DENAR_DB`; adapter `internal/adapter/sqldb` z dialektem, migracje `pressly/goose` osadzone osobno dla każdej bazy (014-postgres).
- **Auth:** argon2id dla haseł, sesje cookie (hash SHA-256 tokenu w bazie), OIDC (`coreos/go-oidc` + `x/oauth2`, PKCE), Bearer `dnr_…` dla botów (REST i MCP).
- **MCP:** `modelcontextprotocol/go-sdk`, Streamable HTTP (stateless, JSON) na `/mcp`.
- **Kursy walut:** API NBP (tabela A), cache w bazie.
- **Powiadomienia:** outbox w bazie + adaptery SMTP, webhook, ntfy, Telegram, Matrix.
- **Kontrakty:** `api/openapi.yaml` (REST), `api/mcp-tools.md` (MCP), `internal/usecase/ports.go`.

> Wersje i API bibliotek potwierdzić przez Context7 przy implementacji. Wzorcowa implementacja: `../para-board` (Straylight).

## Testing / Verification Commands

| Purpose | Command | Notes |
|---------|---------|-------|
| Test na PostgreSQL | `task test:postgres` | Kontener postgres:17; `DENAR_TEST_POSTGRES` włącza suity na PostgreSQL |
| Install (Go) | `go mod download` | |
| Vet | `go vet ./...` | |
| Test | `go test -race ./...` | Domena i use case'y na fake'ach; suity `portstest` na SQLite (i PostgreSQL, gdy ustawione `DENAR_TEST_POSTGRES`) |
| Build | `task build` | Buduje web i binarium |
| Web test | `task web:test` | Vitest + MSW |
| E2E UI | `task web:e2e` | Playwright na zbudowanym binarium, axe |
| Full verify | `task verify` | fmt, vet, web build + test, test -race, build |

## Architectural Constraints

- Zależności tylko do środka: `adapter → usecase → domain`; adaptery nie importują się nawzajem (`internal/arch_test.go`).
- `internal/domain` bez importów spoza stdlib.
- Pieniądze jako `int64` jednostek minimalnych + waluta ISO 4217; kursy jako dziesiętne `math/big.Rat`.
- Brak cgo.

## Observability / Operations

- **Logging:** `log/slog`, sekrety jako `config.Secret` (redakcja).
- **Metrics / Tracing:** poza MVP.

## Technical Decisions

| ID | Decision | Rationale | Link |
|----|----------|-----------|------|
| TD-001 | Monolit Go + Clean Architecture; REST i MCP to adaptery na te same use case'y | Jedna logika, wiele interfejsów | PLAN.md |
| TD-002 | SQLite teraz, Postgres później — zrealizowane w TD-012 | Prostota self-hostingu | PLAN.md |
| TD-003 | Wiele gospodarstw na instancję, role per gospodarstwo | Rodzina + znajomi na jednej instancji | PLAN.md |
| TD-004 | Bot = scope'y ∩ uprawnienia twórcy, opcjonalnie ograniczony do portfeli | Najmniejsze uprawnienia | PLAN.md |
| TD-005 | Koperty wyłącznie w walucie bazowej; transakcja przechowuje snapshot `BaseAmount` | Stabilne inwarianty budżetu | PLAN.md |
| TD-006 | Kursy z NBP z cache | Darmowe, wiarygodne źródło dla PLN | PLAN.md |
| TD-007 | Integracje zewnętrzne tylko przez porty `BankConnector`/`BillingConnector` i wspólny `ImportTransactions` | Gotowość na banki/dostawców bez zmian domeny | PLAN.md |
| TD-008 | Powiadomienia przez transakcyjny outbox | Niezawodna dostawa, retry | PLAN.md |
| TD-009 | i18n: i18next + react-i18next w web, `Render(locale)` w domenie powiadomień; języki pl/en, preferencja w profilu | UI i powiadomienia po polsku i angielsku | specs/013-i18n |
| TD-010 | Repozytorium `github.com/paramah/denar`, licencja MIT, GitHub Actions (CI, release: binaria + GHCR), akcje przypięte do SHA | Publikacja open source | README.md |
| TD-011 | TypeScript 7 obok API TS 6: `tsc` z aliasu `@typescript/native` (TS 7), pakiet `typescript` to `@typescript/typescript6` dla typescript-eslint i openapi-typescript | typescript-eslint nie wspiera jeszcze TS 7 (typescript-eslint#10940); układ zalecany przez typescript-eslint | web/package.json |
| TD-012 | PostgreSQL obok SQLite: wspólne repozytoria z `Dialect` (`?` → `$n`), czasy jako `BIGINT` µs, advisory lock w `WithinTx`, `denar db copy` | Jedna implementacja zgodna z suitami kontraktowymi na obu bazach; semantyka transakcji jak w SQLite | specs/014-postgres |

## Open Questions

- Biblioteka UI/styling (`007-webui`).
- Agregator PSD2 (spec 010).
