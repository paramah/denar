# Product Plan: denar

> Status: Approved
> Source: plan uzgodniony z użytkownikiem w sesji brainstormingu/planowania 2026-10-21
> Last Updated: 2026-10-21
> Status file: `plan:approved` — zatwierdzony przez człowieka 2026-10-21

## Steering Context

- Product: @.ai/steering/product.md
- Tech Stack: @.ai/steering/tech-stack.md
- Conventions: @.ai/steering/conventions.md
- Principles: @.ai/steering/principles.md


## Context

Katalog `/Users/paramah/Projects/eiac.dev/tools/denar` jest pusty (nie jest też repo git). Celem jest nowa aplikacja do zarządzania wydatkami i budżetem domowym, w której:
- **subskrypcje** są obywatelem pierwszej klasy (cykle, przypomnienia, historia cen, dopasowanie do transakcji),
- można prowadzić wiele **portfeli** (gotówka, konta, karty, oszczędności) w wielu walutach,
- **serwer MCP** pozwala asystentom AI czytać analizy i wprowadzać dane,
- dostęp do danych jest kontrolowany osobno dla **użytkowników i botów**,
- architektura (Clean Architecture, porty) jest gotowa na przyszłe **integracje z API banków i dostawców** (OpenAI, Apple…), choć tych integracji nie budujemy w MVP.

Decyzje użytkownika (z rundy pytań):
| Obszar | Decyzja |
|---|---|
| Stack | Jak Straylight (`../para-board`): Go 1.27, Clean Architecture, SQLite (modernc), goose, oapi-codegen strict server, MCP go-sdk, React 19 + Vite SPA osadzone w binarce, Taskfile, Docker |
| Tenancy | Self-hosted, jedna instancja, wiele **gospodarstw** (household); role per gospodarstwo i per portfel |
| Boty | Token z **scope'ami + ograniczeniem do portfeli** |
| Waluty | Wiele walut, przeliczenia do waluty bazowej gospodarstwa po kursie NBP |
| Subskrypcje MVP | cykl + następna płatność, przypomnienia, historia cen, dopasowanie do transakcji |
| Budżet | **Koperty (YNAB-style)** |
| MCP | zapytania/analizy + wprowadzanie danych, transport **remote HTTP** |
| Auth | lokalne hasło + OIDC |
| Powiadomienia | w aplikacji, SMTP, webhook/ntfy, Telegram/Matrix |

Założenia (do korekty): moduł `github.com/paramah/denar`, prefiks env `DENAR_`, prefiks tokenów botów `dnr_`, dokumentacja i komentarze po polsku jak w Straylight.

## Szkielet (przejęty ze Straylight)

Kopiujemy wzorce 1:1 (źródła w `/Users/paramah/Projects/eiac.dev/tools/para-board`):
- `internal/arch_test.go` — reguła zależności `adapter → usecase → domain`, adaptery się nie importują nawzajem (zmienić stałą modułu).
- `internal/usecase/ports.go` + `usecase/fakes/` (in-memory, `tx.go`, `store.go`) + `usecase/portstest/` (wspólne suity kontraktowe repozytoriów).
- Interaktor = struct z `New…` i `Do(ctx, Principal, Input)`; autoryzacja w use case'ach (`access.go`), ukryte zasoby → `ErrNotFound`.
- `internal/adapter/rest` — oapi-codegen (`std-http-server`, `strict-server`, `go:generate` przez `go run …@v2.8.0`, `TestGeneratedCodeIsCurrent`), błędy mapowane wyłącznie w `problem.go` (RFC 7807), middleware z `middleware.go`/`auth_middleware.go`, libopenapi-validator w testach konformancji.
- `internal/adapter/sqlite` — pule writer/reader na WAL, `WithinTx` w kontekście, migracje goose osadzone (`migrations/NNNNN_*.sql`), `CheckSchemaCurrent`, czasy jako INTEGER µs.
- `internal/adapter/mcp` — `NewStreamableHTTPHandler` (Stateless, JSONResponse), `/mcp` POST, Bearer, rate-limit, helper `addTool[In]`, kontrakt w `api/mcp-tools.md`.
- `internal/adapter/oidc` (PKCE + nonce), `internal/adapter/spa` (fallback, CSP), `web/embed.go`.
- `internal/platform/{config,clock,id(ULID),password(argon2id),token}`; `config.Secret`.
- `cmd/denar/{main.go (serve|migrate|token), app.go (wire, rootHandler, jobs)}`.
- `web/` — openapi-typescript → `src/api/schema.d.ts`, openapi-fetch, react-query, Vitest+MSW, Playwright+axe na zbudowanej binarce.
- `Taskfile.yml` (build/test/vet/fmt/verify/dev/compose), `Dockerfile`, `docker-compose.yml`, `.env.example`.
- `.ai/steering/*`, `.ai/sdd/{WORKFLOW.md,PLAN.md,INDEX.md,specs/NNN-*/}` ze statusami `.status`; `AGENTS.md`.

## Model domeny (`internal/domain/*`)

**Money** (`domain/money`): `Amount int64` w jednostkach minimalnych + `Currency` (ISO 4217, z liczbą miejsc). Zero floatów. Kursy jako dziesiętny string → `math/big.Rat`, zaokrąglanie bankierskie przy przeliczeniu.

**Tożsamość i dostęp**
- `user` — konto (e-mail, hasło/OIDC subject, admin instancji).
- `household` — gospodarstwo: nazwa, **waluta bazowa**, strefa czasowa, dzień startu miesiąca budżetowego.
- `membership` — user × household × rola: `owner | admin | member | viewer`.
- `bot` — należy do gospodarstwa, tworzony przez użytkownika (`CreatedBy`); `Scopes []Scope`, `WalletIDs []ID` (puste = wszystkie widoczne dla twórcy), `ExpiresAt?`, `Disabled`, token `dnr_…` przechowywany jako SHA-256. **Efektywne uprawnienia bota = scope'y ∩ uprawnienia twórcy** (bot nigdy nie widzi więcej niż jego właściciel).
- Scope'y: `wallets:read|write`, `transactions:read|write`, `budget:read|write`, `subscriptions:read|write`, `reports:read`, `notifications:read`.
- `audit` — wpis: aktor (user/bot), akcja, zasób, diff, czas. Zapisywany w tej samej transakcji co zmiana.

**Finanse**
- `wallet` — typ (`cash|checking|card|savings|investment|loan`), waluta, saldo otwarcia, `OnBudget bool` (czy wchodzi do kopert), `Visibility shared|private` (+ lista uprawnionych członków dla private), archiwizacja.
- `transaction` — portfel, data, kwota (±) w walucie portfela, **`BaseAmount` w walucie bazowej (snapshot kursu z dnia transakcji)**, payee, envelope?, tagi, notatka, status `pending|cleared|reconciled`, `Source manual|import|subscription|bot`, `ExternalID?` (dedupe importów), `TransferPairID?` (przelewy między portfelami = dwie nogi, nie wpływają na koperty, gdy obie on-budget). Split transakcji = linie `TransactionSplit` (envelope + kwota).
- `payee` — kontrahent z regułami (`pattern` → domyślna koperta).

**Budżet kopertowy** (`domain/budget`) — w walucie bazowej gospodarstwa:
- `EnvelopeGroup`, `Envelope` (nazwa, grupa, cel opcjonalny: kwota do daty / miesięcznie, ukrycie).
- `Assignment` — (envelope, miesiąc) → przydzielona kwota.
- Obliczenia (czyste funkcje domenowe, w pełni testowane):
  - `Activity(env, m)` = suma `BaseAmount` transakcji/splitów z kopertą w miesiącu,
  - `Available(env, m)` = `Available(env, m-1)` + `Assigned` + `Activity` (przeniesienie; ujemne saldo kopert przenosi się jako „overspent” i pomniejsza Ready-to-Assign następnego miesiąca — reguła YNAB),
  - `ReadyToAssign(m)` = przychody on-budget (transakcje bez koperty oznaczone jako `income`) − suma przydziałów do m − overspending z poprzednich miesięcy.
- Operacje: przydziel, przenieś między kopertami, auto-przydział wg celów.

**Subskrypcje** (`domain/subscription`)
- `Subscription` — nazwa, `Vendor` (słownik: OpenAI, Apple, Netflix…, z `ExternalRef` pod przyszłe konektory), kwota+waluta, `Cycle{Unit day|week|month|year, Every int, Anchor date}`, `TrialEndsAt?`, status `trial|active|paused|cancelled`, portfel płatności, koperta, `ReminderOffsets []days`, `MatchRule{PayeePattern, AmountTolerance%, DateWindowDays}`.
- `PricePoint` — historia cen (od daty, kwota, waluta); zmiana ceny = nowy punkt, nie nadpisanie.
- `Charge` — oczekiwane obciążenie: due date, oczekiwana kwota, status `expected|matched|missed|skipped`, `TransactionID?`. Generowane do horyzontu (np. 90 dni) przez job.
- Funkcje domenowe: `NextDue(after)`, `NormalizedMonthly/Yearly` (w walucie bazowej), `MatchScore(charge, tx)`.
- Dopasowanie: MVP — sugestie (score) + ręczne potwierdzenie; zaksięgowanie oczekiwanego obciążenia tworzy transakcję `Source=subscription`. Automatyczne dopasowanie po importach — ten sam use case, tryb `auto` przy score ≥ progu.

**Kursy walut** — `Rate(date, from, to)`; port `RateProvider`, adapter **NBP API (tabela A)**, cache w tabeli `fx_rates`, fallback na ostatni dzień roboczy.

**Powiadomienia** (`domain/notification`)
- `Notification` (in-app, przeczytane/nieprzeczytane), `Channel` (user × typ `smtp|webhook|ntfy|telegram|matrix` × konfiguracja, sekrety szyfrowane), `Preference` (typ zdarzenia → kanały).
- Zdarzenia: `subscription.renewal_upcoming`, `subscription.trial_ending`, `subscription.price_changed`, `subscription.charge_missed`, `envelope.overspent`, `envelope.goal_due`.
- **Outbox** (`notification_outbox`) zapisywany transakcyjnie; worker wysyła przez port `Notifier` z retry/backoff.

**Gotowość na integracje (tylko porty + jeden adapter w MVP)**
- `Connection` — gospodarstwo × provider × status × zaszyfrowane poświadczenia (AES-GCM, klucz z `DENAR_MASTER_KEY[_FILE]`) × `LastSyncAt`.
- Porty: `BankConnector` (ListAccounts, FetchTransactions(since)) i `BillingConnector` (FetchCharges/Subscriptions — OpenAI usage/costs, Apple przez parsowanie maili z paragonami itp.).
- Use case `ImportTransactions(batch)` — wspólny dla wszystkich źródeł: dedupe po `ExternalID`, reguły payee, potem auto-dopasowanie subskrypcji.
- MVP adapter: **import CSV** (generyczny z mapowaniem kolumn + preset mBank/ING), żeby ścieżka importu działała od początku. Banki (PSD2 przez agregator, np. GoCardless Bank Account Data / Enable Banking) i dostawcy — osobne przyszłe specyfikacje.

## Use case'y i autoryzacja

`Principal{User | Bot, HouseholdID}`; helpery w `usecase/access.go`: `requireRole`, `requireScope`, `visibleWallets(p)` (role + private wallets + `bot.WalletIDs`). Każdy use case z danymi portfela filtruje przez `visibleWallets`. Operacje administracyjne (członkowie, boty, kanały, połączenia) — tylko ludzie z rolą `owner|admin`.

Pliki (przykłady): `create_wallet.go`, `record_transaction.go`, `transfer.go`, `assign_to_envelope.go`, `move_between_envelopes.go`, `budget_month.go`, `create_subscription.go`, `change_subscription_price.go`, `generate_charges.go`, `match_charge.go`, `suggest_matches.go`, `subscription_summary.go`, `spending_report.go`, `import_transactions.go`, `create_bot.go`, `rotate_bot_token.go`, `dispatch_notifications.go`.

## Adaptery wejściowe

- **REST** `/api` — kontrakt `api/openapi.yaml`. Sesje cookie dla WebUI **oraz Bearer `dnr_…` dla botów** (różnica vs Straylight — boty mają dostęp też do REST). Ścieżki zagnieżdżone pod `/api/households/{hid}/…`.
- **MCP** `/mcp` — Bearer bota, stateless JSON; kontrakt `api/mcp-tools.md`. Narzędzia:
  - odczyt: `list_wallets`, `get_balances`, `search_transactions`, `budget_overview(month)`, `list_subscriptions`, `upcoming_renewals(days)`, `subscription_costs(period)`, `spending_by(envelope|payee|vendor, period)`, `list_notifications`;
  - zapis: `record_expense`, `record_income`, `transfer`, `assign_to_envelope`, `create_subscription`, `update_subscription`, `confirm_charge_match`.
  - Każde narzędzie wymaga odpowiedniego scope'u; kwoty w odpowiedziach jako `{amount: "12.34", currency, minor}`.
- **Jobs** w `cmd/denar/app.go`: generowanie obciążeń subskrypcji, wykrywanie `missed`, przypomnienia, pobieranie kursów NBP, worker outboxa, sprzątanie sesji.

## Adaptery wyjściowe

`sqlite`, `oidc`, `nbp` (RateProvider), `notify/{smtp,webhook,ntfy,telegram,matrix}` (każdy implementuje `Notifier`), `csvimport`, `crypto` (szyfrowanie sekretów — w `platform`).

## WebUI (`web/`)

Widoki: logowanie, wybór gospodarstwa, **Dashboard** (saldo, Ready-to-Assign, nadchodzące odnowienia, koszt subskrypcji/mies.), **Portfele** + rejestr transakcji (szybkie dodawanie, przelewy, split), **Budżet** (siatka kopert na miesiąc: Assigned/Activity/Available, przenoszenie), **Subskrypcje** (lista, kalendarz odnowień, historia cen, kolejka dopasowań), **Powiadomienia**, **Ustawienia** (członkowie, boty z wyborem scope'ów/portfeli i jednorazowym pokazaniem tokenu, kanały, import CSV). Projekt wizualny — przez skill frontend-design w etapie WebUI.

## Etapy (specyfikacje SDD w `.ai/sdd/specs/`)

Każdy etap: requirements → design → tasks → (zatwierdzenie człowieka) → TDD → `task verify`.

1. **001-core** — szkielet repo (git init, Taskfile, arch_test, config, sqlite, migracje), użytkownicy, logowanie lokalne + OIDC, sesje, gospodarstwa, członkostwa, boty z scope'ami, Bearer na REST, audit log, healthz.
2. **002-wallets-transactions** — Money, waluty, kursy NBP, portfele, transakcje, przelewy, splity, payee + reguły, salda.
3. **003-envelope-budget** — grupy/koperty, przydziały, obliczenia miesiąca, przenoszenie, cele, Ready-to-Assign.
4. **004-subscriptions** — subskrypcje, cykle, historia cen, generowanie obciążeń, sugestie i potwierdzanie dopasowań, podsumowania kosztów.
5. **005-notifications** — in-app, outbox, worker, kanały SMTP/webhook/ntfy/Telegram/Matrix, preferencje, przypomnienia.
6. **006-mcp** — serwer MCP i kontrakt narzędzi.
7. **007-webui** — SPA (może startować równolegle po 002 na zamrożonym kontrakcie OpenAPI).
8. **008-import** — `ImportTransactions`, CSV, porty `BankConnector`/`BillingConnector`, `Connection` + szyfrowanie.
9. **009-hardening-demo** — ochrona SSRF kanałów powiadomień (blokada sieci prywatnych w dialerze, opt-in administratora), dane demo (`denar demo`, `task demo`).
Przyszłe (poza MVP): 010 PSD2 przez agregatora, 011 OpenAI costs API, 012 Apple/e-mail receipts.

> Przenumerowanie (2026-10-21): pierwotnie 009 = PSD2, 010 = OpenAI, 011 = Apple. Numer 009 zajęło utwardzenie po przeglądzie bezpieczeństwa i dane demo na poranny przegląd; przyszłe etapy przesunięte o jeden (010–012).

## Pierwsze kroki po zatwierdzeniu planu

1. `git init` w `denar`, utworzenie `AGENTS.md`, `.ai/steering/{product,tech-stack,conventions,principles}.md` i `.ai/sdd/PLAN.md` z treścią tego planu (decyzje jako TD-xxx), commit.
2. Spec `001-core` (requirements → design → tasks) do Twojej akceptacji — implementacja dopiero przy `tasks:approved` (zgodnie z procesem Straylight i bramką brainstormingu).

## Weryfikacja

- `task verify` (gofmt, go vet, `go test -race ./...`, build web + vitest, build binarki) na każdym etapie; `arch_test.go` pilnuje warstw.
- Testy domeny: tablicowe testy Money/przeliczeń, `Available`/`ReadyToAssign` (przeniesienia, overspending), `NextDue` (koniec miesiąca, 29.02, rok), `MatchScore`.
- Use case'y na fake'ach, w tym macierz autoryzacji: rola × scope bota × ograniczenie portfeli × prywatny portfel (bot nie widzi więcej niż twórca; ukryte → 404).
- Suity `portstest` uruchamiane na SQLite.
- REST: testy konformancji z libopenapi-validator; MCP: testy e2e w `cmd/denar` (token bota → `upcoming_renewals`, `record_expense`, brak scope'u → `forbidden`).
- E2E Playwright na zbudowanej binarce: login → gospodarstwo → portfel → transakcja → przydział koperty → subskrypcja → dopasowanie obciążenia; axe dla dostępności.
- Ręcznie: `./denar migrate && ./denar serve`, podłączenie Claude (remote MCP) z tokenem bota i zapytanie „co odnawia się w tym tygodniu?”.

## Tryb autonomiczny (przebieg nocny 2026-10-21/22)

Użytkownik 2026-10-21: „Wszystko się zgadza, zacznij pracę. Zorganizuj ją tak, żebym nie musiał potwierdzać. […] rano chciałbym mieć możliwość sprawdzenia lokalnie aplikacji.”

Na tej podstawie, wyłącznie dla tego przebiegu:
- `001-core/requirements.md` zatwierdził człowiek (odpowiedzi Q-001…Q-006 przyjęte w całości).
- Pozostałe bramki (`design:approved`, `tasks:approved`, wymagania etapów 002–008) zatwierdza **orkiestrator (Claude) z delegacji użytkownika**. Każde takie zatwierdzenie jest odnotowane w `.status` z dopiskiem `# delegated` i w tabeli poniżej. Użytkownik może je rano zrewidować.
- Pytania otwarte rozstrzyga agent, wybierając najprostsze rozwiązanie zgodne z planem i zasadami; każde rozstrzygnięcie trafia do sekcji „Decyzje autonomiczne” w `design.md` danego etapu.
- Bez `git push`, bez zewnętrznych publikacji. Praca na gałęzi `main` (backend sekwencyjnie), Web UI w osobnym worktree scalanym przez orkiestratora.

| Etap | Bramka | Kto | Data |
|---|---|---|---|
| 001-core | requirements:approved | człowiek | 2026-10-21 |
| 001-core | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 001-core | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 001-core | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 002-wallets-transactions | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 002-wallets-transactions | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 002-wallets-transactions | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 002-wallets-transactions | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 003-envelope-budget | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 003-envelope-budget | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 003-envelope-budget | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 003-envelope-budget | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 004-subscriptions | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 004-subscriptions | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 004-subscriptions | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 004-subscriptions | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 005-notifications | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 005-notifications | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 005-notifications | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 005-notifications | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 006-mcp | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 006-mcp | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 006-mcp | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 006-mcp | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 007-webui | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 007-webui | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 007-webui | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 007-webui | implementation:done (części A, B, C1, C2, C3a, C3b, C4) | agenci części (task verify + task web:e2e zielone; scalenia: orkiestrator) | 2026-10-22 |
| 008-import | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 008-import | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 008-import | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 008-import | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |
| 009-hardening-demo | requirements:approved | orkiestrator (delegacja) | 2026-10-21 |
| 009-hardening-demo | design:approved | orkiestrator (delegacja) | 2026-10-21 |
| 009-hardening-demo | tasks:approved | orkiestrator (delegacja) | 2026-10-21 |
| 009-hardening-demo | implementation:done | agent etapu (task verify zielone) | 2026-10-21 |

## Po MVP (2026-10-21)

Zatwierdzone przez użytkownika planem „wiele języków, GitHub, MIT, pipeline”:

- Moduł Go przemianowany na `github.com/paramah/denar`; remote `git@github.com:paramah/denar.git`.
- `LICENSE` (MIT), `README.md` (EN) i `README.pl.md` (PL), `Dockerfile` budujący Web UI, `.github/workflows/{ci,release}.yml`, Dependabot.
- Etap 013-i18n: Web UI i powiadomienia po polsku i angielsku (spec w `.ai/sdd/specs/013-i18n/`).
