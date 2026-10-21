# Design: Budżet kopertowy (003-envelope-budget)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `.ai/sdd/PLAN.md`, `001-core/design.md`, `002-wallets-transactions/design.md`
> Last Updated: 2026-10-21

## 1. Architektura (nowe elementy)

```
internal/domain/budget/           month.go (Month), envelope.go (Group, Envelope, Goal), ledger.go (Ledger, MonthSummary),
                                  goal.go (GoalStatus), autoassign.go (AutoAssign)
internal/usecase/                 budget_envelopes.go (grupy, koperty), budget_month.go (widok, historia),
                                  budget_assign.go (przydział, przeniesienie, auto-przydział), envelope_check.go (walidacja envelope_id)
internal/usecase/{fakes,portstest} repozytoria grup, kopert, przydziałów; TransactionRepository.BudgetFlows/CountByEnvelope
internal/adapter/sqlite/          00003_envelope_budget.sql, envelope.go, assignment.go, transaction.go (agregaty)
internal/adapter/rest/            handlers_budget.go, mapping_budget.go
cmd/denar/                        wiring, e2e_budget_test.go
```

Zmiany w kodzie 002 (integracja, minimalne):
- `RecordTransaction`, `UpdateTransaction`, `CreateTransfer`, `CreatePayee`, `UpdatePayee` wołają `checkEnvelopes` w transakcji bazy (FR-220). Gdy `Deps.Envelopes == nil`, walidacji nie ma (testy 001/002 bez kopert).
- Domyślna koperta kontrahenta (`RecordTransaction`) jest przypisywana tylko, gdy koperta istnieje (FR-221).
- `TransactionRepository` dostaje `BudgetFlows` i `CountByEnvelope`; fake `HouseholdRepository.Cascade` dostaje nowe repozytoria.
- Testy 002 używające dowolnych identyfikatorów kopert dostają koperty w fixture.

## 2. Domena (`internal/domain/budget`)

### Month
`Month{y int, m time.Month}`; `ParseMonth("2026-10")` (rok 2000–2100), `MonthOf(d domain.Date, startDay int)`, `Start(startDay)`, `End(startDay)` (włącznie), `Next`, `Prev`, `Compare`, `Before`, `After`, `String`, `IsZero`, `MonthsUntil(o)` (liczba miesięcy od m do o).
Etykieta = miesiąc kalendarzowy, w którym miesiąc budżetowy się zaczyna: przy `start_day = 10` miesiąc `2026-10` to 2026-10-10…2026-11-09, a 2026-10-05 należy do `2026-09`.

### Group, Envelope, Goal
- `Group{ID, HouseholdID, Name, Position, CreatedAt, UpdatedAt}`; nazwa 1–100, pozycja 0…10000.
- `Envelope{ID, HouseholdID, GroupID, Name, Position, Hidden, Goal, CreatedAt, UpdatedAt}`; nazwa 1–100.
- `Goal{Type, Amount int64, TargetDate domain.Date}`; `Type`: `""` (brak), `monthly`, `target_by_date`. Kwota 1…10^15 jednostek minimalnych waluty bazowej; `target_by_date` wymaga daty, `monthly` jej nie ma.
- Kolejność: `SortGroups`/`SortEnvelopes` — pozycja, nazwa (bez wielkości liter), id.

### Ledger (czyste obliczenia)
```go
l := budget.NewLedger()
l.SetAssigned(env, m, amount)       // przydział (ostatni wygrywa)
l.AddActivity(env, m, amount)       // aktywność koperty
l.AddInflow(m, amount)              // wpływ do RTA
s, err := l.Month(m)                // MonthSummary; ErrOverflow przy przepełnieniu sumy
```
`MonthSummary{Month, Envelopes map[ID]EnvelopeMonth{Assigned, Activity, CarriedIn, Available}, Inflow, ReadyToAssign, AssignedInFuture, OverspentLastMonth (≤ 0), Assigned, Activity, Available}`.

Algorytm: miesiące z danymi ≤ m sortowane rosnąco; dla każdej koperty `Available = Carry + Assigned + Activity`, `Carry' = max(Available, 0)`, `Overspent = min(Available, 0)`. W miesiącu bez danych `Available = Carry ≥ 0`, więc pomijanie pustych miesięcy nie zmienia wyniku.

```
RTA(m) = Σ_{k≤m} Inflow(k) − Σ_{k≤m} Σ_E Assigned(E,k) + Σ_{k<m} Σ_E Overspent(E,k) − AssignedInFuture(m)
AssignedInFuture(m) = Σ_{k>m} Σ_E Assigned(E,k)
OverspentLastMonth(m) = Σ_E Overspent(E, m−1)
```

### Inwariant (dokładna definicja)
Dla każdego miesiąca m i stanu bazy:

```
Σ_E Available(E, m) + RTA(m) + AssignedInFuture(m) = Σ base_amount wszystkich transakcji i linii splitu
                                                      portfeli on-budget z datą ≤ End(m), bez rate_pending
```

Dowód: `Available(E,m) = Σ_{k≤m}(Assigned+Activity)(E,k) − Σ_{k<m} Overspent(E,k)` (bo `Carry = Available − Overspent`); po dodaniu RTA i przydziałów przyszłych przydziały i przekroczenia się znoszą, zostaje `Σ_{k≤m}(Inflow + Σ_E Activity)` — czyli każda kwota bazowa on-budget dokładnie raz (z kopertą → Activity, bez koperty → Inflow). Dla bieżącego miesiąca bez transakcji z przyszłą datą prawa strona to Σ `base_balance` portfeli on-budget z 002. `ready_to_assign` w API już odejmuje przydziały przyszłe (jak YNAB), dlatego składnik `AssignedInFuture` jest jawny. Test: `TestInvariantRandom` (domena) i `TestBudgetInvariant` (use case na fake'ach z losowymi transakcjami, przelewami, splitami, przydziałami w przeszłości i przyszłości).

### Cele (`GoalStatus`)
`Status(g Goal, e EnvelopeMonth, m Month, startDay int) GoalStatus{Needed, Underfunded int64, Funded bool, Progress int}`:
- `monthly`: `Needed = Amount`; `Underfunded = max(0, Amount − Assigned)`; `Progress = clamp(Assigned·100/Amount)`.
- `target_by_date`: `n = MonthsUntil(MonthOf(TargetDate)) + 1`, min 1 (po terminie 1); `start = Available − Assigned` (środki bez przydziału bieżącego miesiąca); `Needed = ceil(max(0, Amount − start) / n)`; `Underfunded = max(0, Needed − Assigned)`; `Progress = clamp(Available·100/Amount)`.
- `Funded = Underfunded == 0`; `Progress` całkowity 0…100.

### AutoAssign
`AutoAssign(candidates []Candidate{EnvelopeID, Underfunded}, rta int64) []Allocation{EnvelopeID, Amount}` — w kolejności kandydatów `give = min(Underfunded, rta)` dopóki `rta > 0`. Use case podaje kandydatów: nieukryte koperty z celem i `Underfunded > 0`, w kolejności grup i kopert.

## 3. Porty (`usecase/ports.go`)

```go
type EnvelopeGroupRepository interface {
    Create(ctx, budget.Group) error                     // ConflictError duplicate_name
    Update(ctx, budget.Group) error                     // name, position, updated_at; duplicate_name
    Delete(ctx, householdID, id domain.ID) error        // ErrNotFound; koperty grupy → ConflictError group_not_empty
    ByID(ctx, householdID, id domain.ID) (budget.Group, error)
    ListByHousehold(ctx, householdID domain.ID) ([]budget.Group, error) // pozycja, nazwa, id
}
type EnvelopeRepository interface {
    Create(ctx, budget.Envelope) error                  // duplicate_name; obca/nieistniejąca grupa → ErrNotFound
    Update(ctx, budget.Envelope) error                  // group, name, position, hidden, goal, updated_at
    Delete(ctx, householdID, id domain.ID) error        // razem z przydziałami
    ByID(ctx, householdID, id domain.ID) (budget.Envelope, error)
    ListByHousehold(ctx, householdID domain.ID) ([]budget.Envelope, error) // pozycja, nazwa, id
}
type AssignmentRepository interface {
    Set(ctx, budget.Assignment) error                   // upsert; Amount == 0 usuwa
    Get(ctx, householdID, envelopeID domain.ID, m budget.Month) (budget.Assignment, error) // brak → Amount 0, bez błędu
    ListByHousehold(ctx, householdID domain.ID) ([]budget.Assignment, error)  // miesiąc, koperta
}
// w TransactionRepository:
BudgetFlows(ctx, householdID domain.ID, walletIDs []domain.ID, to domain.Date) (BudgetFlows, error)
CountByEnvelope(ctx, householdID, envelopeID domain.ID) (int, error)  // transakcje z kopertą na sobie lub w linii splitu

type BudgetFlow struct { Date domain.Date; EnvelopeID domain.ID /* "" bez koperty */; Kind transaction.Kind; Base int64 }
type BudgetFlows struct { Flows []BudgetFlow /* jeden wiersz na (data, koperta, rodzaj) */; Pending map[domain.Date]int }
```
`BudgetFlows` sumuje `base_amount` transakcji bez splitów i linii splitów (rodzaj linii = rodzaj transakcji) z portfeli z listy, z datą ≤ `to` (zero = bez ograniczenia), bez `rate_pending`; `Pending` liczy transakcje z `rate_pending` per dzień. Use case podaje wszystkie portfele on-budget gospodarstwa (także archiwalne).

`budget.Assignment{HouseholdID, EnvelopeID, Month, Amount int64, UpdatedAt}`.

## 4. Use case'y i autoryzacja

| Use case | Uprawnienie | Uwagi |
|---|---|---|
| `ListEnvelopeGroups`, `ListEnvelopes`, `GetEnvelope` | `budget:read` | `ListEnvelopes` z `include_hidden` |
| `CreateEnvelopeGroup`, `UpdateEnvelopeGroup`, `DeleteEnvelopeGroup` | `budget:write` | pusta grupa only |
| `CreateEnvelope`, `UpdateEnvelope`, `DeleteEnvelope` | `budget:write` | grupa musi istnieć (422 `group_id`); cel w walucie bazowej; usunięcie: 409 `envelope_in_use`, czyści `default_envelope_id` kontrahentów (audyt `payee.updated`) |
| `GetBudgetMonth` | `budget:read` | widok miesiąca (FR-214) |
| `ListEnvelopeActivity` | `budget:read` + `transactions:read` | transakcje koperty w miesiącu, tylko widoczne portfele on-budget |
| `SetAssignment` | `budget:write` | kwota w walucie bazowej (inna → 422 `amount.currency`), zwraca widok miesiąca |
| `MoveBetweenEnvelopes` | `budget:write` | kwota > 0, koperty różne i istniejące; dwa przydziały w jednej transakcji |
| `AutoAssign` | `budget:write` | FR-213; zwraca przydziały i widok miesiąca |

Widoczność (AD-301, AD-302): budżet jest wspólny dla gospodarstwa — sumy liczone ze **wszystkich** portfeli on-budget, także prywatnych portfeli innych osób i portfeli spoza listy bota. Pojedyncze transakcje (`ListEnvelopeActivity`) — tylko z portfeli widocznych dla principala (`visibleWallets`) i dodatkowo `transactions:read`.

Audyt (`resource_type`): `envelope_group` (`envelope_group.created|updated|deleted`), `envelope` (`envelope.created|updated|deleted`), `assignment` (`assignment.set`, `resource_id` = id koperty, zmiany `month`, `assigned` from→to, `via` = `set|move|auto_assign`).

## 5. Schemat SQL (`00003_envelope_budget.sql`)

- `envelope_groups(id PK, household_id → households CASCADE, name, position, created_at, updated_at)` + unikalny `(household_id, name COLLATE NOCASE)`.
- `envelopes(id PK, household_id → households CASCADE, group_id → envelope_groups (NO ACTION), name, position, hidden, goal_type CHECK (NULL|monthly|target_by_date), goal_amount?, goal_date?, created_at, updated_at, CHECK spójności celu)` + unikalny `(household_id, name COLLATE NOCASE)`, indeks `(group_id)`.
- `envelope_assignments(household_id → households CASCADE, envelope_id → envelopes CASCADE, month TEXT 'YYYY-MM', amount INTEGER ≠ 0, updated_at, PK(envelope_id, month))` + indeks `(household_id, month)`.
- Indeks `transaction_splits(transaction_id, envelope_id)` niepotrzebny — `CountByEnvelope` używa istniejących indeksów `envelope_id`.

`group_id` bez `ON DELETE RESTRICT`: kaskada usuwania gospodarstwa kasuje grupy i koperty w jednym poleceniu, a `NO ACTION` sprawdza klucz na końcu polecenia. Brak FK `transactions.envelope_id`/`transaction_splits.envelope_id` (SQLite wymaga przebudowy tabeli) — walidacja w use case'ach (AD-303).

## 6. REST (`api/openapi.yaml`)

| Operacja | Ścieżka |
|---|---|
| `getBudgetMonth` | `GET /households/{householdId}/budget/months/{month}` |
| `listEnvelopeActivity` | `GET /households/{householdId}/budget/months/{month}/envelopes/{envelopeId}/transactions?cursor&limit` |
| `setAssignment` | `PUT /households/{householdId}/budget/months/{month}/assignments/{envelopeId}` `{amount: MoneyInput}` → `BudgetMonth` |
| `moveBetweenEnvelopes` | `POST /households/{householdId}/budget/months/{month}/moves` `{from_envelope_id, to_envelope_id, amount}` → `BudgetMonth` |
| `autoAssign` | `POST /households/{householdId}/budget/months/{month}/auto-assign` → `{assigned: [{envelope_id, amount}], month: BudgetMonth}` |
| `listEnvelopeGroups`, `createEnvelopeGroup` | `GET|POST /households/{householdId}/budget/envelope-groups` |
| `updateEnvelopeGroup`, `deleteEnvelopeGroup` | `PATCH|DELETE /households/{householdId}/budget/envelope-groups/{groupId}` |
| `listEnvelopes`, `createEnvelope` | `GET|POST /households/{householdId}/budget/envelopes` (`?include_hidden=true`) |
| `getEnvelope`, `updateEnvelope`, `deleteEnvelope` | `GET|PATCH|DELETE /households/{householdId}/budget/envelopes/{envelopeId}` |

`BudgetMonth{month, start, end, currency, ready_to_assign, assigned_in_future, overspent_last_month, income, uncategorized, transfers, assigned, activity, available, underfunded, incomplete, pending_transactions, groups[{id, name, position, assigned, activity, available, envelopes[{id, group_id, name, position, hidden, goal, goal_status, assigned, activity, carried_over, available}]}]}` — wszystkie kwoty jako `Money` w walucie bazowej. Widok zawiera także koperty ukryte (`hidden: true`) — inaczej inwariant nie byłby widoczny w sumach.

Nowe powody 409: `group-not-empty`, `envelope-in-use`.

## 7. Przypadki brzegowe

- Miesiąc spoza 2000-01…2100-12 lub zły format → 422 `month`.
- Przydział w walucie innej niż bazowa → 422 `amount.currency`; kwota z za dużą liczbą miejsc → 422.
- Przeniesienie: ta sama koperta → 422 `to_envelope_id`; kwota ≤ 0 → 422; nieznana koperta → 404.
- Przeniesienie może zrobić ujemny przydział/available (jak YNAB) — to dozwolone.
- Transakcje ze starym `envelope_id` bez koperty (sprzed 003) liczą się jako „bez koperty” (wpływ RTA) — inwariant zostaje.
- Zmiana `on_budget` portfela zmienia historię budżetu (liczona z transakcji).
- Koperta ukryta: dalej w obliczeniach i widoku (z flagą); dozwolona w transakcjach; pomijana w auto-przydziale.
- Usunięcie koperty z przydziałami (bez transakcji): przydziały znikają, środki wracają do RTA (audyt `envelope.deleted` z sumą przydziałów).

## 8. Strategia testów

- Domena: tabelaryczne `Month` (start_day 1/10/28, przełom roku), `Ledger` (przeniesienia przez wiele miesięcy, overspending i jego wpływ na RTA następnego miesiąca, przydziały przyszłe, luki miesięcy, przepełnienie), cele (monthly, target_by_date przed/po terminie, z wydatkami), `AutoAssign`; test losowy inwariantu.
- Use case'y na fake'ach: CRUD grup/kopert (konflikty, `group_not_empty`, `envelope_in_use`, czyszczenie kontrahentów), przydział/przeniesienie/auto-przydział z audytem i atomowością, widok miesiąca (przychód, bez koperty, przelewy on↔on w różnych walutach, on→off z kopertą i bez, splity, rate_pending → incomplete, cudze portfele prywatne w sumach), historia koperty (filtr widoczności), walidacja `envelope_id` w 002, test inwariantu z losowymi danymi; **macierz autoryzacji** `budget_authz_test.go`: rola × człowiek/bot × scope × lista portfeli.
- `portstest`: suity envelope group / envelope / assignment, `BudgetFlows`, `CountByEnvelope`, kaskada gospodarstwa — na fake'ach i SQLite.
- REST: konformancja każdej nowej operacji × każda zadeklarowana odpowiedź.
- E2E `cmd/denar`: dochód → grupy i koperty → przydziały → wydatki → overspending → następny miesiąc (RTA pomniejszone, koperta wyzerowana) → przeniesienie → auto-przydział → inwariant z saldami portfeli.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-301 | Budżet wspólny dla gospodarstwa: sumy (activity, RTA) obejmują wszystkie portfele on-budget, także prywatne innych osób | Inwariant i jedno RTA gospodarstwa; prywatność transakcji zachowana w liście (AD-302) |
| AD-302 | Bot z `budget:read` widzi sumy niezależnie od listy portfeli; lista transakcji koperty wymaga `transactions:read` i pokazuje tylko widoczne portfele | Rekomendacja orkiestratora; sumy nie ujawniają pojedynczych transakcji |
| AD-303 | Brak FK `envelope_id` w transakcjach/splitach; walidacja istnienia koperty w use case'ach zapisu (w transakcji bazy) | SQLite nie doda FK do istniejącej kolumny bez przebudowy tabeli |
| AD-304 | Koperty użytej w transakcji/splicie nie da się usunąć (409 `envelope_in_use`) — tylko ukryć; usunięcie nieużywanej kasuje jej przydziały i czyści domyślną kopertę kontrahentów | Historia budżetu nie traci kategorii; przeniesienie transakcji na inną kopertę — poza MVP |
| AD-305 | Koperta ukryta jest dozwolona w transakcjach (walidowane jest tylko istnienie) i liczy się w obliczeniach; auto-przydział ją pomija | Edycja starych transakcji nie może się psuć; ukrycie to porządek UI |
| AD-306 | Każda kwota on-budget bez koperty (przychód, wydatek „bez koperty”, noga przelewu) zasila/obciąża RTA bezpośrednio; widok raportuje osobno `income`, `uncategorized`, `transfers` | Inwariant zgadza się bez wyjątków; różnica kursowa przelewu on↔on trafia do RTA |
| AD-307 | Transakcje `rate_pending` pomijane w sumach; `incomplete = true`, gdy w miesiącu m lub wcześniej jest choć jedna, z liczbą `pending_transactions` | Brak kursu nie blokuje budżetu (jak saldo w 002) |
| AD-308 | `ready_to_assign` odejmuje przydziały miesięcy przyszłych (YNAB „assigned in future”), pole `assigned_in_future` jawne; inwariant zawiera ten składnik | Nie da się wydać dwa razy pieniędzy przydzielonych na przyszłość |
| AD-309 | Etykieta miesiąca = miesiąc kalendarzowy początku; zmiana `month_start_day` nie przelicza przydziałów (zostają przy etykietach) | Najprostsza spójna reguła; zmiana dnia startu poza MVP |
| AD-310 | Przydział może być ujemny; 0 usuwa wiersz; przeniesienie nie sprawdza, czy koperta źródłowa ma środki | Zgodne z YNAB, prosty model |
| AD-311 | Cel `monthly` = „przydziel X co miesiąc” (nie „miej X dostępne”); `target_by_date` rozkłada brakującą kwotę równo na pozostałe miesiące (zaokrąglenie w górę) | Najprostsze cele z PLAN.md; deterministyczne |
| AD-312 | Operacje `setAssignment` i `moveBetweenEnvelopes` zwracają cały widok miesiąca; `autoAssign` — przydziały i widok | Klient po zmianie i tak odświeża siatkę |
| AD-313 | Nazwy grup i kopert unikalne w gospodarstwie (bez wielkości liter), nie w grupie | Jednoznaczne odwołania po nazwie (MCP 006) |
| AD-314 | Kwoty `envelope_id` wskazujące nieistniejącą kopertę (dane sprzed 003) liczą się jako „bez koperty” | Inwariant bez migracji danych |
| AD-315 | Agregacja przez `BudgetFlows` (suma per dzień × koperta × rodzaj); miesiące budżetowe liczy use case | Reguła miesiąca (start_day) zostaje w domenie, SQL tylko sumuje |
| AD-316 | Bot może zarządzać kopertami i przydziałami z `budget:write` (nie są operacjami administracyjnymi) | Asystent może prowadzić budżet; audyt odnotowuje bota |

## Ograniczenia

- Obliczenia od pierwszego miesiąca z danymi przy każdym odczycie (bez materializacji) — wystarczy dla gospodarstwa domowego.
- Brak FK `envelope_id` (AD-303); zmiana `month_start_day` nie przelicza historii (AD-309).
- Brak kart kredytowych w stylu YNAB i celów „needed for spending”.
- Brak przeniesienia transakcji z usuwanej koperty na inną (AD-304).

## Dla etapu 004 (subskrypcje)

- Subskrypcja ma kopertę (`envelope_id`) i portfel płatności: przy zapisie walidować kopertę helperem `Deps.checkEnvelope(ctx, householdID, field, id)` (w `envelope_check.go`) w transakcji bazy; koperta musi istnieć, ukryta dozwolona.
- Zaksięgowanie obciążenia tworzy transakcję `expense` (`Source = subscription`) — przez `insertTransaction`; kopertę subskrypcji ustawiać na transakcji jawnie (domyślna koperta kontrahenta działa tylko w `RecordTransaction`).
- `DeleteEnvelope` sprawdza `Transactions.CountByEnvelope`; 004 powinien dodać sprawdzenie subskrypcji (koperta użyta przez subskrypcję → 409 `envelope_in_use`) albo czyszczenie.
- Kwota miesięczna subskrypcji w walucie bazowej może zasilić cel `monthly` koperty (np. sugestia celu) — domena `budget.Goal`.
- Widok budżetu: `usecase.GetBudgetMonth` → `BudgetMonthView`; obliczenia `budget.Ledger`; miesiąc `budget.Month`/`MonthOf(date, household.MonthStartDay)`.
