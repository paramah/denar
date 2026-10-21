# Feature: Subskrypcje (F04, etap 4)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (plan:approved), etap 4 `004-subscriptions`; brief orkiestratora; `003-envelope-budget/design.md` („Dla etapu 004”)
> Last Updated: 2026-10-21

## Overview

Subskrypcje jako byt pierwszej klasy: dostawca ze słownika, cena z historią (dowolna waluta), cykl z poprawną arytmetyką kalendarza, triale, pauza i anulowanie, portfel płatności i koperta. Z subskrypcji powstają oczekiwane obciążenia (charges) do horyzontu; obciążenia dopasowuje się do transakcji (sugestie z punktacją, potwierdzenie, zaksięgowanie, pominięcie, tryb automatyczny dla importu), a nieopłacone w oknie oznacza jako przegapione. Podsumowania kosztów w walucie bazowej, nadchodzące odnowienia i zdarzenia domenowe w transakcyjnym outboxie pod powiadomienia etapu 005.

## Business Context

- **Problem:** Subskrypcje w różnych walutach, cyklach i z podwyżkami cen „rozpływają się” — nie wiadomo, ile kosztują i co się odnawia.
- **Users:** Członkowie gospodarstwa (owner/admin/member — zapis, viewer — odczyt), boty z `subscriptions:read|write` (asystent AI pytający „co odnawia się w tym tygodniu?”).
- **Value:** Pełny obraz kosztów cyklicznych, przypomnienia przed odnowieniem, wykrywanie przegapionych i zmienionych płatności.
- **Success signal:** Testy tabelaryczne `NextDue` (koniec miesiąca, 29.02, co N), `MatchScore`, macierz autoryzacji, `portstest` na SQLite, konformancja REST i e2e (OpenAI USD z anchor 31 → obciążenia → transakcja → sugestia → potwierdzenie → zmiana ceny → summary w PLN → trial → missed) zielone.

## Goals

- Domena: cykl, `NextDue`, plan obciążeń, cena na dzień, normalizacja kosztu (miesięcznie/rocznie), `MatchScore`, typy zdarzeń.
- Use case'y: CRUD subskrypcji, historia cen, obciążenia (lista, sugestie, potwierdzenie, odłączenie, zaksięgowanie, pominięcie, auto-dopasowanie), podsumowanie, nadchodzące odnowienia, słownik dostawców; joby: generowanie, przegapione, przypomnienia.
- Outbox zdarzeń (zapis w transakcji bazy) z idempotencją przypomnień.

## Non-Goals / Out of Scope

- Wysyłka powiadomień (dispatch, kanały, preferencje) — etap 005.
- Narzędzia MCP (006), UI (007), import CSV i konektory billingowe (008+; tu tylko use case auto-dopasowania i pole `external_ref`).
- Subskrypcje rozliczane w kilku walutach naraz, rabaty/kupony, proporcjonalne rozliczenia przy zmianie planu.

## User Stories

### US-030: Ewidencja subskrypcji
**As a** członek **I want to** zapisać subskrypcję (dostawca, cena w walucie dostawcy, cykl, trial, portfel, koperta, przypomnienia) **So that** wiem, co i kiedy płacę.

### US-031: Historia cen
**As a** członek **I want to** zapisać zmianę ceny od danego dnia **So that** widzę podwyżki, a przyszłe obciążenia mają nową kwotę.

### US-032: Kolejka dopasowań
**As a** członek **I want to** zobaczyć oczekiwane obciążenia z sugerowanymi transakcjami i potwierdzić, zaksięgować lub pominąć **So that** wiem, które płatności przeszły.

### US-033: Koszty i odnowienia
**As a** członek lub bot **I want to** zobaczyć miesięczny i roczny koszt subskrypcji w walucie bazowej, per dostawca i koperta, oraz nadchodzące odnowienia i kończące się triale **So that** kontroluję koszty cykliczne.

### US-034: Przypomnienia
**As a** członek **I want to** dostać przypomnienie N dni przed odnowieniem i końcem triala oraz informację o przegapionej płatności i zmianie ceny **So that** nie mam niespodzianek (dostarczenie — etap 005).

## Functional Requirements

### FR-300: Subskrypcja — Must Have
THE SYSTEM SHALL przechowywać subskrypcję: nazwa (1–100, unikalna w gospodarstwie bez wielkości liter), dostawca ze słownika (klucz), `external_ref` (≤ 200), cena z walutą (dowolna obsługiwana waluta, kwota > 0), cykl `{unit day|week|month|year, every 1–365, anchor}`, `trial_ends_at?`, status `trial|active|paused|cancelled` z `cancelled_at` i `ends_at`, portfel płatności?, koperta?, `reminder_offsets` (0–5 wartości 0–60 dni), reguła dopasowania `{payee_pattern ≤ 100, amount_tolerance_pct 0–50, date_window_days 0–15}`, notatka (≤ 1000), URL zarządzania (http/https, ≤ 500).

### FR-301: Słownik dostawców — Must Have
THE SYSTEM SHALL udostępniać zamknięty słownik dostawców z kluczem i nazwą wyświetlaną (openai, anthropic, apple, google, netflix, spotify, youtube, disney, hbo, amazon, microsoft, adobe, github, icloud, dropbox, other); nieznany klucz → 422.

### FR-302: Cykl i NextDue — Must Have
THE SYSTEM SHALL liczyć kolejne terminy od `anchor` jako `anchor + k·every·unit` (k = 0, 1, …) bez kumulowania dryfu; dla miesięcy i lat dzień wykraczający poza miesiąc przechodzi na jego ostatni dzień (31 → 30/28/29, 29.02 → 28.02 w latach nieprzestępnych), a kolejne terminy wracają do dnia z `anchor`.

### FR-303: Historia cen — Must Have
WHEN principal zmieni cenę od dnia D
THE SYSTEM SHALL dopisać punkt ceny `(effective_from = D, kwota, waluta)` (ten sam dzień nadpisuje punkt), zachować wcześniejsze, przeliczyć przyszłe oczekiwane obciążenia od min(D, dziś) i zapisać zdarzenie `subscription.price_changed`. Cena na dzień = ostatni punkt z `effective_from ≤ dzień` (przed pierwszym punktem — pierwszy punkt).

### FR-304: Generowanie obciążeń — Must Have
THE SYSTEM SHALL generować obciążenia `expected` dla terminów do `dziś + horyzont` (domyślnie 90 dni, `DENAR_CHARGE_HORIZON`) idempotentnie (jedno obciążenie na subskrypcję i termin), z kwotą i walutą ceny na dzień terminu: dla `active`; dla `trial` tylko od `trial_ends_at`; dla `cancelled` tylko przed `ends_at`; dla `paused` żadnych. Przy utworzeniu subskrypcji generowanie zaczyna się od dnia utworzenia minus okno dopasowania.

### FR-305: Regeneracja — Must Have
WHEN zmieni się cykl, cena, status, trial lub `ends_at`
THE SYSTEM SHALL przeliczyć obciążenia `expected` od dziś (cena: od min(D, dziś)): usunąć niepasujące do planu, poprawić kwoty, dodać brakujące terminy od dziś; obciążeń w innych statusach i przeszłych nie zmieniać.

### FR-306: Przegapione — Must Have
WHEN obciążenie `expected` nie zostało dopasowane w ciągu `date_window_days` od terminu (termin + okno < dziś w strefie gospodarstwa)
THE SYSTEM SHALL (job) oznaczyć je `missed`, zapisać audyt i zdarzenie `subscription.charge_missed`. Obciążenie `missed` można nadal dopasować, zaksięgować lub pominąć.

### FR-307: Punktacja dopasowania — Must Have
THE SYSTEM SHALL liczyć `MatchScore(charge, transakcja)` 0–100 (liczba całkowita): kwota w tolerancji (40), data w oknie (30), wzorzec kontrahenta (20), ten sam portfel (10); transakcja spoza tolerancji kwoty lub okna dat nie jest kandydatem. Kandydaci to wydatki (nie przelewy) z ujemną kwotą, niepowiązane z innym obciążeniem, z portfeli widocznych dla principala.

### FR-308: Sugestie — Must Have
THE SYSTEM SHALL zwracać dla obciążenia do N najlepszych transakcji, a dla transakcji pasujące obciążenia (oczekiwane i przegapione), malejąco po punktacji.

### FR-309: Potwierdzenie, odłączenie, zaksięgowanie, pominięcie — Must Have
THE SYSTEM SHALL: potwierdzić dopasowanie (obciążenie `expected|missed` + transakcja-kandydat → `matched`); odłączyć (`matched|posted` → `expected`, lub `missed`, gdy minęło okno; zaksięgowana transakcja zostaje); zaksięgować (tworzy wydatek `source=subscription` w portfelu subskrypcji lub wskazanym, z kopertą subskrypcji, kwotą w walucie portfela podaną jawnie lub przeliczoną kursem z dnia terminu, i wiąże go → `posted`); pominąć (`expected|missed` → `skipped`). Usunięcie powiązanej transakcji przywraca obciążenie do `expected`/`missed`.

### FR-310: Auto-dopasowanie — Must Have
THE SYSTEM SHALL udostępniać use case auto-dopasowania transakcji (dla importu, etap 008): wiąże obciążenie, gdy najlepsza punktacja ≥ 80 i jest jednoznaczna (jedyny kandydat ≥ progu po stronie transakcji i obciążenia). Transakcje zapisane ręcznie lub przez bota nie są dopasowywane automatycznie — sugestie daje osobny endpoint.

### FR-311: Normalizacja kosztów — Must Have
THE SYSTEM SHALL liczyć koszt miesięczny i roczny subskrypcji z ceny bieżącej: rok = cena × liczba terminów w roku (dzień: 365,25/every, tydzień: 365,25/(7·every), miesiąc: 12/every, rok: 1/every), miesiąc = rok / 12, zaokrąglenie half-even; w walucie bazowej po dzisiejszym kursie; brak kursu → subskrypcja pominięta w sumach i podsumowanie `incomplete` z listą walut.

### FR-312: Podsumowanie — Must Have
THE SYSTEM SHALL zwracać podsumowanie widocznych subskrypcji: sumy miesięczna/roczna (aktywne + triale) w walucie bazowej, liczby wg statusu, rozbicie per dostawca i per koperta, najbliższe odnowienia (30 dni), triale kończące się w 14 dni, zmiany cen z 90 dni.

### FR-313: Nadchodzące odnowienia — Must Have
THE SYSTEM SHALL zwracać oczekiwane obciążenia widocznych subskrypcji z terminem w `[dziś, dziś + days]` (`days` 1–365, domyślnie 7) z kwotą w walucie subskrypcji i szacunkiem w walucie bazowej.

### FR-314: Zdarzenia i outbox — Must Have
THE SYSTEM SHALL zapisywać zdarzenia `subscription.renewal_upcoming`, `subscription.trial_ending`, `subscription.price_changed`, `subscription.charge_missed` w outboxie w tej samej transakcji bazy co zmiana, z kluczem deduplikacji unikalnym w gospodarstwie.

### FR-315: Przypomnienia — Must Have
THE SYSTEM SHALL (job) dla każdego oczekiwanego obciążenia aktywnej subskrypcji z terminem ≥ dziś i dla końca triala zapisać zdarzenie przypomnienia raz na (subskrypcja, termin, offset): dla najmniejszego offsetu, którego dzień przypomnienia (termin − offset) już nadszedł.

### FR-320: Autoryzacja — Must Have
THE SYSTEM SHALL wymagać `subscriptions:read` do odczytu i `subscriptions:write` do zmian (viewer tylko czyta; bot = scope ∩ rola twórcy). Subskrypcja z portfelem jest widoczna tylko dla principali widzących ten portfel (dla bota — także z jego listy); bez portfela — dla całego gospodarstwa. Ukryta → 404. Sugestie wymagają też `transactions:read`, potwierdzenie — `transactions:read`, zaksięgowanie — `transactions:write`.

### FR-321: Audyt — Must Have
THE SYSTEM SHALL zapisywać audyt zmian subskrypcji, cen i statusów obciążeń (także przez job) w tej samej transakcji bazy.

### FR-322: Koperty — Must Have
WHEN koperta jest używana przez subskrypcję
THE SYSTEM SHALL odmówić jej usunięcia (409 `envelope_in_use`); koperta subskrypcji musi istnieć (ukryta dozwolona).

### FR-323: Usunięcie gospodarstwa — Must Have
WHEN gospodarstwo jest usuwane THE SYSTEM SHALL usunąć jego subskrypcje, ceny, obciążenia i zdarzenia.

### FR-330: Podpowiedź celu koperty — Should Have
THE SYSTEM SHOULD raportować miesięczny koszt subskrypcji per koperta (w walucie bazowej), by UI mógł zaproponować cel `monthly`.

## Non-Functional Requirements

- **NFR-030:** Zero `float64` na kwotach i w punktacji; przeliczenia `big.Rat` z half-even.
- **NFR-031:** Joby idempotentne (powtórzony przebieg nie tworzy duplikatów obciążeń ani zdarzeń).
- **NFR-032:** Kursy pobierane poza transakcją bazy (jak w 002).
