# Design: Subskrypcje (004-subscriptions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `002-wallets-transactions/design.md`, `003-envelope-budget/design.md`
> Last Updated: 2026-10-21

## 1. Architektura (nowe elementy)

```
internal/domain/subscription/     vendor.go (słownik), cycle.go (Cycle, NextDue, PerYear), subscription.go (Subscription,
                                  PricePoint, Status, MatchRule, Plan, PriceOn), charge.go (Charge, ChargeStatus),
                                  cost.go (Cost — normalizacja), match.go (MatchScore)
internal/domain/event/            event.go — Event (outbox) i typy zdarzeń subskrypcji
internal/usecase/                 subscriptions.go (CRUD, ceny, słownik), subscription_access.go (widoczność),
                                  charges.go (lista, sugestie, potwierdzenie, odłączenie, zaksięgowanie, pominięcie,
                                  auto-dopasowanie), charge_sync.go (plan → obciążenia), subscription_summary.go,
                                  subscription_jobs.go (generowanie, missed, przypomnienia), events.go (emit)
internal/usecase/{fakes,portstest} SubscriptionRepository, ChargeRepository, EventOutbox
internal/adapter/sqlite/          00004_subscriptions.sql, subscription.go, charge.go, event.go
internal/adapter/rest/            handlers_subscriptions.go, mapping_subscriptions.go
internal/platform/config/         DENAR_CHARGE_HORIZON, DENAR_SUBSCRIPTION_INTERVAL
cmd/denar/                        wiring, job subskrypcji, e2e_subscriptions_test.go
```

Zmiany w kodzie 002/003 (integracja, minimalne):
- `DeleteEnvelope`: koperta używana przez subskrypcję → 409 `envelope_in_use` (gdy `Deps.Subscriptions != nil`).
- `DeleteTransaction`: przed usunięciem transakcji powiązanej z obciążeniem obciążenie wraca do `expected`/`missed` (audyt `charge.unmatched`, `via = transaction_deleted`).
- `fakes.Repos` i `HouseholdRepository.Cascade` dostają nowe repozytoria; `portstest` kaskada gospodarstwa sprawdza subskrypcje, obciążenia i zdarzenia.
- `domain.go`: nowe powody `ConflictChargeState` (`charge_state`), `ConflictTransactionLinked` (`transaction_linked`).

## 2. Domena (`internal/domain/subscription`)

### Słownik dostawców
`Vendor{Key, Name}`; `Vendors` (stała kolejność): openai OpenAI, anthropic Anthropic, apple Apple, google Google, netflix Netflix, spotify Spotify, youtube YouTube, disney Disney+, hbo HBO Max, amazon Amazon, microsoft Microsoft, adobe Adobe, github GitHub, icloud iCloud, dropbox Dropbox, other Inne. `LookupVendor(key)`. Konektory przyszłych etapów identyfikują subskrypcję po `(vendor, external_ref)`.

### Cycle
`Cycle{Unit day|week|month|year, Every 1–365, Anchor domain.Date}`.
- `Occurrence(k)`: dzień/tydzień → `Anchor + k·Every·(1|7)` dni; miesiąc/rok → `addMonthsClamped(Anchor, k·Every·(1|12))`: rok i miesiąc liczone od `Anchor`, dzień = `min(Anchor.day, ostatni dzień miesiąca)`. Liczone zawsze od `Anchor`, więc 31.01 → 28.02 → 31.03 (bez dryfu), 29.02.2024 co rok → 28.02.2025 → 29.02.2028.
- `OnOrAfter(d)`: `d ≤ Anchor` → `Anchor`; inaczej szacunek `k` (dni / miesiące między datami podzielone przez krok) i korekta ±1 do najmniejszego `k` z `Occurrence(k) ≥ d`.
- `NextDue(after) = OnOrAfter(after + 1)`; `Between(from, to)` — terminy w `[from, to]` (≤ 1000).
- `PerYear() *big.Rat`: dzień `36525/(100·Every)`, tydzień `36525/(700·Every)`, miesiąc `12/Every`, rok `1/Every`.

### Subscription, PricePoint, MatchRule
```go
Subscription{ID, HouseholdID, Name, Vendor, ExternalRef, Cycle, TrialEndsAt domain.Date, Status, CancelledAt time.Time,
    EndsAt domain.Date, WalletID, EnvelopeID, ReminderOffsets []int, Match MatchRule, Note, ManageURL,
    Prices []PricePoint /* rosnąco po EffectiveFrom, ≥ 1 */, CreatedBy domain.Actor, CreatedAt, UpdatedAt}
PricePoint{ID, EffectiveFrom domain.Date, Amount int64 /* > 0, jednostki minimalne */, Currency, CreatedAt}
MatchRule{PayeePattern string, AmountTolerancePct int /* 0–50, domyślnie 10 */, DateWindowDays int /* 0–15, domyślnie 3 */}
```
- Status `trial` wymaga `TrialEndsAt`; `cancelled` ma `CancelledAt` i `EndsAt` (domyślnie `NextDue(dziś)` — dostęp do końca opłaconego okresu); inne statusy czyszczą `CancelledAt/EndsAt`.
- `Normalize` (trim, offsety unikalne malejąco, domyślne MatchRule), `Validate` (FR-300; URL `http(s)://`).
- `PriceOn(d)` — ostatni punkt z `EffectiveFrom ≤ d`, przed pierwszym — pierwszy.
- `Plan(from, to) []Planned{DueDate, Amount, Currency}` — terminy cyklu w `[from, to]` po filtrze statusu (FR-304) z ceną na dzień.
- `ReminderDue(due, today) (offset int, ok bool)` — najmniejszy offset z `due − offset ≤ today ≤ due` (FR-315).

### Charge
`Charge{ID, HouseholdID, SubscriptionID, DueDate, Amount int64, Currency, Status, TransactionID, CreatedAt, UpdatedAt}`; `Status expected|matched|missed|skipped|posted`; `Open()` = expected ∨ missed; `Linked()` = matched ∨ posted (⇔ `TransactionID ≠ ""`). `MissedOn(window)` = `DueDate + window + 1` (dzień, od którego obciążenie bez dopasowania jest przegapione).

### Cost (normalizacja, FR-311)
`CostOf(price PricePoint, cycle Cycle, rate *big.Rat /* 1 waluta ceny = rate waluty docelowej */, to money.Currency) (Cost{Monthly, Yearly int64}, error)`: `yearly = Amount · PerYear · rate · 10^(to.dec − from.dec)`, `monthly = yearly / 12`, każde zaokrąglone raz half-even (`money.RoundHalfEven`). Bez floatów.

### MatchScore (FR-307)
```go
Candidate{WalletID, Date, Amount int64 /* koszt > 0 w walucie transakcji */, Expected int64 /* kwota obciążenia w walucie
    transakcji, 0 = nieznana (brak kursu) */, Text string /* nazwa kontrahenta + notatka */}
MatchScore(s Subscription, c Charge, x Candidate) (score int, ok bool)
```
| Składnik | Punkty | Reguła |
|---|---|---|
| kwota | 40 − ⌊20·diff/tol⌋ | `diff = |Amount − Expected| / Expected` (w punktach bazowych, `big.Int`); `diff > tol` → nie kandydat; `tol = 0` → tylko dokładna (40); `Expected = 0` → 0 pkt, kandydat |
| data | 30 − ⌊15·d/window⌋ | `d = |Date − DueDate|` dni; `d > window` → nie kandydat; `window = 0` → ten sam dzień (30) |
| kontrahent | 20 | wzorzec (`PayeePattern`, a bez niego nazwa dostawcy ≠ other i nazwa subskrypcji) jest podciągiem `Text` bez wielkości liter |
| portfel | 10 | `x.WalletID == s.WalletID` (subskrypcja z portfelem) |

Próg auto-dopasowania `AutoMatchThreshold = 80`.

## 3. Zdarzenia (`internal/domain/event`) — kontrakt dla etapu 005

```go
type Type string
const (
    SubscriptionRenewalUpcoming Type = "subscription.renewal_upcoming"
    SubscriptionTrialEnding     Type = "subscription.trial_ending"
    SubscriptionPriceChanged    Type = "subscription.price_changed"
    SubscriptionChargeMissed    Type = "subscription.charge_missed"
)
type Event struct {
    ID, HouseholdID domain.ID
    Type            Type
    ResourceType    string    // "subscription"
    ResourceID      domain.ID // id subskrypcji
    DedupeKey       string    // unikalny w gospodarstwie
    Payload         map[string]string
    OccurredAt      time.Time
    DispatchedAt    time.Time // zero = czeka na dispatch
}
```
Payload (wszystkie wartości to stringi; kwoty dziesiętne w walucie z pola obok; daty `YYYY-MM-DD`):

| Typ | DedupeKey | Payload (poza wspólnymi `subscription_id, subscription_name, vendor, vendor_name, wallet_id, envelope_id`) |
|---|---|---|
| `renewal_upcoming` | `renewal:<charge_id>:<offset>` | `charge_id, due_date, amount, currency, days_before` |
| `trial_ending` | `trial:<sub_id>:<trial_ends_at>:<offset>` | `trial_ends_at, days_before, amount, currency` (cena po trialu) |
| `price_changed` | `price:<price_point_id>` | `price_id, effective_from, old_amount, old_currency` (puste przy braku), `new_amount, new_currency` |
| `charge_missed` | `missed:<charge_id>` | `charge_id, due_date, amount, currency` |

`wallet_id` pusty = subskrypcja bez portfela (widoczna dla całego gospodarstwa); etap 005 wybiera odbiorców wśród członków widzących portfel (`wallet.VisibleTo`).

Port (zapis w tej samej transakcji co zmiana):
```go
type EventOutbox interface {
    // Append stores the event unless the household has one with the same DedupeKey; reports whether it stored it.
    Append(ctx, event.Event) (bool, error)
    // ListPending returns up to limit events not dispatched yet, oldest first (by id).
    ListPending(ctx, limit int) ([]event.Event, error)
    // MarkDispatched sets DispatchedAt; ErrNotFound for an unknown id.
    MarkDispatched(ctx, id domain.ID, at time.Time) error
}
```
Tabela `domain_events` z unikalnym `(household_id, dedupe_key)` jest jednocześnie rejestrem wysłanych przypomnień (AD-406).

## 4. Porty (`usecase/ports.go`)

```go
type SubscriptionRepository interface {
    Create(ctx, subscription.Subscription) error           // z cenami; duplicate_name; obcy portfel/koperta → ErrNotFound
    Update(ctx, subscription.Subscription) error           // pola zmienne bez cen, twórcy i created_at
    PutPrice(ctx, householdID, subscriptionID domain.ID, subscription.PricePoint) error // upsert po effective_from
    Delete(ctx, householdID, id domain.ID) error           // z cenami i obciążeniami
    ByID(ctx, householdID, id domain.ID) (subscription.Subscription, error)
    // List: subskrypcje gospodarstwa ("" = wszystkich gospodarstw, dla jobów) wg nazwy, id
    List(ctx, householdID domain.ID) ([]subscription.Subscription, error)
    CountByEnvelope(ctx, householdID, envelopeID domain.ID) (int, error)
}
type ChargeFilter struct { HouseholdID; SubscriptionIDs []domain.ID /* wymagane, puste = nic */;
    Statuses []subscription.ChargeStatus; From, To domain.Date /* włącznie */ }
type ChargeRepository interface {
    Create(ctx, subscription.Charge) error                  // ConflictError duplicate_charge dla (subskrypcja, termin)
    Update(ctx, subscription.Charge) error                  // amount, currency, status, transaction_id, updated_at
    Delete(ctx, householdID, id domain.ID) error
    ByID(ctx, householdID, id domain.ID) (subscription.Charge, error)
    ByTransaction(ctx, householdID, transactionID domain.ID) (subscription.Charge, error)
    List(ctx, ChargeFilter, Page) ([]subscription.Charge, string, error) // termin rosnąco, id
    ListBySubscription(ctx, householdID, subscriptionID domain.ID, from domain.Date) ([]subscription.Charge, error)
    // ListOpen: obciążenia expected|missed (householdID "" = wszystkie) z terminem w [from, to], termin rosnąco, ≤ limit
    ListOpen(ctx, householdID domain.ID, from, to domain.Date, limit int) ([]subscription.Charge, error)
}
```
`Deps` dostaje `Subscriptions`, `Charges`, `Events`, `ChargeHorizonDays` (0 → 90). Gdy `Subscriptions == nil`, integracje 002/003 są wyłączone.

## 5. Use case'y i autoryzacja

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `ListVendors` | zalogowany | słownik |
| `ListSubscriptions`, `GetSubscription`, `ListSubscriptionPrices` | `subscriptions:read` | filtry `status`, `vendor`, `wallet_id`; widok z ceną bieżącą, `next_due`, kosztem mies./rocz. w walucie ceny |
| `CreateSubscription` | `subscriptions:write` | status domyślny `trial` gdy jest `trial_ends_at`, inaczej `active`; pierwszy punkt ceny od `anchor`; obciążenia od (dziś − okno) |
| `UpdateSubscription` | `subscriptions:write` | częściowa zmiana; regeneracja (FR-305); `cancelled` → `CancelledAt = now`, `EndsAt` z wejścia lub `NextDue(dziś)` |
| `DeleteSubscription` | `subscriptions:write` | z cenami i obciążeniami; transakcje zostają |
| `ChangeSubscriptionPrice` | `subscriptions:write` | punkt ceny, regeneracja od min(D, dziś), zdarzenie `price_changed` |
| `ListCharges`, `GetCharge` | `subscriptions:read` | filtry `subscription_id`, `status` (lista), `from`, `to`; kursor |
| `SuggestTransactions` (dla obciążenia) | `subscriptions:read` + `transactions:read` | transakcje z widocznych portfeli w `[due − okno, due + okno]`, top `limit` (domyślnie 5, ≤ 20) |
| `SuggestCharges` (dla transakcji) | `subscriptions:read` + `transactions:read` | obciążenia otwarte widocznych subskrypcji z terminem ±15 dni |
| `ConfirmChargeMatch` | `subscriptions:write` + `transactions:read` | obciążenie otwarte, transakcja widoczna, nie przelew, niepowiązana (409 `transaction_linked`) |
| `UnmatchCharge` | `subscriptions:write` | `matched|posted` → `expected` lub `missed` (po oknie); 409 `charge_state` dla innych |
| `PostCharge` | `subscriptions:write` + `transactions:write` | portfel z wejścia lub subskrypcji (wymagany, zapisywalny); kwota w walucie portfela jawnie lub przeliczona kursem z dnia terminu (brak → 422 `amount`); data = termin; kontrahent z wejścia lub nazwa dostawcy/subskrypcji; koperta subskrypcji, jeśli istnieje; `source = subscription` |
| `SkipCharge` | `subscriptions:write` | otwarte → `skipped` |
| `AutoMatchTransactions` | `subscriptions:write` (+ widoczność transakcji) | dla importu (008); FR-310 |
| `GetSubscriptionSummary` | `subscriptions:read` | FR-312 |
| `ListUpcomingRenewals` | `subscriptions:read` | `days` 1–365, domyślnie 7 |
| `GenerateCharges`, `MarkMissedCharges`, `SendSubscriptionReminders`, `RunSubscriptionJobs` | system (job) | aktor `system/subscriptions` |

Widoczność (`subscription_access.go`): `visibleSubscription(s, p, sub)` = `sub.WalletID == ""` ∨ portfel widoczny dla osoby (bot: twórcy) i dozwolony przez listę bota; ukryta → `ErrNotFound`. Zapis z `wallet_id` wymaga portfela widocznego (404) i niezarchiwizowanego (409 `wallet_archived`). Bot ograniczony do portfeli widzi subskrypcje bez portfela (AD-403). Obciążenie jest widoczne, gdy widoczna jest jego subskrypcja. Scope'y sprawdzane przed widocznością (403 przed 404, jak w 002).

Regeneracja (`charge_sync.go`, `syncCharges(sub, from, createFrom)`): plan = `sub.Plan(from, today + horizon)`; obciążenia `expected` subskrypcji z terminem ≥ from: spoza planu → usunięte, z inną kwotą/walutą → poprawione; terminy planu bez obciążenia i ≥ createFrom → utworzone (`Create` z `duplicate_charge` ignorowanym). Obciążenia nie-`expected` nie są ruszane, więc termin `missed/skipped/matched/posted` nie odradza się.

Audyt (`resource_type`): `subscription` (`subscription.created|updated|deleted|price_changed`), `charge` (`charge.matched|unmatched|posted|skipped|missed`, zmiany `status` from→to, `transaction_id`, `score`, `via` = `manual|auto|transaction_deleted`). Wygenerowane i przeliczone obciążenia `expected` nie są audytowane pojedynczo (AD-408).

## 6. Joby

`RunSubscriptionJobs.Do(ctx)` co `DENAR_SUBSCRIPTION_INTERVAL` (domyślnie `1h`, pierwszy przebieg przy starcie), dla każdego gospodarstwa w jego strefie czasowej, każde w osobnej transakcji bazy na subskrypcję:
1. `GenerateCharges`: `syncCharges(sub, today, today)` dla każdej subskrypcji.
2. `MarkMissedCharges`: `ListOpen(status expected, due < today)` → gdy `today ≥ MissedOn(window)` → `missed` + audyt + zdarzenie.
3. `SendSubscriptionReminders`: obciążenia `expected` subskrypcji `active` z terminem w `[today, today + 60]` → `ReminderDue` → `Append(renewal_upcoming)`; subskrypcje `trial` z `TrialEndsAt ≥ today` → ta sama reguła dla `TrialEndsAt` → `Append(trial_ending)`. Dedupe — klucze z §3.
Wynik: liczby utworzonych obciążeń, przegapionych i zdarzeń (log).

## 7. Schemat SQL (`00004_subscriptions.sql`)

- `subscriptions(id PK, household_id → households CASCADE, name, vendor, external_ref?, cycle_unit CHECK, cycle_every CHECK 1–365, cycle_anchor TEXT, trial_ends_at?, status CHECK, cancelled_at?, ends_at?, wallet_id? → wallets, envelope_id? → envelopes, reminder_offsets JSON, payee_pattern, amount_tolerance_pct, date_window_days, note, manage_url, created_by_kind, created_by_id, created_at, updated_at)` + unikalny `(household_id, name COLLATE NOCASE)`, indeksy `(envelope_id)`, `(wallet_id)`.
- `subscription_prices(id PK, subscription_id → subscriptions CASCADE, effective_from TEXT, amount INTEGER > 0, currency, created_at, UNIQUE(subscription_id, effective_from))`.
- `subscription_charges(id PK, household_id → households CASCADE, subscription_id → subscriptions CASCADE, due_date TEXT, amount INTEGER, currency, status CHECK, transaction_id? → transactions, created_at, updated_at, CHECK((status IN ('matched','posted')) = (transaction_id IS NOT NULL)), UNIQUE(subscription_id, due_date))` + unikalny częściowy `(transaction_id) WHERE transaction_id IS NOT NULL`, indeksy `(household_id, due_date)`, `(status, due_date)`.
- `domain_events(id PK, household_id → households CASCADE, type, resource_type, resource_id, dedupe_key, payload JSON, occurred_at, dispatched_at?, UNIQUE(household_id, dedupe_key))` + częściowy indeks `(id) WHERE dispatched_at IS NULL`.

FK do `wallets`, `envelopes` i `transactions` bez akcji (NO ACTION — sprawdzane na końcu polecenia, więc kaskada gospodarstwa przechodzi): usunięcie koperty blokuje use case (409), transakcji — use case najpierw odłącza obciążenie.

## 8. REST (`api/openapi.yaml`)

| Operacja | Ścieżka |
|---|---|
| `listVendors` | `GET /vendors` |
| `listSubscriptions`, `createSubscription` | `GET|POST /households/{householdId}/subscriptions` (`status, vendor, wallet_id`) |
| `getSubscriptionSummary` | `GET /households/{householdId}/subscriptions/summary` |
| `listUpcomingRenewals` | `GET /households/{householdId}/subscriptions/upcoming?days=` |
| `getSubscription`, `updateSubscription`, `deleteSubscription` | `GET|PATCH|DELETE /households/{householdId}/subscriptions/{subscriptionId}` |
| `listSubscriptionPrices`, `changeSubscriptionPrice` | `GET|POST /households/{householdId}/subscriptions/{subscriptionId}/prices` |
| `listCharges` | `GET /households/{householdId}/charges` (`subscription_id, status (wiele), from, to, cursor, limit`) |
| `getCharge` | `GET /households/{householdId}/charges/{chargeId}` |
| `suggestChargeMatches` | `GET /households/{householdId}/charges/{chargeId}/suggestions?limit=` |
| `matchCharge`, `unmatchCharge`, `postCharge`, `skipCharge` | `POST /households/{householdId}/charges/{chargeId}/match|unmatch|post|skip` |
| `suggestChargesForTransaction` | `GET /households/{householdId}/transactions/{transactionId}/charge-suggestions` |

Schematy: `Subscription{id, name, vendor, vendor_name, external_ref, price (Money bieżąca), price_effective_from, cycle{unit, every, anchor}, trial_ends_at, status, cancelled_at, ends_at, wallet_id, envelope_id, reminder_offsets, match_rule{payee_pattern, amount_tolerance_pct, date_window_days}, note, manage_url, next_due, monthly, yearly (Money w walucie ceny), created_by, created_at, updated_at}`; `SubscriptionCreate` (`price: MoneyInput` w dowolnej walucie); `SubscriptionUpdate` (pola opcjonalne, `null` czyści `wallet_id/envelope_id/trial_ends_at/ends_at/external_ref/manage_url`); `PricePoint{id, effective_from, price, created_at}`; `PriceChange{effective_from, price}`; `Charge{id, subscription_id, subscription_name, vendor, due_date, amount, status, transaction_id, created_at, updated_at}`; `ChargePost{wallet_id?, amount?, payee_name?, note?}` → 201 `{charge, transaction}`; `ChargeSuggestions{items[{score, transaction}]}`; `TransactionChargeSuggestions{items[{score, charge}]}`; `SubscriptionSummary`, `UpcomingRenewalList`, `VendorList`.

Nowe powody 409: `charge-state`, `transaction-linked`, `envelope-in-use` (także dla subskrypcji).

## 9. Przypadki brzegowe

- Anchor 31 co miesiąc: 31.01, 28.02 (29.02 w roku przestępnym), 31.03, 30.04…; co 2 miesiące od 31.08: 31.10, 31.12, 28.02…
- Anchor w przeszłości: `next_due` liczony od dziś; obciążenia sprzed (dzień utworzenia − okno) nie powstają.
- Trial z `trial_ends_at` przed `anchor`: pierwsze obciążenie w `anchor`; po `anchor`: pierwszy termin ≥ `trial_ends_at`.
- Zmiana waluty ceny (np. USD → PLN) dozwolona; przyszłe obciążenia w nowej walucie.
- Subskrypcja USD płacona z portfela PLN: sugestie przeliczają kwotę obciążenia na walutę transakcji kursem z dnia terminu (różnice kursowe banku mieszczą się w tolerancji); brak kursu → składnik kwoty 0 pkt, kandydat nadal (bez auto-dopasowania).
- Ręczne potwierdzenie dowolnej niepowiązanej transakcji (nie przelewu) — także spoza tolerancji (użytkownik wie lepiej).
- Pauza usuwa przyszłe oczekiwane obciążenia; wznowienie tworzy je od dziś (pominięte w czasie pauzy nie wracają).
- Anulowanie: obciążenia ≥ `ends_at` znikają; przypomnień o odnowieniu brak (tylko `active`).
- Usunięcie portfela płatności niemożliwe (002 archiwizuje); zarchiwizowany portfel → `PostCharge` 409 `wallet_archived`.

## 10. Strategia testów

- Domena: tabelaryczne `Cycle` (31 → krótkie miesiące, 29.02, co N dni/tygodni/miesięcy/lat, `OnOrAfter` przed/w/po anchor, brak dryfu przez 48 terminów), `PriceOn`, `Plan` (statusy, trial, cancelled), `ReminderDue`, `CostOf` (USD→PLN, JPY, co 3 miesiące, tydzień), `MatchScore` (granice tolerancji i okna, wzorce, portfel, brak kursu), walidacja.
- Use case'y na fake'ach: CRUD z audytem i regeneracją, zmiana ceny (zdarzenie, przyszłe obciążenia), joby (idempotencja, missed po oknie, przypomnienia raz, najmniejszy offset, trial), sugestie w obie strony z przeliczeniem waluty, potwierdzenie/odłączenie/zaksięgowanie/pominięcie, auto-dopasowanie (jednoznaczny / niejednoznaczny / pod progiem), podsumowanie (PLN/USD, brak kursu → incomplete), upcoming, integracje (`DeleteEnvelope`, `DeleteTransaction`); **macierz autoryzacji** `subscription_authz_test.go`: operacje × rola × człowiek/bot × scope × lista portfeli × subskrypcja (portfel wspólny / prywatny cudzy / bez portfela).
- `portstest`: suity subscription / charge / event outbox + kaskada gospodarstwa — fake'i i SQLite.
- REST: konformancja każdej nowej operacji × każda zadeklarowana odpowiedź.
- E2E `cmd/denar` (zegar ustawiony): OpenAI USD miesięcznie z anchor 31 → job → obciążenia (31.10, 30.11, 31.12) → transakcja PLN → sugestia → potwierdzenie → zmiana ceny → summary w PLN (fake NBP) → trial kończący się (zdarzenie) → job missed.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-401 | Waluta ceny subskrypcji niezależna od portfela; przy księgowaniu kwota w walucie portfela podana jawnie albo przeliczona kursem z dnia terminu (`fx.quote`, krzyżowo przez walutę bazową) | Rekomendacja orkiestratora; OpenAI w USD płacone kartą PLN |
| AD-402 | Auto-dopasowanie wyłącznie jako use case `AutoMatchTransactions` dla importu (008); transakcje manual/bot dostają sugestie przez `GET /transactions/{id}/charge-suggestions` | Rekomendacja orkiestratora; ręczny wpis nie powinien cicho wiązać obciążeń |
| AD-403 | Subskrypcja bez portfela widoczna dla całego gospodarstwa (także botów z listą portfeli); z portfelem — tylko dla widzących portfel | Rekomendacja orkiestratora; brak powiązania z danymi portfela |
| AD-404 | Cena wyłącznie przez historię punktów (`PricePoint`); pierwszy punkt od `anchor`; cena przed pierwszym punktem = pierwszy punkt; ten sam dzień nadpisuje punkt | Jedno źródło prawdy; poprawka literówki bez śmieciowych punktów |
| AD-405 | Wagi punktacji 40/30/20/10, liniowy spadek do połowy składnika na granicy tolerancji/okna, próg auto 80 | Dokładna kwota + data + portfel = 80; typowy import (przewalutowanie, +1 dzień, kontrahent) ≈ 90 |
| AD-406 | Outbox `domain_events` z unikalnym `(household_id, dedupe_key)` pełni rolę tabeli wysłanych przypomnień; etap 005 nie usuwa zdarzeń młodszych niż 400 dni | Jedna tabela zamiast dwóch; idempotencja w jednym miejscu |
| AD-407 | Przypomnienie tylko dla najmniejszego offsetu, którego dzień już nadszedł (zaległe większe offsety nie są wysyłane po fakcie) | Po przestoju serwera użytkownik nie dostaje serii starych przypomnień |
| AD-408 | Obciążenia `expected` generowane/przeliczane przez job nie są audytowane pojedynczo; audytowane są zmiany subskrypcji i każda zmiana statusu obciążenia (także `missed` przez system) | Dane pochodne audytowanej subskrypcji; bez zalewu audytu |
| AD-409 | Obciążenia tworzone tylko w przód (od dziś; przy utworzeniu od dnia utworzenia − okno); regeneracja nie odtwarza terminów pauzy ani przeszłości | Brak fałszywych „missed” dla historii sprzed ewidencji |
| AD-410 | Normalizacja: rok = 365,25 dnia; tydzień = 365,25/7 terminów rocznie; kurs dzisiejszy (z oknem NBP 7 dni) | Średni rok kalendarzowy; deterministyczne, bez floatów |
| AD-411 | Statusy zmieniane swobodnie przez `PATCH` (trial↔active↔paused↔cancelled); `cancelled` ustawia `cancelled_at = now` i `ends_at` (domyślnie najbliższy termin po dziś) | Proste i wystarczające; dostęp do końca opłaconego okresu |
| AD-412 | Ręczne potwierdzenie dopasowania nie wymaga punktacji (dowolna niepowiązana transakcja niebędąca przelewem); jedna transakcja ↔ jedno obciążenie | Użytkownik rozstrzyga przypadki brzegowe (podwyżka, inna data) |
| AD-413 | `PostCharge` wymaga `subscriptions:write` i `transactions:write`; sugestie i potwierdzenie — także `transactions:read` | Bot nie widzi/nie tworzy transakcji spoza swoich scope'ów |
| AD-414 | Usunięcie transakcji powiązanej z obciążeniem przywraca obciążenie (`expected` albo `missed` po oknie) zamiast blokować usunięcie | Transakcje są źródłem prawdy; obciążenie to oczekiwanie |
| AD-415 | Horyzont `DENAR_CHARGE_HORIZON` w dniach (7–730, domyślnie 90); interwał jobów `DENAR_SUBSCRIPTION_INTERVAL` (domyślnie 1h) | Prosty format; przypomnienia o odnowieniu opierają się na obciążeniach, więc offset większy niż horyzont nie zadziała (domyślne 90 > maks. offset 60) |
| AD-416 | Nazwa subskrypcji unikalna w gospodarstwie (bez wielkości liter) | Jednoznaczne odwołania po nazwie z MCP (006) |
| AD-417 | `GET /vendors` poza gospodarstwem, dla każdego zalogowanego principala (także bota bez scope'ów) | Statyczny słownik, nie dane gospodarstwa |

## Ograniczenia

- Brak dispatchu zdarzeń (005), narzędzi MCP (006) i importu (008); `AutoMatchTransactions` gotowy, ale nie wywoływany przez żaden adapter.
- Przeliczenia kursem przez NBP (tabela A) — waluty spoza tabeli tylko przez ręczny kurs gospodarstwa.
- Podsumowanie i joby liczą na żądanie (bez materializacji) — wystarczy dla gospodarstwa domowego.
- Job przetwarza wszystkie subskrypcje instancji w jednym przebiegu (bez stronicowania) — do tysięcy subskrypcji.
- Job `missed` patrzy 90 dni wstecz (≤ 1000 otwartych obciążeń na przebieg); obciążenie pozostawione `expected` dłużej (np. serwer wyłączony) już nie przejdzie w `missed` samo.

## Dla etapu 005 (powiadomienia)

- Konsumuj `EventOutbox.ListPending(limit)` → wyślij → `MarkDispatched(id, at)`; zdarzenia mają typy i payload z §3; `wallet_id` w payload wyznacza odbiorców (członkowie widzący portfel; pusty — wszyscy członkowie gospodarstwa). Nie usuwaj zdarzeń młodszych niż 400 dni (deduplikacja przypomnień).
- Joby subskrypcji (`RunSubscriptionJobs`) już tworzą zdarzenia; 005 dodaje tylko dispatch, preferencje i kanały. Nowe typy (`envelope.overspent`, `envelope.goal_due`) dopisz w `internal/domain/event`.

## Dla etapu 006 (MCP)

Use case'y: `ListSubscriptions`, `GetSubscription`, `CreateSubscription`, `UpdateSubscription`, `ChangeSubscriptionPrice`, `ListUpcomingRenewals` (`upcoming_renewals(days)`), `GetSubscriptionSummary` (`subscription_costs`), `ListCharges`, `SuggestTransactions`/`SuggestCharges`, `ConfirmChargeMatch` (`confirm_charge_match`), `PostCharge`, `SkipCharge`, `ListVendors`.
