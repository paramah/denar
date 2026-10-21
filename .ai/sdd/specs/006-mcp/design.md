# Design: Serwer MCP (006-mcp)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `001-core/design.md`, `004-subscriptions/design.md`, Straylight `internal/adapter/mcp` i `api/mcp-tools.md`
> Last Updated: 2026-10-21

## 1. Architektura

```
api/mcp-tools.md                  kontrakt narzędzi (po polsku)
internal/adapter/mcp/             doc.go (ustalenia probe), server.go (Deps, NewHandler, addTool, invalidArguments),
                                  errors.go (toolError), auth.go (Bearer, limity, clientAddr), middleware.go (log, recover,
                                  POST only, single message, nosniff), types.go (Money, widoki), tools_*.go (narzędzia),
                                  testy: sdk_probe_test, errors_test, auth_test, tools_*_test, contract_test, world_test
internal/usecase/                 resolve.go (Resolve: wallet/envelope/payee/subscription po id lub nazwie),
                                  whoami.go (Whoami), spending_report.go (SpendingReport) + testy i macierz autoryzacji
cmd/denar/                        app.go: mcp.NewHandler pod /mcp; e2e_mcp_test.go (klient go-sdk przez HTTP)
README.md                         sekcja „MCP”
```

Zależności: `github.com/modelcontextprotocol/go-sdk v1.8.0` (jak w Straylight). Adapter importuje `usecase`, `domain` i SDK; nie importuje innych adapterów (`arch_test`).

Zmiany w kodzie wcześniejszych etapów (minimalne, opisane w AD):
- `domain.ForbiddenError` dostaje pole `Scope` (nazwa brakującego scope'u); `requireScope` je wypełnia. REST bez zmian (`problemOf` patrzy tylko na `Reason`).
- `GetBudgetMonth`, `SetAssignment`, `MoveBetweenEnvelopes`: pusty miesiąc = bieżący miesiąc budżetowy gospodarstwa (strefa i dzień startu). REST zawsze podaje miesiąc, więc nic się tam nie zmienia.
- `cmd/denar/app.go`: `/mcp` przestaje być 404.

## 2. Transport i uwierzytelnianie

Przejęte ze Straylight (ustalenia go-sdk v1.8.0 potwierdza `sdk_probe_test.go`, streszcza `doc.go`):

```
recoverPanics → logRequests → noSniff → postOnly → CrossOriginProtection → authenticate → limitBot → singleMessage
  → auth.RequireBearerToken(verifiedPrincipal, AllowMissingExpiration) → NewStreamableHTTPHandler(Stateless, JSONResponse,
    MaxRequestBodyBytes 1 MiB, DisableLocalhostProtection)
```

- `authenticate`: token wyłącznie z `Authorization: Bearer`; `usecase.AuthenticateBot` (ten sam co REST). Adres z ≥ 20 odrzuconymi tokenami w minucie → 429 + `Retry-After` (bez sprawdzania tokenu); każda porażka → jednakowe 401 `{"error":"unauthorized"}` z `WWW-Authenticate: Bearer`. Adres klienta jak w REST (`X-Forwarded-For` tylko od `DENAR_TRUSTED_PROXIES`, IPv6 jako /64).
- `limitBot`: okno przesuwne 1 min na bota, limit `DENAR_BOT_RATE` (domyślnie 120) → 429 + `Retry-After`.
- Principal trafia do narzędzi przez `TokenInfo.Extra` (punkt 2 probe).
- Bot wyłączony lub z wymienionym tokenem w trakcie wywołania → `ErrUnauthenticated` z use case'u → błąd JSON-RPC `-32001 unauthorized`.

## 3. Narzędzia — wspólne

```go
func addTool[In any](s *server, m *sdk.Server, name, description string, fn func(ctx, p usecase.Principal, in In) (any, error))
```
- Typ wyjścia `any` (bez output schema), wynik = `structuredContent` + ten sam JSON jako tekst; błąd = `{code, message}` (punkt 4 probe).
- Timeout 20 s na wywołanie, `recover` z logiem, nieznany błąd → `-32603 internal error`.
- `invalidArguments` (middleware odbiorczy): błędy schematu SDK → `invalid`.
- Gospodarstwo = `p.Bot.HouseholdID`.

### Mapowanie błędów (`errors.go`)

| Błąd use case'u | Kod | Komunikat |
|---|---|---|
| `ErrNotFound` (w tym `RefNotFound{Kind}`) | `not_found` | `not found`, przy odwołaniu: `no wallet you can see has this id or name` |
| `ForbiddenError{Reason: scope, Scope}` | `forbidden` | `the bot lacks the scope transactions:write (or its creator's role does not allow it)` |
| `ForbiddenError{Reason: wallet}` | `forbidden` | `the operation reaches a wallet the bot cannot see` |
| inne `ErrForbidden` | `forbidden` | `not allowed` |
| `ValidationError`(y) | `invalid` | `pole: komunikat; …` |
| `ConflictError` | `conflict` | komunikat wg powodu (`wallet_archived`, `charge_state`, `transaction_linked`, `duplicate_name`, …) |

### Typy wyjścia (`types.go`)

`Money{amount: "12.34", currency: "PLN", minor: 1234}`; `null` zamiast pominięcia pola. Widoki: `Wallet`, `WalletBalance`, `Transaction`, `Payee`, `Envelope`, `BudgetMonth`, `Subscription`, `PricePoint`, `Charge`, `UpcomingRenewal`, `SubscriptionSummary`, `SpendingReport` — pola jak w REST (snake_case), szczegóły w `api/mcp-tools.md`.

## 4. Rozpoznawanie nazw (use case `Resolve`)

```go
type Resolve struct{ d Deps }
func (uc *Resolve) Wallet(ctx, p, householdID, ref string, scopes ...bot.Scope) (wallet.Wallet, error)
func (uc *Resolve) Envelope(ctx, p, householdID, ref string, scopes ...bot.Scope) (budget.Envelope, error)
func (uc *Resolve) Payee(ctx, p, householdID, ref string, scopes ...bot.Scope) (payee.Payee, error)
func (uc *Resolve) Subscription(ctx, p, householdID, ref string, scopes ...bot.Scope) (subscription.Subscription, error)
type RefNotFound struct{ Kind string } // Is(domain.ErrNotFound)
```
1. Najpierw `scopes` (każdy wymagany) — bot bez scope'u dostaje `forbidden`, zanim dowie się czegokolwiek o nazwach.
2. Kandydaci = to, co principal widzi: portfele `visibleWallets` (także zarchiwizowane — zapis i tak odmówi `wallet_archived`), wszystkie koperty (także ukryte), kontrahenci, `visibleSubscriptions`.
3. Kolejność: dokładne id → nazwa bez wielkości liter (nazwy są unikalne w gospodarstwie) → jedyny kandydat, którego nazwa zawiera `ref` (bez wielkości liter).
4. Kilku kandydatów → `ValidationError{Field: <kind>, "is ambiguous: matches \"Karta\", \"Karta firmowa\""}` (do 10 nazw); brak → `RefNotFound{Kind}` (= `not_found`).
5. Pusty `ref` → wartość zerowa bez błędu (pole opcjonalne).

Kontrahent w `record_expense` idzie dalej jako nazwa do `RecordTransaction` (znajdź po nazwie/wzorcu albo utwórz — FR-140 etapu 002). `Resolve.Payee` służy filtrom (`search_transactions`).

## 5. Nowe use case'y

### Whoami
`Whoami.Do(ctx, p) (BotContext, error)` — tylko bot (`requireBot`: człowiek → `ForbiddenError{human_only}` nie ma sensu, więc `ErrForbidden`). `BotContext{Bot BotView, Household, Today domain.Date, Month budget.Month, Wallets []wallet.Wallet}`. Portfele (bez sald) — tylko gdy bot ma którykolwiek scope `wallets:*` lub `transactions:*`; inaczej pusta lista (AD-602).

### SpendingReport
```go
type SpendingReportInput struct { GroupBy string; From, To domain.Date; WalletID string }
type SpendingGroup struct { Key domain.ID /* "" = bez grupy */; Name string; Count int; Total int64 }
type SpendingReport struct { GroupBy string; From, To domain.Date; Currency string; Total int64; Count int;
    Groups []SpendingGroup; PendingTransactions int; Incomplete, Truncated bool }
```
- Scope `reports:read` (człowiek: każda rola czyta). Portfele: `visibleWallets` (lub jeden podany, widoczny → inaczej `not_found`).
- Zakres domyślny: od startu bieżącego miesiąca budżetowego do dziś; `to < from` lub > 366 dni → `invalid`.
- Wydatki = transakcje `kind = expense` w zakresie; kwota = `−BaseAmount` (zwroty pomniejszają). Przelewy i przychody pominięte. Transakcje bez kursu pominięte i policzone w `PendingTransactions` (`Incomplete`).
- Grupowanie: `envelope` (splity po liniach, nazwa koperty; brak = bez koperty), `payee` (nazwa kontrahenta), `wallet` (nazwa portfela), `vendor` (transakcja powiązana z obciążeniem `matched|posted` widocznej subskrypcji → dostawca tej subskrypcji; inne → bez dostawcy).
- Skan do 10 000 transakcji (strony po 200); więcej → `Truncated`. Grupy malejąco po `Total`, potem po nazwie.

## 6. Narzędzia (szczegóły w `api/mcp-tools.md`)

| Narzędzie | Use case(y) | Scope |
|---|---|---|
| `whoami` | `Whoami` | — |
| `list_wallets` | `ListWallets` | `wallets:read` |
| `get_balances` | `ListWallets` (salda, suma w walucie bazowej) | `wallets:read` |
| `search_transactions` | `Resolve` + `SearchTransactions` | `transactions:read` |
| `list_payees` | `ListPayees` | `transactions:read` |
| `list_envelopes` | `ListEnvelopeGroups` + `ListEnvelopes` | `budget:read` |
| `budget_overview` | `GetBudgetMonth` | `budget:read` |
| `spending_by` | `SpendingReport` | `reports:read` |
| `list_subscriptions` | `ListSubscriptions` | `subscriptions:read` |
| `get_subscription` | `Resolve` + `GetSubscription` + `ListSubscriptionPrices` | `subscriptions:read` |
| `upcoming_renewals` | `ListUpcomingRenewals` (domyślnie 30 dni) | `subscriptions:read` |
| `subscription_costs` | `GetSubscriptionSummary` | `subscriptions:read` |
| `list_charges` | `ListCharges` | `subscriptions:read` |
| `list_vendors` | `ListVendors` | — |
| `record_expense` | `Resolve` + `RecordTransaction` (+ `SuggestCharges`) | `transactions:write` (+ `subscriptions:read`, `transactions:read` dla sugestii) |
| `record_income` | `Resolve` + `RecordTransaction` | `transactions:write` |
| `transfer` | `Resolve` + `CreateTransfer` | `transactions:write` |
| `assign_to_envelope` | `Resolve` + `SetAssignment` | `budget:write` |
| `move_between_envelopes` | `Resolve` + `MoveBetweenEnvelopes` | `budget:write` |
| `create_subscription` | `Resolve` + `CreateSubscription` | `subscriptions:write` |
| `update_subscription` | `Resolve` + `UpdateSubscription` | `subscriptions:write` |
| `change_subscription_price` | `Resolve` + `ChangeSubscriptionPrice` | `subscriptions:write` |
| `confirm_charge_match` | `ConfirmChargeMatch` | `subscriptions:write`, `transactions:read` |
| `post_charge` | `Resolve` + `PostCharge` | `subscriptions:write`, `transactions:write` |
| `skip_charge` | `SkipCharge` | `subscriptions:write` |

Kwoty wejściowe: string dziesiętny (`"12.34"`, kropka; przecinek zamieniany na kropkę), waluta opcjonalna (domyślnie waluta portfela / bazowa / bieżącej ceny). `record_expense` i `record_income` biorą kwotę dodatnią; adapter nadaje znak (`money.Parse` + `Neg`).

## 7. Przypadki brzegowe

- Bot z listą portfeli, portfel spoza listy po id lub nazwie → `not_found` (nie zdradza istnienia).
- Bot twórcy z rolą viewer ma scope'y zapisu odfiltrowane (`EffectiveScopes`) → każdy zapis `forbidden`, nic nie trafia do bazy ani audytu.
- Twórca usunięty z gospodarstwa → efektywne scope'y puste → `forbidden` (odczyt też).
- `record_expense` bez scope'ów sugestii → `charge_suggestions: null` (nie błąd).
- Niejednoznaczna nazwa (`"kart"` przy „Karta” i „Karta firmowa”) → `invalid` z listą.
- Archiwalny portfel → `conflict` (`wallet_archived`).
- Batch JSON-RPC → 400; GET → 405; nieznane narzędzie → błąd JSON-RPC (SDK).

## 8. Strategia testów

- `sdk_probe_test.go` — kopia ze Straylight (punkty 1–7 dla go-sdk v1.8.0).
- `errors_test.go` — mapowanie każdego błędu domeny.
- `auth_test.go` — 401 jednakowe dla braków/złych/rotowanych/wyłączonych tokenów, `WWW-Authenticate`, 429 adresu i bota, 405/400/413, CORS, log bez tokenu.
- `tools_*_test.go` — każde narzędzie na fake'ach przez klienta SDK (`httptest`): wynik, `forbidden` z nazwą scope'u, portfel spoza listy → `not_found`, viewer-twórca → brak zapisu i audytu, `source=bot` i audyt z aktorem bot, sugestie po `record_expense`, niejednoznaczna nazwa.
- `contract_test.go` — lista narzędzi z `tools/list` = lista z `api/mcp-tools.md`; każde ma opis.
- Use case'y: `resolve_test.go`, `whoami_test.go`, `spending_report_test.go` (grupowania, zwroty, splity, pending, truncated, zakresy) + macierz autoryzacji `spending_authz_test.go` (rola × człowiek/bot × scope × lista portfeli × portfel prywatny).
- E2E `cmd/denar/e2e_mcp_test.go`: admin tworzy gospodarstwo, portfel, kopertę, subskrypcję i bota przez REST; klient go-sdk (`StreamableClientTransport`) → `whoami` → `record_expense` (nazwy) → `upcoming_renewals` → `subscription_costs` → bot bez `budget:write` → `assign_to_envelope` = `forbidden`; transakcja w REST ma `source=bot`.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-601 | Rozpoznawanie nazw w use case `Resolve` (scope'y najpierw; id → nazwa → jedyny podciąg; wielu → `invalid` z kandydatami; brak → `not_found`) | Reguła widoczności i scope'ów zostaje w use case'ach (P-004); podciąg pozwala na „kartą” → „Karta”, ale nie zgaduje przy kilku |
| AD-602 | `whoami` przez nowy use case `Whoami`; portfele (nazwa, id, waluta, typ — bez sald) tylko przy scope `wallets:*` lub `transactions:*` | Model musi znać nazwy portfeli do zapisu; salda nadal wymagają `wallets:read` |
| AD-603 | `spending_by` = nowy use case `SpendingReport` liczony w pamięci z `TransactionRepository.List` (≤ 10 000 transakcji, zakres ≤ 366 dni) zamiast nowego portu agregującego | Skala gospodarstwa domowego; bez zmian `ports.go` i SQLite |
| AD-604 | `spending_by` wymaga tylko `reports:read`; `vendor` przez widoczne subskrypcje | Raport nie ujawnia więcej niż widoczne transakcje i subskrypcje |
| AD-605 | `ForbiddenError.Scope` — use case podaje nazwę brakującego scope'u, adapter MCP ją cytuje | Model wie, o jaki scope poprosić użytkownika; REST bez zmian |
| AD-606 | Pusty miesiąc w `GetBudgetMonth`/`SetAssignment`/`MoveBetweenEnvelopes` = bieżący miesiąc budżetowy | `budget_overview(month?)`; reguła miesiąca w use case, nie w adapterze |
| AD-607 | Limit żądań MCP liczony osobnym licznikiem adaptera z tą samą wartością `DENAR_BOT_RATE` | Adaptery nie importują się nawzajem; wspólny licznik wymagałby nowego portu — zbędny przy limicie per proces |
| AD-608 | `record_expense`/`record_income` biorą kwotę dodatnią, znak nadaje adapter; przecinek dziesiętny akceptowany | Język naturalny („wydałem 12,50”); to kształt wejścia, nie reguła biznesowa |
| AD-609 | `upcoming_renewals` domyślnie 30 dni (REST 7) | Zgodnie z briefem; asystent zwykle pyta o „najbliższy miesiąc” |
| AD-610 | `assign_to_envelope` ustawia przydział na miesiąc (jak REST `PUT`), nie dodaje | Jedna semantyka w obu interfejsach; wynik pokazuje kopertę po zmianie |
| AD-611 | `mcp.Deps` przyjmuje `usecase.Deps` i sam buduje interaktory | Krótsze okablowanie 30 narzędzi; composition root dalej decyduje o zależnościach |
| AD-612 | Bez `list_notifications` | Powiadomienia to etap 005 (jeszcze niezrealizowany) |
| AD-613 | Bez usuwania danych przez MCP (transakcje, subskrypcje) | Najmniejsze ryzyko przy asystentach AI; usuwanie w UI |
| AD-614 | Wyniki list w MCP bez stronicowania poza `search_transactions` i `list_charges` (kursor jak w REST) | Pozostałe listy są małe (portfele, koperty, subskrypcje) |

## Ograniczenia

- Limit żądań per proces i osobno dla REST i MCP (bot może wykonać `DENAR_BOT_RATE` żądań na minutę w każdym z nich).
- `spending_by` liczy na żądanie; powyżej 10 000 transakcji w zakresie wynik ma `truncated: true`.
- Brak powiadomień MCP o zmianach (serwer bez sesji).
- Zachowanie z etapu 004 widoczne przez MCP: zmiana ceny z `effective_from` wcześniejszym niż pierwszy punkt historii (anchor nowej subskrypcji) nie zmienia obciążeń od anchor — pierwszy punkt nadal obowiązuje od swojego dnia (AD-404). Do rozważenia w 004.

## Dla etapu 007 (Web UI)

- Ekran botów może pokazać gotowe polecenie `claude mcp add --transport http denar <BASE_URL>/mcp --header "Authorization: Bearer <token>"` przy jednorazowym wyświetleniu tokenu.

## Dla etapu 005 (powiadomienia)

- Dodaj narzędzie `list_notifications` (scope `notifications:read`) w `internal/adapter/mcp/tools_*.go` i w `api/mcp-tools.md`; `contract_test.go` wymusza zgodność listy narzędzi z kontraktem.
