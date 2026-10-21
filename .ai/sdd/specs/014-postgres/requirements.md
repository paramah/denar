# Feature: PostgreSQL obok SQLite (F14)

> Status: Approved (użytkownik, przez zatwierdzenie planu 2026-10-21)
> Source: prośba użytkownika „dodaj do aplikacji obsługę postgresql” + odpowiedzi z rundy pytań (kopiowanie danych z SQLite, wspólne repozytoria z dialektem, testy przez `DENAR_TEST_POSTGRES`)
> Last Updated: 2026-10-21

## Overview

Denar może trzymać dane w PostgreSQL zamiast w pliku SQLite. Wybór bazy to jedna zmienna `DENAR_DB`. Istniejącą instalację przenosi komenda `denar db copy`. Porty, domena, REST, MCP i Web UI się nie zmieniają (TD-002).

## Requirements (EARS)

- **FR-1401** — WHEN `DENAR_DB` zaczyna się od `postgres://` albo `postgresql://`, THE serwer i wszystkie komendy SHALL używać PostgreSQL (16 lub nowszego); w przeciwnym razie SHALL traktować wartość jako ścieżkę pliku SQLite (jak dotąd).
- **FR-1402** — THE repozytoria SHALL zachowywać się na PostgreSQL tak samo jak na SQLite: wszystkie suity `portstest` SHALL przechodzić na obu bazach.
- **FR-1403** — THE schemat PostgreSQL SHALL mieć te same wersje migracji co SQLite; `denar migrate`, `DENAR_AUTO_MIGRATE` i `CheckSchemaCurrent` SHALL działać na obu (test zgodności wersji).
- **FR-1404** — THE transakcje zapisujące na PostgreSQL SHALL wykonywać się kolejno, także między procesami, tak jak na SQLite; równoległe transakcje czytające i zapisujące SHALL nie gubić zmian.
- **FR-1405** — WHEN użytkownik uruchamia `denar db copy --from <baza> [--to <baza>]`, THE system SHALL zmigrować bazę docelową, skopiować wszystkie wiersze wszystkich tabel w jednej transakcji, wypisać liczby wierszy po obu stronach i zakończyć błędem, gdy się różnią; `--to` SHALL mieć domyślnie `DENAR_DB`.
- **FR-1406** — IF baza docelowa `denar db copy` zawiera jakiekolwiek wiersze albo źródło nie ma aktualnego schematu, THEN THE system SHALL odmówić kopiowania i nie zmienić bazy docelowej.
- **FR-1407** — THE hasło z URL bazy SHALL nie trafiać do logów ani do wyjścia komend.
- **FR-1408** — THE `denar demo` i testy end-to-end `cmd/denar` SHALL działać na PostgreSQL; CI SHALL uruchamiać suity repozytoriów i testy end-to-end na usłudze PostgreSQL.

## Out of Scope

- Kilka instancji denar na jednej bazie (zadania w tle nie są koordynowane).
- Kopiowanie z PostgreSQL do SQLite jako zadeklarowana ścieżka (kod jest symetryczny, ale nie jest dokumentowany).
- Natywne typy PostgreSQL (`TIMESTAMPTZ`, `JSONB`, `CITEXT`) i optymalizacje zapytań pod PostgreSQL.
- Playwright na PostgreSQL (Web UI nie zależy od bazy).
