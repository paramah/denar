# Design: PostgreSQL (014)

> Status: Approved (użytkownik, przez zatwierdzenie planu 2026-10-21)
> Source: `requirements.md`
> Last Updated: 2026-10-21

## Adapter `internal/adapter/sqldb`

- Pakiet `internal/adapter/sqlite` przemianowany na `sqldb`; migracje w `migrations/sqlite/` i `migrations/postgres/`, osadzone obie (`embed.go`), `DB.Migrations()` zwraca zestaw dialektu.
- `Dialect` (`sqlite`, `postgres`) i `DialectOf(target)` (`dialect.go`); `Open(target)` wybiera `openSQLite` albo `openPostgres` (`postgres.go`, `github.com/jackc/pgx/v5/stdlib`, jedna pula — `writer` i `reader` to ten sam `*sql.DB`).
- Repozytoria piszą SQL z `?`. Na PostgreSQL `DB.Exec`/`DB.Query` zwracają `pgExec`, które przez `rebind` zamienia `?` na `$n` (z pominięciem literałów, identyfikatorów w cudzysłowie i komentarzy `--`).
- SQL wspólny dla obu baz: `TRUE`/`FALSE` zamiast `1`/`0` dla kolumn logicznych, `NOT x` zamiast `x = 0`, `ON CONFLICT DO NOTHING` zamiast `INSERT OR IGNORE`, `SUM(CASE WHEN … THEN 1 ELSE 0 END)` zamiast sumowania kolumny logicznej.
- Różnice jako metody `Dialect`: `fold` (SQLite: funkcja `fold`, PostgreSQL: `lower`), `nocase` (SQLite: `COLLATE NOCASE`, PostgreSQL: `lower(x)`), `hasTag` (SQLite: `json_each`, PostgreSQL: `(col)::jsonb @> jsonb_build_array(?::text)` — bez operatora `?`, który kolidowałby z placeholderami).
- Błędy (`convert.go`): `isUnique`/`isCheck`/`isForeignKey` rozpoznają `*pgconn.PgError` (SQLSTATE 23505/23514/23503). Cel `"tabela.kolumna"` sprawdzany przez `TableName` i kolumny z `Detail` („Key (household_id, lower(name))=…”), więc wywołania w repozytoriach są bez zmian.

## Schemat PostgreSQL

- Te same tabele, kolumny, klucze i numery wersji. Różnice: `INTEGER` → `BIGINT` (czasy w µs, kwoty), 8 kolumn logicznych jako `BOOLEAN`, `BLOB` → `BYTEA`, unikalne nazwy bez względu na wielkość liter jako indeks na `lower(name)`, trigger `audit_log_no_update` jako funkcja plpgsql.
- ID i JSON zostają `TEXT`, daty `TEXT 'YYYY-MM-DD'` — kod skanowania jest wspólny, a kopiowanie nie konwertuje wartości (poza logicznymi).

## Decyzje

- **AD-1 Wspólne repozytoria z dialektem** zamiast osobnego adaptera: jedna implementacja, suity kontraktowe na obu bazach pilnują zgodności. Koszt: SQL ograniczony do części wspólnej.
- **AD-2 Advisory lock zamiast SERIALIZABLE:** `WithinTx` na PostgreSQL zaczyna od `SELECT pg_advisory_xact_lock(<stała>)`. Odpowiada to `_txlock=immediate` i jednemu połączeniu writera w SQLite — use case'y czytają przed zapisem, więc nie trzeba ponawiać transakcji. Zapisy poza transakcją to pojedyncze instrukcje, jak na SQLite. Test: 20 równoległych odczyt-zapis bez zgubionej zmiany (bez blokady test pada).
- **AD-3 Wielkość liter:** `lower()` w PostgreSQL składa litery spoza ASCII (np. „Ł”/„ł”), `NOCASE` w SQLite tylko ASCII. Na PostgreSQL „Łódź” i „łódź” to więc ta sama nazwa portfela, na SQLite nie. Świadomie akceptujemy, PostgreSQL zachowuje się tu lepiej.
- **AD-4 `SUM(bigint)`** w PostgreSQL zwraca `numeric`; `database/sql` skanuje go do `int64`, więc bez rzutowania w SQL (pokryte suitą sald).
- **AD-5 Kopiowanie** (`copy.go`): lista tabel w kolejności kluczy obcych (`copyTables`, test porównuje ją ze schematem), odczyt w jednej transakcji źródła, zapis w jednej transakcji celu (`WithinTx`, więc z blokadą), wsady do 30 000 parametrów, konwersja 0/1 → `bool` według typów kolumn celu, porównanie liczby wierszy.
- **AD-6 Testy:** `DENAR_TEST_POSTGRES` (URL serwera) włącza podtesty `postgres` w suitach i testach `sqldb`; każdy test dostaje świeży schemat (`search_path` w URL). `DENAR_TEST_E2E_POSTGRES=1` przełącza testy end-to-end `cmd/denar` na PostgreSQL. `task test:postgres` uruchamia kontener; CI ma usługę `postgres:17-alpine` (przypiętą digestem) w jobie Verify.

## Konfiguracja

- `config.Config.DatabasePath` → `Database`; `DatabaseForLog()` i `RedactURL` ukrywają hasło z URL w logach i wyjściu `db copy`. Hasło można też podać przez `PGPASSWORD` / `~/.pgpass` (pgx).
