# Tasks: Portfele i transakcje (002-wallets-transactions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-101 Domena: Date i Money** — `internal/domain/date.go`, `internal/domain/money/money.go`, `rate.go` (+testy tabelaryczne: parse/format, add, convert half-even, ConvertParts, ParseRate/FormatRate, overflow). FR-100, NFR-010.
- [x] **T-102 Domena: wallet, transaction, payee** — walidacja, normalizacja, `VisibleTo`, splity, tagi, `Resolve` (+testy). FR-120, FR-130, FR-131, FR-140.
- [x] **T-103 Porty, fakes, portstest** — nowe porty w `ports.go`, fakes (wallet, transaction, payee, fx rate, fx override, RateProvider), suity `portstest` + kaskada gospodarstwa. FR-161.
- [x] **T-104 Dostęp** — ponowny odczyt bota w `access.household`, `visibleWallets`, `validateBotWallets` zamiast `WalletDirectory`. FR-150, FR-151.
- [x] **T-105 Kursy** — `fx.go`: quote, cache, okno offline, override, `QuoteFxRate`, `ListFxOverrides`, `SetFxOverride`, `FillPendingRates` (+testy). FR-110…FR-112.
- [x] **T-106 Portfele** — Create/List/Get/Update z saldami i transakcją otwarcia (+testy). FR-120, FR-121.
- [x] **T-107 Transakcje, przelewy, kontrahenci** — Record/Get/Update/Delete/Search, CreateTransfer, payees CRUD, audyt (+testy, macierz autoryzacji). FR-130…FR-133, FR-140, FR-150, FR-160.
- [x] **T-108 SQLite** — `00002_wallets_transactions.sql`, repozytoria; portstest zielone na SQLite. FR-161.
- [x] **T-109 Adapter NBP** — `internal/adapter/nbp` + testy na `httptest.Server`. FR-110, NFR-011, NFR-012.
- [x] **T-110 REST** — `api/openapi.yaml` (Money, operacje), `task generate`, handlery, mapowanie, konformancja. FR-101.
- [x] **T-111 Composition root** — wiring, konfiguracja `DENAR_NBP_URL`/`DENAR_FX_INTERVAL`, job, e2e (PLN + USD, fake NBP, przelew, saldo, bot, offline → job). FR-111.
- [x] **T-112 Zamknięcie** — `task verify`, statusy, INDEX.md, PLAN.md, README.

## Wynik

- Zadania T-101…T-112 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (tabelaryczne Money/Convert/ConvertParts/RoundHalfEven/ParseRate, Date, walidacja portfeli, transakcji, splitów, tagów, `payee.Resolve`); use case'y na fake'ach (kursy: identity bez sieci, cache tylko dni przeszłych, weekend → piątek, offline → cache z okna albo `rate_pending`, job, kurs krzyżowy, nadpisanie; portfele; transakcje; przelewy; kontrahenci; atomowość z audytem), **macierz autoryzacji** `wallet_authz_test.go` (7 operacji × 3 portfele × 4 role × człowiek/bot × scope × lista portfeli + outsider = 609 przypadków), ponowny odczyt bota (wyłączony/wygasły/zrotowany/usunięty → 401); suity `portstest` wallet/transaction/payee/fx rate/fx override + kaskada gospodarstwa na fake'ach i SQLite; NBP na `httptest.Server`; konformancja REST wszystkich 17 nowych operacji × każda zadeklarowana odpowiedź; e2e `TestFinancesEndToEnd` (PLN + USD, kurs z fake NBP, przelew, salda, bot z listą portfeli, offline → job, ręczny kurs, usunięcie przelewu).
