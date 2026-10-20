# Design: Core (001-core)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md` (requirements:approved), `.ai/sdd/PLAN.md`
> Last Updated: 2026-10-21

## 1. Architektura

Monolit Go 1.27, moduł `git.eiac.dev/tools/denar`, Clean Architecture skopiowana ze Straylight (`../para-board`):

```
cmd/denar/                 main.go (serve|migrate|token|version), app.go (wire, rootHandler, joby), token.go
internal/domain/           domain.go (ID, Actor, błędy, Optional)
  money/                   Currency (ISO 4217 + miejsca dziesiętne) — Amount dojdzie w 002
  user/ household/ membership/ bot/ audit/
internal/usecase/          ports.go, principal.go, access.go, interaktory, fakes/, portstest/
internal/adapter/rest      oapi-codegen strict server + middleware (auth, CSRF, limity, problem+json)
internal/adapter/sqlite    writer/reader WAL, WithinTx, goose (00001_core.sql)
internal/adapter/oidc      Authorization Code + PKCE + nonce (kopia Straylight)
internal/adapter/spa       pliki web/dist + fallback index.html; zaślepka, gdy dist pusty
internal/platform/         config clock id password token
api/openapi.yaml           kontrakt REST (OpenAPI 3.1)
web/embed.go, web/dist/.gitkeep
```

Reguła zależności pilnowana przez `internal/arch_test.go` (domena: tylko stdlib; usecase: domena + usecase; adaptery nie importują się nawzajem).

## 2. Domena

- `domain.ID` (ULID z `platform/id`), `domain.Actor{Kind user|bot|system, ID}`.
- Błędy: `ErrNotFound`, `ErrConflict`, `ErrForbidden`, `ErrInvalid`, `ErrUnauthenticated`, `ErrBadRequest`, `ErrRateLimited`; `ValidationError{Field,Message}`, `ConflictError{Reason}` (`duplicate_email`, `last_owner`, `already_member`, `invitation_used`, `duplicate_name`), `ForbiddenError{Reason}` (`human_only`, `role`, `scope`, `owner_only`).
- `money.Currency{Code, Decimals}` + `money.LookupCurrency`, `money.Currencies()` — zamknięta lista najczęstszych walut ISO 4217 (PLN, EUR, USD, GBP, CHF, CZK, SEK, NOK, DKK, HUF, JPY, …). Etap 002 dopisze `Amount`.
- `user.User{ID, Email, Name, PasswordHash, OIDCIssuer, OIDCSubject, InstanceAdmin, Active, CreatedAt}`; `NormalizeEmail`, `ValidateEmail`, `ValidateName`, `ValidatePassword` (12–128 znaków).
- `household.Household{ID, Name, BaseCurrency, Timezone, MonthStartDay, CreatedBy, CreatedAt}`; `Validate` (nazwa 1–100, waluta z listy, strefa przez `time.LoadLocation` — `cmd/denar` importuje `time/tzdata`, dzień 1–28).
- `membership.Role` `owner > admin > member > viewer`; `Membership`; reguły:
  - `CanManageMembers(role)` — owner/admin,
  - `CheckRoleChange(actor, targetOld, targetNew)` — admin nie nadaje/odbiera `owner` ani nie zmienia ownera,
  - `CheckLastOwner(members, target, newRole, removed)` → `ConflictError{last_owner}`,
  - `RolePermitsScope(role, scope)` — viewer tylko `:read`, reszta wszystko,
  - `Invitation{ID, HouseholdID, Email, Role, TokenHash, CreatedBy, CreatedAt, ExpiresAt, AcceptedAt, AcceptedBy, RevokedAt}` + `State(now)`: `pending|accepted|revoked|expired`.
- `bot.Scope` — zamknięta lista FR-031; `ParseScopes` (nieznany → `ValidationError{scopes}`, dedupe, sort); `Bot{ID, HouseholdID, Name, Scopes, WalletIDs, CreatedBy, CreatedAt, ExpiresAt?, Disabled, TokenHash, TokenPrefix, LastUsedAt}`; `Usable(now)`; `EffectiveScopes(creatorRole)` = scope'y ∩ `RolePermitsScope`.
- `audit.Entry{ID, HouseholdID, At, Actor, Action, ResourceType, ResourceID, Changes map[string]Change{From,To}}` — bez sekretów (tokenów, hashy, haseł).

## 3. Use case'y i autoryzacja

### Principal (`usecase/principal.go`)

```go
type Principal struct {
    User user.User // człowiek; zero dla bota
    Bot  bot.Bot   // bot; zero dla człowieka
}
func (p Principal) IsBot() bool
func (p Principal) Actor() domain.Actor
```

Principal nie jest przypięty do gospodarstwa (człowiek działa w wielu); kontekst gospodarstwa daje `access`.

### access.go

```go
type HouseholdScope struct {
    Household household.Household
    Role      membership.Role   // człowiek: własna rola; bot: aktualna rola twórcy ("" gdy twórca nie jest członkiem)
    Bot       *bot.Bot
    Scopes    []bot.Scope       // bot: efektywne scope'y (∩ rola twórcy)
    WalletIDs []domain.ID       // bot: ograniczenie portfeli (puste = bez ograniczenia)
}
func (a access) household(ctx, p, householdID) (HouseholdScope, error) // ukryte → ErrNotFound
func requireHuman(p) error                 // bot → ForbiddenError{human_only} (FR-033)
func requireRole(hs, min membership.Role) error
func requireScope(hs, scope bot.Scope) error  // człowiek: RolePermitsScope; bot: scope ∈ efektywnych
func (hs HouseholdScope) WalletAllowed(id) bool  // punkt rozszerzenia 002 (lista bota ∩ widoczne twórcy)
```

Rola twórcy bota jest czytana z `MembershipRepository` **przy każdym żądaniu** (`access.household`), więc utrata roli zawęża bota natychmiast (FR-032b). Twórca bez członkostwa → bot widzi gospodarstwo jako istniejące, ale każdy `requireScope` daje 403.

Punkt rozszerzenia portfeli: port `WalletDirectory` (opcjonalny, w 001 `nil`): `ValidateWalletIDs(ctx, householdID, creatorID, ids) error`. `CreateBot`/`UpdateBot` wołają go, gdy jest podany; w 001 lista jest tylko walidowana składniowo (niepuste, unikalne, ≤100) i zapisywana w `bot_wallets`. Etap 002 wstrzyknie implementację na repozytorium portfeli i dopisze filtr widocznych portfeli twórcy w `WalletAllowed`/`visibleWallets`.

### Interaktory (jeden plik na use case lub grupę)

| Obszar | Use case'y | Uprawnienia |
|---|---|---|
| Auth | `Authenticate` (sesja, przesuwny TTL, touch ≤1/h), `AuthenticateBot` (hash, usable, `last_used_at` ≤1/min), `LoginLocal`, `LoginOIDCStart`, `LoginOIDCCallback`, `Logout`, `GetAuthConfig`, `BootstrapAdmin` | publiczne |
| Me | `GetMe` — człowiek: user + gospodarstwa z rolami; bot: bot + efektywne scope'y | każdy |
| Gospodarstwa | `CreateHousehold`, `ListHouseholds`, `GetHousehold`, `UpdateHousehold`, `DeleteHousehold` | create: człowiek; get: członek lub bot gospodarstwa; update: owner/admin; delete: owner |
| Członkowie | `ListMembers`, `UpdateMemberRole`, `RemoveMember` | list: członek (człowiek); zmiana: owner/admin wg reguł; usunięcie siebie dozwolone każdemu |
| Zaproszenia | `CreateInvitation`, `ListInvitations`, `RevokeInvitation`, `PreviewInvitation`, `AcceptInvitation`, `RegisterWithInvitation` | owner/admin; preview/register publiczne (token w body) |
| Boty | `CreateBot`, `ListBots`, `GetBot`, `UpdateBot`, `RotateBotToken`, `DeleteBot`, `CreateBotCLI` | owner/admin: wszystkie; member: własne; viewer: brak; bot: nigdy |
| Audit | `ListAudit` | owner/admin (człowiek) |

`RemoveMember` w tej samej transakcji wyłącza boty usuniętego (twórcy) w gospodarstwie i zapisuje audyt `bot.disabled` dla każdego. Każda zmiana gospodarstwa zapisuje `audit.Entry` w transakcji (`auditor.record(ctx, …)` wołane wewnątrz `WithinTx`).

### Porty (`usecase/ports.go`)

`Clock`, `IDGenerator`, `TokenGenerator`, `PasswordHasher`, `TxManager`, `Page`, `SessionRepository`, `OIDCStateRepository`, `OIDCProvider`, `UserRepository`, `HouseholdRepository`, `MembershipRepository`, `InvitationRepository`, `BotRepository`, `AuditRepository`, `WalletDirectory` (opcjonalny).

## 4. Schemat SQL (`00001_core.sql`)

Czasy: INTEGER µs UTC; ID: TEXT (ULID).

- `users(id PK, email UNIQUE, name, password_hash?, oidc_issuer?, oidc_subject?, instance_admin, active, created_at)` + unikalny indeks `(oidc_issuer, oidc_subject)`.
- `sessions(token_hash PK, user_id → users ON DELETE CASCADE, created_at, last_seen_at, expires_at)`.
- `oidc_login_states(state_hash PK, nonce, code_verifier, return_to, created_at, expires_at)`.
- `households(id PK, name, base_currency, timezone, month_start_day CHECK 1..28, created_by → users, created_at)`.
- `memberships(household_id → households CASCADE, user_id → users, role CHECK, created_at, PK(household_id, user_id))`.
- `invitations(id PK, household_id → households CASCADE, email, role CHECK (admin|member|viewer|owner), token_hash UNIQUE, created_by, created_at, expires_at, accepted_at?, accepted_by?, revoked_at?)`.
- `bots(id PK, household_id → households CASCADE, name, scopes TEXT (JSON), created_by → users, created_at, expires_at?, disabled, token_hash UNIQUE, token_prefix, last_used_at?)` + unikalny `(household_id, name COLLATE NOCASE)`.
- `bot_wallets(bot_id → bots CASCADE, wallet_id, PK)` — FK do `wallets` doda 002 (przebudowa tabeli) albo pozostanie walidacja w use case.
- `audit_log(id PK, household_id → households CASCADE, at, actor_kind, actor_id, action, resource_type, resource_id, changes TEXT JSON)` + indeks `(household_id, id DESC)`.

## 5. REST (`api/openapi.yaml`)

- Serwer `/api`; bezpieczeństwo globalne: `session` (cookie `denar_session`) lub `bot` (HTTP Bearer `dnr_…`). Cookie + Bearer → 400 `bad_request`.
- Problem RFC 7807: `type` = `urn:denar:problem:<slug>`, `code` ∈ `bad_request|validation|unauthenticated|forbidden|not_found|conflict|rate_limited|internal`, `title`, `status`, `detail`, `errors[]{field,message}`. Powód 403/409 w ostatnim segmencie `type` (np. `last-owner`, `human-only`).
- Endpointy:
  - `GET /auth/config`, `GET /auth/login` (OIDC start, 302), `POST /auth/login`, `GET /auth/callback`, `POST /auth/logout`
  - `GET /me`, `GET /currencies`
  - `GET|POST /households`; `GET|PATCH|DELETE /households/{householdId}`
  - `GET /households/{householdId}/members`; `PATCH|DELETE /households/{householdId}/members/{userId}`
  - `GET|POST /households/{householdId}/invitations`; `DELETE /households/{householdId}/invitations/{invitationId}`
  - `POST /invitations/preview` (publiczne), `POST /invitations/accept` (sesja), `POST /invitations/register` (publiczne, tworzy konto + sesję)
  - `GET|POST /households/{householdId}/bots`; `GET|PATCH|DELETE /households/{householdId}/bots/{botId}`; `POST /households/{householdId}/bots/{botId}/token`
  - `GET /households/{householdId}/audit?cursor&limit&actor_kind&actor_id&resource_type`
- Link zaproszenia: `{DENAR_BASE_URL}/invite#<token>` — token we fragmencie, nie trafia do logów serwera; SPA wysyła go w body.
- Jawny token bota tylko w odpowiedzi `createBot` i `rotateBotToken` z `Cache-Control: no-store`.

## 6. Middleware REST

Kolejność: `noSniff → logRequests → recoverPanics → protectCrossOrigin (http.CrossOriginProtection, FR-014) → authenticate (cookie/Bearer, 400 przy obu) → requireJSON → router`. Limity (sliding window w pamięci, `Retry-After`):
- logowanie lokalne: 5 nieudanych/min na (IP, e-mail) i 20/min na IP → 429 (FR-011),
- start OIDC: 20/min na IP,
- rejestracja z zaproszenia/preview: 20/min na IP,
- Bearer: 20 nieudanych uwierzytelnień/min na IP → 429; poprawny bot: `DENAR_BOT_RATE` żądań/min (domyślnie 120) → 429.

## 7. Konfiguracja (`DENAR_*`)

`DENAR_ADDR` (`:8080`), `DENAR_DB` (`denar.db`), `DENAR_ADMIN_EMAIL`, `DENAR_ADMIN_PASSWORD[_FILE]` (≥12), `DENAR_ADMIN_NAME`, `DENAR_LOCAL_LOGIN` (true), `DENAR_BASE_URL` (`http://localhost:8080`, wymagany przy OIDC), `DENAR_SESSION_TTL` (168h), `DENAR_INVITE_TTL` (168h), `DENAR_OIDC_ISSUER|_CLIENT_ID|_CLIENT_SECRET`, `DENAR_OIDC_AUTO_CREATE` (false), `DENAR_AUTO_MIGRATE` (false), `DENAR_TRUSTED_PROXIES`, `DENAR_INSTANCE_NAME` (Denar), `DENAR_BOT_RATE` (120).

## 8. CLI

- `denar serve`, `denar migrate`, `denar version`.
- `denar token create --household <id> --name <nazwa> --scopes a,b [--as <email>] [--wallets id,id] [--expires 2027-01-01T00:00:00Z]` — wypisuje token raz; twórca = `--as` (musi być członkiem) albo pierwszy owner; audyt z aktorem `system`.

## 9. Przypadki brzegowe

- Ostatni owner: degradacja/usunięcie/wyjście → 409 `last-owner`.
- Admin nie zmienia/usuwa ownerów i nie nadaje roli owner (403 `role`); admin może zapraszać tylko admin/member/viewer.
- Zaproszenie: zużyte/odwołane/wygasłe → 409 `invitation-used`/404 (nieznane i odwołane → 404, wygasłe → 409 `invitation-expired`); już członek → 409 `already-member`; rejestracja na istniejący e-mail → 409 `duplicate-email`.
- Bot: wyłączony/wygasły/usunięty/zrotowany → 401; twórca viewer → tylko `:read`; twórca usunięty → bot wyłączony (i tak 401); twórca utracił członkostwo inaczej (np. wyjście) → wyłączany tą samą ścieżką.
- OIDC: konto po `issuer+subject`; nowe konto tylko przy `DENAR_OIDC_AUTO_CREATE` lub oczekującym zaproszeniu na zweryfikowany e-mail; nigdy nie wiążemy z istniejącym kontem po e-mailu.

## 10. Strategia testów

- Domena: testy tabelaryczne (role, last owner, scope'y, efektywne uprawnienia, waluty, walidacja).
- Use case'y na fake'ach: auth, gospodarstwa, członkowie, zaproszenia, boty, audit (atomowość: błąd → brak wpisu), **macierz autoryzacji** rola × scope × status twórcy × user/bot (`authz_matrix_test.go`).
- `portstest`: suity kontraktowe wszystkich repozytoriów, uruchamiane na fake'ach i SQLite.
- REST: konformancja każdej operacji × każdej zadeklarowanej odpowiedzi z `api/openapi.yaml` (libopenapi-validator), cookie+Bearer → 400, limity → 429, CSRF → 403.
- E2E `cmd/denar`: serwer na tymczasowej bazie → login → gospodarstwo → bot → wywołanie Bearer; `token create`; OIDC na `oidctest`.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-001 | Principal bez gospodarstwa; kontekst gospodarstwa przez `access.household` | Człowiek działa w wielu gospodarstwach; bot ma jedno, sprawdzane w `access` |
| AD-002 | Rola twórcy bota czytana przy każdym żądaniu w `access.household` | FR-032b bez cache'u; koszt: 1 zapytanie po PK |
| AD-003 | Bot w 001 może tylko `GET /me` i `GET /households/{id}` (swojego); reszta 403 `human-only` | Brak danych finansowych w 001; FR-033 |
| AD-004 | Kod `bad_request` (400) i `rate_limited` (429) dodane do enum `Problem.code` | FR-035 i FR-011 wymagają 400/429 |
| AD-005 | Token zaproszenia przekazywany w body (`/invitations/preview|accept|register`), link z tokenem we fragmencie URL | Sekret nie trafia do logów i nagłówka Referer |
| AD-006 | Przyjęcie zaproszenia przez zalogowanego nie wymaga zgodności e-maila | Link jest jednorazowym sekretem; OIDC może mieć inny adres. Zapisywane w audycie |
| AD-007 | OIDC nie łączy kont po e-mailu; nowe konto przy `DENAR_OIDC_AUTO_CREATE` lub oczekującym zaproszeniu na zweryfikowany e-mail | Ochrona przed przejęciem konta (jak Straylight) |
| AD-008 | Usunięcie bota jest twarde (wiersz znika), audyt zostaje | Prostota; audyt niesie tylko ID |
| AD-009 | Usunięcie gospodarstwa kasuje kaskadowo członkostwa, zaproszenia, boty i audyt | W 001 brak danych finansowych; 002+ dopisze własne kaskady lub blokadę |
| AD-010 | Lista portfeli bota w tabeli `bot_wallets` bez FK; port `WalletDirectory` (nil w 001) | Zapis już teraz, walidacja w 002 |
| AD-011 | Waluty: zamknięta lista ~30 kodów ISO 4217 w `domain/money` | Prosta, testowalna; rozszerzalna |
| AD-012 | `DeleteHousehold` tylko owner; audytu usunięcia nie ma (kasowany z gospodarstwem) — wpis do logu slog | Spójne z AD-009 |
| AD-013 | Web UI: tylko embed + wbudowana zaślepka `index.html`, gdy `web/dist` pusty; zadania `web:*` w Taskfile pomijane bez `web/package.json` | Etap 007 doda SPA |
| AD-014 | Limity w pamięci procesu (jak Straylight) | Jedna instancja self-hosted |
| AD-015 | Brak zmiany własnego hasła i administracji kontami instancji w 001 | Poza zakresem FR; do dopisania w 007/później |

| AD-016 | `/healthz` sprawdza bazę (`DB.Ping`, wstrzykiwane jako `rest.Deps.Health`): 200 albo 503 | US-001 („gdy baza jest dostępna”) |
| AD-017 | `GET /households` i `GET /me` dostępne też dla bota (jego gospodarstwo, rola twórcy) | Introspekcja dla botów i MCP (006) |
| AD-018 | Limity: 5 nieudanych logowań/min na (IP, e-mail), 20/min na IP; 20/min na IP dla startu OIDC, preview i register; 20 odrzuconych Bearer/min na IP; `DENAR_BOT_RATE` żądań bota/min — wszystkie 429 + `Retry-After` | FR-011, NFR-001 |

## Ograniczenia

- Brak resetu hasła i zmiany własnego hasła (poza zakresem 001).
- MCP (006) użyje `AuthenticateBot` i `access`; w 001 brak `/mcp` (ścieżka zarezerwowana w `rootHandler`, odpowiada 404).
- Limity żądań trzymane w pamięci procesu (jedna instancja).
- Bot jest czytany przy uwierzytelnieniu żądania; use case'y zapisujące dane przez bota (002+) powinny na początku transakcji ponownie odczytać bota (`Bots.ByID` + `Usable` + zgodny `TokenHash`), jak `currentPrincipal` w Straylight — w 001 boty niczego nie zapisują.
- `WalletAllowed` sprawdza tylko listę bota; filtr portfeli widocznych dla twórcy (prywatne portfele) dopisuje 002.
