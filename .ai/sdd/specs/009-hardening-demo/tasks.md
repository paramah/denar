# Tasks: Utwardzenie powiadomień (SSRF) i dane demo (009-hardening-demo)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race` dotkniętych pakietów; na końcu `task verify`.

- [x] **T-901 Polityka celów** — `domain/notification/target.go` (zakresy, `TargetPolicy`, `CheckHost`, `CheckTargets`) (+tabela zakresów v4/v6/mapped/NAT64). FR-901, FR-902, FR-904.
- [x] **T-902 Walidacja kanałów** — `Deps.NotifyTargets`, `CreateChannel`/`UpdateChannel` (+testy 422 i opt-in). FR-904.
- [x] **T-903 Klient i błędy** — `notifyhttp.NewClient` (dialer `Control`, bez proxy), `Do` bez ciała w błędzie + log, Matrix `errcode`, pole `Log` notifierów (+testy dialera, allowlisty, proxy, przekierowania, braku ciała; adaptery na kliencie z opt-in). FR-901, FR-903, FR-905, FR-906.
- [x] **T-904 Konfiguracja i wiring** — `DENAR_NOTIFY_ALLOW_PRIVATE`, `DENAR_NOTIFY_ALLOWED_CIDRS` (+testy), `wireNotifications`, e2e (422, `HTTP 500` bez ciała), `web/playwright.config.ts`, README, `.env.example`, `docker-compose.yml`. FR-902.
- [x] **T-905 Seed demo** — `cmd/denar/demo.go` (`denar demo`, zegar demo, use case'y), `e2e_demo_test.go` (liczby, inwariant, idempotencja, komenda). FR-910…FR-912.
- [x] **T-906 Szybki start** — `task demo`, `task dev`, `.env.dev.example`, README „Szybki start (lokalnie)”, smoke test (`/healthz`, logowanie, `/api/me`, subskrypcje, MCP `whoami`). FR-913.
- [x] **T-907 Zamknięcie** — PLAN.md (przenumerowanie 010–012, tabela), INDEX.md, `task verify`.
