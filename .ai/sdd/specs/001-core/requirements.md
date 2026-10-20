# Feature: Core (F01, etap 1)

> Status: Approved (zatwierdzone przez człowieka 2026-10-21 — „Wszystko się zgadza”; odpowiedzi Q-001…Q-006 przyjęte)
> Source: `.ai/sdd/PLAN.md` (plan:approved), etap 1 `001-core`
> Last Updated: 2026-10-21

## Overview

Rdzeń `denar`: szkielet repozytorium i binarium, trwały zapis w SQLite, użytkownicy z logowaniem lokalnym i OIDC, sesje, **gospodarstwa** z członkostwami i rolami, **boty** z tokenami, scope'ami i ograniczeniem do portfeli, uwierzytelnianie Bearer w REST oraz audit log. Etap nie zawiera danych finansowych (portfele, transakcje — etap 002), ale ustala model `Principal` i helpery autoryzacji, z których korzystają wszystkie następne etapy.

## Business Context

- **Problem:** Dane finansowe wielu osób i botów na jednej instancji wymagają izolacji gospodarstw i precyzyjnych uprawnień, zanim powstanie jakikolwiek byt finansowy.
- **Users:** Członek gospodarstwa (role `owner`, `admin`, `member`, `viewer`), admin instancji, bot.
- **Value:** Bezpieczne logowanie, zakładanie gospodarstw, zapraszanie członków i wydawanie tokenów botom; stabilny fundament dla etapów 002–008.
- **Success signal:** Testy domeny/use case'ów, suity kontraktowe SQLite i testy REST zgodne z `api/openapi.yaml` zielone; macierz autoryzacji bot/rola zielona; `task verify` przechodzi.

## Goals

- Jedno binarium `denar` z komendami `serve`, `migrate`, `token` i `/healthz`.
- Logowanie lokalne (argon2id) i OIDC (PKCE), sesje cookie z przesuwanym TTL.
- Gospodarstwa z walutą bazową, strefą czasową i dniem startu miesiąca budżetowego.
- Członkostwa z rolami i zaproszeniami.
- Boty: tworzenie, scope'y, lista portfeli (na razie lista ID weryfikowana w etapie 002), wygasanie, wyłączanie, rotacja tokenu.
- Bearer `dnr_…` akceptowany przez REST; uprawnienia bota nigdy szersze niż twórcy.
- Audit log zmian.

## Non-Goals / Out of Scope

- Portfele, transakcje, koperty, subskrypcje, powiadomienia (etapy 002–005).
- Serwer MCP (etap 006) — w tym etapie tylko wspólne uwierzytelnianie botów, którego MCP użyje.
- Web UI (etap 007); w etapie 001 osadzamy jedynie zaślepkę SPA, żeby szkielet `web/embed.go` działał.
- Samodzielna rejestracja kont bez zaproszenia (zob. Q-001).
- Reset hasła e-mailem (wymaga SMTP z etapu 005).

## User Stories

### US-001: Pierwsze uruchomienie

**As a** administrator instancji  
**I want to** uruchomić `denar migrate` i `denar serve` z danymi konta startowego w zmiennych środowiskowych  
**So that** mogę zalogować się bez zewnętrznego IdP

**Acceptance Criteria:**
- [ ] `denar migrate` tworzy lub aktualizuje schemat; `serve` odmawia startu na nieaktualnym schemacie, chyba że ustawiono `DENAR_AUTO_MIGRATE`.
- [ ] Przy pierwszym starcie z `DENAR_ADMIN_EMAIL` i `DENAR_ADMIN_PASSWORD[_FILE]` powstaje konto admina instancji; przy kolejnych startach konto nie jest nadpisywane.
- [ ] `/healthz` odpowiada 200, gdy baza jest dostępna.

### US-002: Logowanie lokalne i OIDC

**As a** użytkownik  
**I want to** zalogować się hasłem albo przez OIDC (np. Pocket ID)  
**So that** mam sesję w Web UI i REST

**Acceptance Criteria:**
- [ ] Poprawne hasło tworzy sesję (cookie HttpOnly, Secure przy HTTPS, SameSite=Lax); błędne zwraca 401 bez ujawniania, czy konto istnieje.
- [ ] Logowanie OIDC tworzy sesję dla konta powiązanego z `issuer+subject` albo z potwierdzonym e-mailem zaproszonego użytkownika.
- [ ] Wylogowanie unieważnia sesję.
- [ ] Bez sesji ani tokenu chronione endpointy zwracają 401.

### US-003: Gospodarstwa

**As a** użytkownik  
**I want to** założyć gospodarstwo z walutą bazową  
**So that** mogę prowadzić w nim budżet

**Acceptance Criteria:**
- [ ] Twórca zostaje `owner`.
- [ ] Widzę tylko gospodarstwa, w których mam członkostwo; inne zwracają 404.
- [ ] `owner`/`admin` może zmienić nazwę, strefę czasową i dzień startu miesiąca; waluty bazowej nie można zmienić po utworzeniu (zob. Q-003).

### US-004: Członkowie i zaproszenia

**As a** `owner` lub `admin`  
**I want to** zaprosić osobę e-mailem z wybraną rolą, zmienić rolę i usunąć członka  
**So that** rodzina korzysta z jednego budżetu

**Acceptance Criteria:**
- [ ] Zaproszenie tworzy jednorazowy link ważny ograniczony czas; przyjęcie tworzy konto (hasło) lub wiąże OIDC i nadaje rolę.
- [ ] `admin` nie może nadać ani odebrać roli `owner`; gospodarstwo zawsze ma co najmniej jednego `owner`.
- [ ] Usunięcie członka wyłącza boty, których jest twórcą, w tym gospodarstwie.

### US-005: Boty

**As a** członek z rolą `owner`, `admin` lub `member`  
**I want to** utworzyć bota z wybranymi scope'ami, opcjonalnie ograniczonego do portfeli i z datą wygaśnięcia  
**So that** asystent AI ma tylko taki dostęp, jakiego potrzebuje

**Acceptance Criteria:**
- [ ] Token `dnr_…` jest pokazywany jeden raz; w bazie tylko jego hash.
- [ ] Mogę wyłączyć bota, zrotować token (stary przestaje działać natychmiast) i usunąć bota.
- [ ] Bot z tokenem wywołuje REST nagłówkiem `Authorization: Bearer dnr_…`.
- [ ] Bot nie może zarządzać członkami, botami ani ustawieniami gospodarstwa.
- [ ] `member` zarządza tylko botami, których jest twórcą; `owner`/`admin` — wszystkimi botami gospodarstwa; `viewer` nie tworzy botów.
- [ ] Gdy twórca traci rolę lub członkostwo, efektywne uprawnienia bota natychmiast się zawężają.

### US-006: Audit log

**As a** `owner` lub `admin`  
**I want to** przeglądać, kto (człowiek czy bot) i kiedy zmienił dane gospodarstwa  
**So that** mogę kontrolować działania botów

**Acceptance Criteria:**
- [ ] Każda zmiana w gospodarstwie (członkostwa, boty, ustawienia) ma wpis z aktorem, akcją, zasobem i zmianą.
- [ ] Lista audytu jest stronicowana i filtrowalna po aktorze i typie zasobu.

## Functional Requirements

### Szkielet i uruchomienie

### FR-001: Komendy binarium — Must Have

THE SYSTEM SHALL udostępniać komendy `serve`, `migrate` i `token` oraz endpoint `/healthz`  
SO THAT instancję można uruchomić i utrzymywać bez dodatkowych narzędzi

### FR-002: Migracje — Must Have

WHEN `serve` startuje na schemacie starszym niż wbudowane migracje i `DENAR_AUTO_MIGRATE` nie jest ustawione  
THE SYSTEM SHALL zakończyć się błędem wskazującym `denar migrate`

### FR-003: Konto startowe — Must Have

WHEN ustawiono `DENAR_ADMIN_EMAIL` i hasło (min. 12 znaków) i konto o tym e-mailu nie istnieje  
THE SYSTEM SHALL utworzyć konto z flagą admina instancji

### Uwierzytelnianie

### FR-010: Logowanie lokalne — Must Have

WHEN użytkownik poda poprawny e-mail i hasło, a logowanie lokalne jest włączone (`DENAR_LOCAL_LOGIN`)  
THE SYSTEM SHALL utworzyć sesję i ustawić cookie z tokenem, przechowując w bazie tylko hash SHA-256 tokenu

### FR-011: Ograniczenie prób logowania — Must Have

WHEN z jednego adresu lub dla jednego e-maila przekroczono limit nieudanych prób  
THE SYSTEM SHALL odpowiadać 429 z `Retry-After`

### FR-012: OIDC — Must Have

WHEN skonfigurowano `DENAR_OIDC_ISSUER`, `_CLIENT_ID` i `_CLIENT_SECRET`  
THE SYSTEM SHALL obsługiwać logowanie Authorization Code z PKCE i nonce, wiążąc konto po `issuer+subject`

### FR-013: Sesje — Must Have

THE SYSTEM SHALL wygaszać sesję po `DENAR_SESSION_TTL` bezczynności, odnawiać ją przy użyciu i usuwać wygasłe sesje w tle

### FR-014: Ochrona CSRF — Must Have

WHEN żądanie zmieniające stan jest uwierzytelnione cookie  
THE SYSTEM SHALL odrzucać żądania cross-origin (sprawdzanie `Origin`/`Sec-Fetch-Site`)

### Gospodarstwa i członkostwa

### FR-020: Tworzenie gospodarstwa — Must Have

WHEN zalogowany człowiek utworzy gospodarstwo z nazwą, walutą bazową (ISO 4217), strefą czasową (IANA) i dniem startu miesiąca (1–28)  
THE SYSTEM SHALL zapisać je i nadać twórcy rolę `owner`

### FR-021: Izolacja gospodarstw — Must Have

WHEN principal odwołuje się do gospodarstwa, w którym nie ma członkostwa (bot: spoza swojego gospodarstwa)  
THE SYSTEM SHALL odpowiedzieć jak dla nieistniejącego zasobu (404)

### FR-022: Role — Must Have

THE SYSTEM SHALL egzekwować role: `owner` (wszystko, w tym usunięcie gospodarstwa i zarządzanie ownerami), `admin` (członkowie poza ownerami, boty, ustawienia), `member` (odczyt i zapis danych finansowych w kolejnych etapach), `viewer` (tylko odczyt)

### FR-023: Zaproszenia — Must Have

WHEN `owner`/`admin` zaprosi e-mail z rolą  
THE SYSTEM SHALL utworzyć jednorazowy token zaproszenia (hash w bazie, ważność `DENAR_INVITE_TTL`, domyślnie 7 dni) i zwrócić link do przekazania; przyjęcie zaproszenia przez zalogowanego lub nowego użytkownika tworzy członkostwo

### FR-024: Ostatni owner — Must Have

IF operacja pozostawiłaby gospodarstwo bez `owner`  
THE SYSTEM SHALL ją odrzucić (409)

### Boty

### FR-030: Tworzenie bota — Must Have

WHEN `owner`, `admin` lub `member` utworzy bota z nazwą, listą scope'ów, opcjonalną listą ID portfeli i opcjonalnym `expires_at`  
THE SYSTEM SHALL zapisać bota z `created_by`, wygenerować token `dnr_` + 32 bajty losowe (base64url), zapisać tylko jego hash i zwrócić token jeden raz

### FR-031: Scope'y — Must Have

THE SYSTEM SHALL znać zamkniętą listę scope'ów: `wallets:read`, `wallets:write`, `transactions:read`, `transactions:write`, `budget:read`, `budget:write`, `subscriptions:read`, `subscriptions:write`, `reports:read`, `notifications:read`; `*:write` nie implikuje `*:read`, a nieznany scope jest błędem walidacji

### FR-032: Efektywne uprawnienia — Must Have

WHEN bot wykonuje operację  
THE SYSTEM SHALL zezwolić tylko, gdy (a) bot ma wymagany scope, (b) twórca nadal jest członkiem gospodarstwa z rolą dającą to uprawnienie (`viewer` → tylko scope'y `:read`), (c) operacja dotyczy portfela z listy bota (gdy lista niepusta) i widocznego dla twórcy

### FR-033: Zakaz administracji — Must Have

WHEN bot wywoła operację administracyjną (członkowie, zaproszenia, boty, ustawienia gospodarstwa, audit log)  
THE SYSTEM SHALL odpowiedzieć 403

### FR-034: Cykl życia tokenu — Must Have

THE SYSTEM SHALL odrzucać tokeny botów wyłączonych, wygasłych, usuniętych lub sprzed rotacji (401) i aktualizować `last_used_at` najwyżej raz na minutę

### FR-035: Bearer w REST — Must Have

WHEN żądanie REST ma nagłówek `Authorization: Bearer dnr_…`  
THE SYSTEM SHALL uwierzytelnić bota zamiast sesji; żądanie z obydwoma (cookie i Bearer) jest odrzucane (400)

### FR-036: Komenda `token` — Should Have

THE SYSTEM SHALL pozwalać administratorowi instancji wygenerować z CLI token bota dla istniejącego gospodarstwa (`denar token create --household … --scopes …`) na potrzeby automatyzacji bez UI

### Audit

### FR-040: Wpisy audytu — Must Have

WHEN use case zmienia stan gospodarstwa  
THE SYSTEM SHALL zapisać w tej samej transakcji wpis: czas, aktor (user/bot + ID), akcja, typ i ID zasobu, zmienione pola (bez sekretów)

### FR-041: Przegląd audytu — Must Have

WHEN `owner`/`admin` pobierze audit log gospodarstwa  
THE SYSTEM SHALL zwrócić wpisy od najnowszych, stronicowane kursorem, z filtrami po aktorze i typie zasobu

### Kontrakt i błędy

### FR-050: Kontrakt REST — Must Have

THE SYSTEM SHALL opisać wszystkie endpointy w `api/openapi.yaml` (OpenAPI 3.1) pod `/api`, z zasobami gospodarstwa pod `/api/households/{householdId}/…`

### FR-051: Format błędów — Must Have

THE SYSTEM SHALL zwracać błędy jako `application/problem+json` (RFC 7807) z `type`, `code`, `title`, `detail` i `errors[]{field,message}`: 401, 403, 404, 409, 422, 429, 500

## Non-Functional Requirements

### NFR-001: Security

- Hasła argon2id; tokeny sesji, zaproszeń i botów jako SHA-256; porównania w stałym czasie.
- Sekrety konfiguracji jako typ redagowany w logach.
- Nagłówki bezpieczeństwa (CSP dla SPA, `X-Content-Type-Options`, `Referrer-Policy`).
- Rate limit na logowanie i Bearer.

### NFR-002: Testability

- Domena i use case'y testowane na fake'ach portów; macierz autoryzacji (rola × scope × status twórcy × typ principala).
- Suity kontraktowe repozytoriów (`portstest`) na SQLite.
- Testy konformancji REST z `api/openapi.yaml`.
- `internal/arch_test.go` pilnuje reguły zależności.

### NFR-003: Operability

- Jedno binarium bez cgo; obraz Docker non-root z wolumenem `/data`; konfiguracja wyłącznie przez `DENAR_*`.
- Logi strukturalne `slog`.

### NFR-004: Performance

- Uwierzytelnienie żądania (sesja lub Bearer) < 5 ms p95 na SQLite lokalnie.

## Questions

| ID | Question | Proposed answer | Status |
|----|----------|-----------------|--------|
| Q-001 | Czy dopuszczamy samodzielną rejestrację kont? | Nie — konta tylko przez konto startowe, zaproszenia lub OIDC (opcja `DENAR_OIDC_AUTO_CREATE` dla dowolnego użytkownika IdP, domyślnie wyłączona). | answered |
| Q-002 | Kto może zakładać gospodarstwa? | Każdy zalogowany człowiek. | answered |
| Q-003 | Czy można zmienić walutę bazową gospodarstwa? | Nie w MVP (zmiana wymagałaby przeliczenia wszystkich snapshotów `BaseAmount`). | answered |
| Q-004 | Czy wysyłać zaproszenia e-mailem? | W 001 tylko link do skopiowania; e-mail po SMTP w etapie 005. | answered |
| Q-005 | Domyślna ważność tokenu bota? | Bez wygaśnięcia, chyba że podano `expires_at`; UI w 007 zasugeruje 90 dni. | answered |
| Q-006 | Czy `member` może tworzyć własne boty? | Tak — skoro bot nie przekracza uprawnień twórcy, `member` może tworzyć i zarządzać botami, których jest twórcą; `owner`/`admin` zarządzają wszystkimi botami gospodarstwa. Wymaga zmiany FR-030 i US-005. | answered |

## Decisions

| ID | Decision | Source |
|----|----------|--------|
| D-001 | Wiele gospodarstw na instancję, role `owner/admin/member/viewer` | PLAN.md, TD-003 |
| D-002 | Bot = scope'y ∩ uprawnienia twórcy ∩ lista portfeli; boty bez operacji administracyjnych | PLAN.md, TD-004, P-003 |
| D-003 | Bearer botów akceptowany w REST i (od 006) w MCP | PLAN.md |
| D-004 | Logowanie lokalne + OIDC | PLAN.md |
| D-005 | Brak samodzielnej rejestracji; konta z zaproszeń, OIDC lub konta startowego; `DENAR_OIDC_AUTO_CREATE` domyślnie wyłączone | Q-001 |
| D-006 | Każdy zalogowany człowiek może zakładać gospodarstwa | Q-002 |
| D-007 | Waluta bazowa niezmienna w MVP | Q-003 |
| D-008 | Zaproszenia w 001 jako link do skopiowania; e-mail od 005 | Q-004 |
| D-009 | Token bota domyślnie bez wygaśnięcia; UI sugeruje 90 dni | Q-005 |
| D-010 | `member` tworzy i zarządza własnymi botami; `owner`/`admin` wszystkimi; `viewer` nie tworzy botów | Q-006 |

## Traceability

| Requirement | User Story |
|-------------|------------|
| FR-001–003 | US-001 |
| FR-010–014 | US-002 |
| FR-020–022 | US-003 |
| FR-022–024 | US-004 |
| FR-030–036 | US-005 |
| FR-040–041 | US-006 |
| FR-050–051 | wszystkie |
