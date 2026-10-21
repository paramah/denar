# Feature: Serwer MCP (F06, etap 6)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (plan:approved, sekcja „Adaptery wejściowe → MCP”), brief orkiestratora etapu 006, `001-core/design.md` (AD-017, „Dla etapu 006”), `004-subscriptions/design.md` („Dla etapu 006”)
> Last Updated: 2026-10-21

## Overview

Serwer MCP pod `/mcp` (Streamable HTTP, bez sesji, odpowiedzi JSON) dla asystentów AI. Uwierzytelnia wyłącznie token bota (`Bearer dnr_…`), a każde narzędzie woła istniejący use case, więc uprawnienia (scope'y ∩ rola twórcy ∩ lista portfeli), audyt i `source=bot` działają jak w REST. Narzędzia czytają finanse, budżet, subskrypcje i raport wydatków oraz zapisują wydatki, przychody, przelewy, przydziały kopert i zmiany subskrypcji. Kontrakt w `api/mcp-tools.md`.

## Business Context

- **Problem:** Wprowadzanie wydatków i pytania („co odnawia się w tym tygodniu?”, „ile wydałem na jedzenie?”) wymagają otwierania UI.
- **Users:** Asystenci AI (Claude Code, Claude Desktop, inne klienty MCP) działający tokenem bota gospodarstwa.
- **Value:** Rozmowa z budżetem: zapis wydatku jednym zdaniem, odpowiedzi z danymi, bez wychodzenia poza uprawnienia bota.
- **Success signal:** Probe SDK, testy narzędzi na fake'ach (brak scope'u, portfel spoza listy, viewer-twórca), e2e klientem go-sdk (token → `whoami` → `record_expense` → `upcoming_renewals` → `subscription_costs` → `forbidden`) i `task verify` zielone.

## Goals

- Endpoint `/mcp` z uwierzytelnianiem botów, limitami i ochroną jak w Straylight.
- 30 narzędzi (odczyt i zapis) na istniejących use case'ach; nowe use case'y tylko tam, gdzie brakuje logiki (raport wydatków, rozpoznawanie nazw, kontekst bota).
- Kontrakt `api/mcp-tools.md` (po polsku), opisy narzędzi i pól po angielsku.
- README: podłączenie klientów.

## Non-Goals / Out of Scope

- Uwierzytelnianie ludzi (sesje, OIDC) w MCP; OAuth MCP.
- Sesje MCP, strumienie SSE, powiadomienia serwera, zasoby i prompty MCP.
- Narzędzia administracyjne (członkowie, boty, kanały) — boty ich nie wykonują (P-003).
- `list_notifications` — powiadomienia powstają w etapie 005; narzędzie dopisze ten etap.
- Usuwanie danych (transakcji, subskrypcji) przez MCP.

## User Stories

### US-060: Asystent zapisuje wydatek
**As a** użytkownik asystenta **I want to** powiedzieć „wydałem 42,50 w Biedronce z karty na jedzenie” **So that** wydatek trafia do portfela i koperty bez otwierania UI.

### US-061: Asystent odpowiada na pytania
**As a** użytkownik **I want to** pytać o salda, budżet miesiąca, odnowienia subskrypcji i wydatki według kopert/kontrahentów **So that** dostaję odpowiedź z aktualnych danych.

### US-062: Bezpieczeństwo
**As a** właściciel gospodarstwa **I want to** żeby bot widział i zmieniał tylko to, na co pozwalają jego scope'y, rola twórcy i lista portfeli **So that** asystent nie wycieka ani nie psuje danych.

## Functional Requirements

### Transport i uwierzytelnianie

- **FR-600:** The system SHALL serve MCP over Streamable HTTP at `/mcp`, POST only, stateless (no `Mcp-Session-Id`), answering `application/json`; GET/DELETE/OPTIONS give 405, a JSON-RPC batch 400, a body over 1 MiB 413.
- **FR-601:** The system SHALL authenticate every request only by `Authorization: Bearer dnr_…` with the bot authentication of stage 001; a missing, unknown, rotated, disabled or expired token SHALL give the same 401 with `WWW-Authenticate: Bearer`.
- **FR-602:** WHEN an address sends more than 20 rejected tokens per minute THEN the system SHALL answer 429 with `Retry-After`; WHEN a bot exceeds `DENAR_BOT_RATE` requests per minute THEN 429 with `Retry-After`.
- **FR-603:** The system SHALL refuse cross-origin browser requests and log one line per request (method, route, status, duration, bot id) without headers or tokens.
- **FR-604:** Each tool call SHALL be limited to 20 s; a panic or unknown error SHALL be logged and answered as a JSON-RPC internal error without details.

### Wspólne zasady narzędzi

- **FR-610:** Tools SHALL act in the bot's household without a `household_id` argument.
- **FR-611:** A tool result SHALL be a JSON object in `structuredContent` and the same JSON as the only text block; an error SHALL be `isError: true` with `{code, message}`, codes `not_found`, `forbidden`, `invalid`, `conflict`.
- **FR-612:** WHEN the bot lacks a scope THEN the tool SHALL answer `forbidden` naming the missing scope; a hidden resource SHALL answer `not_found` exactly like a missing one.
- **FR-613:** Amounts SHALL be decimal strings in input and `{amount, currency, minor}` in output; dates `YYYY-MM-DD`, months `YYYY-MM`.
- **FR-614:** Tools SHALL accept a wallet, envelope, payee and subscription by id or by name (case-insensitive); WHEN a name matches several THEN `invalid` listing the candidates; WHEN nothing visible matches THEN `not_found`.
- **FR-615:** Arguments not matching the input schema SHALL give `invalid`.

### Narzędzia

- **FR-620:** Read tools: `whoami`, `list_wallets`, `get_balances`, `search_transactions`, `list_payees`, `list_envelopes`, `budget_overview`, `spending_by`, `list_subscriptions`, `get_subscription`, `upcoming_renewals`, `subscription_costs`, `list_charges`, `list_vendors`.
- **FR-621:** Write tools: `record_expense`, `record_income`, `transfer`, `assign_to_envelope`, `move_between_envelopes`, `create_subscription`, `update_subscription`, `change_subscription_price`, `confirm_charge_match`, `post_charge`, `skip_charge`.
- **FR-622:** `record_expense` SHALL take a positive amount, default the date to today in the household's time zone, and return the transaction with charge suggestions WHEN the bot has `subscriptions:read` and `transactions:read` (else `null`).
- **FR-623:** `spending_by` SHALL group expenses of a period (default: the current budget month up to today) by envelope, payee, vendor or wallet in the base currency, over the wallets the bot sees, with scope `reports:read`; vendor through the charge linked to a transaction.
- **FR-624:** Every change made through MCP SHALL be audited with the bot as actor; recorded transactions SHALL have `source=bot` (subscription postings `source=subscription`).

## Non-Functional Requirements

- **NFR-060:** Contract `api/mcp-tools.md` and tool descriptions SHALL stay in step; a test SHALL check the tool list against the contract.
- **NFR-061:** Adapter SHALL contain no business rules (P-004); new logic lives in `internal/usecase` with tests and an authorization matrix.

## Acceptance Criteria

- `go test -race ./internal/adapter/mcp/... ./internal/usecase/... ./cmd/denar/...` zielone; `task verify` zielone.
- E2E: token bota → `whoami` → `record_expense` (po nazwie portfela i koperty) → `upcoming_renewals` → `subscription_costs` → narzędzie bez scope'u → `forbidden`.
