[English](README.md) | Polski

# Denar

[![CI](https://github.com/paramah/denar/actions/workflows/ci.yml/badge.svg)](https://github.com/paramah/denar/actions/workflows/ci.yml)
[![Licencja: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Self-hosted budżet domowy z portfelami w wielu walutach, kopertami w stylu YNAB i śledzeniem subskrypcji, dostępny przez Web UI, REST i serwer MCP dla asystentów AI, z osobnymi uprawnieniami dla domowników i botów.

Denar to jedno binarium Go (Clean Architecture, SQLite), które serwuje REST API, serwer MCP i Web UI osadzone w binarium.

## Funkcje

- **Konta i gospodarstwa:** logowanie hasłem (argon2id) i OIDC z PKCE, sesje, wiele gospodarstw na instancję z rolami `owner/admin/member/viewer`, zaproszenia, audit log.
- **Boty:** tokeny `dnr_…` ze scope'ami, jako Bearer w REST i w MCP. Bot nie dostaje więcej niż jego twórca: ograniczają go scope'y, bieżąca rola twórcy i portfele, które twórca widzi.
- **Portfele i transakcje:** portfele w wielu walutach (współdzielone i prywatne), transakcje z kwotą bazową po kursie NBP (cache, ręczny kurs, praca offline), przelewy, splity, kontrahenci, salda.
- **Budżet kopertowy (YNAB):** grupy, koperty z celami, przydziały, przeniesienia, auto-przydział, Ready-to-Assign z regułą overspendingu YNAB.
- **Subskrypcje:** słownik dostawców, cena w dowolnej walucie, cykle bez dryfu, okresy próbne, historia cen, oczekiwane obciążenia dopasowywane do transakcji.
- **Powiadomienia:** w aplikacji, e-mail (SMTP), webhook (z podpisem HMAC), ntfy, Telegram, Matrix; ochrona SSRF kanałów wychodzących.
- **Serwer MCP** dla asystentów AI (Claude Code, Claude Desktop i inne klienty MCP).
- **Import CSV** wyciągów z presetami mBank, ING, PKO i Revolut, pomijaniem duplikatów i cofaniem.
- **Web UI po polsku i angielsku:** język z profilu użytkownika, a bez niego z przeglądarki (domyślnie polski). Powiadomienia powstają w języku odbiorcy.

Zrealizowane etapy: 001–008 (rdzeń, portfele i transakcje, budżet kopertowy, subskrypcje, powiadomienia, MCP, Web UI, import CSV) i 009-hardening-demo (ochrona SSRF kanałów, dane demo). Szczegóły: `.ai/sdd/PLAN.md`, `.ai/sdd/specs/`.

## Szybki start

### Lokalne demo

Wymagania: Go 1.27, Node.js (Web UI), [Task](https://taskfile.dev), `openssl`.

```sh
task demo
```

To buduje binarium z Web UI, tworzy (albo odtwarza) bazę `.dev/demo.db` z danymi demo i uruchamia serwer na **http://localhost:8080**. Zaloguj się:

| Login | Hasło |
|---|---|
| `demo@denar.local` | `denar-demo-2026` |

> **Uwaga:** hasło demo jest stałe i jawne (README, kod) — tylko do lokalnego oglądania. Nie wystawiaj instancji demo do sieci. Inne hasło: `./denar demo --password …` albo `DENAR_DEMO_PASSWORD` (działa przy pierwszym utworzeniu konta).

`task demo` wypisuje na końcu token bota „Asystent AI” (odczyt + zapis transakcji) i gotowe polecenie dla Claude Code:

```sh
claude mcp add --transport http denar http://localhost:8080/mcp --header "Authorization: Bearer dnr_…"
```

Każde uruchomienie `task demo` robi gospodarstwo demo od nowa (`denar demo --reset`), więc token bota się zmienia. Klucz sekretów kanałów (`.dev/master.key`) powstaje przy pierwszym uruchomieniu.

Co obejrzeć w gospodarstwie „Dom (demo)” (PLN, ~3 miesiące historii):

- **Pulpit** — salda sześciu portfeli (konto, karta, gotówka, konto USD, gotówka EUR, prywatne oszczędności poza budżetem), Ready-to-Assign, nadchodzące odnowienia, koszt subskrypcji.
- **Budżet** — grupy Stałe / Codzienne / Subskrypcje / Cele / Przyjemności z przydziałami na każdy miesiąc; w bieżącym miesiącu koperta **Restauracje** jest przekroczona; cele: Wakacje (kwota do daty) i Poduszka finansowa (miesięcznie).
- **Portfele i transakcje** — pensja, czynsz, zakupy (Biedronka, Lidl), paliwo (Orlen), media, restauracje, przelewy na oszczędności i spłata karty, wymiana PLN → USD i PLN → EUR, kolacja w Berlinie w EUR. Kursy USD/EUR są ustawione ręcznie na każdy dzień (działa bez sieci, bez NBP).
- **Subskrypcje** — Netflix, Spotify (z podwyżką ceny), ChatGPT Plus i Claude Pro (20 USD z konta USD), iCloud+ (jedna przegapiona płatność), YouTube Premium w okresie próbnym kończącym się za kilka dni, Adobe (anulowana); kolejka dopasowań z najnowszymi płatnościami do potwierdzenia.
- **Powiadomienia** — dzwonek z nieprzeczytanymi: zbliżające się odnowienia, koniec okresu próbnego, brak płatności, przekroczona koperta (starsze są przeczytane).
- **Ustawienia** — członkowie, bot „Asystent AI”, kanały powiadomień (lokalnie `DENAR_NOTIFY_ALLOW_PRIVATE=true`, więc webhook do n8n na tym komputerze zadziała).
- **MCP** — po `claude mcp add …` zapytaj np. „co odnawia się w tym tygodniu?” albo „ile wydałem na restauracje w tym miesiącu?”.

Dane demo na dowolnej bazie: `DENAR_DB=moja.db ./denar demo` (odmawia, gdy gospodarstwo demo już jest; `--reset` je usuwa i tworzy od nowa).

### Docker

Obrazy dla `linux/amd64` i `linux/arm64` są publikowane w GitHub Container Registry jako `ghcr.io/paramah/denar` (tagi: `latest`, `<major>.<minor>`, `<wersja>`):

```sh
docker run -d --name denar -p 127.0.0.1:8080:8080 -v denar-data:/data \
  -e DENAR_AUTO_MIGRATE=true \
  -e DENAR_ADMIN_EMAIL=admin@example.com -e DENAR_ADMIN_PASSWORD='hasło-co-najmniej-12' \
  -e DENAR_MASTER_KEY="$(openssl rand -base64 32)" \
  ghcr.io/paramah/denar:latest
```

Obraz działa jako użytkownik bez uprawnień roota, trzyma bazę w wolumenie `/data` (`DENAR_DB=/data/denar.db`), nasłuchuje na porcie 8080 i ma healthcheck na `/healthz`. Zachowaj klucz: jego zmiana unieważnia zapisane sekrety kanałów.

### Docker Compose

```sh
cp .env.example .env   # ustaw DENAR_ADMIN_PASSWORD (min. 12 znaków)
task compose:up
```

To buduje obraz z repozytorium i uruchamia kontener: port 8080 na 127.0.0.1, dane w wolumenie `denar-data`. Ruch nie jest szyfrowany, więc do sieci wystawia go reverse proxy z TLS, a nie ten port (`DENAR_BIND`, `DENAR_PORT` w `.env`). Do tego `task compose:down` (dane zostają w wolumenie) i `task compose:logs`. Inny silnik: `COMPOSE="podman compose" task compose:up`.

### Bez Taska

```sh
go build -o denar ./cmd/denar
export DENAR_DB=./denar.db DENAR_ADMIN_EMAIL=admin@example.com DENAR_ADMIN_PASSWORD='hasło-co-najmniej-12'
./denar migrate
./denar serve
```

Samo `go build` osadza Web UI tylko wtedy, gdy `web/dist` jest już zbudowane (`task web:build`); binaria z wydań i obraz Dockera zawsze je mają.

### Komendy

| Komenda | Opis |
|---|---|
| `denar serve` | serwer HTTP (`/api`, `/healthz`, Web UI pod `/`) |
| `denar migrate` | aktualizuje schemat bazy; `serve` bez `DENAR_AUTO_MIGRATE` odmawia startu na starym schemacie |
| `denar token create …` | tworzy bota i wypisuje jego token |
| `denar demo [--reset] [--password …]` | migruje bazę i tworzy gospodarstwo demo z kontem `demo@denar.local` (tylko lokalnie) |
| `denar version` | wersja |

## REST API w praktyce

Kontrakt REST: [`api/openapi.yaml`](api/openapi.yaml). Sprawdzenie na działającym serwerze:

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

Token bota z CLI (bez UI): `./denar token create --household <ID> --name cron --scopes reports:read [--as email] [--expires 2027-01-01T00:00:00Z]` — token trafia na stdout.

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

## Konfiguracja

Denara konfigurują zmienne środowiskowe `DENAR_*`. [`.env.example`](.env.example) to opisany szablon dla Docker Compose.

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
| `DENAR_OIDC_ISSUER`, `DENAR_OIDC_CLIENT_ID`, `DENAR_OIDC_CLIENT_SECRET` | — | OIDC (komplet włącza, np. Pocket ID); redirect URI `<BASE_URL>/api/auth/callback` |
| `DENAR_OIDC_AUTO_CREATE` | `false` | konto dla każdego użytkownika IdP (inaczej tylko zaproszone e-maile) |
| `DENAR_AUTO_MIGRATE` | `false` | migracja przy starcie `serve` |
| `DENAR_TRUSTED_PROXIES` | — | CIDR-y zaufanych reverse proxy (`X-Forwarded-For`), po przecinku |
| `DENAR_INSTANCE_NAME` | `Denar` | nazwa na ekranie logowania (do 64 znaków) |
| `DENAR_BOT_RATE` | `120` | limit żądań bota na minutę (10–6000) |
| `DENAR_NBP_URL` | `https://api.nbp.pl` | API kursów NBP (tabela A) |
| `DENAR_CHARGE_HORIZON` | `90` | na ile dni naprzód generować oczekiwane obciążenia subskrypcji (7–730) |
| `DENAR_SUBSCRIPTION_INTERVAL` | `1h` | co ile uruchamiać joby subskrypcji (obciążenia, przegapione, przypomnienia) |
| `DENAR_FX_INTERVAL` | `30m` | co ile uzupełniać kwoty bazowe transakcji zapisanych bez kursu |
| `DENAR_MASTER_KEY` / `DENAR_MASTER_KEY_FILE` | — | klucz szyfrowania sekretów kanałów: 32 bajty w base64 (`openssl rand -base64 32`) |
| `DENAR_SMTP_HOST` | — | serwer SMTP (włącza kanał e-mail i zaproszenia e-mailem) |
| `DENAR_SMTP_PORT` | `587` (`465` dla `tls`, `25` dla `none`) | port SMTP |
| `DENAR_SMTP_USERNAME`, `DENAR_SMTP_PASSWORD` / `DENAR_SMTP_PASSWORD_FILE` | — | logowanie SMTP (AUTH PLAIN, tylko po TLS) |
| `DENAR_SMTP_FROM` | — | nadawca, np. `Denar <denar@example.com>` (wymagany przy SMTP) |
| `DENAR_SMTP_TLS` | `starttls` | `starttls`, `tls` (od początku) albo `none` (relay w zaufanej sieci) |
| `DENAR_NOTIFY_INTERVAL` | `30s` | co ile rozsyłać zdarzenia i wysyłać dostawy |
| `DENAR_BUDGET_CHECK_INTERVAL` | `15m` | co ile sprawdzać koperty (przekroczenia, terminy celów) |
| `DENAR_GOAL_DUE_DAYS` | `7` | ile dni przed terminem celu koperty przypominać (1–60) |
| `DENAR_TELEGRAM_API_URL` | `https://api.telegram.org` | adres Bot API Telegrama (np. własny serwer Bot API; w sieci lokalnej wymaga `DENAR_NOTIFY_ALLOWED_CIDRS`) |
| `DENAR_NOTIFY_ALLOW_PRIVATE` | `false` | kanały powiadomień mogą wysyłać do sieci prywatnych i pętli zwrotnej (bez link-local) — patrz [Ochrona SSRF](#ochrona-ssrf) |
| `DENAR_NOTIFY_ALLOWED_CIDRS` | — | sieci (CIDR albo adresy, po przecinku), do których kanały mogą wysyłać mimo blokady, np. `192.168.1.0/24` |
| `DENAR_DEV_FAKE_CONNECTOR` | `false` | dodaje udawany bank `fake` do providerów połączeń (tylko do rozwoju, nie w produkcji) |

Tylko Docker Compose: `DENAR_PORT` (publikowany port, `8080`), `DENAR_BIND` (adres hosta, `127.0.0.1`; zmień na `0.0.0.0` tylko w zaufanej sieci, ruch nie jest szyfrowany), `DENAR_VERSION` (tag obrazu i wersja w buildzie). `denar demo` czyta też `DENAR_DEMO_PASSWORD`.

## MCP: podłączenie Claude i innych asystentów AI

Serwer MCP działa pod `http://localhost:8080/mcp` (Streamable HTTP, bez sesji, odpowiedzi JSON). Uwierzytelnia **wyłącznie token bota**: `Authorization: Bearer dnr_…`. Bot należy do jednego gospodarstwa, więc narzędzia nie pytają o nie; widzi i zmienia tylko to, na co pozwalają jego scope'y, bieżąca rola twórcy (bot viewera tylko czyta) i lista portfeli. Zmiany bota trafiają do audytu, a jego transakcje mają `source: bot`. Pełny kontrakt: [`api/mcp-tools.md`](api/mcp-tools.md).

1. Utwórz bota (Ustawienia → Boty w Web UI, `POST /api/households/<ID>/bots` albo CLI) z potrzebnymi scope'ami, np. asystent wydatków:
   ```sh
   ./denar token create --household <ID> --name Claude \
     --scopes wallets:read,transactions:read,transactions:write,budget:read,budget:write,subscriptions:read,subscriptions:write,reports:read
   ```
2. Podłącz klienta:
   - **Claude Code:**
     ```sh
     claude mcp add --transport http denar http://localhost:8080/mcp --header "Authorization: Bearer dnr_…"
     ```
   - **Claude Desktop / claude.ai i inne klienty z „remote MCP”:** dodaj serwer HTTP o adresie `https://<twoja-instancja>/mcp` z nagłówkiem `Authorization: Bearer dnr_…`. Klient bez obsługi nagłówków może użyć mostu, np. w `claude_desktop_config.json`:
     ```json
     { "mcpServers": { "denar": { "command": "npx",
         "args": ["-y", "mcp-remote", "http://localhost:8080/mcp", "--header", "Authorization: Bearer dnr_…"] } } }
     ```
3. Zapytaj np. „co odnawia się w tym tygodniu?” albo „wydałem 42,50 w Biedronce z karty na jedzenie”.

Narzędzia (portfele, koperty, kontrahenci i subskrypcje po id albo nazwie; kwoty jako `"12.34"`, daty `YYYY-MM-DD`, miesiące `YYYY-MM`):

| Narzędzie | Scope |
|---|---|
| `whoami`, `list_vendors` | — |
| `list_wallets`, `get_balances` | `wallets:read` |
| `search_transactions`, `list_payees` | `transactions:read` |
| `record_expense`, `record_income`, `transfer` | `transactions:write` (sugestie obciążeń po `record_expense` — także `subscriptions:read`, `transactions:read`) |
| `list_envelopes`, `budget_overview` | `budget:read` |
| `assign_to_envelope`, `move_between_envelopes` | `budget:write` |
| `spending_by` (koperta / kontrahent / dostawca / portfel) | `reports:read` |
| `list_subscriptions`, `get_subscription`, `upcoming_renewals`, `subscription_costs`, `list_charges` | `subscriptions:read` |
| `create_subscription`, `update_subscription`, `change_subscription_price`, `skip_charge` | `subscriptions:write` |
| `confirm_charge_match` | `subscriptions:write` + `transactions:read` |
| `post_charge` | `subscriptions:write` + `transactions:write` |
| `list_notifications` (powiadomienia twórcy bota) | `notifications:read` |
| `list_imports` | `transactions:read` |

Brak scope'u daje błąd `forbidden` z nazwą brakującego scope'u. Limit żądań: `DENAR_BOT_RATE` na minutę (liczony osobno dla MCP i REST).

## Powiadomienia

Zdarzenia (odnowienie za N dni, koniec okresu próbnego, zmiana ceny, przegapiona płatność, przekroczona koperta, zbliżający się termin celu koperty) trafiają do outboxu, a job co `DENAR_NOTIFY_INTERVAL` zamienia je na **powiadomienia w aplikacji** i **dostawy na kanały** wybrane w preferencjach. Odbiorcy: członkowie gospodarstwa (każda rola), którzy widzą portfel zdarzenia — subskrypcja z prywatnego portfela nie trafia do tych, którzy go nie widzą. Treści są w języku odbiorcy (polski albo angielski), z linkiem do Web UI (`DENAR_BASE_URL`).

- **Kanały** (per użytkownik, `/api/me/channels`): `email` (adres, domyślnie e-mail konta; wymaga SMTP), `webhook` (POST JSON, opcjonalny sekret HMAC → nagłówek `X-Denar-Signature: sha256=<hex>`), `ntfy` (serwer, temat, opcjonalny token), `telegram` (chat id + token bota), `matrix` (homeserver, id pokoju `!…:serwer`, access token). „Wyślij test”: `POST /api/me/channels/<id>/test`.
- **Preferencje** (per użytkownik i gospodarstwo, `/api/me/households/<ID>/notification-preferences`): dla każdego typu zdarzenia in-app tak/nie i lista kanałów. Domyślnie: wszystko w aplikacji, żadnych kanałów.
- **Sekrety** kanałów (tokeny, klucz HMAC) są szyfrowane AES-256-GCM kluczem `DENAR_MASTER_KEY` i nigdy nie wracają w API (`has_secret`). Bez klucza kanały z sekretami są niedostępne; e-mail, in-app, webhook bez podpisu i ntfy bez tokenu działają. Klucz: `openssl rand -base64 32`. Zmiana klucza unieważnia zapisane sekrety (trzeba je ustawić ponownie).
- **Ponawianie**: nieudana dostawa wraca po 1, 2, 4, 8, 16 min; po 6 próbach jest `failed`, a kanał pokazuje `last_error`.
- **Retencja**: przeczytane powiadomienia znikają po 180 dniach, zakończone dostawy po 30, zdarzenia po 400.
- **Zaproszenia e-mailem**: przy skonfigurowanym SMTP `POST /api/households/<ID>/invitations` z `"send_email": true` wysyła link (`email_sent` w odpowiedzi).

### Ochrona SSRF

Kanały webhook, ntfy i Matrix wysyłają tylko na adresy publiczne. Serwer odrzuca adresy pętli zwrotnej, sieci prywatne (RFC 1918), link-local (w tym `169.254.169.254` — metadane chmury), CGNAT `100.64.0.0/10`, ULA `fc00::/7`, multicast, adresy nieokreślone i IPv4 zapisane w IPv6. Kontrola działa dwa razy: przy zapisie kanału (literalny IP albo `localhost` → 422 „adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE”) i przy każdym połączeniu, na adresie po rozwiązaniu DNS, więc nazwa wskazująca na sieć lokalną (także po DNS rebinding) też nie przejdzie. Zmienne proxy (`HTTP(S)_PROXY`) nie są używane przez powiadomienia, przekierowania nie są śledzone.

Self-hostowany ntfy, n8n albo Home Assistant w LAN? To wymaga jawnej zgody. Otwórz tylko jego sieć: `DENAR_NOTIFY_ALLOWED_CIDRS=192.168.1.0/24` (zalecane) albo wszystkie sieci prywatne: `DENAR_NOTIFY_ALLOW_PRIVATE=true` (link-local zostaje zamknięty; otwiera go tylko jawny CIDR). Pamiętaj, że każdy użytkownik instancji może wtedy wysyłać żądania do tych sieci. Błąd kanału (`last_error`, wynik „Wyślij test”) to tylko `HTTP <kod>` — treść odpowiedzi trafia wyłącznie do logu serwera.

```sh
# ntfy: powiadomienia na telefon (aplikacja ntfy, subskrybuj temat denar-dom-7f3a)
curl -s -b jar -H 'Content-Type: application/json' localhost:8080/api/me/channels \
  -d '{"type":"ntfy","name":"Telefon","config":{"topic":"denar-dom-7f3a"}}'
# Telegram: utwórz bota u @BotFather, napisz do niego, chat_id odczytasz z https://api.telegram.org/bot<TOKEN>/getUpdates
curl -s -b jar -H 'Content-Type: application/json' localhost:8080/api/me/channels \
  -d '{"type":"telegram","name":"Telegram","config":{"chat_id":"123456789"},"secrets":{"bot_token":"123:ABC…"}}'
curl -s -b jar -X POST localhost:8080/api/me/channels/<CH>/test        # {"ok":true,…}
# odnowienia na Telegram i w aplikacji, reszta tylko w aplikacji
curl -s -b jar -X PUT -H 'Content-Type: application/json' localhost:8080/api/me/households/$H/notification-preferences \
  -d '{"items":[{"type":"subscription.renewal_upcoming","in_app":true,"channel_ids":["<CH>"]}]}'
curl -s -b jar 'localhost:8080/api/me/notifications?unread=true'
```

Weryfikacja podpisu webhooka: `HMAC-SHA256(sekret, surowe body)` w hex, porównane ze stałym czasem z wartością po `sha256=`; `X-Denar-Delivery` to id dostawy (ponowienie ma to samo id).

## Import CSV

Wyciąg z banku (CSV) trafia do portfela przez podgląd i import. Każdy wiersz staje się transakcją `source=import` (ujemna kwota — wydatek, dodatnia — przychód, status `cleared`); kontrahent i koperta przychodzą z reguł kontrahentów (nazwa albo wzorzec w surowym opisie), surowy opis zostaje w notatce, a nowe transakcje są od razu dopasowywane do oczekiwanych obciążeń subskrypcji (próg 80 pkt, jak w sugestiach). Duplikaty są pomijane po `external_id`: z kolumny ID pliku albo — bez niej — hash daty, kwoty, opisu i numeru wystąpienia w pliku, więc ten sam plik wgrany drugi raz nie dodaje niczego. Partię można cofnąć (usuwa jej transakcje i odpina obciążenia).

- **Presety** (`GET /api/import-presets`): `generic` (wykrywa kodowanie, separator, format daty i liczb, kolumny po typowych nazwach), `mbank`, `ing`, `pko`, `revolut`. Presety banków powstały z opisu formatów eksportu i testów na próbkach syntetycznych — jeśli bank zmienił kolumny, nadpisz mapowanie w opcjach (`columns`).
- **Opcje** (część `options`, JSON): `wallet_id` (wymagany), `preset`, `delimiter` (`;`, `,`, `|`, `tab`), `encoding` (`utf-8`, `windows-1250`), `date_format` (np. `DD.MM.YYYY`), `decimal_separator` (`,`, `.`), `skip_rows`, `has_header`, `invert_amount`, `accept_states`, `columns` (`date`, `amount` albo `debit`/`credit`, `payee`, `memo`, `external_id`, `currency`, `fee`, `state` — listy nazw nagłówka lub numerów kolumn od 1), w podglądzie `preview_limit`.
- **Limity**: plik do 5 MiB, do 10 000 wierszy; błędne wiersze (zła data, kwota, inna waluta niż portfel) nie przerywają importu — są w `errors` partii.
- **Uprawnienia**: import i cofnięcie — `transactions:write` (bot też, przez REST); lista partii — `transactions:read`; auto-dopasowanie subskrypcji wymaga dodatkowo `subscriptions:write`. MCP: `list_imports`.

```sh
# podgląd: pierwsze wiersze, duplikaty, błędy, wykryte kodowanie i nagłówek
curl -s -b jar localhost:8080/api/households/$H/imports/csv/preview \
  -F file=@lista_operacji.csv -F 'options={"wallet_id":"<W>","preset":"mbank"};type=application/json'
# import (201: liczniki created/duplicate/error/matched)
curl -s -b jar localhost:8080/api/households/$H/imports/csv \
  -F file=@lista_operacji.csv -F 'options={"wallet_id":"<W>","preset":"mbank"};type=application/json'
# własne mapowanie pliku bez nagłówka: data w 1. kolumnie, opis w 2., kwota w 4.
curl -s -b jar localhost:8080/api/households/$H/imports/csv \
  -F file=@eksport.csv -F 'options={"wallet_id":"<W>","has_header":false,"delimiter":";","date_format":"DD.MM.YYYY","columns":{"date":["1"],"payee":["2"],"amount":["4"]}}'
curl -s -b jar localhost:8080/api/households/$H/imports            # partie
curl -s -b jar -X DELETE localhost:8080/api/households/$H/imports/<ID>   # cofnięcie
```

**Połączenia z bankami i dostawcami** (PSD2, OpenAI, Apple) mają na razie tylko model i porty (`BankConnector`, `BillingConnector`); `GET /api/households/<ID>/connections` i `GET /api/connection-providers` są do odczytu. `DENAR_DEV_FAKE_CONNECTOR=1` dodaje udawany bank `fake` do rozwoju.

## Rozwój

Wymagania: Go 1.27, Node.js 22 (`web/.nvmrc`), [Task](https://taskfile.dev), `openssl`.

```sh
cp .env.dev.example .env.dev      # ustaw DENAR_ADMIN_PASSWORD (min. 12 znaków)
task dev                          # API na :8080 (baza .dev/denar.db) i Vite z HMR na http://localhost:5173
```

Samo API: `task dev:api`; sam frontend: `task web:dev`. Dane demo w bazie deweloperskiej: `DENAR_DB=.dev/denar.db go run ./cmd/denar demo`.

```sh
task verify     # gofmt, go vet, web build + testy, go test -race, go build
task generate   # regeneracja kodu z api/openapi.yaml
task web:types  # regeneracja web/src/api/schema.d.ts z api/openapi.yaml (sprawdza ją task web:types:check)
task web:e2e    # buduje binarium z Web UI i uruchamia testy Playwright
```

Przed pierwszym `task web:e2e` zainstaluj przeglądarkę: `task web:e2e:install`. CI (GitHub Actions) uruchamia `task web:types:check`, `task verify` i `task web:e2e` przy każdym pushu do `main` i dla pull requestów.

## Architektura

- **Jedno binarium:** `cmd/denar` (komendy `serve`, `migrate`, `token`, `demo`, `version`); SPA w React z `web/` Vite buduje do `web/dist`, a `go:embed` osadza je w binarium (`web/embed.go`).
- **Clean Architecture:** zależności tylko do środka, `adapter → usecase → domain` (pilnuje tego `internal/arch_test.go`). Reguły biznesowe (salda, koperty, cykle subskrypcji, uprawnienia) należą do `internal/domain` i `internal/usecase`, nie do adapterów.
- **Kwoty** zawsze jako `int64` w jednostkach minimalnych plus waluta; nigdy `float64`.
- **Dane:** SQLite (sterownik w czystym Go, bez CGO) z migracjami.
- **Interfejsy:** REST ([`api/openapi.yaml`](api/openapi.yaml), kod generowany przez oapi-codegen), MCP ([`api/mcp-tools.md`](api/mcp-tools.md)) i Web UI na REST.
- **Kontrola dostępu:** każdy use case filtruje po portfelach widocznych dla principala; ukryte zasoby zwracają „nie znaleziono”. Bot nigdy nie dostaje więcej niż jego twórca.

## Proces

Denar powstaje według specyfikacji. Steering produktowy i techniczny jest w `.ai/steering/`, plan w `.ai/sdd/PLAN.md`, a specyfikacja każdego etapu (wymagania, projekt, zadania) w `.ai/sdd/specs/NNN-*/`. Instrukcje dla agentów: [`AGENTS.md`](AGENTS.md).

## Licencja

[MIT](LICENSE) © 2026 Aleksander Cynarski
