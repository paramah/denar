# Tasks: Import transakcji i porty integracji (008-import)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-801 Domena** — `domain/imports` (Batch, RowError, HashExternalID, NormalizeText), `domain/connection` (Connection, Account, walidacja), powody `import_reverted`, `provider_unavailable`, zasoby audytu (+testy). FR-801, FR-830.
- [x] **T-802 Porty, fake'i, portstest** — `ports.go` (ImportedTransaction, ImportBatchRepository, ConnectionRepository, CSVParser, konektory, `TransactionRepository.ExternalIDs`), fake'i (+kaskada), `fakes.CSVParser`, `fakes.Connector`, suity `RunImportBatchSuite`, `RunConnectionSuite`, `ExternalIDs` w suicie transakcji. FR-804, FR-830, FR-840.
- [x] **T-803 SQLite** — `00006_import.sql`, `import_batch.go`, `connection.go`, `ExternalIDs`; suity kontraktowe na SQLite. FR-804, FR-840.
- [x] **T-804 Rdzeń ImportTransactions** — `import_transactions.go` (dedupe, hash z wystąpieniem, kontrahenci, koperta domyślna, kursy, dry-run, audyt, auto-dopasowanie) (+testy, macierz autoryzacji). FR-800…FR-806.
- [x] **T-805 Partie** — `imports.go` (ListImports, GetImport, RevertImport) (+testy: cofnięcie z obciążeniem, ponowne cofnięcie, archiwalny portfel, ukryty portfel). FR-820, FR-821.
- [x] **T-806 Parser CSV** — `adapter/csvimport` (kodowania, separatory, liczby, daty, nagłówek, kolumny, presety, testdata) (+testy). FR-810, FR-811.
- [x] **T-807 Use case'y CSV** — `import_csv.go` (ListImportPresets, PreviewCSVImport, ImportCSV; limit 5 MiB) (+testy na fake parserze). FR-812, FR-813.
- [x] **T-808 Połączenia** — `connections.go` (ListConnectionProviders, ListConnections, CreateConnection, MapConnectionAccount, SyncConnection), `adapter/fakeconnector` (+testy z fake connectorem: bank, billing, błąd, brak klucza, uprawnienia). FR-830…FR-833.
- [x] **T-809 REST** — `api/openapi.yaml`, `go generate`, `handlers_imports.go`, `mapping_imports.go`, multipart w `requireJSON`, konformancja (+`task web:types`). FR-811…FR-833, NFR-082.
- [x] **T-810 MCP** — `list_imports`, `api/mcp-tools.md`, contract_test (27) (+test narzędzia).
- [x] **T-811 Composition root i e2e** — wiring (parser, rejestr, `DENAR_DEV_FAKE_CONNECTOR`), `e2e_import_test.go` (mBank Netflix → match → duplikaty → cofnięcie).
- [x] **T-812 Zamknięcie** — README (sekcja Import), `task verify`, statusy, INDEX.md, PLAN.md.

## Wynik

- Zadania T-801…T-812 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (`HashExternalID` — stabilność, normalizacja, wystąpienia; walidacja połączeń i kont); parser CSV (presety mbank — nowy i stary format, preambuła, stopka, Windows-1250; ing — blokada bez kwoty, stopka; pko — kolumny bez nagłówka; revolut — opłata, stan, czas w dacie, waluta; generic — auto-wykrywanie UTF-8/Windows-1250, separatora, liczb i dat; mapowanie po numerach bez nagłówka, debet/kredyt, `skip_rows`, odwrócenie znaku, nawiasy; tabelaryczne formaty liczb i dat; błędy pliku); use case'y na fake'ach (dedupe w portfelu, w partii i hash z wystąpieniem, reguły kontrahenta i koperta domyślna, przychód/wydatek, błędy wierszy, kursy USD i `rate_pending`, dry-run bez zapisu i audytu, puste i za duże partie, auto-dopasowanie z subskrypcją, bot bez `subscriptions:write`, macierz autoryzacji 12 przypadków × import/podgląd, lista/szczegóły z portfelem prywatnym i kursorem, cofnięcie z odpięciem obciążenia i ręcznie usuniętą transakcją, ponowne cofnięcie, archiwalny portfel; CSV — limit 5 MiB, preset, `preview_limit`, kolejność autoryzacji; połączenia — providerzy, lista bez poświadczeń, tworzenie z `SecretBox`, brak klucza, mapowanie kont z walidacją portfela, `SyncConnection` bank/billing/błąd konektora/brak providera); `portstest` partii (CRUD, stronicowanie, powiązania, usunięcie transakcji, kaskada) i połączeń (CRUD, konta, kaskada) oraz `ExternalIDs` na fake'ach i SQLite; REST — konformancja 8 nowych operacji × każda odpowiedź (multipart), przeniesienie opcji z części `options`, multipart tylko na ścieżkach uploadu, limit rozmiaru; MCP — `list_imports`, kontrakt 27 narzędzi; e2e `TestImportEndToEnd` (mBank Windows-1250 → podgląd → import → obciążenie Netflix `matched` → ponowny import = 3 duplikaty → cofnięcie → `expected` → import botem) i `TestFakeConnectorEndToEnd` (`DENAR_DEV_FAKE_CONNECTOR=1`, połączenie → mapowanie → sync → `GET /connections`, partia).
