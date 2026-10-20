# Feature: Portfele i transakcje (F02, etap 2)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (plan:approved), etap 2 `002-wallets-transactions`; brief orkiestratora
> Last Updated: 2026-10-21

## Overview

Pierwsze dane finansowe Denara: kwoty (`Money`), kursy walut z NBP z cache i ręcznym nadpisaniem, portfele w wielu walutach (współdzielone i prywatne), transakcje z kwotą w walucie portfela i snapshotem w walucie bazowej, przelewy między portfelami, splity, kontrahenci z regułami oraz salda. Etap domyka punkty rozszerzenia z 001: walidację listy portfeli bota i filtr portfeli widocznych dla twórcy.

## Business Context

- **Problem:** Bez portfeli i transakcji nie ma budżetu (003), subskrypcji (004) ani analiz (006).
- **Users:** Członkowie gospodarstwa (owner/admin/member/viewer), boty.
- **Value:** Ewidencja wydatków w wielu walutach, której nie blokuje brak sieci.
- **Success signal:** Testy domeny, macierz autoryzacji, `portstest` na SQLite, konformancja REST i e2e (PLN + USD, kurs z fake NBP, przelew, saldo) zielone; `task verify` przechodzi.

## Goals

- Kwoty bez floatów: działania, parsowanie i formatowanie dziesiętne, przeliczenie kursem (`big.Rat`, zaokrąglenie bankierskie).
- Kursy NBP (tabela A) z cache, fallbackiem do 7 dni wstecz, kursami krzyżowymi przez PLN i ręcznym nadpisaniem.
- Portfele, transakcje, przelewy, splity, kontrahenci, salda, wyszukiwanie z kursorem.
- Autoryzacja: role, portfele prywatne, bot = scope ∩ twórca ∩ lista portfeli; audyt wszystkich zmian.

## Non-Goals / Out of Scope

- Koperty i ich walidacja (etap 003) — tu tylko kolumna `envelope_id` bez FK.
- Import CSV i konektory (008), dopasowania subskrypcji (004), narzędzia MCP (006), UI (007).
- Usuwanie portfeli (tylko archiwizacja), uzgadnianie (reconcile) jako proces — tylko status.

## User Stories

### US-010: Portfele

**As a** członek z rolą member+  
**I want to** zakładać portfele (gotówka, konto, karta, oszczędności, inwestycje, kredyt) w dowolnej obsługiwanej walucie z saldem otwarcia  
**So that** widzę wszystkie swoje pieniądze w jednym miejscu

**Acceptance Criteria:**
- [ ] Portfel ma typ, walutę (niezmienną), `on_budget`, widoczność `shared|private`, listę członków dla `private` (twórca zawsze), archiwizację.
- [ ] Lista portfeli pokazuje saldo bieżące w walucie portfela, saldo uzgodnione (cleared+reconciled) i saldo w walucie bazowej.
- [ ] Portfel prywatny nie istnieje (404) dla nieuprawnionych, także dla ownera i botów ich twórców.

### US-011: Transakcje

**As a** członek z rolą member+ lub bot z `transactions:write`  
**I want to** zapisywać wydatki i przychody z datą, kwotą, kontrahentem, kopertą, tagami, notatką i statusem  
**So that** mam pełną ewidencję

**Acceptance Criteria:**
- [ ] Kwota (±) w walucie portfela; snapshot `base_amount` w walucie bazowej po kursie z dnia transakcji.
- [ ] Gdy kurs jest niedostępny (brak sieci), transakcja zapisuje się z `rate_pending`, a job uzupełnia kwotę bazową później.
- [ ] Transakcja bota ma `source = bot`; `external_id` jest unikalny w portfelu.
- [ ] Split: linie (koperta, kwota), suma = kwota transakcji.
- [ ] Wyszukiwanie po portfelu, zakresie dat, kontrahencie, tekście, rodzaju, statusie, tagu (i kopercie) ze stronicowaniem kursorem.

### US-012: Przelewy

**As a** członek  
**I want to** przenieść pieniądze między portfelami, także w różnych walutach  
**So that** salda obu portfeli się zgadzają

**Acceptance Criteria:**
- [ ] Przelew to dwie transakcje `kind = transfer` z jednym `transfer_pair_id`.
- [ ] Przy różnych walutach podaję jawnie kwotę docelową.
- [ ] Usunięcie jednej nogi usuwa obie.

### US-013: Kontrahenci

**As a** członek  
**I want to** mieć listę kontrahentów, tworzoną automatycznie z nazwy, z regułami dopasowania i domyślną kopertą  
**So that** wprowadzanie jest szybkie, a import (008) przypisze kontrahenta sam

### US-014: Kursy walut

**As a** owner/admin  
**I want to** ręcznie ustawić kurs na dany dzień  
**So that** mogę pracować offline lub poprawić kurs

## Functional Requirements

### FR-100: Money — Must Have

THE SYSTEM SHALL reprezentować kwoty jako `int64` jednostek minimalnych z walutą ISO 4217, dodawać tylko kwoty tej samej waluty, parsować i formatować ciągi dziesiętne zgodnie z liczbą miejsc waluty (więcej miejsc → błąd walidacji) i przeliczać kursem `math/big.Rat` z zaokrągleniem bankierskim (half-even).

### FR-101: JSON kwot — Must Have

THE SYSTEM SHALL przesyłać kwoty w REST jako `{"amount":"12.34","currency":"PLN"}` (w odpowiedziach dodatkowo `minor`), nigdy jako liczby zmiennoprzecinkowe.

### FR-110: Kursy NBP — Must Have

WHEN potrzebny jest kurs waluty X do waluty bazowej na dzień D  
THE SYSTEM SHALL użyć kolejno: tożsamości (X = baza, bez sieci), ręcznego nadpisania gospodarstwa na dzień D, cache `fx_rates`, API NBP tabeli A (ostatni dzień roboczy w oknie 7 dni ≤ D); kurs krzyżowy liczyć przez PLN.

### FR-111: Brak kursu nie blokuje — Must Have

IF kurs jest niedostępny  
THE SYSTEM SHALL zapisać transakcję z `base_amount = null` i `rate_pending = true`, a job w tle SHALL uzupełniać brakujące kwoty bazowe.

### FR-112: Ręczny kurs — Must Have

WHEN owner/admin (człowiek) ustawi kurs waluty na dzień  
THE SYSTEM SHALL zapisać nadpisanie dla gospodarstwa, uzupełnić nim oczekujące transakcje tego dnia i waluty, i zapisać audyt.

### FR-120: Portfele — Must Have

THE SYSTEM SHALL zarządzać portfelami: nazwa (unikalna w gospodarstwie), typ `cash|checking|card|savings|investment|loan`, waluta (niezmienna), saldo otwarcia (jako transakcja otwarcia), `on_budget`, `visibility shared|private` z listą członków (twórca zawsze), archiwizacja.

### FR-121: Salda — Must Have

THE SYSTEM SHALL liczyć saldo portfela jako sumę jego transakcji — w walucie portfela (całość i cleared+reconciled) oraz w walucie bazowej (suma snapshotów, z flagą, gdy któraś czeka na kurs).

### FR-130: Transakcje — Must Have

THE SYSTEM SHALL zapisywać transakcje z polami: portfel, data (dzień), kwota ±, `base_amount`, kontrahent, `envelope_id` (opcjonalny), `kind expense|income|transfer`, tagi, notatka, status `pending|cleared|reconciled`, `source manual|import|subscription|bot`, `external_id` (unikalny w portfelu), splity; z odczytem, zmianą i usunięciem.

### FR-131: Splity — Must Have

WHEN transakcja ma linie splitu  
THE SYSTEM SHALL wymagać, by suma linii była równa kwocie, transakcja nie miała własnej koperty i była wydatkiem; kwotę bazową rozdzielić proporcjonalnie (metoda największej reszty) tak, by suma się zgadzała.

### FR-132: Przelewy — Must Have

WHEN principal utworzy przelew z portfela A do B  
THE SYSTEM SHALL zapisać nogę −kwota w A i +kwota docelowa w B (jawna przy różnych walutach) z jednym `transfer_pair_id`, w jednej transakcji bazy.

### FR-133: Wyszukiwanie — Must Have

THE SYSTEM SHALL wyszukiwać transakcje widocznych portfeli po portfelu, zakresie dat, kontrahencie, tekście (notatka, nazwa kontrahenta), rodzaju, statusie, tagu i kopercie, od najnowszych, ze stronicowaniem kursorem.

### FR-140: Kontrahenci — Must Have

THE SYSTEM SHALL zarządzać kontrahentami (nazwa unikalna, wzorce dopasowania, domyślna koperta) i tworzyć kontrahenta automatycznie, gdy transakcja poda nazwę, która nie pasuje do istniejącego (najpierw nazwa, potem wzorce).

### FR-150: Autoryzacja — Must Have

THE SYSTEM SHALL pozwalać czytać viewerowi i zapisywać member+; portfel prywatny pokazywać tylko jego członkom; bota ograniczać do scope'ów ∩ uprawnień twórcy ∩ listy portfeli; ukryte zasoby → 404. Lista portfeli bota SHALL być walidowana wobec portfeli widocznych dla twórcy.

### FR-151: Ponowny odczyt bota — Must Have

WHEN bot wykonuje operację  
THE SYSTEM SHALL ponownie odczytać bota w tej operacji (istnieje, aktywny, ten sam token).

### FR-160: Audyt — Must Have

THE SYSTEM SHALL zapisywać audyt każdej zmiany portfela, transakcji, przelewu, kontrahenta i kursu w tej samej transakcji bazy.

### FR-161: Usunięcie gospodarstwa — Must Have

WHEN gospodarstwo jest usuwane  
THE SYSTEM SHALL usunąć jego portfele, transakcje, splity, kontrahentów i nadpisania kursów.

## Non-Functional Requirements

- **NFR-010:** Zero `float64` w ścieżce kwot i kursów (także JSON NBP — `json.Number`).
- **NFR-011:** Wywołanie NBP z timeoutem (5 s), poza transakcją bazy.
- **NFR-012:** Testy NBP wyłącznie na `httptest.Server`.
