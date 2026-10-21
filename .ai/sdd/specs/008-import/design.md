# Design: Import transakcji i porty integracji (008-import)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `002-wallets-transactions/design.md`, `004-subscriptions/design.md`, `005-notifications/design.md`
> Last Updated: 2026-10-21

## 1. Architektura (nowe elementy)

```
internal/domain/imports/          Batch, Source, Status, RowError, HashExternalID, NormalizeText
internal/domain/connection/       Connection, Kind, Status, Account, walidacja
internal/usecase/                 import_transactions.go (rdzeń), import_csv.go (podgląd, import CSV, presety),
                                  imports.go (lista, szczegóły, cofnięcie), connections.go (lista, providerzy,
                                  tworzenie, mapowanie kont, SyncConnection)
internal/usecase/{fakes,portstest} ImportBatchRepository, ConnectionRepository, TransactionRepository.ExternalIDs,
                                  fakes.Connector (bank + billing), fakes.CSVParser (prosty format, patrz §12)
internal/adapter/csvimport/       Parser (implementuje usecase.CSVParser), presety, testdata/*.csv
internal/adapter/fakeconnector/   provider `fake` (bank + billing) z danymi demo, dla DENAR_DEV_FAKE_CONNECTOR=1
internal/adapter/sqlite/          00006_import.sql, import_batch.go, connection.go, TransactionRepository.ExternalIDs
internal/adapter/rest/            handlers_imports.go, mapping_imports.go; requireJSON przepuszcza multipart na 2 ścieżkach
internal/adapter/mcp/             narzędzie list_imports
cmd/denar/                        wiring (parser, rejestr providerów, DENAR_DEV_FAKE_CONNECTOR), e2e_import_test.go
```

Adapter REST nie może importować `csvimport` (adaptery nie importują się nawzajem), więc parser jest portem `usecase.CSVParser`, wstrzykiwanym w `Deps.CSV`. Use case `ImportCSV` czyta bajty pliku, woła parser, a potem rdzeń `ImportTransactions`.

Zmiany w kodzie wcześniejszych etapów (minimalne):
- `TransactionRepository.ExternalIDs` (nowa metoda portu) — fake, SQLite, suita kontraktowa.
- `Deps`: `ImportBatches`, `Connections`, `CSV`, `Connectors`.
- REST `requireJSON`: multipart/form-data dozwolone tylko dla `POST …/imports/csv` i `…/imports/csv/preview`, limit 6 MiB (NFR-082). Ochrona CSRF dla nich to `http.CrossOriginProtection` (Sec-Fetch-Site/Origin), która i tak obejmuje każde żądanie zmieniające stan.
- `bodyError` rozpoznaje błąd multipart (`body: must be multipart/form-data`).
- `config`: `DENAR_DEV_FAKE_CONNECTOR`.
- MCP: `contract_test` liczy 27 narzędzi.

## 2. Domena

### imports
```go
type Source string // "csv" | "connection"
type Status string // "completed" | "reverted"
type RowError struct{ Row int; Field, Message string }  // Row = numer linii pliku (1-based); 0 = cała partia
type Batch struct {
    ID, HouseholdID, WalletID domain.ID
    Source Source; SourceRef string /* nazwa pliku albo id połączenia */; Preset string
    Status Status
    Rows, Created, Duplicates, Failed, Matched, Reverted int
    Errors []RowError /* ≤ MaxStoredErrors = 100 */
    CreatedBy domain.Actor; CreatedAt time.Time
    RevertedBy domain.Actor; RevertedAt time.Time
}
const MaxRows = 10000; MaxStoredErrors = 100
func NormalizeText(s string) string            // trim, pojedyncze spacje, małe litery
func HashExternalID(date domain.Date, amount int64, text string, occurrence int) string
    // "h:" + hex(sha256("v1|YYYY-MM-DD|amount|text|occurrence"))[:32]
```

### connection
```go
type Kind string   // bank | billing
type Status string // active | error | disabled
type Connection struct {
    ID, HouseholdID domain.ID; Provider string; Kind Kind; Status Status; DisplayName string
    SealedCredentials []byte; LastSyncAt time.Time; LastError string
    CreatedBy domain.ID; CreatedAt, UpdatedAt time.Time
}
type Account struct { ConnectionID domain.ID; ExternalID, Name, Currency string; WalletID domain.ID /* "" = niezmapowane */ }
```
`Validate`: provider 1–40 `[a-z0-9_-]`, nazwa 1–100, znany kind/status, `LastError` ≤ 500; konto: external id 1–200, nazwa ≤ 100.

## 3. Porty (`usecase/ports.go`)

```go
// Transakcja z dowolnego źródła, w walucie portfela.
type ImportedTransaction struct {
    Row int; ExternalID string; Date domain.Date
    Amount string   // dziesiętny ("-12.34"), bez floatów
    Currency string // "" = waluta portfela; inna → błąd wiersza
    PayeeRaw, Memo string
}
type ImportBatchFilter struct{ HouseholdID domain.ID; WalletIDs []domain.ID /* puste = nic */ }
type ImportBatchRepository interface {
    Create(ctx, imports.Batch) error
    Update(ctx, imports.Batch) error            // status, liczniki (matched, reverted), reverted_by/at
    ByID(ctx, householdID, id domain.ID) (imports.Batch, error)
    List(ctx, ImportBatchFilter, Page) ([]imports.Batch, string, error) // najnowsze (id malejąco)
    AddTransactions(ctx, batchID domain.ID, ids []domain.ID) error
    TransactionIDs(ctx, householdID, batchID domain.ID) ([]domain.ID, error) // istniejące, rosnąco
}
// TransactionRepository (nowa metoda)
ExternalIDs(ctx, householdID, walletID domain.ID, ids []string) (map[string]domain.ID, error)

type CSVColumns struct { Date, Amount, Debit, Credit, ExternalID, Currency, Fee, State []string; Payee, Memo []string }
type CSVOptions struct {
    Preset, Delimiter, Encoding, DateFormat, Decimal string // "" = z presetu / auto
    SkipRows int; HasHeader *bool; Columns CSVColumns; InvertAmount *bool; AcceptStates []string
}
type CSVParseResult struct { Rows []ImportedTransaction; Errors []imports.RowError
    Delimiter, Encoding string; HeaderLine int; Headers []string }
type CSVPreset struct { Key, Name, Description string; Options CSVOptions /* pełne domyślne */ }
type CSVParser interface {
    Presets() []CSVPreset
    Parse(data []byte, opts CSVOptions) (CSVParseResult, error) // błąd pliku: domain.ValidationError
}

type ConnectorAccount struct{ ExternalID, Name, Currency string }
type BillingCharge struct{ ExternalID string; Date domain.Date; Amount, Currency /* dodatnia kwota kosztu */, Description, SubscriptionRef string }
type BankConnector interface {
    ListAccounts(ctx, credentials []byte) ([]ConnectorAccount, error)
    FetchTransactions(ctx, credentials []byte, accountExternalID string, since domain.Date) ([]ImportedTransaction, error)
}
type BillingConnector interface {
    FetchCharges(ctx, credentials []byte, since domain.Date) ([]BillingCharge, error)
}
type ConnectorProvider struct { Key, Name string; Kind connection.Kind; Bank BankConnector; Billing BillingConnector }
type ConnectionRepository interface {
    Create, Update(ctx, connection.Connection) error // Update: status, nazwa, poświadczenia, last_sync/last_error, updated_at
    ByID(ctx, householdID, id) (connection.Connection, error)
    ListByHousehold(ctx, householdID) ([]connection.Connection, error) // wg nazwy, id
    PutAccount(ctx, householdID domain.ID, a connection.Account) error  // upsert (connection, external_id)
    ListAccounts(ctx, householdID, connectionID domain.ID) ([]connection.Account, error) // wg external_id
}
```
`Deps.Connectors map[string]ConnectorProvider` (rejestr; nil = pusty).

## 4. Rdzeń `ImportTransactions` (`import_transactions.go`)

`Do(ctx, p, householdID, ImportInput{WalletID, Source, SourceRef, Preset, Rows []ImportedTransaction, RowErrors []imports.RowError, DryRun bool, PreviewLimit int}) (ImportOutcome, error)`

1. `access.household` + `requireScope(transactions:write)`; `writableWallet` (widoczny, niearchiwalny; ukryty → 404).
2. `len(Rows)+len(RowErrors) == 0` → 422 `file: has no transactions`; > `MaxRows` → 422.
3. Dla każdego wiersza: `money.Parse("amount", Amount, wallet.Currency)` (wcześniej: obcięcie `+`), zero → błąd, `Currency` ≠ portfela → błąd, data zerowa → błąd; `Note` = złączenie `PayeeRaw` i `Memo` (różne, niepuste, ` · `), obcięte do 1000 znaków; `external_id` obcięty (trim), > 200 znaków → błąd; brak → `HashExternalID(date, amount, NormalizeText(payee + " " + memo), n)`, gdzie `n` to numer wystąpienia (1, 2, …) tej samej trójki w partii.
4. Duplikaty: w partii (drugi raz ten sam `external_id`) i w portfelu (`ExternalIDs`).
5. Kontrahent: `payee.Resolve(payees, PayeeRaw)`, a gdy brak — `payee.Resolve(payees, Note)`; kontrahent znaleziony i transakcja `expense` → domyślna koperta, o ile istnieje (`envelopeExists`).
6. Kursy (poza transakcją bazy, NFR-080): jeden `fx.quote` na dzień (mapa), `applyRate`; brak → `markPending`.
7. Dry-run: wynik bez zapisu (liczniki, pierwsze N wierszy ze statusem, błędy).
8. `WithinTx`: ponowna autoryzacja i `writableWallet`; ponowne `ExternalIDs` (wyścig); `insertTransaction` (audyt `transaction.created`); konflikt `duplicate_external_id` z `Create` → duplikat; `ImportBatches.Create` + `AddTransactions`; audyt `import.created` (wallet, source, ref, liczniki).
9. Po zatwierdzeniu: jeśli principal ma `subscriptions:write` i `transactions:read` (`requireScope` bez błędu) — `AutoMatchTransactions` na nowych id; liczba dopasowań → `Batch.Matched` (`Update` w krótkiej transakcji). Błąd auto-dopasowania nie cofa importu: jest logowany, `Matched` zostaje 0 (AD-806).

`ImportOutcome{Batch imports.Batch; Preview []ImportPreviewRow; Matches []AutoMatch}`; `ImportPreviewRow{Row, Date, Amount int64, Currency, PayeeRaw, Memo, ExternalID, Status new|duplicate, Kind, PayeeID, PayeeName, EnvelopeID}`.

## 5. CSV (`import_csv.go`, `adapter/csvimport`)

Use case'y: `ListImportPresets` (każdy uwierzytelniony principal), `PreviewCSVImport`, `ImportCSV` (oba: `CSVImportInput{WalletID, FileName, Data []byte, Options CSVOptions, PreviewLimit}`). Plik > 5 MiB → 422 `file`; pusty → 422 `file`; nieznany preset → 422 `options.preset`; `PreviewLimit` 1–100 (domyślnie 20). Nazwa pliku (bez ścieżki, ≤ 200 znaków) → `SourceRef`.

Parser (`csvimport.Parser`):
- **Kodowanie:** `utf-8` | `windows-1250` (`golang.org/x/text/encoding/charmap`); auto = UTF-8, jeśli bajty są poprawnym UTF-8, inaczej Windows-1250. BOM UTF-8 usuwany.
- **Separator:** `;` | `,` | `tab` (`\t`); auto = najczęstszy z `; , \t` w pierwszych 20 liniach poza cudzysłowami.
- **CSV:** `encoding/csv`, `LazyQuotes`, zmienna liczba pól; puste linie i linie zaczynające się od `#` w danych (stopki mBanku) pomijane po cichu; tak samo wiersze z pustą komórką daty.
- **Nagłówek:** `has_header` (domyślnie true): po `skip_rows` pierwszy wiersz, który zawiera kolumnę daty (po nazwie) — preambuły banków nie wymagają liczenia linii. Nazwy porównywane bez wielkości liter, po trim i bez wiodącego `#`. Bez nagłówka kolumny to numery 1-based.
- **Kolumny:** każda pozycja to lista alternatyw (nazwa lub numer); `date`, `amount`, `debit`, `credit`, `external_id`, `currency`, `fee`, `state` — pierwsza istniejąca kolumna; `payee` — pierwsza niepusta wartość w wierszu spośród istniejących; `memo` — wszystkie istniejące, złączone ` · `, a do każdej doklejane kolejne kolumny bez nagłówka (PKO). Brak daty albo kwoty (ani `amount`, ani `debit`/`credit`) → błąd pliku 422 `options.columns`.
- **Kwota:** `amount`, albo `|credit| − |debit|`; minus `fee`; `invert_amount` odwraca znak. Normalizacja: usuwa litery (kody walut), spacje, NBSP, `'`; minus Unicode → `-`; separator dziesiętny `,`/`.`; auto: gdy oba — późniejszy to dziesiętny; sam przecinek — dziesiętny; sama kropka — dziesiętna. Wynik: string dziesiętny z kropką.
- **Data:** format z tokenów (`YYYY`, `MM`, `DD`, `M`, `D`, `HH`, `mm`, `ss`) → układ Go; wartość z czasem przy formacie bez czasu jest przycinana do pierwszego słowa; auto: `YYYY-MM-DD`, `DD.MM.YYYY`, `DD-MM-YYYY`, `DD/MM/YYYY`, `YYYY/MM/DD`, `YYYY.MM.DD`.
- **Stan:** gdy `state` i `accept_states` — wiersz o innym stanie → błąd wiersza `state`.
- Błędy wiersza: `{row: linia pliku, field: date|amount|currency|state|row, message}`.

| Preset | Kodowanie | Sep. | Data | Dziesiętny | Kolumny (alternatywy) |
|---|---|---|---|---|---|
| `generic` | auto | auto | auto | auto | date: date, data, data operacji, data transakcji, data księgowania; amount: amount, kwota, kwota operacji, kwota transakcji; payee: payee, kontrahent, nadawca/odbiorca, odbiorca, description, opis; memo: memo, tytuł, title, opis operacji; external_id: id, external_id, transaction id |
| `mbank` | windows-1250 | `;` | YYYY-MM-DD | `,` | date: Data operacji, Data księgowania; amount: Kwota; payee: Nadawca/Odbiorca, Opis operacji; memo: Tytuł, Opis operacji |
| `ing` | windows-1250 | `;` | YYYY-MM-DD | `,` | date: Data transakcji; amount: Kwota transakcji (waluta rachunku); payee: Dane kontrahenta; memo: Tytuł, Szczegóły; external_id: Nr transakcji; currency: Waluta |
| `pko` | windows-1250 | `,` | YYYY-MM-DD | `.` | date: Data operacji; amount: Kwota; currency: Waluta; memo: Typ transakcji, Opis transakcji (+ kolumny bez nagłówka) |
| `revolut` | utf-8 | `,` | YYYY-MM-DD (czas przycięty) | `.` | date: Completed Date, Started Date; amount: Amount; fee: Fee; currency: Currency; payee: Description; state: State ∈ {COMPLETED} |

> **Presety wymagają weryfikacji na prawdziwych plikach.** Powstały z wiedzy o formatach eksportów (mBank: „Historia → eksport CSV”, ING: „Historia → eksport CSV”, PKO BP: „Historia → eksport CSV”, Revolut: „Statement → CSV”) i są testowane na syntetycznych próbkach w `internal/adapter/csvimport/testdata/`. Nazwy kolumn bywają zmieniane przez banki; nadpisanie mapowania w żądaniu działa bez zmiany kodu.

## 6. Partie (`imports.go`)

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `ListImports` | `transactions:read` | portfele widoczne; `wallet_id` ukryty → 404; kursor/limit |
| `GetImport` | `transactions:read` | portfel partii niewidoczny → 404 |
| `RevertImport` | `transactions:write` | `reverted` → 409 `import_reverted`; portfel archiwalny → 409 `wallet_archived`; dla każdej istniejącej transakcji partii: `unlinkCharge`, `Delete`, audyt `transaction.deleted`; partia: `reverted`, `reverted_by/at`, `Reverted = n`; audyt `import.reverted` |

## 7. Połączenia (`connections.go`)

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `ListConnectionProviders` | człowiek | rejestr wg klucza; `{key, name, kind}` |
| `ListConnections` | człowiek owner/admin | z kontami; bez poświadczeń (`has_credentials`) |
| `CreateConnection` | człowiek owner/admin | provider z rejestru (inny → 422 `provider`); poświadczenia → `Secrets.Seal(…, aad = id)`; brak klucza → 409 `secrets_unavailable`; status `active`; audyt `connection.created` (bez poświadczeń). Bez REST w 008. |
| `MapConnectionAccount` | człowiek owner/admin | upsert konta; portfel widoczny dla człowieka, niearchiwalny, waluta konta = waluta portfela (gdy podana); audyt `connection.account_mapped`. Bez REST w 008. |
| `SyncConnection` | człowiek owner/admin | §7.1. Bez REST w 008. |

### 7.1 SyncConnection
1. Połączenie, provider z rejestru (brak → 409 `provider_unavailable`), `Secrets.Open(sealed, aad = id)` (brak klucza → 409 `secrets_unavailable`).
2. `since` = `LastSyncAt` (data w strefie gospodarstwa) − 7 dni, a przy pierwszej synchronizacji dziś − 90 dni.
3. Bank: dla każdego konta z portfelem — `FetchTransactions(creds, account, since)` → `ImportTransactions{Source: connection, SourceRef: id połączenia}`. Billing: `FetchCharges(creds, since)` → wiersze `{ExternalID, Date, Amount: −kwota, Currency, PayeeRaw: nazwa providera, Memo: opis}` → `ImportTransactions` do portfela pierwszego zmapowanego konta (brak → 422 `accounts`).
4. Błąd konektora: połączenie `status = error`, `last_error` (≤ 500 znaków), wynik z błędem, bez zwracania błędu do wywołującego (partie z kont przed błędem zostają). Sukces: `active`, `last_sync_at = now`, `last_error = ""`. Audyt `connection.synced` (liczniki).
5. Wywołania konektora przed transakcjami bazy (NFR-080).

## 8. Schemat SQL (`00006_import.sql`)

```sql
CREATE TABLE import_batches (
  id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  wallet_id TEXT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  source TEXT NOT NULL CHECK (source IN ('csv','connection')), source_ref TEXT NOT NULL DEFAULT '', preset TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('completed','reverted')),
  rows_total, created_count, duplicate_count, error_count, matched_count, reverted_count INTEGER NOT NULL DEFAULT 0,
  errors TEXT NOT NULL DEFAULT '[]', created_by_kind, created_by_id, created_at INTEGER,
  reverted_by_kind, reverted_by_id TEXT NOT NULL DEFAULT '', reverted_at INTEGER);
CREATE INDEX … (household_id, id DESC), (wallet_id, id DESC);
CREATE TABLE import_batch_transactions (batch_id → import_batches CASCADE, transaction_id → transactions CASCADE, PK(batch_id, transaction_id));
CREATE TABLE connections (id PK, household_id → households CASCADE, provider, kind CHECK, status CHECK, display_name,
  credentials BLOB, last_sync_at INTEGER, last_error TEXT DEFAULT '', created_by → users, created_at, updated_at);
CREATE TABLE connection_accounts (connection_id → connections CASCADE, external_id, name, currency, wallet_id → wallets ON DELETE SET NULL,
  PRIMARY KEY (connection_id, external_id));
```
Usunięcie transakcji ręcznie usuwa jej powiązanie z partią (FK), więc cofnięcie widzi tylko istniejące.

## 9. REST (`api/openapi.yaml`, tag `imports`, `connections`)

| Operacja | Ścieżka | Wejście → wyjście |
|---|---|---|
| `listImportPresets` | `GET /import-presets` | → `ImportPresetList{items: [ImportPreset{key, name, description, options: CsvImportOptions}]}` |
| `previewCsvImport` | `POST /households/{householdId}/imports/csv/preview` | multipart `file` (binary), `options` (JSON `CsvImportOptions`) → 200 `ImportPreview` |
| `importCsv` | `POST /households/{householdId}/imports/csv` | multipart jak wyżej → 201 `ImportBatch` |
| `listImports` | `GET /households/{householdId}/imports?wallet_id&cursor&limit` | → `ImportBatchPage{items, next_cursor}` |
| `getImport` | `GET /households/{householdId}/imports/{importId}` | → `ImportBatch` |
| `revertImport` | `DELETE /households/{householdId}/imports/{importId}` | → 200 `ImportBatch` (status `reverted`) |
| `listConnectionProviders` | `GET /connection-providers` | → `{items: [{key, name, kind}]}` |
| `listConnections` | `GET /households/{householdId}/connections` | → `{items: [Connection]}` |

`CsvImportOptions` (wszystko opcjonalne poza `wallet_id`): `wallet_id`, `preset`, `delimiter` (`;`, `,`, `tab`), `encoding` (`utf-8`, `windows-1250`), `date_format`, `decimal_separator` (`,`, `.`), `skip_rows` (0–1000), `has_header`, `invert_amount`, `accept_states` (lista), `columns{date, amount, debit, credit, payee, memo, external_id, currency, fee, state}` — każde pole to lista nazw kolumn lub numerów 1-based jako stringi; pole podane w żądaniu zastępuje wartość presetu w całości. `preview_limit` (1–100) tylko w podglądzie.

`ImportPreview`: `wallet_id, preset, detected{delimiter, encoding, header_line (nullable), headers[]}, total_rows, new_count, duplicate_count, error_count, rows[ImportPreviewRow{row, date, amount: Money, payee_raw, memo, external_id, kind, status: new|duplicate, payee_id?, payee_name?, envelope_id?}], errors[ImportRowError{row, field, message}]`.

`ImportBatch`: `id, wallet_id, source (csv|connection), source_ref, preset (nullable), status (completed|reverted), total_rows, created_count, duplicate_count, error_count, matched_count, reverted_count, errors[ImportRowError], created_by: Actor, created_at, reverted_by (nullable Actor), reverted_at (nullable)`.

`Connection`: `id, provider, provider_name, kind, status, display_name, has_credentials, last_sync_at (nullable), last_error (nullable), accounts[{external_id, name, currency (nullable), wallet_id (nullable)}], created_at`.

Kody: 200/201, 401, 403 (scope/role/human-only), 404, 409 (`import-reverted`, `wallet-archived`), 422 (plik, opcje, wiersze nie dają 422 — trafiają do `errors`). Opcje: niepoprawny JSON → 422 `options`.

## 10. MCP

`list_imports` (`transactions:read`): `wallet` (id lub nazwa, opcjonalnie), `limit`, `cursor` → partie z licznikami. Kontrakt `api/mcp-tools.md` — 27 narzędzi.

## 11. Przypadki brzegowe

- Ten sam plik dwa razy → druga partia: `created 0`, `duplicates = n` (partia zapisana, bez transakcji).
- Dwie identyczne transakcje tego samego dnia bez ID (dwie kawy po 12 zł) → różne hashe (wystąpienie 1 i 2); nakładający się wyciąg z tymi samymi dwiema → oba duplikaty.
- Cofnięta partia → ponowny import tworzy transakcje od nowa (hash ten sam, transakcji już nie ma).
- Wiersz z kwotą `12.345` w PLN → błąd wiersza `amount`.
- Revolut z kartą w EUR do portfela PLN → błąd wiersza `currency`.
- Bot z `transactions:write`, bez `subscriptions:write` → import bez auto-dopasowania (`matched_count = 0`).
- Usunięcie transakcji z partii ręcznie → cofnięcie usuwa pozostałe, `reverted_count` mniejszy niż `created_count`.

## 12. Strategia testów

- Domena: `HashExternalID` (stabilność, wystąpienia), `NormalizeText`, walidacja połączeń.
- Parser: tabelaryczne formaty liczb i dat, auto-wykrywanie separatora/kodowania, każdy preset na próbce `testdata/*.csv` (Windows-1250 zapisane bajtowo), preambuła i stopka mBanku, kolumny bez nagłówka PKO, opłata i stan Revoluta, błędne wiersze, brak kolumn.
- Use case'y na fake'ach. `arch_test` sprawdza także pliki testów, więc `usecase` i `rest` nie importują `csvimport`: rdzeń dostaje gotowe `ImportedTransaction`, a `ImportCSV`/REST — prosty `fakes.CSVParser` (linie `data;kwota;kontrahent[;id]`). Prawdziwy parser działa w e2e `cmd/denar`. Dedupe, reguły kontrahenta, koperta domyślna, kursy (USD, brak kursu), auto-dopasowanie z subskrypcją, bot bez scope'u subskrypcji, cofnięcie (z dopasowanym obciążeniem), macierz autoryzacji (rola × bot ze scope'em/bez × lista portfeli × portfel prywatny), `SyncConnection` z fake connectorem (bank, billing, błąd, brak klucza).
- `portstest`: `RunImportBatchSuite`, `RunConnectionSuite`, `ExternalIDs` w suicie transakcji, kaskada gospodarstwa i usunięcia transakcji.
- REST: konformancja każdej nowej operacji × odpowiedź (multipart w przypadkach testowych).
- E2E `cmd/denar`: CSV mBanku (Windows-1250) z obciążeniem Netflix → podgląd → import → obciążenie `matched` → ponowny import = same duplikaty → cofnięcie → obciążenie znów `expected`.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-801 | Multipart (`file` + `options` jako JSON w jednym polu) zamiast base64 w JSON; oapi-codegen strict daje `*multipart.Reader`, handler czyta części sam | Działa w generatorze bez obejść; Web UI wysyła `FormData`; typy opcji nadal z kontraktu (`CsvImportOptions`) |
| AD-802 | Parser jako port `usecase.CSVParser` w `Deps.CSV` | Adaptery nie importują się nawzajem; REST → use case → port → csvimport |
| AD-803 | Powiązanie partii z transakcjami w tabeli `import_batch_transactions`, nie kolumną w `transactions` | Bez zmian modelu transakcji i jej repozytorium; FK usuwa powiązanie, gdy transakcja zniknie |
| AD-804 | Niedopasowany kontrahent → transakcja bez kontrahenta, surowy tekst w notatce (bez autotworzenia) | Opisy bankowe tworzyłyby setki śmieciowych kontrahentów; dopasowanie subskrypcji czyta notatkę |
| AD-805 | Kwota dodatnia → `income`, ujemna → `expense`, status `cleared` | Wyciąg zawiera operacje zaksięgowane; YNAB: wpływ bez koperty → Ready-to-Assign |
| AD-806 | Auto-dopasowanie po zatwierdzeniu partii (poza jej transakcją), tylko gdy principal ma `subscriptions:write` i `transactions:read`; błąd nie cofa importu | Kursy w dopasowaniu mogą wołać sieć (NFR-080); import jest ważniejszy niż dopasowanie; bot z samym `transactions:write` nie zmienia subskrypcji |
| AD-807 | Cofnięcie usuwa wszystkie istniejące transakcje partii (także edytowane po imporcie) i odpina ich obciążenia; import nie tworzy przelewów, więc problem nóg nie występuje | Prosta, przewidywalna semantyka; ręcznie usunięte transakcje są pomijane |
| AD-808 | Hash `external_id` = `h:` + 32 znaki hex SHA-256 z (wersja, data, kwota w jednostkach minimalnych, znormalizowany opis, numer wystąpienia) | Deterministyczny między plikami; wersja pozwala zmienić algorytm bez kolizji |
| AD-809 | Dry-run to ten sam rdzeń z flagą (bez transakcji bazy); kursy w podglądzie nie są liczone | Podgląd ma być szybki i bez sieci; liczy duplikaty i reguły dokładnie jak import |
| AD-810 | Partia z samymi duplikatami jest zapisywana (licznik), pusta (bez wierszy) → 422 | Historia prób importu; pusty plik to błąd użytkownika |
| AD-811 | Połączenia: tylko odczyt w REST; tworzenie/mapowanie/sync to use case'y bez endpointów, testowane na fake'u | Brief: bez tworzenia prawdziwych połączeń w 008; 009 doda endpointy razem z przepływem zgody banku |
| AD-812 | Billing w `SyncConnection` importuje koszty jako wydatki do portfela zmapowanego konta | Jedna ścieżka (`ImportTransactions`) także dla dostawców; powiązanie z subskrypcją zrobi auto-dopasowanie |
| AD-813 | Wiersze z pustą datą i linie zaczynające się od `#` po nagłówku są pomijane po cichu (stopki), inne błędne — raportowane | Stopki mBanku/ING nie są błędami użytkownika |
| AD-814 | Nagłówek wyszukiwany jako pierwszy wiersz zawierający kolumnę daty | Preambuły banków mają zmienną liczbę linii |
| AD-815 | Limit 10 000 wierszy na partię, 100 zapisanych błędów (liczniki dokładne) | Jedna transakcja SQLite i odpowiedź o rozsądnym rozmiarze |

## Ograniczenia

- Presety oparte na wiedzy o formatach, nie na prawdziwych plikach — do weryfikacji (patrz §5).
- Hash bez ID banku: nakładające się wyciągi dają poprawne duplikaty tylko wtedy, gdy oba pliki zawierają wszystkie identyczne operacje danego dnia (numer wystąpienia liczony w pliku).
- Lista transakcji nie filtruje po partii (brak `import_id` w `searchTransactions`) — UI pokazuje liczniki partii.
- Brak joba synchronizacji połączeń i endpointów zarządzania nimi.
- Import nie rozpoznaje przelewów między własnymi portfelami (powstają wydatek i przychód).

## Plan kolejnych specyfikacji

### 009 — PSD2 przez agregatora (GoCardless Bank Account Data / Enable Banking)
- Adapter `internal/adapter/connector/gocardless` (i/lub `enablebanking`) implementujący `BankConnector`; klient HTTP na `httptest` w testach.
- Przepływ zgody: endpointy `POST /households/{hid}/connections` (provider, instytucja) → URL autoryzacji banku; `GET /api/connections/callback` (requisition/session id) → zapis poświadczeń (`SecretBox`, AAD = id); `DELETE …/connections/{id}`; `PUT …/connections/{id}/accounts/{ext}` (mapowanie na portfel); `POST …/connections/{id}/sync`.
- Use case'y gotowe: `CreateConnection`, `MapConnectionAccount`, `SyncConnection`; do dodania: odświeżanie tokenów agregatora (poświadczenia jako JSON z `refresh`), wygasanie zgody (90/180 dni) → status `expired` + zdarzenie powiadomienia `connection.consent_expiring`.
- Job w `cmd/denar/app.go`: `SyncConnections` co `DENAR_SYNC_INTERVAL` (aktor `system/sync`, uprawnienia twórcy połączenia — trzeba dodać principal systemowy z rolą twórcy).
- Konfiguracja: `DENAR_GOCARDLESS_SECRET_ID/KEY` (albo klucz aplikacji Enable Banking), limity zapytań agregatora (4/dzień na konto u GoCardless) → `since` i harmonogram.
- Mapowanie: `transactionId`/`internalTransactionId` → `external_id`, `bookingDate`, `transactionAmount{amount, currency}` (string), `creditorName`/`debtorName` → `payee_raw`, `remittanceInformationUnstructured` → `memo`; transakcje `pending` pomijane.

### 010 — OpenAI costs/usage API → BillingConnector
- Adapter `connector/openai` (`GET /v1/organization/costs` z kluczem admina organizacji): dzienne koszty → `BillingCharge{ExternalID: "openai:"+data+line_item, Date, Amount (USD, string), Description: line_item/projekt}`.
- Agregacja dzienna/miesięczna do jednej transakcji (opcja połączenia), żeby nie zalewać rejestru.
- Powiązanie z subskrypcją przez `SubscriptionRef`/`Vendor.ExternalRef` (004): nowe pole w `ImportedTransaction` (`SubscriptionRef`) i preferencja dopasowania do subskrypcji o tym `external_ref` zamiast punktacji.
- Wykrywanie zmian ceny/zużycia → zdarzenie `subscription.price_changed`.

### 011 — Apple (e-mail z paragonami)
- `BillingConnector` czytający skrzynkę IMAP (poświadczenia: host, login, hasło aplikacji) albo webhook przekazujący e-maile (np. adres przekierowania) — do wyboru w specyfikacji.
- Parser paragonów Apple (`no_reply@email.apple.com`, „Your receipt from Apple”): numer zamówienia → `external_id`, data, pozycje (nazwa aplikacji/subskrypcji), kwota z walutą.
- Dopasowanie pozycji do subskrypcji vendor `apple` po nazwie; obsługa zwrotów.
- Prywatność: przechowywać tylko wyciągnięte pola, nie treść e-maili; poświadczenia IMAP w `SecretBox`.
