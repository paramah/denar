# Tasks: Import transakcji i porty integracji (008-import)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-801 Domena** — `domain/imports` (Batch, RowError, HashExternalID, NormalizeText), `domain/connection` (Connection, Account, walidacja), powody `import_reverted`, `provider_unavailable`, zasoby audytu (+testy). FR-801, FR-830.
- [ ] **T-802 Porty, fake'i, portstest** — `ports.go` (ImportedTransaction, ImportBatchRepository, ConnectionRepository, CSVParser, konektory, `TransactionRepository.ExternalIDs`), fake'i (+kaskada), `fakes.CSVParser`, `fakes.Connector`, suity `RunImportBatchSuite`, `RunConnectionSuite`, `ExternalIDs` w suicie transakcji. FR-804, FR-830, FR-840.
- [ ] **T-803 SQLite** — `00006_import.sql`, `import_batch.go`, `connection.go`, `ExternalIDs`; suity kontraktowe na SQLite. FR-804, FR-840.
- [ ] **T-804 Rdzeń ImportTransactions** — `import_transactions.go` (dedupe, hash z wystąpieniem, kontrahenci, koperta domyślna, kursy, dry-run, audyt, auto-dopasowanie) (+testy, macierz autoryzacji). FR-800…FR-806.
- [ ] **T-805 Partie** — `imports.go` (ListImports, GetImport, RevertImport) (+testy: cofnięcie z obciążeniem, ponowne cofnięcie, archiwalny portfel, ukryty portfel). FR-820, FR-821.
- [ ] **T-806 Parser CSV** — `adapter/csvimport` (kodowania, separatory, liczby, daty, nagłówek, kolumny, presety, testdata) (+testy). FR-810, FR-811.
- [ ] **T-807 Use case'y CSV** — `import_csv.go` (ListImportPresets, PreviewCSVImport, ImportCSV; limit 5 MiB) (+testy na fake parserze). FR-812, FR-813.
- [ ] **T-808 Połączenia** — `connections.go` (ListConnectionProviders, ListConnections, CreateConnection, MapConnectionAccount, SyncConnection), `adapter/fakeconnector` (+testy z fake connectorem: bank, billing, błąd, brak klucza, uprawnienia). FR-830…FR-833.
- [ ] **T-809 REST** — `api/openapi.yaml`, `go generate`, `handlers_imports.go`, `mapping_imports.go`, multipart w `requireJSON`, konformancja (+`task web:types`). FR-811…FR-833, NFR-082.
- [ ] **T-810 MCP** — `list_imports`, `api/mcp-tools.md`, contract_test (27) (+test narzędzia).
- [ ] **T-811 Composition root i e2e** — wiring (parser, rejestr, `DENAR_DEV_FAKE_CONNECTOR`), `e2e_import_test.go` (mBank Netflix → match → duplikaty → cofnięcie).
- [ ] **T-812 Zamknięcie** — README (sekcja Import), `task verify`, statusy, INDEX.md, PLAN.md.
