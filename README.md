# Denar

Self-hosted budżet domowy: gospodarstwa, portfele, transakcje, koperty (YNAB), subskrypcje. Jedno binarium Go (Clean Architecture, SQLite), REST + (od etapu 006) MCP, Web UI osadzone w binarium (od etapu 007).

Stan: **etap 001-core** — konta (hasło argon2id, OIDC z PKCE), sesje, gospodarstwa z rolami `owner/admin/member/viewer`, zaproszenia, boty z tokenami `dnr_…` i scope'ami, Bearer w REST, audit log. Szczegóły: `.ai/sdd/PLAN.md`, `.ai/sdd/specs/001-core/`.

## Szybki start (lokalnie)

Wymagania: Go 1.27, [Task](https://taskfile.dev).

```sh
cp .env.dev.example .env.dev      # ustaw DENAR_ADMIN_PASSWORD (min. 12 znaków)
task dev:api                      # migruje bazę (.dev/denar.db) i startuje na :8080
```

Albo bez Taska:

```sh
go build -o denar ./cmd/denar
export DENAR_DB=./denar.db DENAR_ADMIN_EMAIL=admin@example.com DENAR_ADMIN_PASSWORD='hasło-co-najmniej-12'
./denar migrate
./denar serve
```

Sprawdzenie:

```sh
curl -s localhost:8080/healthz
# logowanie (cookie sesji w pliku jar)
curl -s -c jar -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"hasło-co-najmniej-12"}' localhost:8080/api/auth/login
# gospodarstwo
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"name":"Dom","base_currency":"PLN","timezone":"Europe/Warsaw","month_start_day":1}' localhost:8080/api/households
# bot (token pokazywany raz) i wywołanie jako bot
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"name":"Claude","scopes":["wallets:read","reports:read"]}' localhost:8080/api/households/<ID>/bots
curl -s -H 'Authorization: Bearer dnr_…' localhost:8080/api/me
```

Token bota z CLI (bez UI): `./denar token create --household <ID> --name cron --scopes reports:read [--as email] [--expires 2027-01-01T00:00:00Z]` — token trafia na stdout.

Docker: `cp .env.example .env`, ustaw hasło, `task compose:up` (port 8080 na 127.0.0.1, dane w wolumenie `denar-data`).

## Komendy

| Komenda | Opis |
|---|---|
| `denar serve` | serwer HTTP (`/api`, `/healthz`, Web UI pod `/`) |
| `denar migrate` | aktualizuje schemat bazy; `serve` bez `DENAR_AUTO_MIGRATE` odmawia startu na starym schemacie |
| `denar token create …` | tworzy bota i wypisuje jego token |
| `denar version` | wersja |

## Zmienne środowiskowe

| Zmienna | Domyślnie | Opis |
|---|---|---|
| `DENAR_ADDR` | `:8080` | adres nasłuchu |
| `DENAR_DB` | `denar.db` | plik SQLite |
| `DENAR_ADMIN_EMAIL`, `DENAR_ADMIN_PASSWORD` / `DENAR_ADMIN_PASSWORD_FILE` | — | konto startowe (tworzone raz, hasło ≥ 12 znaków) |
| `DENAR_ADMIN_NAME` | `Admin` | nazwa konta startowego |
| `DENAR_LOCAL_LOGIN` | `true` | logowanie hasłem |
| `DENAR_BASE_URL` | `http://localhost:8080` | publiczny origin (wymagany przy OIDC); linki zaproszeń, redirect URI |
| `DENAR_SESSION_TTL` | `168h` | wygaśnięcie sesji po bezczynności |
| `DENAR_INVITE_TTL` | `168h` | ważność zaproszenia |
| `DENAR_OIDC_ISSUER`, `DENAR_OIDC_CLIENT_ID`, `DENAR_OIDC_CLIENT_SECRET` | — | OIDC (komplet włącza); redirect URI `<BASE_URL>/api/auth/callback` |
| `DENAR_OIDC_AUTO_CREATE` | `false` | konto dla każdego użytkownika IdP (inaczej tylko zaproszone e-maile) |
| `DENAR_AUTO_MIGRATE` | `false` | migracja przy starcie `serve` |
| `DENAR_TRUSTED_PROXIES` | — | CIDR-y zaufanych reverse proxy |
| `DENAR_INSTANCE_NAME` | `Denar` | nazwa na ekranie logowania |
| `DENAR_BOT_RATE` | `120` | limit żądań bota na minutę |

## Rozwój

```sh
task verify     # gofmt, go vet, (web build + test, gdy istnieje web/), go test -race, build
task generate   # regeneracja kodu z api/openapi.yaml
```

Architektura: `adapter → usecase → domain` (pilnuje `internal/arch_test.go`). Kontrakt REST: `api/openapi.yaml`. Instrukcje dla agentów: `AGENTS.md`.
