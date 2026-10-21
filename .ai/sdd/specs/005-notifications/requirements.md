# Feature: Powiadomienia (F05, etap 5)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (plan:approved, „Powiadomienia”), brief orkiestratora etapu 005, `004-subscriptions/design.md` („Dla etapu 005”), `006-mcp/design.md` („Dla etapu 005”), `001-core` (Q-004/D-008: zaproszenia e-mailem)
> Last Updated: 2026-10-21

## Overview

Zdarzenia z outboxu (`domain_events`, etap 004) zamieniają się w powiadomienia: w aplikacji (in-app, per użytkownik) i na kanałach zewnętrznych użytkownika (e-mail SMTP, webhook z podpisem HMAC, ntfy, Telegram, Matrix). Użytkownik wybiera per gospodarstwo i typ zdarzenia, czy chce powiadomienie in-app i na które kanały. Dostawy mają własny outbox z ponawianiem (backoff wykładniczy). Sekrety kanałów są szyfrowane AES-GCM kluczem `DENAR_MASTER_KEY`. Dochodzą zdarzenia budżetu: przekroczona koperta i zbliżający się termin celu.

## Business Context

- **Problem:** Przypomnienia o odnowieniach leżą w outboxie, ale nikt ich nie widzi.
- **Users:** Członkowie gospodarstwa (każda rola, także viewer); boty czytają powiadomienia twórcy przez MCP.
- **Value:** Brak niespodzianek: odnowienia, końce triali, podwyżki, przegapione płatności, przekroczone koperty — tam, gdzie użytkownik je czyta.
- **Success signal:** e2e: subskrypcja z przypomnieniem → job → powiadomienie in-app → webhook do `httptest` z poprawnym podpisem; `task verify` zielone.

## Goals

- Dispatch zdarzeń do odbiorców (widoczność portfeli), powiadomienia in-app, preferencje, kanały, worker dostaw z retry.
- Adaptery `notify/{smtp,webhook,ntfy,telegram,matrix}` za portem `Notifier`.
- Szyfrowanie sekretów (`platform/crypto`, port `SecretBox`).
- REST pod `/api/me/…`, narzędzie MCP `list_notifications`, zaproszenia e-mailem (Should).

## Non-Goals / Out of Scope

- Powiadomienia push przeglądarki, SSE/WebSocket na żywo (UI odpytuje licznik).
- Kanały gospodarstwa (wspólne) — kanały należą do użytkownika.
- Szablony edytowalne przez użytkownika, inne języki niż polski.
- Ochrona przed SSRF adresów kanałów (instancja self-hosted z zaufanymi członkami) — odnotowane w ograniczeniach.

## User Stories

### US-050: Widzę powiadomienia w aplikacji
**As a** członek gospodarstwa **I want to** listę powiadomień z licznikiem nieprzeczytanych **So that** wiem, co się odnawia i co wymaga uwagi.

### US-051: Dostaję powiadomienia tam, gdzie je czytam
**As a** użytkownik **I want to** dodać kanał (e-mail, webhook, ntfy, Telegram, Matrix), wysłać test i wybrać, które zdarzenia tam trafiają **So that** nie muszę otwierać aplikacji.

### US-052: Prywatność
**As a** właściciel prywatnego portfela **I want to** żeby powiadomienia o subskrypcjach z mojego portfela dostawali tylko ci, którzy go widzą **So that** inni członkowie nie poznają moich wydatków.

### US-053: Asystent czyta powiadomienia
**As a** użytkownik asystenta **I want to** zapytać „co nowego?” **So that** bot streszcza moje powiadomienia (w zakresie swojego gospodarstwa i portfeli).

## Functional Requirements

### Zdarzenia budżetu

- **FR-500:** WHEN an envelope's available amount in the current budget month is below zero THEN the system SHALL append `envelope.overspent` once per envelope and month (dedupe `overspent:<envelope>:<YYYY-MM>`).
- **FR-501:** WHEN an envelope has a `target_by_date` goal with the target date within the next `DENAR_GOAL_DUE_DAYS` days (today included) and its available amount is below the goal amount THEN the system SHALL append `envelope.goal_due` once per envelope and target date.
- **FR-502:** The budget events SHALL be detected by a periodic job (`DENAR_BUDGET_CHECK_INTERVAL`), in each household's time zone and month start day, and carry `wallet_id = ""` (the budget is the household's).

### Dispatch

- **FR-510:** The dispatch job SHALL take pending events oldest first and, per event in one database transaction, create the in-app notifications and channel deliveries of every recipient and mark the event dispatched.
- **FR-511:** Recipients SHALL be the active users who are members of the event's household (any role, viewer included) and see the wallet of the event's `wallet_id` (`wallet.VisibleTo`); an empty `wallet_id` means every member; an unknown wallet means nobody. Bots SHALL never be recipients.
- **FR-512:** For each recipient the preference of (user, household, event type) SHALL decide: in-app on → one notification; each listed channel that is the user's and enabled → one delivery. Without a stored preference: in-app on, no channels.
- **FR-513:** Notification texts SHALL be Polish, rendered from the event type and payload (title, body, link). Links SHALL follow the Web UI routes: `/h/{hid}/subscriptions/{sid}`, `/h/{hid}/subscriptions/charges`, `/h/{hid}/budget/{YYYY-MM}`; external channels SHALL get the absolute URL `{DENAR_BASE_URL}{link}`.
- **FR-514:** Dispatching SHALL be idempotent: at most one notification per (event, user) and one delivery per (event, user, channel).

### Dostawy

- **FR-520:** The delivery worker SHALL send due pending deliveries through the `Notifier` of the channel type, outside the database transaction, and store the outcome.
- **FR-521:** WHEN a send fails THEN the system SHALL retry after `1 min · 2^(attempt−1)` (at most 1 h); after 6 attempts the delivery SHALL become `failed` and the channel SHALL keep `last_error`.
- **FR-522:** A delivery to a channel that was deleted SHALL disappear; to a disabled channel or one whose secrets cannot be opened SHALL fail at once with the reason.
- **FR-523:** A successful send SHALL set the channel's `verified_at` and clear `last_error`.

### Kanały

- **FR-530:** A person SHALL manage their own channels (at most 20): type `email|webhook|ntfy|telegram|matrix`, name (1–100, unique per user ignoring case), configuration, secrets, enabled.
- **FR-531:** Configuration per type: email `address` (default: the user's e-mail); webhook `url` (http/https), secret `secret` (optional, HMAC key); ntfy `server` (default `https://ntfy.sh`), `topic`, secret `token` (optional); telegram `chat_id`, secret `bot_token` (required); matrix `homeserver`, `room_id`, secret `access_token` (required).
- **FR-532:** Secrets SHALL be stored only encrypted (AES-256-GCM, key `DENAR_MASTER_KEY[_FILE]`, 32 bytes base64, the channel id as associated data) and never returned (only `has_secret`).
- **FR-533:** WHEN no master key is configured THEN creating or updating a channel with secrets SHALL fail with 409 `secrets-unavailable`, and deliveries to such channels SHALL fail; in-app and e-mail SHALL work.
- **FR-534:** WHEN SMTP is not configured THEN e-mail channels SHALL be refused with 409 `smtp-not-configured`.
- **FR-535:** "Send test" SHALL send a test message synchronously and return `{ok, error}`, updating `verified_at`/`last_error`.
- **FR-536:** Bots SHALL NOT manage channels or preferences (403 `human-only`).

### Adaptery

- **FR-540:** SMTP: global configuration `DENAR_SMTP_HOST|PORT|USERNAME|PASSWORD[_FILE]|FROM|TLS` (`starttls` default, `tls`, `none`), `net/smtp`, UTF-8 plain-text message.
- **FR-541:** Webhook: POST JSON `{id, type, title, body, url, household{id,name}, occurred_at, data}` with `X-Denar-Signature: sha256=<hex HMAC-SHA256(secret, body)>` when the channel has a secret, `X-Denar-Event`, `X-Denar-Delivery`.
- **FR-542:** ntfy: POST `{server}/{topic}` with the body as message, `Title` and `Click` headers, `Authorization: Bearer <token>` when set.
- **FR-543:** Telegram: Bot API `sendMessage` (`DENAR_TELEGRAM_API_URL`, default `https://api.telegram.org`).
- **FR-544:** Matrix: `PUT {homeserver}/_matrix/client/v3/rooms/{room}/send/m.room.message/{txn}` with the access token; `txn` = delivery id (idempotent).
- **FR-545:** A response other than 2xx (and for Telegram `ok:false`) SHALL be an error with a short reason, without secrets.

### Powiadomienia in-app i preferencje

- **FR-550:** A person SHALL list their notifications newest first with a cursor, filtered by household and unread; count unread; mark one or all (optionally of a household) as read.
- **FR-551:** A person SHALL read and replace their preferences per household for every event type; channel ids must be their own channels (else 422).
- **FR-552:** Read notifications older than 180 days SHALL be deleted by a job; finished deliveries older than 30 days too; dispatched events SHALL NOT be deleted before 400 days.

### MCP i boty

- **FR-560:** MCP tool `list_notifications` (scope `notifications:read`) SHALL list the notifications of the bot's creator in the bot's household, newest first, hiding those of wallets outside the bot's wallet list; REST `GET /me/notifications` and `/unread-count` SHALL behave the same for a bot; marking as read SHALL be for people only.

### Zaproszenia

- **FR-570 (Should):** WHEN SMTP is configured and the invitation asks for it (`send_email`) THEN the system SHALL e-mail the invitation link after creating it and report `email_sent`; without SMTP `send_email` SHALL give 409 `smtp-not-configured`.

## Non-Functional Requirements

- **NFR-050:** Secrets never in logs, audit, API answers or error messages.
- **NFR-051:** Network calls to channels outside database transactions, timeout 10 s each.
- **NFR-052:** Adapters contain no business rules; templates, recipients, preferences and retry policy live in domain/use cases (P-004).

## Acceptance Criteria

- `go test -race ./...` zielone; `task verify` zielone.
- E2E: subskrypcja z przypomnieniem → job subskrypcji → dispatch → powiadomienie in-app (REST) → webhook do `httptest` z poprawnym podpisem.
