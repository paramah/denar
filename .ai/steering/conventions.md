# Conventions Steering

> Status: Draft
> Last Updated: 2026-10-21

## Code Style

- Go: `gofmt`, idiomatyczny styl; błędy domenowe jako typy/wartości w `internal/domain`, mapowane na kody wyłącznie w adapterach (`rest/problem.go`, `mcp/errors.go`).
- TS: typy REST wyłącznie z generatora (`openapi-typescript`), bez ręcznych duplikatów.
- Komentarze i dokumentacja artefaktów SDD po polsku; identyfikatory w kodzie, nazwy plików i statusy po angielsku.

## Architecture Patterns

- **Folders:**
  ```
  cmd/denar/                 # composition root: serve, migrate, token; joby
  internal/domain/           # money household user bot wallet transaction budget subscription notification audit
  internal/usecase/          # interaktory + ports.go + access.go; fakes/, portstest/
  internal/adapter/          # rest mcp sqlite oidc spa nbp notify/* csvimport
  internal/platform/         # config clock id password token crypto
  api/                       # openapi.yaml, mcp-tools.md
  web/                       # React + Vite + TS
  .ai/steering/  .ai/sdd/
  ```
- **Reguła zależności:** `adapter → usecase → domain`.
- **Interaktor:** struct z `New…(deps)` i `Do(ctx, Principal, Input) (Output, error)`; jeden plik na use case.
- **Autoryzacja:** w use case'ach (`access.go`: `requireRole`, `requireScope`, `visibleWallets`); ukryte zasoby → `ErrNotFound`.
- **Transakcje bazy:** `TxManager.WithinTx`; zdarzenia i outbox zapisywane w tej samej transakcji.
- **Konfiguracja:** `internal/platform/config.FromEnv()` (zmienne `DENAR_*`), wstrzykiwana w `cmd/denar`.

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| Pakiety Go | małe, jednowyrazowe | `wallet`, `usecase` |
| Pliki Go | snake_case | `record_transaction.go` |
| Testy Go | `*_test.go` obok kodu | `record_transaction_test.go` |
| Migracje | `NNNNN_nazwa.sql` | `00001_core.sql` |
| Tokeny botów | prefiks `dnr_` | `dnr_…` |
| Spec slug | `NNN-nazwa` | `001-core` |
| Komponenty React | PascalCase | `EnvelopeGrid` |

## Testing Rules

- Testy jednostkowe domeny (tabelaryczne: Money, przeliczenia, koperty, cykle) i use case'ów na fake'ach.
- Macierz autoryzacji dla każdego use case'a z danymi: rola × scope × ograniczenie portfeli × portfel prywatny.
- Suity kontraktowe repozytoriów (`portstest`) na SQLite w pliku tymczasowym.
- REST: konformancja odpowiedzi z `api/openapi.yaml` (libopenapi-validator); MCP: e2e klientem SDK; Playwright dla UI.
- Mock tylko na granicy portów; zewnętrzne API (NBP, SMTP, Telegram…) testowane na `httptest.Server`.

## Security / Privacy Rules

- Hasła argon2id; tokeny sesji i botów przechowywane jako hash SHA-256; jawny token bota pokazywany raz.
- Sekrety kanałów powiadomień i połączeń szyfrowane AES-GCM kluczem `DENAR_MASTER_KEY`.
- Każda zmiana danych przez bota (i człowieka) trafia do audit logu.
- Sekretów nie logować ani nie commitować.

## Workflow Rules

- Komendy projektu są zadaniami w `Taskfile.yml`. Bez Makefile i luźnych `.sh`.
- Każdy etap to osobna specyfikacja w `.ai/sdd/specs/` z bramkami zatwierdzanymi przez człowieka.
- Kontrakty (`api/`, `usecase/ports.go`) zmienia się świadomie i koordynuje centralnie.

## Anti-Patterns

- Logika biznesowa w handlerach REST/MCP lub repozytoriach SQLite.
- `float64` dla kwot.
- Sprawdzanie uprawnień tylko w adapterze.
- Traktowanie istnienia pliku jako zatwierdzenia (źródłem prawdy jest `.status`).
