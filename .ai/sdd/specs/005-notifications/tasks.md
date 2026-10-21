# Tasks: Powiadomienia (005-notifications)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-501 Domena** — `event` (nowe typy kopert), `notification` (channel, notification, preference, delivery, render) + testy tabelaryczne. FR-500, FR-501, FR-513, FR-521, FR-531.
- [x] **T-502 Crypto** — `internal/platform/crypto` (AES-256-GCM, ParseKey) + testy. FR-532.
- [x] **T-503 Porty i fake'i** — `ports.go` (4 repozytoria, Notifier, Mailer, SecretBox, `ListIDs`, `DeleteDispatchedBefore`), `fakes`, `portstest` suity. 
- [x] **T-504 SQLite** — `00005_notifications.sql` + repozytoria; suity `portstest` zielone.
- [x] **T-505 Use case'y in-app, kanały, preferencje** — `notifications.go`, `channels.go`, `preferences.go` (+testy, bot z listą portfeli, brak klucza, brak SMTP). FR-530…FR-536, FR-550, FR-551, FR-560.
- [x] **T-506 Dispatch, worker, budżet, retencja** — `notification_dispatch.go`, `notification_delivery.go`, `budget_events.go`, `notification_jobs.go` (+testy odbiorców, preferencji, idempotencji, retry na fałszywym zegarze). FR-500…FR-523, FR-552.
- [x] **T-507 Adaptery notify** — `notify/{smtp,webhook,ntfy,telegram,matrix}` + testy na `httptest`/fałszywym SMTP. FR-540…FR-545.
- [x] **T-508 Konfiguracja** — `config` (SMTP, MASTER_KEY, interwały, GOAL_DUE_DAYS, TELEGRAM_API_URL) + testy.
- [x] **T-509 REST** — `openapi.yaml`, `go generate`, handlery, konformancja; `task web:types`. Zaproszenia e-mailem (FR-570).
- [x] **T-510 MCP** — `list_notifications`, `api/mcp-tools.md`, `contract_test.go` (26).
- [x] **T-511 Composition root i e2e** — wiring, joby, `e2e_notifications_test.go` (przypomnienie → in-app → webhook z podpisem).
- [x] **T-512 Zamknięcie** — README („Powiadomienia”), `.env.example`, `task verify`, statusy, INDEX.md, PLAN.md.

## Wynik

- Zadania T-501…T-512 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (szablony 6 typów + `kiedy`, konfiguracja i sekrety kanałów per typ, backoff i próby, preferencje), crypto (round-trip, AAD, zły klucz, modyfikacja, format klucza), use case'y na fake'ach (odbiorcy: portfel wspólny/prywatny/bez portfela/nieznany, nieaktywny, były członek, viewer, boty nie; preferencje i obce kanały; idempotencja dispatchu; worker: sukces z sekretem i nazwą gospodarstwa, 5 ponowień z backoffem 1–16 min na fałszywym zegarze i `failed` po 6 + `last_error`; kanał wyłączony/usunięty/brak klucza; CRUD kanałów, limity, 409 bez klucza i bez SMTP, test kanału; lista z kursorem, read/read-all, bot z listą portfeli, bot bez scope'u; budżet: overspent i goal_due raz; retencja 180/400 dni; zaproszenie e-mailem), `portstest` (4 nowe repozytoria + `ListIDs`, `DeleteDispatchedBefore`, kaskady) na fake'ach i SQLite, adaptery (webhook z podpisem, ntfy z nagłówkami RFC 2047, Telegram `ok:false`, Matrix txn i błędy, SMTP: plain, STARTTLS+AUTH, implicit TLS na fałszywym serwerze z certyfikatem testowym), konfiguracja, REST (konformancja 13 nowych operacji × wszystkie odpowiedzi; sekrety nie wracają), MCP (`list_notifications`, kontrakt 26 narzędzi), e2e `TestNotificationsEndToEnd` (subskrypcja → job → in-app → webhook po ponowieniu z poprawnym `X-Denar-Signature`).
