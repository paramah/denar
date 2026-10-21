# Feature: Budżet kopertowy (F03, etap 3)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (plan:approved), etap 3 `003-envelope-budget`; brief orkiestratora; `002-wallets-transactions/design.md` („Dla etapu 003”)
> Last Updated: 2026-10-21

## Overview

Budżet kopertowy w stylu YNAB w walucie bazowej gospodarstwa: grupy i koperty z celami, miesięczne przydziały, aktywność liczona z transakcji portfeli on-budget, dostępne środki z przeniesieniem między miesiącami, Ready-to-Assign, przenoszenie między kopertami i auto-przydział wg celów. Etap domyka punkt rozszerzenia 002: walidację `envelope_id` w transakcjach, splitach, przelewach i domyślnej kopercie kontrahenta.

## Business Context

- **Problem:** Ewidencja wydatków (002) nie mówi, ile jeszcze można wydać na daną kategorię.
- **Users:** Członkowie gospodarstwa (owner/admin/member — zapis, viewer — odczyt), boty z `budget:read|write`.
- **Value:** Każda złotówka ma przypisane zadanie; widać przekroczenia i brakujące środki na cele.
- **Success signal:** Inwariant „Σ Available + RTA (+ przydziały przyszłe) = Σ sald on-budget w bazie” zielony w teście losowym; testy domeny, macierz autoryzacji, `portstest` na SQLite, konformancja REST i e2e zielone.

## Goals

- Czyste funkcje domenowe: miesiąc budżetowy, Available z przeniesieniem, overspending (reguła YNAB), RTA, postęp celów, auto-przydział.
- CRUD grup i kopert, przydziały, przeniesienia, widok miesiąca, historia aktywności koperty.
- Spójność z 002: walidacja `envelope_id`, domyślna koperta kontrahenta.

## Non-Goals / Out of Scope

- Koperty w walutach innych niż bazowa; zmiana `month_start_day` z przeliczeniem historii (przydziały zostają przy etykietach miesięcy).
- Karty kredytowe w stylu YNAB (koperta płatności karty), cele „needed for spending”, harmonogramy celów powtarzalnych.
- Narzędzia MCP (006), UI (007), powiadomienia `envelope.overspent`/`goal_due` (005).

## User Stories

### US-020: Koperty i grupy

**As a** członek  
**I want to** pogrupować koperty, ustawić ich kolejność, ukryć nieużywane i nadać cel  
**So that** budżet odzwierciedla moje kategorie wydatków

**Acceptance Criteria:**
- [ ] Nazwa grupy i koperty jest unikalna w gospodarstwie (bez wielkości liter).
- [ ] Grupy z kopertami nie da się usunąć (409).
- [ ] Koperty z transakcjami nie da się usunąć (409) — można ją ukryć.

### US-021: Przydział i przeniesienie

**As a** członek  
**I want to** przydzielić kwotę do koperty na miesiąc i przenieść środki między kopertami  
**So that** planuję wydatki z Ready-to-Assign

### US-022: Widok miesiąca

**As a** członek lub viewer  
**I want to** zobaczyć dla miesiąca grupy, koperty (assigned, activity, available, cel), Ready-to-Assign i przekroczenia z poprzedniego miesiąca  
**So that** wiem, ile mogę wydać

### US-023: Auto-przydział

**As a** członek  
**I want to** jednym poleceniem uzupełnić niedofinansowane cele z Ready-to-Assign  
**So that** nie liczę ręcznie

### US-024: Historia koperty

**As a** członek  
**I want to** zobaczyć transakcje koperty w miesiącu  
**So that** rozumiem jej aktywność

## Functional Requirements

### FR-200: Miesiąc budżetowy — Must Have

THE SYSTEM SHALL identyfikować miesiąc budżetowy etykietą `YYYY-MM` miesiąca kalendarzowego, w którym się zaczyna; miesiąc `Y-M` obejmuje dni od `Y-M-{month_start_day}` do dnia przed `Y-(M+1)-{month_start_day}`; „bieżący miesiąc” wynika z dzisiejszej daty w strefie gospodarstwa.

### FR-201: Aktywność — Must Have

THE SYSTEM SHALL liczyć aktywność koperty E w miesiącu jako Σ `base_amount` transakcji portfeli on-budget z `envelope_id = E` bez splitów plus Σ `base_amount` linii splitu z `envelope_id = E`, z datą w miesiącu. Transakcje z oczekującym kursem SHALL być pominięte, a miesiąc oznaczony jako niepełny z liczbą takich transakcji.

### FR-202: Wpływy do Ready-to-Assign — Must Have

THE SYSTEM SHALL zaliczać do wpływów RTA każdą kwotę bazową transakcji (lub linii splitu) portfela on-budget bez koperty: przychody (z saldem otwarcia), wydatki bez koperty („bez koperty”), nogi przelewów bez koperty (przelew on↔on w jednej walucie daje 0, w różnych — różnicę kursową; on→off bez koperty — wypływ). Kwoty z `envelope_id` nieistniejącej koperty traktuje jak „bez koperty”.

### FR-203: Available i przeniesienie — Must Have

THE SYSTEM SHALL liczyć `Available(E, m) = Carry(E, m−1) + Assigned(E, m) + Activity(E, m)`, gdzie `Carry = max(Available, 0)`; ujemne `Available` (przekroczenie) zeruje się na początku następnego miesiąca i pomniejsza RTA następnego miesiąca (reguła YNAB).

### FR-204: Ready-to-Assign — Must Have

THE SYSTEM SHALL liczyć `RTA(m) = Σ_{k≤m} Wpływy(k) − Σ_{k≤m} Assigned(k) + Σ_{k<m} Przekroczenia(k) − AssignedInFuture(m)`, gdzie `AssignedInFuture(m)` to suma przydziałów miesięcy po m.

### FR-205: Inwariant — Must Have

THE SYSTEM SHALL spełniać dla każdego miesiąca m: `Σ_E Available(E, m) + RTA(m) + AssignedInFuture(m) = Σ base_amount` transakcji portfeli on-budget z datą ≤ końca m (bez oczekujących na kurs).

### FR-206: Cele — Must Have

THE SYSTEM SHALL obsługiwać cele kopert: `monthly` (kwota do przydzielenia co miesiąc) i `target_by_date` (kwota do zgromadzenia do daty), z wyliczeniem kwoty potrzebnej w miesiącu, niedofinansowania, flagi „funded” i postępu w procentach (liczba całkowita).

### FR-210: Grupy i koperty — Must Have

THE SYSTEM SHALL zarządzać grupami (nazwa, pozycja) i kopertami (nazwa, grupa, pozycja, ukrycie, cel); usunięcie grupy z kopertami → 409 `group_not_empty`; usunięcie koperty użytej w transakcji lub splicie → 409 `envelope_in_use`; usunięcie koperty usuwa jej przydziały i zdejmuje ją jako domyślną z kontrahentów.

### FR-211: Przydział — Must Have

WHEN principal ustawi kwotę przydziału koperty na miesiąc  
THE SYSTEM SHALL zapisać ją (kwota w walucie bazowej, dowolny znak; 0 usuwa przydział) i zapisać audyt.

### FR-212: Przeniesienie — Must Have

WHEN principal przeniesie kwotę > 0 z koperty A do B w miesiącu  
THE SYSTEM SHALL zmniejszyć przydział A i zwiększyć przydział B w jednej transakcji bazy.

### FR-213: Auto-przydział — Must Have

WHEN principal uruchomi auto-przydział miesiąca  
THE SYSTEM SHALL dla widocznych (nieukrytych) kopert z celem, w kolejności grup i kopert, zwiększyć przydział o min(niedofinansowanie, pozostałe RTA), dopóki RTA > 0.

### FR-214: Widok miesiąca — Must Have

THE SYSTEM SHALL zwracać widok miesiąca: grupy → koperty (assigned, activity, carried_over, available, cel i jego status), sumy, RTA, przydziały przyszłe, przekroczenia z poprzedniego miesiąca, wpływy (przychody, bez koperty, przelewy), `incomplete` i liczbę oczekujących transakcji.

### FR-215: Historia koperty — Must Have

THE SYSTEM SHALL listować transakcje koperty w miesiącu (także te ze splitem) z portfeli on-budget widocznych dla principala, stronicowane kursorem.

### FR-220: Walidacja kopert w transakcjach — Must Have

WHEN transakcja, linia splitu, przelew lub domyślna koperta kontrahenta wskazuje kopertę  
THE SYSTEM SHALL wymagać, by koperta istniała w gospodarstwie (ukryta jest dozwolona), inaczej 422 na polu.

### FR-221: Domyślna koperta kontrahenta — Must Have

WHEN wydatek bez koperty i splitów ma kontrahenta z domyślną kopertą  
THE SYSTEM SHALL przypisać tę kopertę (zachowanie z 002), o ile koperta istnieje.

### FR-230: Autoryzacja — Must Have

THE SYSTEM SHALL wymagać `budget:read` do odczytu budżetu i `budget:write` do zmian (viewer tylko czyta; bot = scope ∩ rola twórcy). Sumy budżetu SHALL obejmować wszystkie portfele on-budget gospodarstwa (także prywatne innych osób i spoza listy bota), a lista transakcji koperty SHALL wymagać też `transactions:read` i pokazywać tylko widoczne portfele.

### FR-231: Audyt — Must Have

THE SYSTEM SHALL zapisywać audyt każdej zmiany grup, kopert i przydziałów w tej samej transakcji bazy.

### FR-232: Usunięcie gospodarstwa — Must Have

WHEN gospodarstwo jest usuwane  
THE SYSTEM SHALL usunąć jego grupy, koperty i przydziały.

## Non-Functional Requirements

- **NFR-020:** Zero `float64` w obliczeniach budżetu; procent postępu jako liczba całkowita; sumy z kontrolą przepełnienia.
- **NFR-021:** Agregacja aktywności w SQL (suma per dzień × koperta × rodzaj), nie przez stronicowanie transakcji.
