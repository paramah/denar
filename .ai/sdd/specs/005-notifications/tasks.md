# Tasks: Powiadomienia (005-notifications)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [ ] **T-501 Domena** — `event` (nowe typy kopert), `notification` (channel, notification, preference, delivery, render) + testy tabelaryczne. FR-500, FR-501, FR-513, FR-521, FR-531.
- [ ] **T-502 Crypto** — `internal/platform/crypto` (AES-256-GCM, ParseKey) + testy. FR-532.
- [ ] **T-503 Porty i fake'i** — `ports.go` (4 repozytoria, Notifier, Mailer, SecretBox, `ListIDs`, `DeleteDispatchedBefore`), `fakes`, `portstest` suity. 
- [ ] **T-504 SQLite** — `00005_notifications.sql` + repozytoria; suity `portstest` zielone.
- [ ] **T-505 Use case'y in-app, kanały, preferencje** — `notifications.go`, `channels.go`, `preferences.go` (+testy, bot z listą portfeli, brak klucza, brak SMTP). FR-530…FR-536, FR-550, FR-551, FR-560.
- [ ] **T-506 Dispatch, worker, budżet, retencja** — `notification_dispatch.go`, `notification_delivery.go`, `budget_events.go`, `notification_jobs.go` (+testy odbiorców, preferencji, idempotencji, retry na fałszywym zegarze). FR-500…FR-523, FR-552.
- [ ] **T-507 Adaptery notify** — `notify/{smtp,webhook,ntfy,telegram,matrix}` + testy na `httptest`/fałszywym SMTP. FR-540…FR-545.
- [ ] **T-508 Konfiguracja** — `config` (SMTP, MASTER_KEY, interwały, GOAL_DUE_DAYS, TELEGRAM_API_URL) + testy.
- [ ] **T-509 REST** — `openapi.yaml`, `go generate`, handlery, konformancja; `task web:types`. Zaproszenia e-mailem (FR-570).
- [ ] **T-510 MCP** — `list_notifications`, `api/mcp-tools.md`, `contract_test.go` (26).
- [ ] **T-511 Composition root i e2e** — wiring, joby, `e2e_notifications_test.go` (przypomnienie → in-app → webhook z podpisem).
- [ ] **T-512 Zamknięcie** — README („Powiadomienia”), `.env.example`, `task verify`, statusy, INDEX.md, PLAN.md.
