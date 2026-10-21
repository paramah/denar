# Tasks: PostgreSQL (014)

> Status: Approved (użytkownik, przez zatwierdzenie planu 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

- [x] **T-1401 sqlite → sqldb** — przeniesienie pakietu, migracje per dialekt, `DB.Migrations()`, `Open` z wyborem dialektu. FR-1401.
- [x] **T-1402 Dialekt i SQL wspólny** — `rebind`, `pgExec`, `fold`/`nocase`/`hasTag`, literały logiczne, klasyfikacja błędów pgx, testy. FR-1402.
- [x] **T-1403 Schemat PostgreSQL** — migracje 00001–00007, trigger audytu, test zgodności wersji, testy migracji na obu bazach. FR-1403.
- [x] **T-1404 Transakcje** — advisory lock w `WithinTx`, test współbieżności. FR-1404.
- [x] **T-1405 Konfiguracja** — `Config.Database`, redakcja hasła. FR-1401, FR-1407.
- [x] **T-1406 `denar db copy`** — `sqldb.Copy`, komenda, testy (SQLite→SQLite, SQLite→PostgreSQL, dane demo tabela po tabeli). FR-1405, FR-1406.
- [x] **T-1407 Testy i CI** — suity na obu bazach, e2e `cmd/denar` na PostgreSQL, `task test:postgres`, usługa w CI. FR-1402, FR-1408.
- [x] **T-1408 Dokumentacja** — README EN/PL, tech-stack (TD-012), PLAN, INDEX.
