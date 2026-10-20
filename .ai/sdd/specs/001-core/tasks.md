# Tasks: Core (001-core)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `go test -race ./...` dla dotkniętych pakietów; na końcu `task verify`.

## MVP Slice

- [x] **T-001 Szkielet repo** — `go.mod`, `internal/arch_test.go`, `internal/platform/{clock,id,password,token}` (+testy), `web/embed.go`, `web/dist/.gitkeep`, `Taskfile.yml`, `.gitignore`. Kryterium: `go test ./internal/...` zielone, arch_test działa. FR-001, NFR-003.
- [x] **T-002 Konfiguracja** — `internal/platform/config` (`DENAR_*`, `Secret`, walidacja). Testy tabelaryczne. FR-002, FR-003, FR-012, FR-013, FR-023.
- [x] **T-003 Domena** — `domain.go`, `money`, `user`, `household`, `membership` (role, reguły, zaproszenia), `bot` (scope'y, efektywne), `audit`. Testy tabelaryczne. FR-020…FR-024, FR-030…FR-034, FR-040.
- [x] **T-004 Porty, fakes, portstest** — `ports.go`, `fakes/*`, `portstest/*` (users, sessions, oidc states, households, memberships, invitations, bots, audit) zielone na fake'ach. NFR-002.
- [x] **T-005 Use case'y auth** — principal, access, Authenticate, AuthenticateBot, LoginLocal, OIDC start/callback, Logout, BootstrapAdmin, GetAuthConfig, GetMe. FR-003, FR-010, FR-012, FR-013, FR-034.
- [x] **T-006 Use case'y gospodarstw i członków** — Create/List/Get/Update/Delete household, ListMembers, UpdateMemberRole, RemoveMember (+ wyłączenie botów), audyt. FR-020…FR-022, FR-024, FR-040.
- [x] **T-007 Zaproszenia** — Create/List/Revoke/Preview/Accept/Register. FR-023.
- [x] **T-008 Boty i audyt** — Create/List/Get/Update/Rotate/Delete bot, CreateBotCLI, ListAudit, macierz autoryzacji. FR-030…FR-036, FR-041.
- [x] **T-009 SQLite** — db/tx/migrate, `00001_core.sql`, repozytoria; portstest zielone na SQLite. FR-002.
- [x] **T-010 Kontrakt REST** — `api/openapi.yaml`, generacja `oapi-codegen`, `TestGeneratedCodeIsCurrent`. FR-050, FR-051.
- [x] **T-011 Adapter REST** — server, middleware (CSRF, auth cookie/Bearer, 400 przy obu, JSON), limity 429, problem+json, handlery; testy konformancji. FR-011, FR-014, FR-035, FR-051.
- [x] **T-012 OIDC i SPA** — `adapter/oidc` (kopia Straylight + testy), `adapter/spa` z zaślepką. FR-012, NFR-001.
- [x] **T-013 Composition root** — `cmd/denar` (serve, migrate, token create, version), joby sprzątające, e2e. FR-001…FR-003, FR-013, FR-036.
- [x] **T-014 Operacje** — `Dockerfile`, `docker-compose.yml`, `.env.example`, `.env.dev.example`, `README.md`; `task verify` zielone. NFR-003.

## Wynik

- Wszystkie zadania T-001…T-014 zakończone; `task verify` zielone (2026-10-21).
- Testy: domena (tabelaryczne), use case'y na fake'ach (w tym macierz autoryzacji 200 przypadków: rola × scope × status twórcy × przyznany scope, oraz FR-033 dla każdego use case'u administracyjnego), suity `portstest` na fake'ach i SQLite, konformancja REST (115 przypadków, każda zadeklarowana odpowiedź każdej operacji), e2e w `cmd/denar` (login → gospodarstwo → bot → Bearer, zaproszenie i rejestracja, audyt, rotacja, OIDC z PKCE na `oidctest`, `denar token create`).
