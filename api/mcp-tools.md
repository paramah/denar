# Kontrakt narzędzi MCP

Serwer MCP działa pod `/mcp` (Streamable HTTP). Asystent uwierzytelnia się tokenem bota gospodarstwa: `Authorization: Bearer dnr_…`. Implementacja: etapy `006-mcp`, `005-notifications` (`list_notifications`) i `008-import` (`list_imports`), `internal/adapter/mcp`; każde narzędzie woła use case, a reguły (scope'y, rola twórcy, lista portfeli, rozpoznawanie nazw, audyt) zostają w `internal/usecase`. Opisy narzędzi i pól widoczne dla modelu są po angielsku; ten dokument jest ich źródłem i opisem zachowania.

## Uwierzytelnianie i transport

- Token bota podaje się wyłącznie w nagłówku `Authorization: Bearer <token>`; nigdy w adresie ani w ciele żądania. Tokeny tworzy człowiek (owner/admin) w Ustawieniach → Boty albo `denar token create`.
- Brak tokenu, token nieznany, wymieniony (po rotacji), należący do wyłączonego albo wygasłego bota dają tę samą odpowiedź: `401` z nagłówkiem `WWW-Authenticate: Bearer` i ciałem `{"error":"unauthorized"}`.
- Ponad 20 odrzuconych tokenów na minutę z jednego adresu (IPv6: /64) → `429` z `Retry-After` dla tego adresu, aż okno minie. Bot może wykonać `DENAR_BOT_RATE` żądań na minutę (domyślnie 120; licznik MCP jest osobny od REST) → potem `429` z `Retry-After` (sekundy).
- Tylko `POST`; `GET`, `DELETE` i `OPTIONS` dają `405`. Jedno żądanie = jedna wiadomość JSON-RPC (batch → `400`). Ciało najwyżej 1 MiB (`413`). Żądania cross-origin z przeglądarki → `403`.
- Serwer działa bez sesji: nie nadaje `Mcp-Session-Id`, odpowiedzi mają typ `application/json` (bez SSE). `initialize` jest obsługiwane, ale niewymagane.
- Wywołanie narzędzia trwa najwyżej 20 s. Błąd serwera → błąd JSON-RPC `-32603 internal error` (szczegóły tylko w logu). Bot wyłączony lub z wymienionym tokenem w trakcie wywołania → `-32001 unauthorized`.

## Zasady wspólne

- Bot należy do jednego gospodarstwa — narzędzia nie przyjmują `household_id`.
- Uprawnienia: efektywne scope'y bota = jego scope'y, które dopuszcza bieżąca rola twórcy (viewer — tylko `*:read`; twórca spoza gospodarstwa — żadne) ∩ lista portfeli bota (pusta = wszystkie widoczne dla twórcy). Ukryty zasób (portfel spoza listy, cudzy portfel prywatny, subskrypcja płacona z takiego portfela) wygląda jak nieistniejący: `not_found`.
- Wynik narzędzia to obiekt JSON w `structuredContent` oraz ten sam JSON jako jedyny blok tekstu. Pole bez wartości ma `null` (nie jest pomijane).
- Błąd to wynik z `isError: true` i obiektem `{ "code", "message" }`:

| Kod | Kiedy |
|---|---|
| `not_found` | zasób nie istnieje albo bot go nie widzi; przy odwołaniu po nazwie: `wallet: no wallet you can see has this id or name` |
| `forbidden` | brak scope'u — komunikat podaje nazwę, np. `the bot lacks the scope transactions:write (or its creator's role does not allow it)…`; operacja sięga portfela, którego bot nie widzi |
| `invalid` | argumenty niezgodne ze schematem (typ, brak wymaganego, nieznane pole) albo walidacja use case'u: `pole: komunikat; …`; niejednoznaczna nazwa |
| `conflict` | stan nie pozwala: portfel zarchiwizowany, nazwa zajęta, obciążenie już dopasowane/zaksięgowane/pominięte, transakcja opłaca inne obciążenie |

- Odwołania po nazwie: portfel, koperta, kontrahent i subskrypcja przyjmują id albo nazwę. Kolejność: dokładne id → nazwa bez wielkości liter → jedyna nazwa zawierająca podany tekst. Kilka pasujących → `invalid` z listą kandydatów (do 10), np. `wallet: is ambiguous: "kar" matches "Karta", "Karta firmowa"; give the id or the full name`. Rozpoznawanie sprawdza najpierw scope narzędzia, więc bot bez uprawnień nie poznaje nazw.
- Formaty: kwoty wejściowe to string dziesiętny `"12.34"` (przecinek jest zamieniany na kropkę; najwyżej tyle miejsc, ile ma waluta), daty `YYYY-MM-DD`, miesiące budżetowe `YYYY-MM`. „Dziś” i „bieżący miesiąc” liczone są w strefie czasowej gospodarstwa (i od jego dnia startu miesiąca).
- Zmiany wykonane przez bota trafiają do audytu z aktorem `bot`; transakcje zapisane przez bota mają `source: "bot"` (zaksięgowane obciążenie subskrypcji — `source: "subscription"`).
- Listy `search_transactions` i `list_charges` są stronicowane: `limit` (1–200, domyślnie 50), `next_cursor` z wyniku podaje się jako `cursor` przy tych samych filtrach.

## Typy

```jsonc
// Money — kwota w jednostkach dziesiętnych, waluta ISO 4217 i jednostki minimalne
{ "amount": "-42.50", "currency": "PLN", "minor": -4250 }

// Wallet
{ "id": "…", "name": "Karta", "type": "card",          // cash | checking | card | savings | investment | loan
  "currency": "PLN", "on_budget": true, "visibility": "shared", "archived": false }

// Transaction — kwota ujemna = wypływ
{ "id": "…", "wallet_id": "…", "date": "2026-10-21", "amount": Money, "base_amount": Money | null,   // null: czeka na kurs
  "kind": "expense",                                   // expense | income | transfer
  "status": "cleared",                                 // pending | cleared | reconciled
  "source": "bot",                                     // manual | import | subscription | bot
  "payee_id": "…" | null, "payee_name": "Biedronka" | null, "envelope_id": "…" | null,
  "tags": ["dom"], "note": "…", "transfer_wallet_id": "…" | null,
  "splits": [ { "envelope_id": "…" | null, "amount": Money, "note": "" } ],
  "created_by": { "kind": "bot", "id": "…" } }

// Subscription — price: cena obowiązująca dziś; monthly/yearly w walucie ceny
{ "id": "…", "name": "Netflix", "vendor": "netflix", "vendor_name": "Netflix",
  "price": Money, "price_effective_from": "2026-01-15",
  "cycle": { "unit": "month", "every": 1, "anchor": "2026-01-15" },   // day | week | month | year
  "status": "active",                                  // trial | active | paused | cancelled
  "trial_ends_at": null, "ends_at": null, "wallet_id": "…" | null, "envelope_id": "…" | null,
  "reminder_offsets": [3], "match_rule": { "payee_pattern": "NETFLIX", "amount_tolerance_pct": 10, "date_window_days": 3 },
  "note": "", "manage_url": null, "next_due": "2026-11-15" | null, "monthly": Money, "yearly": Money }

// Charge — oczekiwane obciążenie subskrypcji
{ "id": "…", "subscription_id": "…", "subscription_name": "Netflix", "vendor": "netflix", "wallet_id": "…" | null,
  "due_date": "2026-11-15", "amount": Money,
  "status": "expected",                                // expected | matched | missed | skipped | posted
  "transaction_id": "…" | null }

// UpcomingRenewal — szacunek w walucie bazowej po dzisiejszym kursie (null bez kursu)
{ "charge": Charge, "base_amount": Money | null }
```

## Narzędzia

### Kontekst i finanse (odczyt)

| Narzędzie | Wejście | Wynik | Scope |
|---|---|---|---|
| `whoami` | — | `{ bot: { id, name, scopes: string[], wallet_ids: string[], expires_at }, household: { id, name, base_currency, timezone, month_start_day }, today, current_month, wallets: Wallet[] }` | — |
| `list_wallets` | `include_archived?: bool` | `{ wallets: Wallet[] }` | `wallets:read` |
| `get_balances` | `include_archived?: bool` | `{ wallets: (Wallet & { balance, cleared, base_balance: Money, base_pending: bool })[], total_base: Money, base_pending: bool }` | `wallets:read` |
| `search_transactions` | `wallet?`, `payee?`, `envelope?`, `from?`, `to?`, `text?`, `kind?`, `status?`, `tag?`, `limit?`, `cursor?` | `{ transactions: Transaction[], next_cursor: string \| null }` | `transactions:read` |
| `list_payees` | — | `{ payees: { id, name, patterns: string[], default_envelope_id }[] }` | `transactions:read` |
| `list_imports` | `wallet?`, `limit?`, `cursor?` | `{ imports: { id, wallet_id, source: "csv"\|"connection", source_ref, status: "completed"\|"reverted", rows, created, duplicates, failed, matched, reverted, created_at, reverted_at }[], next_cursor: string \| null }` | `transactions:read` |
| `spending_by` | `group_by`, `from?`, `to?`, `wallet?` | `SpendingReport` (niżej) | `reports:read` |
| `list_vendors` | — | `{ vendors: { key, name }[] }` | — |

- `whoami` podaje portfele (bez sald) tylko botowi z którymkolwiek scope'em `wallets:*` lub `transactions:*`; inaczej `wallets: []`. `scopes` to scope'y efektywne.
- `get_balances.total_base` sumuje salda w walucie bazowej wymienionych portfeli; `base_pending: true` — część transakcji czeka na kurs.
- `search_transactions`: wyniki od najnowszych; `envelope` pasuje też do linii splitu; `text` szuka w notatce i nazwie kontrahenta.
- `spending_by`:
  - `group_by`: `envelope` | `payee` | `vendor` | `wallet`; zakres domyślnie od początku miesiąca budżetowego daty `to` do dziś; najwyżej 366 dni.
  - Wydatki = transakcje `expense` (zwroty, czyli dodatnie `expense`, pomniejszają sumy); przelewy i przychody pominięte. Kwoty w walucie bazowej, dodatnie = wydane.
  - `vendor`: transakcja opłacająca obciążenie (`matched`/`posted`) widocznej subskrypcji → dostawca subskrypcji. Splity liczą się po liniach (dla `envelope`).
  - Wynik: `{ group_by, from, to, currency, total: Money, count, groups: { key: string | null, name: string | null, count, total: Money }[], incomplete, pending_transactions, truncated }`. Grupa `key: null` zbiera wydatki bez koperty / kontrahenta / subskrypcji. `incomplete` — `pending_transactions` transakcji czeka na kurs i nie jest liczonych; `truncated` — zakres ma ponad 10 000 wydatków.

### Budżet

| Narzędzie | Wejście | Wynik | Scope |
|---|---|---|---|
| `list_envelopes` | `include_hidden?: bool` | `{ currency, envelopes: { id, name, group_id, group_name, hidden, goal: { type, amount: Money, target_date } \| null }[] }` | `budget:read` |
| `budget_overview` | `month?` | `BudgetMonth` (niżej) | `budget:read` |
| `assign_to_envelope` | `envelope`, `amount`, `month?` | `{ month, ready_to_assign: Money, envelope: BudgetEnvelope }` | `budget:write` |
| `move_between_envelopes` | `from_envelope`, `to_envelope`, `amount`, `month?` | `{ month, ready_to_assign: Money, from: BudgetEnvelope, to: BudgetEnvelope }` | `budget:write` |

- `BudgetMonth`: `{ month, start, end, currency, ready_to_assign, assigned_in_future, overspent_last_month, income, uncategorized, assigned, activity, available, underfunded: Money, incomplete, pending_transactions, groups: { id, name, assigned, activity, available, envelopes: BudgetEnvelope[] }[] }`.
- `BudgetEnvelope`: `{ id, name, hidden, carried_in, assigned, activity, available: Money, overspent: bool, goal_status: { needed, underfunded: Money, funded, progress } | null }`.
- Kwoty budżetu są w walucie bazowej. `month` domyślnie bieżący miesiąc budżetowy.
- `assign_to_envelope` **ustawia** przydział koperty na miesiąc (zastępuje poprzedni, `0` usuwa); żeby dodać 50 do 200, podaj 250.
- `move_between_envelopes` przenosi dodatnią kwotę przydziału; koperta źródłowa może zejść poniżej zera. Ta sama koperta → `invalid`.

### Zapis transakcji

| Narzędzie | Wejście | Wynik | Scope |
|---|---|---|---|
| `record_expense` | `wallet`, `amount`, `currency?`, `date?`, `payee?`, `envelope?`, `note?`, `tags?`, `status?` | `{ transaction: Transaction, charge_suggestions: { charge: Charge, score }[] \| null }` | `transactions:write` |
| `record_income` | `wallet`, `amount`, `currency?`, `date?`, `payee?`, `note?`, `tags?`, `status?` | `Transaction` | `transactions:write` |
| `transfer` | `from_wallet`, `to_wallet`, `amount`, `to_amount?`, `date?`, `envelope?`, `note?`, `tags?` | `{ from: Transaction, to: Transaction }` | `transactions:write` |

- `amount` jest dodatni (`"42.50"`); znak nadaje narzędzie (wydatek ujemny). `currency` musi być walutą portfela (domyślnie nią jest). `date` domyślnie dziś w strefie gospodarstwa. `status` domyślnie `cleared`.
- `payee` to nazwa: istniejący kontrahent (po nazwie albo wzorcu) albo nowy. Bez `envelope` wydatek dostaje domyślną kopertę kontrahenta, jeśli ją ma.
- `charge_suggestions` (najlepsze najpierw, do 20) — oczekiwane obciążenia subskrypcji, które wydatek może opłacać; `null`, gdy bot nie ma `subscriptions:read` i `transactions:read`. Potwierdzenie: `confirm_charge_match`.
- `transfer`: w tej samej walucie `to_amount` = `amount`; między walutami `to_amount` jest wymagane. `envelope` tylko dla przelewu między portfelem w budżecie a poza nim.
- Portfel zarchiwizowany → `conflict`; portfel spoza listy bota → `not_found`.

### Subskrypcje

| Narzędzie | Wejście | Wynik | Scope |
|---|---|---|---|
| `list_subscriptions` | `status?`, `vendor?`, `wallet?` | `{ subscriptions: Subscription[] }` | `subscriptions:read` |
| `get_subscription` | `subscription` | `{ subscription: Subscription, prices: { effective_from, price: Money }[], open_charges: Charge[] }` | `subscriptions:read` |
| `upcoming_renewals` | `days?` (1–365, domyślnie 30) | `{ currency, renewals: UpcomingRenewal[] }` | `subscriptions:read` |
| `subscription_costs` | — | `SubscriptionSummary` (niżej) | `subscriptions:read` |
| `list_charges` | `subscription?`, `status?: string[]`, `from?`, `to?`, `limit?`, `cursor?` | `{ charges: Charge[], next_cursor: string \| null }` | `subscriptions:read` |
| `create_subscription` | `name`, `amount`, `vendor?`, `currency?`, `cycle_unit?`, `every?`, `anchor?`, `trial_ends_at?`, `wallet?`, `envelope?`, `reminder_offsets?`, `payee_pattern?`, `note?`, `manage_url?` | `Subscription` | `subscriptions:write` |
| `update_subscription` | `subscription` + dowolne z: `name`, `vendor`, `status`, `cycle_unit`, `every`, `anchor`, `trial_ends_at`, `ends_at`, `wallet`, `envelope`, `reminder_offsets`, `payee_pattern`, `note`, `manage_url` | `Subscription` | `subscriptions:write` |
| `change_subscription_price` | `subscription`, `amount`, `currency?`, `effective_from?` | `Subscription` | `subscriptions:write` |
| `confirm_charge_match` | `charge_id`, `transaction_id` | `Charge` | `subscriptions:write`, `transactions:read` |
| `post_charge` | `charge_id`, `wallet?`, `amount?`, `date?`, `payee?`, `note?` | `{ charge: Charge, transaction: Transaction }` | `subscriptions:write`, `transactions:write` |
| `skip_charge` | `charge_id` | `Charge` | `subscriptions:write` |

- Subskrypcja bez portfela jest widoczna dla całego gospodarstwa (także botów z listą portfeli); z portfelem — tylko dla widzących portfel.
- `upcoming_renewals`: tylko obciążenia `expected`, od dziś do dziś + `days`, najbliższe najpierw.
- `SubscriptionSummary`: `{ currency, monthly, yearly: Money, incomplete, missing_currencies: string[], counts: { trial, active, paused, cancelled }, by_vendor: CostGroup[], by_envelope: CostGroup[], upcoming: UpcomingRenewal[] (30 dni), trials_ending: { subscription_id, name, trial_ends_at, price }[] (14 dni), price_changes: { subscription_id, name, effective_from, old_price, new_price }[] (90 dni) }`, gdzie `CostGroup = { key, name, count, monthly, yearly }` (`key: null` — bez koperty; `name` koperty tylko przy `budget:read`). Sumy liczą subskrypcje `active` i `trial` po dzisiejszej cenie i kursie; brak kursu → `incomplete` i waluta w `missing_currencies`.
- `create_subscription`: `vendor` domyślnie `other` (klucze: `list_vendors`), `currency` domyślnie waluta bazowa, `cycle_unit` domyślnie `month`, `every` 1, `anchor` (dzień płatności, od którego liczy się cykl) domyślnie dziś; `trial_ends_at` ustawia status `trial`; `reminder_offsets` domyślnie `[3]`. Nazwa unikalna w gospodarstwie bez wielkości liter (`conflict`). Oczekiwane obciążenia powstają od dziś.
- `update_subscription` zmienia tylko podane pola. Pusty string w `trial_ends_at`, `wallet`, `envelope`, `manage_url` czyści pole. Podanie części cyklu (`cycle_unit`, `every`, `anchor`) zachowuje pozostałe. `paused`/`cancelled` usuwa przyszłe oczekiwane obciążenia; `cancelled` ustawia `ends_at` (domyślnie najbliższy termin).
- `change_subscription_price`: nowy punkt historii cen od `effective_from` (domyślnie dziś; ten sam dzień nadpisuje punkt); `currency` domyślnie waluta ostatniej ceny. Przyszłe oczekiwane obciążenia przyjmują nową kwotę.
- `confirm_charge_match`: obciążenie `expected`/`missed` + wydatek widoczny dla bota (nie przelew, nie przychód). Obciążenie w innym stanie albo transakcja opłacająca inne obciążenie → `conflict`.
- `post_charge`: tworzy wydatek (`source: subscription`) i wiąże go z obciążeniem. `wallet` domyślnie portfel subskrypcji; `amount` (dodatni, w walucie portfela) domyślnie kwota obciążenia przeliczona kursem z dnia terminu; `date` domyślnie termin.
- `skip_charge`: obciążenie `expected`/`missed` → `skipped`.

### Powiadomienia

| Narzędzie | Wejście | Wynik | Scope |
|---|---|---|---|
| `list_notifications` | `unread_only?`, `limit?`, `cursor?` | `{ notifications: Notification[], unread_count, next_cursor: string \| null }` | `notifications:read` |

- `Notification`: `{ id, type, title, body, link, url, created_at, read }`; `type` ∈ `subscription.renewal_upcoming`, `subscription.trial_ending`, `subscription.price_changed`, `subscription.charge_missed`, `envelope.overspent`, `envelope.goal_due`; `title` i `body` po polsku; `link` — ścieżka Web UI (`/h/{hid}/subscriptions/{id}`, `/h/{hid}/subscriptions/charges`, `/h/{hid}/budget/{YYYY-MM}`), `url` — pełny adres (`DENAR_BASE_URL` + `link`).
- Bot czyta powiadomienia **swojego twórcy** w swoim gospodarstwie (bot ≤ twórca). Bot z listą portfeli nie widzi powiadomień o subskrypcjach płaconych z portfeli spoza listy; powiadomienia bez portfela (subskrypcje bez portfela, koperty) widzi. `unread_count` liczy z tym samym ograniczeniem.
- Bot nie oznacza powiadomień jako przeczytanych (tylko człowiek w Web UI); stronicowanie jak w `list_charges`.

## Poza zakresem

- Usuwanie danych, operacje administracyjne (członkowie, boty, kanały, preferencje powiadomień) — tylko w Web UI.
