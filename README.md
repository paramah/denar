# Denar

Self-hosted budżet domowy: gospodarstwa, portfele, transakcje, koperty (YNAB), subskrypcje. Jedno binarium Go (Clean Architecture, SQLite), REST + (od etapu 006) MCP, Web UI osadzone w binarium (od etapu 007).

Stan: **etapy 001-core, 002-wallets-transactions i 003-envelope-budget** — konta (hasło argon2id, OIDC z PKCE), sesje, gospodarstwa z rolami `owner/admin/member/viewer`, zaproszenia, boty z tokenami `dnr_…` i scope'ami, Bearer w REST, audit log; portfele w wielu walutach (współdzielone i prywatne), transakcje z kwotą bazową po kursie NBP (cache, ręczny kurs, praca offline), przelewy, splity, kontrahenci, salda; budżet kopertowy (grupy, koperty z celami, przydziały, przeniesienia, auto-przydział, Ready-to-Assign z regułą overspendingu YNAB). Szczegóły: `.ai/sdd/PLAN.md`, `.ai/sdd/specs/`.

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
# portfel z saldem otwarcia i wydatek (kwoty zawsze jako string dziesiętny + waluta)
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"name":"Konto","type":"checking","currency":"PLN","opening_balance":{"amount":"1000.00","currency":"PLN"}}' \
  localhost:8080/api/households/<ID>/wallets
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"wallet_id":"<WALLET>","amount":{"amount":"-25.99","currency":"PLN"},"payee_name":"Biedronka"}' \
  localhost:8080/api/households/<ID>/transactions
curl -s -b jar localhost:8080/api/households/<ID>/wallets
```

Transakcja w walucie innej niż bazowa dostaje kwotę bazową po kursie NBP z dnia transakcji. Bez sieci zapisuje się z `rate_pending: true`, a job (co `DENAR_FX_INTERVAL`) uzupełnia ją później; owner/admin może też ustawić kurs ręcznie: `PUT /api/households/<ID>/fx-rates/overrides`.

### Budżet kopertowy

Koperty i przydziały są w walucie bazowej gospodarstwa; miesiąc budżetowy to `YYYY-MM` (od `month_start_day`). Aktywność koperty liczy się z transakcji portfeli on-budget z jej `envelope_id` (także linii splitu); przychody i wydatki bez koperty trafiają prosto do Ready-to-Assign. Przekroczenie (ujemne `available`) zeruje się w kolejnym miesiącu i pomniejsza jego Ready-to-Assign.

```sh
B=localhost:8080/api/households/<ID>/budget
curl -s -b jar -H 'Content-Type: application/json' -d '{"name":"Codzienne"}' $B/envelope-groups
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"group_id":"<GROUP>","name":"Jedzenie","goal":{"type":"monthly","amount":{"amount":"800.00","currency":"PLN"}}}' $B/envelopes
curl -s -b jar -X PUT -H 'Content-Type: application/json' -d '{"amount":{"amount":"500.00","currency":"PLN"}}' \
  $B/months/2026-10/assignments/<ENVELOPE>
curl -s -b jar -X POST $B/months/2026-10/auto-assign          # uzupełnia cele z Ready-to-Assign
curl -s -b jar $B/months/2026-10                              # grupy → koperty, ready_to_assign, incomplete
```

Przeniesienie: `POST $B/months/<M>/moves` (`from_envelope_id`, `to_envelope_id`, `amount`); transakcje koperty: `GET $B/months/<M>/envelopes/<ENVELOPE>/transactions`. Koperty z transakcjami nie da się usunąć — ukryj ją (`PATCH … {"hidden":true}`). Boty potrzebują `budget:read`/`budget:write`.

### Subskrypcje

Subskrypcja ma dostawcę ze słownika (`GET /api/vendors`: openai, anthropic, apple, netflix…, `other`), cenę w dowolnej walucie (nie musi być walutą portfela), cykl `{unit: day|week|month|year, every, anchor}` liczony zawsze od `anchor` (31 → 28/29.02 → 31.03, bez dryfu), opcjonalny trial, portfel płatności i kopertę. Z subskrypcji powstają **oczekiwane obciążenia** (`charges`) do horyzontu `DENAR_CHARGE_HORIZON` dni; job (co `DENAR_SUBSCRIPTION_INTERVAL`) dopisuje kolejne, oznacza nieopłacone po oknie jako `missed` i zapisuje przypomnienia w outboxie zdarzeń (wysyłka — etap 005).

```sh
S=localhost:8080/api/households/<ID>
curl -s -b jar -H 'Content-Type: application/json' -d '{"name":"ChatGPT Plus","vendor":"openai",
  "price":{"amount":"20.00","currency":"USD"},"cycle":{"unit":"month","anchor":"2026-01-31"},
  "wallet_id":"<WALLET>","envelope_id":"<ENVELOPE>","reminder_offsets":[7,1]}' $S/subscriptions
curl -s -b jar "$S/charges?status=expected&status=missed"          # kolejka obciążeń
curl -s -b jar $S/charges/<CHARGE>/suggestions                     # transakcje z punktacją 0–100
curl -s -b jar -H 'Content-Type: application/json' -d '{"transaction_id":"<TX>"}' $S/charges/<CHARGE>/match
curl -s -b jar -X POST $S/charges/<CHARGE>/post                    # albo: zaksięguj wydatek z obciążenia
curl -s -b jar -H 'Content-Type: application/json' \
  -d '{"effective_from":"2026-12-01","price":{"amount":"25.00","currency":"USD"}}' $S/subscriptions/<SUB>/prices
curl -s -b jar $S/subscriptions/summary                            # koszt mies./rocz. w walucie bazowej
curl -s -b jar "$S/subscriptions/upcoming?days=7"                  # co się odnawia
```

Statusy subskrypcji: `trial` (obciążenia od `trial_ends_at`), `active`, `paused` (bez obciążeń), `cancelled` (bez obciążeń od `ends_at`). Obciążenia: `expected`, `matched`, `missed`, `skipped` (`POST …/skip`), `posted`; `POST …/unmatch` odłącza transakcję. Dla wydatku zapisanego ręcznie pasujące obciążenia daje `GET $S/transactions/<TX>/charge-suggestions`. Subskrypcja z portfelem jest widoczna tylko dla widzących ten portfel; bez portfela — dla całego gospodarstwa. Boty potrzebują `subscriptions:read`/`subscriptions:write` (sugestie i potwierdzenie także `transactions:read`, zaksięgowanie `transactions:write`).

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
| `DENAR_NBP_URL` | `https://api.nbp.pl` | API kursów NBP (tabela A) |
| `DENAR_CHARGE_HORIZON` | `90` | na ile dni naprzód generować oczekiwane obciążenia subskrypcji (7–730) |
| `DENAR_SUBSCRIPTION_INTERVAL` | `1h` | co ile uruchamiać joby subskrypcji (obciążenia, przegapione, przypomnienia) |
| `DENAR_FX_INTERVAL` | `30m` | co ile uzupełniać kwoty bazowe transakcji zapisanych bez kursu |

## Rozwój

```sh
task verify     # gofmt, go vet, (web build + test, gdy istnieje web/), go test -race, build
task generate   # regeneracja kodu z api/openapi.yaml
```

Architektura: `adapter → usecase → domain` (pilnuje `internal/arch_test.go`). Kontrakt REST: `api/openapi.yaml`. Instrukcje dla agentów: `AGENTS.md`.
