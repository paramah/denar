# Feature: Interfejs i powiadomienia po polsku i angielsku (F13)

> Status: Approved (użytkownik, przez zatwierdzenie planu 2026-10-21)
> Source: prośba użytkownika „Dodaj obsługę wielu języków” + odpowiedzi z rundy pytań (zakres: UI + powiadomienia; język z profilu, domyślnie z przeglądarki)
> Last Updated: 2026-10-21

## Overview

Denar ma działać w dwóch językach: polskim (dotychczasowy, domyślny) i angielskim. Język dotyczy Web UI oraz treści wysyłanych do użytkownika (powiadomienia in-app, e-mail, kanały). API (problem+json), opisy narzędzi MCP i dane demo pozostają bez zmian.

## Requirements (EARS)

- **FR-1301** — THE system SHALL obsługiwać języki `pl` i `en`; użytkownik SHALL mieć preferencję `locale` (`pl`, `en` albo brak = „automatycznie”).
- **FR-1302** — WHEN zalogowany człowiek wysyła `PATCH /api/me {locale}`, THE serwer SHALL zapisać preferencję (z wpisem audytu) i zwrócić użytkownika; bot SHALL dostać 403; nieznana wartość SHALL dać 422.
- **FR-1303** — THE Web UI SHALL wybrać język: preferencja użytkownika → wybór zapisany w przeglądarce → `navigator.languages` → `pl`; i ustawić `<html lang>`.
- **FR-1304** — THE Web UI SHALL pozwolić zmienić język na ekranie logowania (zapis w przeglądarce) i na stronie profilu (zapis przez `PATCH /me`); zmiana SHALL działać bez przeładowania strony.
- **FR-1305** — THE Web UI SHALL formatować kwoty, daty, liczby, nazwy walut i odmianę liczebników według aktywnego języka.
- **FR-1306** — THE Web UI SHALL tłumaczyć komunikaty walidacji i błędy serwera na aktywny język (dotychczasowe reguły `messages.ts`).
- **FR-1307** — WHEN powstaje powiadomienie, THE serwer SHALL wyrenderować tytuł i treść osobno dla każdego odbiorcy w jego języku (brak preferencji → `pl`); powiadomienia już zapisane SHALL zostać w języku z chwili utworzenia.
- **FR-1308** — THE e-mail z zaproszeniem, stopka e-maila i wiadomość testowa kanału SHALL być w języku użytkownika, który wywołał akcję.
- **FR-1309** — THE polskie brzmienie istniejących tekstów SHALL pozostać bez zmian (istniejące testy po polskim tekście przechodzą bez modyfikacji).
- **FR-1310** — THE zbiory kluczy tłumaczeń `pl` i `en` SHALL być identyczne, z tymi samymi parametrami (test).

## Out of Scope

- Tłumaczenie problem+json, opisów MCP, danych demo i presetów CSV.
- Ponowne renderowanie zapisanych powiadomień po zmianie języka.
- Języki inne niż `pl` i `en` (struktura ma pozwalać je dodać).
