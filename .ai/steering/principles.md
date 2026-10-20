# Project Principles

> Status: Active
> Last Updated: 2026-10-21

## Purpose

Zasady dla PRD, SPEC, TASKS, EXEC i REVIEW w `denar`. Wynikają z planu uzgodnionego z użytkownikiem 2026-10-21.

## How to Use These Principles

- `MUST` — nienegocjowalne bez jawnej zgody użytkownika.
- `SHOULD` — silne domyślne; odstępstwo wymaga uzasadnienia.
- `MAY` — dozwolone opcje.
- Konflikt z zatwierdzonym artefaktem SDD: zatrzymaj się i zapytaj, które źródło zaktualizować.

## Principles

### P-001: Reguła zależności Clean Architecture

**Level:** MUST  
**Rule:** Zależności wskazują tylko do środka: `adapter → usecase → domain`. Porty należą do `usecase`; domena nie ma importów spoza stdlib.  
**Reason:** REST, MCP, joby i przyszłe konektory bankowe to wymienne adaptery na te same use case'y.  
**Applies to:** SPEC, TASKS, EXEC, REVIEW

### P-002: Poprawność pieniędzy

**Level:** MUST  
**Rule:** Kwoty to `int64` w jednostkach minimalnych z walutą; przeliczenia przez `math/big.Rat` z jawnym zaokrągleniem. Operacje na różnych walutach bez przeliczenia są błędem.  
**Reason:** Błędy zaokrągleń w finansach są niedopuszczalne.  
**Applies to:** SPEC, EXEC, REVIEW

### P-003: Najmniejsze uprawnienia dla botów

**Level:** MUST  
**Rule:** Efektywne uprawnienia bota = jego scope'y ∩ uprawnienia twórcy ∩ lista portfeli. Bot nie wykonuje operacji administracyjnych (członkowie, boty, kanały, połączenia).  
**Reason:** Boty i asystenci AI mają dostęp do wrażliwych danych finansowych.  
**Applies to:** PRD, SPEC, TASKS, EXEC, REVIEW

### P-004: Reguły biznesowe w domenie i use case'ach

**Level:** MUST  
**Rule:** Salda, koperty, cykle subskrypcji, dopasowania i autoryzacja egzekwowane w domenie/use case'ach, nie w adapterach.  
**Reason:** Spójność niezależnie od interfejsu (REST, MCP, import).  
**Applies to:** SPEC, TASKS, EXEC, REVIEW

### P-005: Kontrakt przed implementacją

**Level:** MUST  
**Rule:** `api/openapi.yaml`, `api/mcp-tools.md` i `usecase/ports.go` są źródłem typów i ustalane przed pracą równoległą.  
**Reason:** Web UI i backend mogą powstawać równolegle.  
**Applies to:** SPEC, TASKS, EXEC

### P-006: Audytowalność

**Level:** SHOULD  
**Rule:** Każda zmiana danych finansowych zapisuje wpis audytu (aktor, akcja, zasób, zmiana) w tej samej transakcji.  
**Reason:** Wiadomo, co zrobił bot, a co człowiek.  
**Applies to:** SPEC, EXEC

### P-007: Weryfikowalność

**Level:** SHOULD  
**Rule:** Każdy wymóg ma test lub jawny krok weryfikacji; zakończenie wymaga świeżych dowodów (`task verify`).  
**Applies to:** TASKS, EXEC, REVIEW

### P-008: Integracje za portami

**Level:** SHOULD  
**Rule:** Każde zewnętrzne źródło danych (bank, dostawca, CSV) przechodzi przez port i wspólny use case importu; domena nie zna dostawców.  
**Reason:** Przyszłe integracje bez przebudowy rdzenia.  
**Applies to:** SPEC, EXEC

## Decision Rules

1. Bezpieczeństwo i prywatność danych finansowych mają priorytet.
2. Zatwierdzone wymagania produktowe mają priorytet nad preferencjami implementacyjnymi.
3. Prostsze rozwiązanie, o ile spełnia zatwierdzone wymagania.
4. Gdy kompromis niejasny — pytaj użytkownika.

## Change Policy

- Zmiany `MUST` wymagają jawnej zgody użytkownika.
- Zmiany zasad nie przepisują po cichu zatwierdzonych artefaktów.
