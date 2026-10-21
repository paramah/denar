# Design: Powiadomienia (005-notifications)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `004-subscriptions/design.md` §3, `006-mcp/design.md`
> Last Updated: 2026-10-21

## 1. Architektura (nowe elementy)

```
internal/domain/event/            + EnvelopeOverspent, EnvelopeGoalDue, ResourceEnvelope
internal/domain/notification/     channel.go (ChannelType, Spec, Channel, Normalize), notification.go (Notification),
                                  preference.go (Preference, Default), delivery.go (Delivery, Backoff), render.go (szablony PL)
internal/platform/crypto/         AES-256-GCM Box (Seal/Open), ParseKey
internal/usecase/                 notifications.go (in-app), channels.go (kanały, test), preferences.go,
                                  notification_dispatch.go (dispatch, odbiorcy), notification_delivery.go (worker),
                                  budget_events.go (overspent/goal_due), notification_jobs.go (retencja, RunNotificationJobs)
internal/usecase/{fakes,portstest} NotificationRepository, ChannelRepository, PreferenceRepository, DeliveryRepository;
                                  EventOutbox.DeleteDispatchedBefore; HouseholdRepository.ListIDs
internal/adapter/sqlite/          00005_notifications.sql, notification.go, channel.go, preference.go, delivery.go
internal/adapter/notify/{smtp,webhook,ntfy,telegram,matrix}   każdy implementuje usecase.Notifier (smtp także Mailer)
internal/adapter/rest/            handlers_notifications.go, mapping_notifications.go
internal/adapter/mcp/             tools_notifications.go (list_notifications)
internal/platform/config/         SMTP, MASTER_KEY, NOTIFY_INTERVAL, BUDGET_CHECK_INTERVAL, GOAL_DUE_DAYS, TELEGRAM_API_URL
cmd/denar/                        wiring notifierów, joby, e2e_notifications_test.go
```

Zmiany w kodzie wcześniejszych etapów (integracja, minimalne):
- `HouseholdRepository.ListIDs(ctx)` — job budżetu musi przejść po wszystkich gospodarstwach.
- `EventOutbox.DeleteDispatchedBefore(ctx, before)` — retencja zdarzeń (≥ 400 dni).
- `CreateInvitation`: pole `SendEmail`, wynik `EmailSent`/`EmailError`; `InvitationConfig.BaseURL`.
- `fakes.Repos`, kaskada gospodarstwa (powiadomienia, preferencje, dostawy), `portstest.Env`.
- `domain.go`: powody 409 `secrets_unavailable`, `smtp_not_configured`.

## 2. Domena

### Zdarzenia budżetu (`domain/event`)
```go
EnvelopeOverspent Type = "envelope.overspent"   // dedupe overspent:<envelope_id>:<YYYY-MM>
EnvelopeGoalDue   Type = "envelope.goal_due"    // dedupe goal_due:<envelope_id>:<target_date>
ResourceEnvelope = "envelope"
```
Payload: wspólne `envelope_id, envelope_name, month, available, currency, wallet_id ("")`; `goal_due` dodatkowo `target_date, goal_amount, missing, days_left`. `event.Types` ma 6 typów (stała kolejność: 4 subskrypcji, potem 2 kopert).

### notification
- `ChannelType`: `email|webhook|ntfy|telegram|matrix`; `ChannelTypes` (kolejność stała).
- `Spec(t) TypeSpec{Config, Secrets []Field{Name, Required}}`:

| Typ | config | secrets |
|---|---|---|
| email | `address` (opcjonalny — domyślnie e-mail użytkownika) | — |
| webhook | `url` (wymagany, http/https) | `secret` (opcjonalny) |
| ntfy | `server` (opcjonalny, domyślnie `https://ntfy.sh`), `topic` (wymagany, `[A-Za-z0-9_-]{1,64}`) | `token` (opcjonalny) |
| telegram | `chat_id` (wymagany, ≤ 64) | `bot_token` (wymagany) |
| matrix | `homeserver` (wymagany, http/https), `room_id` (wymagany, zaczyna się od `!`) | `access_token` (wymagany) |

- `Channel{ID, UserID, Type, Name, Config map[string]string, SealedSecrets []byte, Enabled, VerifiedAt, LastError, LastErrorAt, CreatedAt, UpdatedAt}`; `HasSecret()`.
- `NormalizeConfig(t, config, defaultEmail) (map, error)` — trim, nieznane klucze → `ValidationError{config.<klucz>}`, wymagane, domyślne, walidacja URL/e-mail/topicu. `CheckSecrets(t, secrets, required bool)` — nieznane klucze odrzucone, puste wartości pomijane, wymagane sprawdzane przy tworzeniu.
- `Notification{ID, UserID, HouseholdID, EventID, Type, Title, Body, Link, WalletID, ResourceType, ResourceID, CreatedAt, ReadAt}`; `Read()`.
- `Preference{Type, InApp bool, ChannelIDs []ID}`; `DefaultPreference(t)` = in-app, bez kanałów; `Merge(stored []Preference) []Preference` — pełna lista dla `event.Types`.
- `Delivery{ID, EventID, UserID, HouseholdID, ChannelID, Type, Title, Body, URL, Data map[string]string, OccurredAt, Status pending|sent|failed, Attempts, NextAttemptAt, LastError, CreatedAt, UpdatedAt, SentAt}`.
  - `MaxAttempts = 6`; `Backoff(attempts) = 1 min · 2^(attempts−1)`, maks. 1 h; `Failure(now, reason)` → attempts+1, `failed` przy `attempts ≥ MaxAttempts`, inaczej `NextAttemptAt = now + Backoff`.
- `Render(e event.Event) Message{Title, Body, Link}` — szablony polskie (§6).

## 3. Porty (`usecase/ports.go`)

```go
type NotificationFilter struct {
    UserID      domain.ID   // wymagane
    HouseholdID domain.ID   // "" = wszystkie
    UnreadOnly  bool
    // RestrictWallets: tylko powiadomienia bez portfela albo z portfelem z WalletIDs (bot z listą portfeli)
    RestrictWallets bool
    WalletIDs       []domain.ID
}
type NotificationRepository interface {
    Create(ctx, notification.Notification) (bool, error)   // false: (event, user) już jest
    ByID(ctx, userID, id domain.ID) (notification.Notification, error)
    List(ctx, NotificationFilter, Page) ([]notification.Notification, string, error) // id malejąco
    CountUnread(ctx, NotificationFilter) (int, error)
    MarkRead(ctx, userID, id domain.ID, at time.Time) error         // już przeczytane — bez zmiany
    MarkAllRead(ctx, NotificationFilter, at time.Time) (int, error)
    DeleteReadBefore(ctx, before time.Time) (int, error)
}
type ChannelRepository interface {
    Create(ctx, notification.Channel) error                // duplicate_name (per użytkownik, bez wielkości liter)
    Update(ctx, notification.Channel) error                // wszystko poza id, użytkownikiem, typem, created_at
    Delete(ctx, userID, id domain.ID) error                // z dostawami
    ByID(ctx, userID, id domain.ID) (notification.Channel, error)
    ListByUser(ctx, userID domain.ID) ([]notification.Channel, error) // wg nazwy, id
}
type PreferenceRepository interface {
    List(ctx, userID, householdID domain.ID) ([]notification.Preference, error)
    Replace(ctx, userID, householdID domain.ID, prefs []notification.Preference) error
}
type DeliveryRepository interface {
    Create(ctx, notification.Delivery) (bool, error)       // false: (event, user, channel) już jest
    ListDue(ctx, now time.Time, limit int) ([]notification.Delivery, error) // pending, next_attempt_at ≤ now
    Update(ctx, notification.Delivery) error               // status, attempts, next_attempt_at, last_error, updated_at, sent_at
    DeleteFinishedBefore(ctx, before time.Time) (int, error)
}
type ChannelTarget struct { Type notification.ChannelType; Config, Secrets map[string]string }
type OutgoingMessage struct { ID, Type, Title, Body, URL string; HouseholdID domain.ID; HouseholdName string;
    OccurredAt time.Time; Data map[string]string }
type Notifier interface { Send(ctx, ChannelTarget, OutgoingMessage) error }
type Mailer interface { SendMail(ctx, to, subject, body string) error }
type SecretBox interface { Seal(plaintext, aad []byte) ([]byte, error); Open(sealed, aad []byte) ([]byte, error) }
```
`Deps` dostaje `Notifications, Channels, Preferences, Deliveries`, `Notifiers map[notification.ChannelType]Notifier`, `Mailer`, `Secrets SecretBox` (nil = brak klucza), `BaseURL`, `GoalDueDays` (0 → 7).

## 4. Use case'y i autoryzacja

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `ListNotifications` | człowiek: własne; bot: `notifications:read` | filtr `household_id`, `unread`; człowiek z `household_id` spoza członkostw → 404; bot: zawsze swoje gospodarstwo (inne → 404), powiadomienia twórcy, lista portfeli bota |
| `CountUnreadNotifications` | jak wyżej | |
| `MarkNotificationRead`, `MarkAllNotificationsRead` | człowiek | cudze/nieistniejące → 404; read-all opcjonalnie w gospodarstwie |
| `ListChannelTypes` | człowiek | `available` + `unavailable_reason` (`smtp_not_configured`, `secrets_unavailable` dla typów z wymaganym sekretem) |
| `ListChannels`, `GetChannel`, `CreateChannel`, `UpdateChannel`, `DeleteChannel`, `TestChannel` | człowiek, własne kanały | ≤ 20 kanałów; sekrety → `SecretBox` (brak → 409 `secrets_unavailable`); email bez SMTP → 409 `smtp_not_configured` |
| `GetNotificationPreferences`, `SetNotificationPreferences` | człowiek, członek gospodarstwa (każda rola) | PUT zastępuje całość; brakujące typy = domyślne; nieznany typ / obcy kanał → 422 |
| `CheckBudgets` | system (job) | §5 |
| `DispatchEvents`, `DeliverNotifications`, `PruneNotifications`, `RunNotificationJobs` | system (job) | aktor `system/notifications` |

Kanały i preferencje są ustawieniami osobistymi (nie dane gospodarstwa) — bez audytu gospodarstwa (AD-507).

### Dispatch (`notification_dispatch.go`)
`DispatchEvents.Do(ctx)`: `ListPending(100)`; dla każdego zdarzenia `WithinTx`:
1. odbiorcy: `Memberships.ListByHousehold` → `Users.ByIDs` (aktywni) → filtr portfela: `wallet_id ≠ ""` → `Wallets.ByID` (brak → nikt) → `w.VisibleTo(user)`;
2. `Render(event)`; dla odbiorcy preferencja typu (`Preferences.List` + `Merge`); in-app → `Notifications.Create`; kanały z preferencji, które są kanałami użytkownika i są włączone → `Deliveries.Create` (pending, `NextAttemptAt = now`, URL absolutny);
3. `MarkDispatched(id, now)`.
Gospodarstwo zdarzenia czytane raz (nazwa do treści kanałów).

### Dostawy (`notification_delivery.go`)
`DeliverNotifications.Do(ctx) (DeliveryResult{Sent, Failed, Retried})`: `ListDue(now, 50)`; dla każdej: kanał (`ByID`; brak → dostawa usunięta kaskadą, pominięta), wyłączony → `failed` „kanał wyłączony”; sekrety `Open` (błąd/brak klucza → `failed`); notifier typu (brak → `failed`); `Send` z timeoutem 10 s **poza transakcją**; potem `WithinTx`: ponowny odczyt dostawy? (nie — jeden worker na proces), `Update` dostawy + kanału (`VerifiedAt`/`LastError`). Błąd ostateczny (`failed`) zapisuje `LastError` kanału; błąd przejściowy — tylko w dostawie.

`TestChannel`: komunikat „Test kanału Denar” z ID losowym; wynik `{OK, Error, TestedAt}`; kanał zapisuje `VerifiedAt` albo `LastError`.

### Zaproszenia e-mailem
`CreateInvitationInput.SendEmail`: bez `Mailer` → 409 `smtp_not_configured` przed zapisem. Po zatwierdzeniu transakcji `Mailer.SendMail(email, "Zaproszenie do gospodarstwa …", treść z linkiem `{BaseURL}/invite#<token>`)`; błąd → `EmailSent=false`, `EmailError` (log), zaproszenie zostaje.

## 5. Joby

- `CheckBudgets.Do(ctx)` co `DENAR_BUDGET_CHECK_INTERVAL` (domyślnie 15m): `Households.ListIDs` → dla każdego gospodarstwa z kopertami: dziś w strefie, bieżący miesiąc budżetowy, `budgetMonth` → `overspent` dla kopert z `Available < 0`; `goal_due` dla celów `target_by_date` z `today ≤ TargetDate ≤ today + GoalDueDays` i `Available < Goal.Amount`. Jedna transakcja na gospodarstwo; dedupe outboxu.
- `RunNotificationJobs.Do(ctx)` co `DENAR_NOTIFY_INTERVAL` (domyślnie 30s): `DispatchEvents` → `DeliverNotifications`.
- `PruneNotifications.Do(ctx)` co godzinę (pętla sprzątania): przeczytane > 180 dni, dostawy zakończone > 30 dni, zdarzenia rozesłane > 400 dni.

## 6. Szablony (`notification.Render`)

| Typ | Tytuł | Treść | Link |
|---|---|---|---|
| renewal_upcoming | `Odnowienie: {name} {kiedy}` (`dziś`/`jutro`/`za N dni`) | `{name} ({vendor_name}) odnowi się {due_date}: {amount} {currency}.` | `/h/{hid}/subscriptions/{sid}` |
| trial_ending | `Koniec okresu próbnego: {name} {kiedy}` | `Okres próbny {name} kończy się {trial_ends_at}. Potem {amount} {currency} za okres.` | j.w. |
| price_changed | `Zmiana ceny: {name}` | `Od {effective_from} {name} kosztuje {new} (było {old}).` / bez starej: `…kosztuje {new}.` | j.w. |
| charge_missed | `Brak płatności: {name}` | `Nie znaleziono płatności {name} z {due_date} ({amount} {currency}). Sprawdź dopasowania.` | `/h/{hid}/subscriptions/charges` |
| overspent | `Przekroczona koperta: {envelope_name}` | `Koperta {envelope_name} jest na minusie w miesiącu {month}: {available} {currency}.` | `/h/{hid}/budget/{month}` |
| goal_due | `Cel koperty {envelope_name} {kiedy}` | `Termin celu {target_date}: brakuje {missing} {currency} z {goal_amount} {currency}.` | `/h/{hid}/budget/{month}` |

Nieznany typ: tytuł = typ, treść pusta, link `/h/{hid}`. Kanały zewnętrzne dopisują do treści stopkę `Gospodarstwo: {nazwa}` i URL.

## 7. Schemat SQL (`00005_notifications.sql`)

- `notifications(id PK, user_id → users CASCADE, household_id → households CASCADE, event_id, type, title, body, link, wallet_id, resource_type, resource_id, created_at, read_at?, UNIQUE(event_id, user_id))` + indeksy `(user_id, id DESC)`, `(user_id, read_at)`.
- `notification_channels(id PK, user_id → users CASCADE, type CHECK, name, config JSON, secrets BLOB?, enabled, verified_at?, last_error, last_error_at?, created_at, updated_at)` + unikalny `(user_id, name COLLATE NOCASE)`.
- `notification_preferences(user_id → users CASCADE, household_id → households CASCADE, type, in_app, channel_ids JSON, PK(user_id, household_id, type))`.
- `notification_deliveries(id PK, event_id, user_id → users CASCADE, household_id → households CASCADE, channel_id → notification_channels CASCADE, type, title, body, url, data JSON, occurred_at, status CHECK, attempts, next_attempt_at, last_error, created_at, updated_at, sent_at?, UNIQUE(event_id, user_id, channel_id))` + częściowy indeks `(next_attempt_at) WHERE status = 'pending'`.
- Brak FK `event_id` → `domain_events` (retencja zdarzeń niezależna od powiadomień).

## 8. REST (`api/openapi.yaml`, tag `notifications`)

| Operacja | Ścieżka | Odpowiedzi |
|---|---|---|
| `listNotifications` | `GET /me/notifications?household_id&unread&cursor&limit` | 200 `NotificationPage`, 403, 404, 422 |
| `countUnreadNotifications` | `GET /me/notifications/unread-count?household_id` | 200 `UnreadCount{count}`, 403, 404 |
| `markNotificationRead` | `POST /me/notifications/{notificationId}/read` | 200 `Notification`, 403, 404 |
| `markAllNotificationsRead` | `POST /me/notifications/read-all` body `{household_id?}` | 200 `ReadAllResult{updated}`, 403, 404 |
| `listChannelTypes` | `GET /me/channel-types` | 200 `ChannelTypeList`, 403 |
| `listChannels`, `createChannel` | `GET|POST /me/channels` | 200/201, 403, 409, 422 |
| `getChannel`, `updateChannel`, `deleteChannel` | `GET|PATCH|DELETE /me/channels/{channelId}` | 200/204, 403, 404, 409, 422 |
| `testChannel` | `POST /me/channels/{channelId}/test` | 200 `ChannelTestResult{ok, error, tested_at}`, 403, 404 |
| `getNotificationPreferences`, `setNotificationPreferences` | `GET|PUT /me/households/{householdId}/notification-preferences` | 200 `NotificationPreferences`, 403, 404, 422 |

Schematy: `Notification{id, household_id, type, title, body, link, resource_type, resource_id, created_at, read_at}`; `Channel{id, type, name, config{…: string}, has_secret, enabled, verified_at, last_error, last_error_at, created_at, updated_at}`; `ChannelCreate{type, name, config?, secrets?, enabled?}`; `ChannelUpdate{name?, config? (zastępuje), secrets? (obiekt zastępuje, null czyści), enabled?}`; `ChannelTypeInfo{type, available, unavailable_reason, config_fields[], secret_fields[]: {name, required}}`; `NotificationPreference{type, in_app, channel_ids}`. `InvitationCreate.send_email?`, `InvitationCreated.email_sent`.

## 9. MCP

`list_notifications` (`notifications:read`): wejście `unread_only?`, `limit?` (1–200, 50), `cursor?`; wynik `{ notifications: [{id, type, title, body, link, url, created_at, read}], unread_count, next_cursor }`. Kontrakt `api/mcp-tools.md`; `contract_test.go` oczekuje 26 narzędzi.

## 10. Przypadki brzegowe

- Zdarzenie subskrypcji z prywatnego portfela: tylko członkowie portfela; owner bez dostępu do portfela nie dostaje (prywatność ponad rolę).
- Członek usunięty po zdarzeniu, przed dispatchem: nie dostaje (odbiorcy liczeni przy dispatchu).
- Kanał usunięty → jego dostawy znikają (CASCADE); preferencje ze starym id — pomijane przy dispatchu i w GET.
- Brak klucza po restarcie (kanał miał sekrety) → dostawa `failed` „brak DENAR_MASTER_KEY”; `TestChannel` → `ok:false`.
- Zmiana klucza → `Open` się nie uda → jak wyżej; użytkownik ustawia sekrety ponownie.
- Webhook bez sekretu → bez nagłówka podpisu.
- Retry: zegar fałszywy w testach; `NextAttemptAt` liczony od chwili błędu.

## 11. Strategia testów

- Domena: szablony (każdy typ, `kiedy`, brak starej ceny), `NormalizeConfig`/`CheckSecrets` (tabelarycznie per typ), `Backoff`/`Failure`, `Merge`, crypto (round-trip, AAD, zły klucz, zmieniony szyfrogram).
- Use case'y na fake'ach: odbiorcy (portfel wspólny / prywatny / bez portfela / nieznany, viewer, nieaktywny, bot nie), preferencje (domyślne, in-app off, kanały wyłączone/obce), idempotencja dispatchu, worker (sukces, retry z backoffem na fałszywym zegarze, failed po 6 próbach + `last_error`, wyłączony kanał, brak klucza), kanały (CRUD, limity, sekrety tylko zaszyfrowane, brak klucza → 409, email bez SMTP → 409, test), powiadomienia (lista, filtr, kursor, read/read-all, cudze → 404, bot z listą portfeli), budżet (overspent raz w miesiącu, goal_due w oknie), retencja, zaproszenie e-mailem.
- `portstest`: suity nowych repozytoriów + `ListIDs`, `DeleteDispatchedBefore`, kaskady — fake'i i SQLite.
- Adaptery `notify/*` na `httptest.Server` (podpis, nagłówki, błędy 4xx/5xx, Telegram `ok:false`, Matrix txn); SMTP na fałszywym serwerze SMTP w teście (`none`, `starttls`, `tls` z certyfikatem testowym).
- REST: konformancja każdej nowej operacji × każda odpowiedź; MCP: test narzędzia + kontrakt.
- E2E `cmd/denar`: subskrypcja (offset 3) → job subskrypcji → `RunNotificationJobs` → `GET /api/me/notifications` → kanał webhook (`httptest`, sekret) z preferencją → dostawa z poprawnym `X-Denar-Signature`.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-501 | Zdarzenia budżetu wykrywa tylko job (`DENAR_BUDGET_CHECK_INTERVAL`, 15 min), nie hak po zapisie transakcji/przydziału | Jeden punkt detekcji zamiast wpinania w 6 use case'ów zapisu; opóźnienie ≤ 15 min akceptowalne dla budżetu domowego |
| AD-502 | `goal_due` = cel `target_by_date` z terminem w `[dziś, dziś + DENAR_GOAL_DUE_DAYS]` (domyślnie 7) i `Available < Amount` | Najprostsza interpretacja „niedofinansowany”; dedupe per koperta × data celu |
| AD-503 | Bot (MCP i REST) widzi powiadomienia twórcy w gospodarstwie bota, bez tych z portfelem spoza listy bota; nie oznacza przeczytanych | Rekomendacja orkiestratora; bot ≤ twórca; `notifications:read` to scope tylko do odczytu |
| AD-504 | Kanały i preferencje tylko dla ludzi; kanały per użytkownik, nie per gospodarstwo; preferencje per (użytkownik, gospodarstwo, typ) | Kanał to adres osoby; różne gospodarstwa mogą chcieć różnych kanałów |
| AD-505 | Dostawa przechowuje kopię treści (tytuł, treść, URL, payload), więc działa także przy wyłączonym in-app | Brak zależności dostawy od powiadomienia in-app |
| AD-506 | Retry: 6 prób, 1, 2, 4, 8, 16 min (maks. 1 h); jeden worker na proces, bez blokad wierszy | Jedna instancja self-hosted; Matrix dedupuje po `txn = id dostawy`, webhook dostaje `X-Denar-Delivery` |
| AD-507 | Zmiany kanałów i preferencji bez wpisów audytu gospodarstwa | Ustawienia osobiste, nie dane finansowe; audit log jest per gospodarstwo |
| AD-508 | Podpis webhooka: `X-Denar-Signature: sha256=<hex HMAC-SHA256(secret, surowe body)>`; sekret opcjonalny | Konwencja GitHub/Stripe; webhook bez klucza głównego działa bez podpisu |
| AD-509 | Sekrety kanału jako jeden zaszyfrowany obiekt JSON, AAD = id kanału; format `0x01 ‖ nonce(12) ‖ szyfrogram` | Szyfrogram nie da się przenieść na inny kanał; wersja pozwala na rotację formatu |
| AD-510 | Bez klucza głównego: kanał z sekretem → 409 `secrets-unavailable`; webhook/ntfy bez sekretów i e-mail działają | Zgodnie z briefem; czytelny błąd zamiast cichego zapisu jawnie |
| AD-511 | `UpdateChannel.secrets`: obiekt zastępuje wszystkie sekrety, `null` czyści, brak pola zostawia | Proste, jednoznaczne; sekretów nie da się odczytać, więc częściowe łączenie byłoby mylące |
| AD-512 | ntfy: tytuł w nagłówku `Title` kodowany RFC 2047 dla znaków spoza ASCII | Polskie znaki w nagłówkach HTTP; ntfy to obsługuje |
| AD-513 | Zaproszenie e-mailem wysyłane po zatwierdzeniu transakcji; błąd SMTP nie cofa zaproszenia (`email_sent: false`) | Link nadal można skopiować z odpowiedzi |
| AD-514 | Retencja: przeczytane powiadomienia > 180 dni, zakończone dostawy > 30 dni, rozesłane zdarzenia > 400 dni (AD-406) | Zgodnie z briefem; dostawy to dane techniczne |
| AD-515 | `GET /me/channel-types` zwraca dostępność typów, żeby UI nie zgadywało konfiguracji serwera | UI potrzebuje wiedzieć o SMTP i kluczu głównym |
| AD-516 | Link w powiadomieniu in-app to ścieżka względna (`/h/…`); kanały dostają `{DENAR_BASE_URL}{link}` | SPA nawiguje ścieżką, zewnętrzne kanały potrzebują pełnego adresu |

## Ograniczenia

- Brak ochrony SSRF: webhook/ntfy/Matrix mogą wskazywać adresy w sieci lokalnej serwera (zaufani członkowie instancji).
- Overspent wykrywany z opóźnieniem do `DENAR_BUDGET_CHECK_INTERVAL`; po naprawieniu i ponownym przekroczeniu w tym samym miesiącu nie ma drugiego powiadomienia.
- Jeden worker dostaw na proces; dwie instancje na jednej bazie mogłyby wysłać dostawę dwukrotnie.
- Brak powiadomień na żywo (UI odpytuje `unread-count`).
