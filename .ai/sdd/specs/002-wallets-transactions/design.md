# Design: Portfele i transakcje (002-wallets-transactions)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `001-core/design.md`
> Last Updated: 2026-10-21

## 1. Architektura (nowe elementy)

```
internal/domain/date.go           domain.Date — dzień kalendarzowy (bez strefy)
internal/domain/money/            Money, Parse/String, Convert (big.Rat, half-even), ParseRate/FormatRate, ConvertParts
internal/domain/wallet/           Wallet, Type, Visibility, VisibleTo
internal/domain/transaction/      Transaction, Split, Kind, Status, Source, walidacja, tagi
internal/domain/payee/            Payee, wzorce, Resolve
internal/usecase/                 wallets.go, wallet_access.go, transactions.go, transfer.go, payees.go, fx.go
internal/usecase/{fakes,portstest} nowe repozytoria + RateProvider fake
internal/adapter/nbp/             RateProvider na API NBP (tabela A)
internal/adapter/sqlite/          00002_wallets_transactions.sql + repozytoria
internal/adapter/rest/            handlers_wallets.go, handlers_transactions.go, handlers_fx.go, mapping_money.go
cmd/denar/                        wiring, job uzupełniania kursów, e2e
```

Zmiany w kodzie 001 (integracja, minimalne):
- `access.household` ponownie odczytuje bota (`Bots.ByID`, `Usable`, zgodny `TokenHash`) przy każdej operacji — dotyczy też odczytów; dalej używa świeżej kopii (scope'y, portfele).
- Port `WalletDirectory` usunięty; `Deps.Wallets` jest teraz `WalletRepository`, a walidację listy portfeli bota robi `validateBotWallets` w use case (portfele istnieją w gospodarstwie i są widoczne dla twórcy). Gdy `Deps.Wallets == nil` (stare testy) — tylko walidacja składni, jak w 001.
- `WalletAllowed` bez zmian semantyki; filtr widoczności dla twórcy robi `visibleWallets` przed nim.
- Fake `HouseholdRepository.Cascade` dostaje nowe repozytoria; kaskady SQL przez FK.
- Test `TestWalletAllowed` zapisuje listę portfeli bota w repozytorium (bot jest teraz czytany ponownie).

## 2. Domena

### domain.Date
`Date{t time.Time}` (UTC północ; zero = brak). `ParseDate("2006-01-02")`, `DateOf(t, loc)`, `String`, `AddDays`, `Compare/Before/After`, `IsZero`. Kolejność leksykalna stringów = kolejność dat (SQL `TEXT`).

Nowe powody: `ConflictDuplicateExternalID` (`duplicate_external_id`), `ConflictWalletArchived` (`wallet_archived`); `ForbiddenWallet` (`wallet` — druga noga przelewu w portfelu niewidocznym).

### money
- `Money{Amount int64, Currency Currency}`; `New(amount, code)`, `Zero(c)`, `Add`/`Sub` (różne waluty → `ErrCurrencyMismatch`; przepełnienie → `ErrOverflow`), `Neg`, `Sign`, `IsZero`.
- `Parse(field, s, code)`: `-?\d+(\.\d{1,dec})?`, bez spacji/przecinka/wykładnika/`+`; więcej miejsc niż waluta lub poza `int64` → `ValidationError{field}`. `String()` → `"-12.34"`, JPY `"1200"`.
- `ParseRate(field, s)`: dodatni dziesiętny, ≤ 12 miejsc; wynik `*big.Rat`. `FormatRate(r)`: 8 miejsc, bez końcowych zer.
- `Convert(m, rate, to)`: `m.Amount × rate × 10^(to.Decimals − m.Decimals)`, zaokrąglenie half-even (`RoundHalfEven(*big.Rat) (int64, error)`). Kurs znaczy: 1 jednostka `m.Currency` = `rate` jednostek `to`.
- `ConvertParts(parts, rate, from, to, total)`: przelicza linie splitu i dopisuje resztę (total − Σ) do linii o największym |kwota| — suma zawsze równa kwocie bazowej transakcji.
- `CrossRate(fromPLN, toPLN)` = `fromPLN / toPLN`.

### wallet
`Wallet{ID, HouseholdID, Name, Type, Currency, OnBudget, Visibility, MemberIDs, CreatedBy, CreatedAt, ArchivedAt}`. Typy `cash|checking|card|savings|investment|loan`; `Visibility shared|private`. `Normalize` (trim, dla private dodaje twórcę do `MemberIDs`, dla shared czyści listę, sortuje), `Validate` (nazwa 1–100, typ, waluta z listy, ≤ 50 członków), `VisibleTo(userID)` = shared ∨ userID ∈ MemberIDs, `Archived()`.

### transaction
`Transaction{ID, HouseholdID, WalletID, Date, Amount int64, Currency, BaseAmount int64, RatePending bool, FxRate string, PayeeID, EnvelopeID, Kind, Status, Source, Tags []string, Note, ExternalID, TransferPairID, TransferWalletID, Splits []Split, CreatedBy domain.Actor, CreatedAt, UpdatedAt}`; `Split{EnvelopeID, Amount, BaseAmount, Note}`.
- `Kind expense|income|transfer`; `Status pending|cleared|reconciled`; `Source manual|import|subscription|bot`.
- `Validate`: data ustawiona; status/kind/source znane; notatka ≤ 1000; `external_id` ≤ 200; tagi (`NormalizeTags`: trim, lower-case, dedupe, sort; 1–40 znaków, ≤ 20); `income` bez koperty i splitów; splity tylko dla `expense`, bez koperty na transakcji, ≤ 50 linii, suma = kwota; `transfer` wymaga `TransferPairID` i nie ma splitów ani kontrahenta.
- Znak kwoty nie jest wiązany z rodzajem (zwrot = dodatni `expense`).
- `RatePending ⇔ BaseAmount nieznany`; `FxRate` informacyjny (8 miejsc).

### payee
`Payee{ID, HouseholdID, Name, Patterns []string, DefaultEnvelopeID, CreatedAt, UpdatedAt}`; nazwa 1–100, ≤ 20 wzorców po 1–100 znaków (podciąg, bez regexów — brak ReDoS). `Resolve(payees, text)`: najpierw nazwa (bez wielkości liter), potem pierwszy kontrahent (wg nazwy), którego wzorzec jest podciągiem tekstu.

## 3. Porty (`usecase/ports.go`)

```go
type WalletRepository interface {
    Create(ctx, wallet.Wallet) error                 // ConflictError duplicate_name
    Update(ctx, wallet.Wallet) error                 // name, type, on_budget, visibility, members, archived_at
    ByID(ctx, householdID, id domain.ID) (wallet.Wallet, error)
    ListByHousehold(ctx, householdID domain.ID) ([]wallet.Wallet, error) // wg nazwy, z archiwalnymi
}
type TransactionFilter struct { HouseholdID; WalletIDs []domain.ID /* wymagane */; PayeeID, EnvelopeID domain.ID;
    From, To domain.Date; Text string; Kind transaction.Kind; Status transaction.Status; Tag string }
type WalletBalance struct { Total, Cleared, Base int64; Pending int }
type TransactionRepository interface {
    Create(ctx, transaction.Transaction) error       // ConflictError duplicate_external_id
    Update(ctx, transaction.Transaction) error       // wszystko poza id, gospodarstwem, portfelem, źródłem, twórcą, created_at
    Delete(ctx, householdID, id domain.ID) error
    ByID(ctx, householdID, id domain.ID) (transaction.Transaction, error)
    ByPair(ctx, householdID, pairID domain.ID) ([]transaction.Transaction, error)
    List(ctx, TransactionFilter, Page) ([]transaction.Transaction, string, error) // data malejąco, id malejąco
    Balances(ctx, householdID domain.ID, walletIDs []domain.ID) (map[domain.ID]WalletBalance, error)
    ListRatePending(ctx, householdID domain.ID /* "" = wszystkie */, limit int) ([]transaction.Transaction, error)
}
type PayeeRepository interface {
    Create, Update(ctx, payee.Payee) error           // duplicate_name
    Delete(ctx, householdID, id) error               // transakcje tracą kontrahenta
    ByID(ctx, householdID, id) (payee.Payee, error)
    ListByHousehold(ctx, householdID) ([]payee.Payee, error) // wg nazwy
}
type FxRate struct { Currency string; Date, RateDate domain.Date; Rate string /* PLN za 1 */; FetchedAt time.Time }
type FxRateRepository interface {
    Get(ctx, currency string, date domain.Date) (FxRate, error)
    Put(ctx, FxRate) error                            // upsert
    Latest(ctx, currency string, from, to domain.Date) (FxRate, error) // najnowszy RateDate w [from, to]
}
type FxOverride struct { HouseholdID domain.ID; Currency string; Date domain.Date; Rate string /* baza za 1 */; CreatedBy domain.Actor; CreatedAt time.Time }
type FxOverrideRepository interface {
    Put(ctx, FxOverride) error                        // upsert
    Get(ctx, householdID, currency, date) (FxOverride, error)
    List(ctx, householdID) ([]FxOverride, error)      // data malejąco, waluta
}
type RateProvider interface {
    // PLNRate: ile PLN kosztuje 1 jednostka waluty wg tabeli A NBP z najnowszego dnia roboczego w [date−6, date].
    PLNRate(ctx, currency string, date domain.Date) (rate string, rateDate domain.Date, err error) // ErrRateUnavailable
}
var ErrRateUnavailable = errors.New("exchange rate unavailable")
```

Repozytoria transakcji, kontrahentów i portfeli przyjmują `householdID` w zapytaniach po id — obcy identyfikator daje `ErrNotFound`.

## 4. Kursy (`usecase/fx.go`)

`fx.quote(ctx, household, currency, date) (quote{Rate *big.Rat, RateDate, Source identity|manual|nbp}, error)`:
1. `currency == base` → 1, `identity` (bez sieci i bez bazy).
2. Nadpisanie gospodarstwa na dokładnie ten dzień → `manual`.
3. Kurs krzyżowy przez PLN: `plnRate(currency) / plnRate(base)` (`plnRate(PLN) = 1`). `RateDate` = starsza z dat.

`plnRate(currency, D)`: cache `Get(currency, D)` → provider → zapis do cache tylko gdy `D < dziś (UTC)` (przeszłe kursy są ostateczne; dzisiejsza tabela może się jeszcze ukazać) → przy błędzie providera `Latest(currency, D−6, D)` z cache (offline) → `ErrRateUnavailable`.

Wywołania kursów odbywają się **przed** transakcją bazy (NFR-011): use case najpierw czyta (autoryzacja, portfel, waluta), liczy kurs, a w `WithinTx` ponownie sprawdza dostęp i zapisuje.

## 5. Use case'y i autoryzacja

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `CreateWallet` | `wallets:write` | bot tylko `shared`; saldo otwarcia ≠ 0 → transakcja `income`, `reconciled`, notatka „Saldo otwarcia”, data `opened_on` (domyślnie dziś w strefie gospodarstwa) |
| `ListWallets`, `GetWallet` | `wallets:read` | z saldami; `include_archived` |
| `UpdateWallet` | `wallets:write` | nazwa, typ, `on_budget`, `archived`; `visibility`/`member_ids` — tylko człowiek: twórca, albo owner/admin dla portfela shared |
| `RecordTransaction` | `transactions:write` | `expense|income`; `payee_id` albo `payee_name` (Resolve/autocreate); domyślna koperta kontrahenta dla `expense` bez koperty i splitów; `source = bot` dla bota, inaczej `manual` |
| `GetTransaction`, `SearchTransactions` | `transactions:read` | tylko widoczne portfele; filtr po ukrytym portfelu → 404 |
| `UpdateTransaction` | `transactions:write` | zmiana kwoty/daty → nowy kurs; portfel niezmienny; noga przelewu: tylko data (obie nogi), status, notatka, tagi |
| `DeleteTransaction` | `transactions:write` | przelew → obie nogi; druga noga w niewidocznym portfelu → 403 `wallet` |
| `CreateTransfer` | `transactions:write` | portfele różne, widoczne, niearchiwalne; kwota > 0; różne waluty → `to_amount` wymagane; `envelope_id` tylko gdy dokładnie jeden portfel on-budget (trafia na jego nogę) |
| `ListPayees` | `transactions:read` | |
| `CreatePayee`, `UpdatePayee`, `DeletePayee` | `transactions:write` | |
| `QuoteFxRate`, `ListFxOverrides` | `transactions:read` | kurs waluty → baza na dzień; `unavailable` zamiast błędu |
| `SetFxOverride` | człowiek owner/admin | upsert + uzupełnienie oczekujących transakcji (gospodarstwo, waluta, dzień) |
| `FillPendingRates` | system (job) | do 200 oczekujących na przebieg; aktor `system/fx` |

Widoczność (`wallet_access.go`): `visibleWallets(s, p)` = portfele gospodarstwa, gdzie `VisibleTo(s.ActorUserID(p)) && s.WalletAllowed(id)`; `visibleWallet` → `ErrNotFound` dla ukrytego. Owner nie widzi cudzych portfeli prywatnych (prywatność ponad rolę). Członkowie portfela prywatnego muszą być członkami gospodarstwa.

Zapisy do portfela zarchiwizowanego (nowe transakcje, zmiany, przelewy) → 409 `wallet_archived`.

Audyt (`resource_type`): `wallet` (`wallet.created|updated`), `transaction` (`transaction.created|updated|deleted|rate_filled`), `payee` (`payee.created|updated|deleted`), `fx_rate` (`fx_rate.set`, `resource_id = "USD/2026-10-20"`). Zmiany kwot jako stringi `"-12.34 USD"`.

## 6. Schemat SQL (`00002_wallets_transactions.sql`)

- `wallets(id PK, household_id → households CASCADE, name, type CHECK, currency, on_budget, visibility CHECK, created_by → users, created_at, archived_at?)` + unikalny `(household_id, name COLLATE NOCASE)`.
- `wallet_members(wallet_id → wallets CASCADE, user_id → users, PK)`.
- `payees(id PK, household_id → households CASCADE, name, patterns JSON, default_envelope_id?, created_at, updated_at)` + unikalny `(household_id, name COLLATE NOCASE)`.
- `transactions(id PK, household_id → households CASCADE, wallet_id → wallets CASCADE, date TEXT, amount INTEGER, currency, base_amount INTEGER?, rate_pending CHECK (rate_pending = (base_amount IS NULL)), fx_rate?, payee_id → payees ON DELETE SET NULL, envelope_id? (bez FK — etap 003), kind CHECK, status CHECK, source CHECK, tags JSON, note, external_id?, transfer_pair_id?, transfer_wallet_id?, created_by_kind, created_by_id, created_at, updated_at)`; indeksy: unikalny `(wallet_id, external_id) WHERE external_id IS NOT NULL`, `(household_id, date, id)`, `(wallet_id, date)`, `(transfer_pair_id)`, `(envelope_id, date)`, częściowy `(rate_pending) WHERE rate_pending = 1`.
- `transaction_splits(transaction_id → transactions CASCADE, position, envelope_id?, amount, base_amount?, note, PK(transaction_id, position))`.
- `fx_rates(currency, date, rate_date, rate TEXT, fetched_at, PK(currency, date))` — globalny cache NBP.
- `fx_rate_overrides(household_id → households CASCADE, currency, date, rate TEXT, created_by_kind, created_by_id, created_at, PK(household_id, currency, date))`.

`bot_wallets` z 001 zostaje bez FK (portfeli się nie usuwa, tylko archiwizuje).

## 7. REST (`api/openapi.yaml`)

Wspólne schematy: `Money{amount, currency, minor}` (odpowiedzi), `MoneyInput{amount, currency}` (wejście; waluta musi się zgadzać z portfelem). Daty transakcji `format: date`.

| Operacja | Ścieżka |
|---|---|
| `listWallets`, `createWallet` | `GET|POST /households/{householdId}/wallets` (`?include_archived=true`) |
| `getWallet`, `updateWallet` | `GET|PATCH /households/{householdId}/wallets/{walletId}` |
| `searchTransactions`, `createTransaction` | `GET|POST /households/{householdId}/transactions` (`wallet_id, from, to, payee_id, envelope_id, q, kind, status, tag, cursor, limit`) |
| `getTransaction`, `updateTransaction`, `deleteTransaction` | `GET|PATCH|DELETE /households/{householdId}/transactions/{transactionId}` |
| `createTransfer` | `POST /households/{householdId}/transfers` → `{from, to}` |
| `listPayees`, `createPayee` | `GET|POST /households/{householdId}/payees` |
| `updatePayee`, `deletePayee` | `PATCH|DELETE /households/{householdId}/payees/{payeeId}` |
| `quoteFxRate` | `GET /households/{householdId}/fx-rates?currency&date` |
| `listFxOverrides`, `setFxOverride` | `GET|PUT /households/{householdId}/fx-rates/overrides` |

Kody: 200/201/204, 401, 403 (rola/scope/`human-only`/`wallet`), 404 (ukryte), 409 (`duplicate-name`, `duplicate-external-id`, `wallet-archived`), 422.

## 8. Adapter NBP

`nbp.New(Config{BaseURL (domyślnie https://api.nbp.pl), HTTP *http.Client (timeout 5 s)})`. Jedno zapytanie zakresowe `GET {base}/api/exchangerates/rates/a/{code}/{D−6}/{D}/?format=json` (NBP zwraca wszystkie notowania z zakresu — to jest fallback 7 dni bez 7 zapytań); bierze ostatnie notowanie. JSON dekodowany z `UseNumber` — `mid` jako string, bez floatów. 404 → `ErrRateUnavailable`; inne błędy sieci/statusy → `ErrRateUnavailable` opakowany (log). `PLN` nie trafia do sieci (use case go nie pyta).

## 9. Joby i konfiguracja

- `DENAR_NBP_URL` (domyślnie `https://api.nbp.pl`) — adres API (testy e2e: `httptest`).
- `DENAR_FX_INTERVAL` (domyślnie `30m`) — co ile job `FillPendingRates` uzupełnia oczekujące kwoty bazowe (pierwszy przebieg przy starcie).

## 10. Przypadki brzegowe

- Kwota z większą liczbą miejsc niż waluta (`12.345 PLN`) → 422; waluta `MoneyInput` ≠ waluta portfela → 422 `amount.currency`.
- Przelew do tego samego portfela → 422; ta sama waluta i `to_amount` ≠ `amount` → 422.
- `payee_id` i `payee_name` naraz → 422; nieznany `payee_id` → 422 `payee_id`.
- Zmiana listy członków portfela prywatnego nie może usunąć twórcy (dodawany automatycznie).
- Usunięcie kontrahenta: transakcje zostają bez kontrahenta.
- Kurs z NBP na weekend/święto → ostatni dzień roboczy z okna 7 dni; brak w oknie → `rate_pending`.
- Override na dzień, dla którego transakcje mają już kwotę bazową — nie przelicza ich (tylko oczekujące).

## 11. Strategia testów

- Domena: tabelaryczne `Parse/String/Add/Convert/RoundHalfEven/ConvertParts/ParseRate`, `Date`, walidacja portfela/transakcji/splitów/tagów, `payee.Resolve`.
- Use case'y na fake'ach: portfele, transakcje, przelewy, kontrahenci, kursy (cache, okno offline, override, identity bez providera), job, audyt i atomowość; **macierz autoryzacji** `wallet_authz_test.go`: rola × (człowiek/bot z/bez scope'u) × portfel shared/prywatny cudzy/prywatny własny × lista portfeli bota → oczekiwany wynik (dozwolone/403/404).
- `portstest`: suity wallet/transaction/payee/fx rate/fx override + kaskada gospodarstwa, na fake'ach i SQLite.
- NBP na `httptest.Server`: kurs, weekend (ostatni z zakresu), 404, 500, timeout, ścieżka i `format=json`.
- REST: przypadki konformancji każdej nowej operacji × każda zadeklarowana odpowiedź.
- E2E `cmd/denar`: portfel PLN + USD, transakcja USD z kursem z fake NBP, przelew PLN→USD, salda (w walucie i bazowe), bot zapisuje wydatek (`source = bot`), brak sieci → `rate_pending` → job uzupełnia.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-101 | `envelope_id` (transakcja i split) to tylko kolumna bez FK i bez walidacji istnienia; FK/walidację doda 003 (przebudowa tabeli albo walidacja w use case) | Koperty powstają w 003; brief dopuszcza |
| AD-102 | Saldo otwarcia = transakcja otwarcia (`income`, `reconciled`, „Saldo otwarcia”), nie pole portfela | Jedno źródło prawdy: saldo = Σ transakcji, kwota bazowa i `rate_pending` działają tak samo; 003 widzi otwarcie jako wpływ do Ready-to-Assign |
| AD-103 | Owner/admin nie widzą cudzych portfeli prywatnych | „Prywatne widoczne tylko dla uprawnionych”; prywatność ponad rolę |
| AD-104 | Ręczny kurs per gospodarstwo i dzień (tabela `fx_rate_overrides`), oznacza „1 waluta = kurs w walucie bazowej”; działa tylko na ten dzień i uzupełnia tylko oczekujące transakcje | Nadpisanie jednego gospodarstwa nie może zmieniać kursów innych; przewidywalność, brak cichych zmian historii budżetu |
| AD-105 | Cache NBP zapisuje tylko kursy dni przeszłych; przy braku sieci używa najnowszego kursu z cache z okna 7 dni | Dzisiejsza tabela może jeszcze nie istnieć; offline nie blokuje |
| AD-106 | Fallback 7 dni przez jedno zapytanie zakresowe NBP (`/{start}/{end}/`) zamiast 7 zapytań dziennych | Mniej ruchu, ten sam wynik |
| AD-107 | Portfel niezmienny w transakcji; noga przelewu zmienia tylko datę, status, notatkę, tagi (kwoty — usuń i utwórz ponownie) | Unika rozjazdu walut i par |
| AD-108 | Kwota bazowa splitów = przeliczenie każdej linii + reszta na linię o największym |kwota| | Suma splitów zawsze = kwota bazowa transakcji (koperty w 003) |
| AD-109 | Wzorce kontrahenta to podciągi bez wielkości liter, nie regexy | Prostota, bez ReDoS; wystarcza importowi |
| AD-110 | Bot może tworzyć tylko portfele `shared` i nie zmienia widoczności/członków | Ustawienia prywatności to decyzja człowieka |
| AD-111 | Brak usuwania portfeli — tylko archiwizacja; zapisy do zarchiwizowanego → 409 `wallet_archived` | Historia transakcji i budżetu zostaje |
| AD-112 | Port `WalletDirectory` z 001 zastąpiony `WalletRepository` w `Deps.Wallets` + `validateBotWallets` w use case | Logika widoczności należy do use case'u; jedno źródło portfeli |
| AD-113 | Ponowny odczyt bota w `access.household` przy każdej operacji (także odczytach) | FR-151 w jednym miejscu; koszt: 1 zapytanie po PK |
| AD-114 | Znak kwoty nie zależy od `kind` (zwrot to dodatni `expense`); `income` bez koperty i splitów; splity tylko dla `expense` | Model YNAB: income → Ready-to-Assign, wydatki/zwroty → koperty |
| AD-115 | `GET /fx-rates` zwraca `source: unavailable` i `rate: null` zamiast błędu | Brak sieci to stan, nie błąd klienta |
| AD-116 | Tekst wyszukiwania przeszukuje notatkę i nazwę kontrahenta (bez wielkości liter, `LIKE` z escapowaniem) | Wystarczające dla MVP; FTS później |
| AD-117 | Przelew zapisuje dwa wpisy audytu `transaction.created` (po jednym na nogę) | Filtr audytu po `resource_type = transaction` widzi obie nogi |

## Ograniczenia

- Brak FK `envelope_id` (003) i brak FK `bot_wallets.wallet_id`.
- Kurs krzyżowy wymaga obu walut w tabeli A NBP (waluty spoza tabeli A tylko przez ręczny kurs).
- Salda liczone zapytaniem agregującym przy każdym odczycie listy portfeli (bez materializacji) — wystarczy dla gospodarstwa domowego.
- Zmiana `on_budget` portfela z historią zmienia budżet wstecz (003 liczy z transakcji).
