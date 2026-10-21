# Feature: Web UI — część A (F07A, etap 7)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (sekcja WebUI), `api/openapi.yaml` etapu 001
> Last Updated: 2026-10-21

## Overview

Pierwsza część SPA Denara (`web/`, React 19 + Vite + TS), osadzonej w binarium. Obejmuje to, co da się zbudować na kontrakcie etapu 001: powłokę aplikacji, logowanie, zaproszenia, gospodarstwa, członków, boty i audit log, a także komponenty bazowe pod widoki finansowe. **Część B** (portfele, transakcje, budżet — po etapach 002–003) i **część C** (subskrypcje, powiadomienia, pulpit, MCP — po 004–006) dojdą jako kolejne przyrosty tej specyfikacji.

## Goals

- Działające SPA pod `/` serwowane przez `denar serve`, budowane i testowane w `task verify`.
- Pełna obsługa kont, gospodarstw, członków, zaproszeń, botów i audytu z UI.
- Komponenty bazowe (kwoty, listy, formularze, dialogi, toasty, puste stany, skeletony) gotowe dla części B/C.
- Dostępność (WCAG 2.2 AA w zakresie sprawdzanym przez axe), tryb jasny/ciemny, używalność na telefonie.

## Non-Goals

- Widoki portfeli, transakcji, budżetu, subskrypcji, powiadomień i pulpitu z danymi (części B/C); w części A są tylko dopracowane puste stany „wkrótce”.
- Zmiany backendu i kontraktu `api/openapi.yaml`.
- Zmiana własnego hasła, administracja kontami instancji (brak w API 001).

## Requirements (EARS)

### Powłoka i sesja

- **FR-A01** Aplikacja SHALL pokazywać nawigację: Pulpit, Portfele, Budżet, Subskrypcje, Powiadomienia, Ustawienia; na wąskim ekranie nawigacja SHALL otwierać się z przycisku menu.
- **FR-A02** WHEN użytkownik należy do więcej niż jednego gospodarstwa THE SYSTEM SHALL pozwalać przełączać gospodarstwo; wybór SHALL być częścią adresu (`/h/:householdId/…`) i zapamiętywany lokalnie.
- **FR-A03** Menu użytkownika SHALL pokazywać nazwę i e-mail, wybór motywu (systemowy/jasny/ciemny) i wylogowanie.
- **FR-A04** WHEN dowolne wywołanie API (poza logowaniem) zwróci 401 THE SYSTEM SHALL wyczyścić dane sesji i przejść na `/login?return_to=<bieżąca ścieżka>`.
- **FR-A05** Język interfejsu SHALL być polski; kwoty i daty formatowane przez `Intl` z locale `pl-PL`.

### Logowanie i zaproszenia

- **FR-A10** Ekran logowania SHALL pokazywać formularz e-mail/hasło, gdy `local_login`, i przycisk OIDC, gdy `oidc_enabled` (`GET /api/auth/login?return_to=…`).
- **FR-A11** WHEN logowanie się nie uda THE SYSTEM SHALL pokazać komunikat bez rozróżniania przyczyny; przy 429 — informację o czasie oczekiwania.
- **FR-A12** Strona `/invite#<token>` SHALL pobrać podgląd zaproszenia; dla nowego e-maila — formularz rejestracji (imię, hasło 12–128 znaków); dla istniejącego konta — logowanie z powrotem na zaproszenie; zalogowany SHALL móc przyjąć zaproszenie jednym przyciskiem. Token nie trafia do adresu wysyłanego do serwera.
- **FR-A13** Zaproszenia nieznane/odwołane, zużyte i wygasłe SHALL mieć osobne, zrozumiałe komunikaty.

### Gospodarstwa

- **FR-A20** WHEN użytkownik nie ma gospodarstw THE SYSTEM SHALL pokazać pusty stan z akcją utworzenia.
- **FR-A21** Formularz gospodarstwa SHALL mieć nazwę, walutę bazową (lista z `/api/currencies`), strefę czasową (domyślnie strefa przeglądarki) i dzień startu miesiąca 1–28.
- **FR-A22** Błędy walidacji problem+json (`errors[]`) SHALL być pokazywane przy właściwych polach; pozostałe — nad formularzem.

### Ustawienia gospodarstwa

- **FR-A30** Zakładka „Gospodarstwo”: edycja nazwy, strefy, dnia startu (owner/admin), waluta tylko do odczytu; usunięcie gospodarstwa (owner) z potwierdzeniem przez wpisanie nazwy.
- **FR-A31** Zakładka „Członkowie”: lista z rolami; zmiana roli i usunięcie zgodnie z uprawnieniami; opuszczenie gospodarstwa przez siebie.
- **FR-A32** Zakładka „Zaproszenia”: tworzenie (e-mail, rola), jednorazowe pokazanie linku z kopiowaniem, lista ze stanem, odwołanie oczekujących.
- **FR-A33** Zakładka „Boty”: tworzenie z wyborem scope'ów pogrupowanych według obszaru (odczyt/zapis), wygaśnięcie (domyślnie za 90 dni, można wyłączyć), jednorazowe pokazanie tokenu z kopiowaniem i ostrzeżeniem; lista z efektywnymi scope'ami, ostatnim użyciem i stanem; wyłączanie/włączanie, rotacja tokenu i usuwanie (z potwierdzeniem).
- **FR-A34** Zakładka „Dziennik”: wpisy audytu od najnowszych z filtrami (rodzaj aktora, typ zasobu) i stronicowaniem kursorem („Pokaż starsze”); czytelne nazwy akcji i aktorów.
- **FR-A35** Zakładki niedostępne dla roli SHALL być ukryte (viewer i member: Gospodarstwo w trybie odczytu, Członkowie, Boty — member tylko własne, viewer bez botów; Zaproszenia i Dziennik tylko owner/admin).

### Komponenty bazowe i dostępność

- **FR-A40** Komponenty: `Money`, `DataList` (stronicowanie „pokaż więcej”), pola formularza z błędami, `Dialog`/`ConfirmDialog`, toasty, `EmptyState`, `Skeleton`, `QueryState`.
- **FR-A41** `Money` SHALL formatować kwotę z decimal stringa bez konwersji na liczbę zmiennoprzecinkową, cyframi tabelarycznymi, ze znakiem i kolorem dla wpływu/wydatku.
- **NFR-A01** Każdy widok obsługiwalny klawiaturą, widoczny focus, link „Przejdź do treści”, semantyczne nagłówki i landmarki; e2e sprawdza axe (bez naruszeń serious/critical).
- **NFR-A02** Zgodność z CSP serwera (`script-src 'self'`, `style-src 'self'`, `font-src 'self'`): bez skryptów i elementów `<style>` inline, fonty z pakietów npm.
- **NFR-A03** Testy: Vitest (komponenty, hooki; MSW z walidacją odpowiedzi względem `api/openapi.yaml`), Playwright e2e na zbudowanym binarium.
