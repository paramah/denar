# Feature: Import transakcji i porty integracji (F08, etap 8)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (sekcja „Gotowość na integracje”, etap 8), brief orkiestratora etapu 008, `002-wallets-transactions/design.md` (`external_id`, wzorce kontrahentów, kursy), `004-subscriptions/design.md` (`AutoMatchTransactions`, AD-402), `005-notifications/design.md` (`SecretBox`)
> Last Updated: 2026-10-21

## Overview

Wspólny use case `ImportTransactions` przyjmuje partię transakcji z dowolnego źródła (plik CSV dziś, konektory bankowe i billingowe później), odrzuca duplikaty po `external_id`, przypisuje kontrahentów i koperty z reguł, zapisuje wszystko w jednej transakcji bazy razem z partią `ImportBatch`, a potem automatycznie dopasowuje nowe transakcje do oczekiwanych obciążeń subskrypcji. Adapter CSV parsuje wyciągi z mapowaniem kolumn i presetami polskich banków oraz Revoluta, z podglądem bez zapisu. Partię można cofnąć. Model `Connection` i porty `BankConnector`/`BillingConnector` przygotowują etapy 009–011.

## Business Context

- **Problem:** Ręczne przepisywanie wyciągu jest żmudne; bez importu auto-dopasowanie subskrypcji (004) nie ma danych.
- **Users:** Członkowie gospodarstwa (Web UI), boty z `transactions:write` (REST).
- **Value:** Wyciąg z banku → transakcje z kontrahentami i kopertami → obciążenia subskrypcji odhaczone same; ponowny import tego samego pliku nic nie psuje.
- **Success signal:** e2e: CSV mBanku z obciążeniem Netflix → transakcje → obciążenie `matched` → ponowny import = same duplikaty → cofnięcie; `task verify` zielone.

## Goals

- `ImportTransactions` jako jedyna ścieżka zapisu importów (P-008).
- Parser CSV: mapowanie kolumn, separator, kodowanie UTF-8/Windows-1250, format daty i liczby, nagłówki; presety `generic`, `mbank`, `ing`, `pko`, `revolut`.
- Podgląd (dry-run), lista partii, szczegóły, cofnięcie.
- Domena `connection`, porty konektorów, rejestr providerów, `SyncConnection` na fake'u, tabele `connections` i `connection_accounts`.

## Non-Goals / Out of Scope

- Prawdziwe konektory (PSD2, OpenAI, Apple) i tworzenie połączeń przez REST — etapy 009–011.
- Formaty inne niż CSV (MT940, OFX, CAMT) — mogą dojść jako kolejne adaptery tego samego use case'u.
- Tworzenie kontrahentów z surowych opisów bankowych; edycja reguł z poziomu importu.
- Import przelewów jako par nóg (import tworzy wydatki/przychody).
- Job synchronizacji połączeń w tle.

## User Stories

### US-080: Import wyciągu
**As a** członek gospodarstwa **I want to** wgrać CSV z banku do portfela **So that** nie przepisuję transakcji ręcznie.

### US-081: Podgląd przed importem
**As a** członek **I want to** zobaczyć pierwsze wiersze, wykryte duplikaty i błędy przed zapisem **So that** poprawię mapowanie kolumn zanim cokolwiek trafi do bazy.

### US-082: Bezpieczny ponowny import
**As a** członek **I want to** wgrywać nakładające się wyciągi bez duplikatów i móc cofnąć partię **So that** pomyłka jest odwracalna.

### US-083: Subskrypcje same się odhaczają
**As a** członek **I want to** żeby zaimportowane obciążenie Netflixa dopasowało się do oczekiwanego obciążenia subskrypcji **So that** nie robię tego ręcznie.

### US-084: Gotowość na banki i dostawców
**As a** opiekun projektu **I want to** mieć model połączeń i porty konektorów **So that** etapy 009–011 dodają tylko adaptery.

## Functional Requirements

### Wspólny import

- **FR-800:** The system SHALL import a batch of transactions into one wallet the principal may write to (`transactions:write`, visible, not archived) through one use case for every source; each imported transaction SHALL have `source = import`, status `cleared`, kind `expense` for a negative amount and `income` for a positive one, and its base amount from the rates of stage 002 (none → `rate_pending`).
- **FR-801:** The system SHALL skip as duplicates rows whose `external_id` the wallet already has, and later rows of the same batch with an `external_id` seen earlier; a row without an `external_id` SHALL get a deterministic one: a hash of the date, the amount, the normalised description and the row's occurrence number among rows of the batch with the same three values.
- **FR-802:** The system SHALL resolve the payee of a row with the payee rules of stage 002 (name, then patterns) against the raw payee text and description; a resolved payee's default envelope SHALL be set on expenses while the envelope exists; an unresolved row SHALL keep no payee. The raw text SHALL be kept in the note.
- **FR-803:** The system SHALL refuse a row (row error, not the batch) whose date is missing or invalid, whose amount is missing, zero, malformed, has more decimals than the wallet's currency, or whose currency differs from the wallet's; the batch SHALL go on with the other rows.
- **FR-804:** The system SHALL store the batch (who, when, source, source reference, preset, wallet, counters of rows, created, duplicates, errors, matched; up to 100 row errors) and its transactions in one database transaction together with their audit entries.
- **FR-805:** WHEN the batch is stored THEN the system SHALL run the automatic subscription matching of stage 004 on its new transactions, if the principal holds `subscriptions:write` and `transactions:read`, and store the number of matches on the batch.
- **FR-806:** The system SHALL refuse an empty batch (no rows and no row errors) and a batch over 10 000 rows with 422.

### CSV

- **FR-810:** The system SHALL parse CSV with a delimiter (`;`, `,`, tab or auto-detected), an encoding (UTF-8 or Windows-1250, auto-detected when not given), a date format (tokens `YYYY`, `MM`, `DD`, `M`, `D`; auto-detected among common ones), a decimal separator (`,`/`.`/auto) with spaces, non-breaking spaces and apostrophes as thousand separators and currency codes after the number ignored, optional header detection with rows to skip, and column mapping by header name or 1-based index: date, amount or debit/credit, payee (first non-empty of several), memo (joined), external id, currency, fee, state with accepted values, and amount inversion.
- **FR-811:** The system SHALL provide the presets `generic`, `mbank`, `ing`, `pko`, `revolut`, each a set of defaults the request may override, and list them at `GET /api/import-presets`.
- **FR-812:** The system SHALL limit the file to 5 MiB (422 `file`).
- **FR-813:** The system SHALL preview an import without writing: counters, the first N rows (default 20, up to 100) with their status (new or duplicate), resolved payee and envelope, the row errors and what it detected (delimiter, encoding, header line, headers).

### Partie

- **FR-820:** The system SHALL list the batches of the wallets the principal sees, newest first, paged, filterable by wallet (`transactions:read`), and return one batch (hidden → 404).
- **FR-821:** The system SHALL revert a batch (`transactions:write`): delete its transactions that still exist, unlinking their subscription charges, mark it `reverted` with who, when and how many; a reverted batch SHALL give 409 `import-reverted`, an archived wallet 409 `wallet-archived`; each deletion SHALL be audited.

### Połączenia

- **FR-830:** The system SHALL model connections of a household (provider, kind `bank|billing`, status, display name, credentials sealed with the master key, last sync time, last error) and their accounts mapped to wallets.
- **FR-831:** The system SHALL define ports `BankConnector` (list accounts, fetch transactions since a date) and `BillingConnector` (fetch charges since a date) and a registry of providers in the composition root; the registry SHALL be empty unless `DENAR_DEV_FAKE_CONNECTOR=1` adds the `fake` provider.
- **FR-832:** The system SHALL sync a connection through `ImportTransactions` for each account mapped to a wallet (bank) or into the mapped wallet (billing), and store the outcome (status, last sync, last error) on the connection.
- **FR-833:** Connections SHALL be managed by people with the role owner or admin only (bots: 403 `human-only`); `GET /api/households/{hid}/connections` and `GET /api/connection-providers` SHALL never return credentials.

### Kaskady i audyt

- **FR-840:** Deleting a household SHALL delete its batches, connections and their accounts.
- **FR-841:** The system SHALL audit `import.created`, `import.reverted`, `connection.created`, `connection.account_mapped`, `connection.synced` and every created or deleted transaction.

## Non-Functional Requirements

- **NFR-080:** Exchange rates and connector calls SHALL happen outside the database transaction.
- **NFR-081:** Credentials SHALL be stored only sealed (AES-GCM, AAD = connection id); without a master key creating a connection SHALL give 409 `secrets-unavailable`.
- **NFR-082:** Multipart requests SHALL be accepted only on the two CSV import operations, capped at 6 MiB; every other operation keeps JSON-only bodies.

## Acceptance Criteria

- [ ] Parser: presets on synthetic samples in `testdata/`, both encodings, number and date formats, broken rows.
- [ ] Use case: dedupe (wallet, in-file, hash with occurrence), payee rules and default envelope, auto-match with a subscription, revert, authorization matrix.
- [ ] `portstest` for the new repositories on fakes and SQLite; cascade of the household.
- [ ] REST conformance of every new operation and response.
- [ ] e2e in `cmd/denar` as in the success signal.
