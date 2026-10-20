# Tasks: Portfele i transakcje (002-wallets-transactions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-101 Domena: Date i Money** — `internal/domain/date.go`, `internal/domain/money/money.go`, `rate.go` (+testy tabelaryczne: parse/format, add, convert half-even, ConvertParts, ParseRate/FormatRate, overflow). FR-100, NFR-010.
- [ ] **T-102 Domena: wallet, transaction, payee** — walidacja, normalizacja, `VisibleTo`, splity, tagi, `Resolve` (+testy). FR-120, FR-130, FR-131, FR-140.
- [ ] **T-103 Porty, fakes, portstest** — nowe porty w `ports.go`, fakes (wallet, transaction, payee, fx rate, fx override, RateProvider), suity `portstest` + kaskada gospodarstwa. FR-161.
- [ ] **T-104 Dostęp** — ponowny odczyt bota w `access.household`, `visibleWallets`, `validateBotWallets` zamiast `WalletDirectory`. FR-150, FR-151.
- [ ] **T-105 Kursy** — `fx.go`: quote, cache, okno offline, override, `QuoteFxRate`, `ListFxOverrides`, `SetFxOverride`, `FillPendingRates` (+testy). FR-110…FR-112.
- [ ] **T-106 Portfele** — Create/List/Get/Update z saldami i transakcją otwarcia (+testy). FR-120, FR-121.
- [ ] **T-107 Transakcje, przelewy, kontrahenci** — Record/Get/Update/Delete/Search, CreateTransfer, payees CRUD, audyt (+testy, macierz autoryzacji). FR-130…FR-133, FR-140, FR-150, FR-160.
- [ ] **T-108 SQLite** — `00002_wallets_transactions.sql`, repozytoria; portstest zielone na SQLite. FR-161.
- [ ] **T-109 Adapter NBP** — `internal/adapter/nbp` + testy na `httptest.Server`. FR-110, NFR-011, NFR-012.
- [ ] **T-110 REST** — `api/openapi.yaml` (Money, operacje), `task generate`, handlery, mapowanie, konformancja. FR-101.
- [ ] **T-111 Composition root** — wiring, konfiguracja `DENAR_NBP_URL`/`DENAR_FX_INTERVAL`, job, e2e (PLN + USD, fake NBP, przelew, saldo, bot, offline → job). FR-111.
- [ ] **T-112 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.
