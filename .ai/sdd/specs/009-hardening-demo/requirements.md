# Feature: Utwardzenie powiadomień (SSRF) i dane demo (F09, etap 9)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: zgłoszenie automatycznego przeglądu bezpieczeństwa (HIGH, SSRF w `notifyhttp`), brief orkiestratora etapu 009, `005-notifications/design.md`, `PLAN.md` (tryb autonomiczny: „rano chciałbym mieć możliwość sprawdzenia lokalnie aplikacji”)
> Last Updated: 2026-10-21

## Overview

Dwie niezależne rzeczy przed porannym przeglądem: (1) kanały powiadomień (webhook, ntfy, Matrix, Telegram) nie mogą służyć do wysyłania żądań z serwera do jego sieci wewnętrznej ani do odczytywania odpowiedzi stamtąd; (2) jedno polecenie (`task demo`) stawia lokalnie aplikację z realistycznymi danymi, żeby dało się ją obejrzeć bez ręcznego wprowadzania.

## Business Context

- **Problem:** Każdy użytkownik instancji może ustawić URL kanału na `http://169.254.169.254/…` albo `http://127.0.0.1:…` i z wyniku „Wyślij test” (fragment ciała odpowiedzi) czytać usługi wewnętrzne. Pusta instancja nie pokazuje, jak aplikacja działa.
- **Users:** administrator instancji (konfiguracja), użytkownicy kanałów, właściciel projektu oglądający demo.
- **Success signal:** test dialera odrzuca loopback bez zgody i przepuszcza ze zgodą; `task demo` → logowanie `demo@denar.local` → pulpit, budżet, subskrypcje i MCP z danymi; `task verify` zielony.

## Requirements (EARS)

### SSRF

- **FR-901** — WHEN kanał wysyła żądanie, THE serwer SHALL sprawdzić każdy adres, z którym się łączy (po rozwiązaniu DNS), i odmówić połączenia z loopback, RFC 1918, link-local (w tym 169.254.169.254), CGNAT 100.64/10, ULA fc00::/7, adresami nieokreślonymi, multicast, zarezerwowanymi i IPv4 w IPv6 (mapped, NAT64).
- **FR-902** — WHERE administrator ustawił `DENAR_NOTIFY_ALLOW_PRIVATE=true`, THE serwer SHALL dopuścić sieci prywatne i loopback (bez link-local); WHERE ustawił `DENAR_NOTIFY_ALLOWED_CIDRS`, THE serwer SHALL dopuścić adresy z tych sieci (także link-local).
- **FR-903** — THE klient powiadomień SHALL ignorować zmienne proxy środowiska i nie śledzić przekierowań.
- **FR-904** — WHEN użytkownik tworzy lub zmienia kanał z URL-em, którego host to literalny zakazany adres IP albo `localhost`/`*.localhost`, THE serwer SHALL odpowiedzieć 422 z komunikatem „adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE”, chyba że zgoda z FR-902 obejmuje ten adres.
- **FR-905** — WHEN cel odpowiada statusem spoza 2xx, THE błąd widoczny dla użytkownika (`last_error`, wynik testu) SHALL zawierać tylko `HTTP <kod>`; fragment ciała SHALL trafić wyłącznie do logu serwera (host bez ścieżki).
- **FR-906** — WHEN połączenie zostało odrzucone przez FR-901, THE błąd SHALL brzmieć jak w FR-904.

### Demo

- **FR-910** — THE polecenie `denar demo` SHALL zmigrować bazę i utworzyć przez use case'y konto `demo@denar.local` (hasło z `--password`, `DENAR_DEMO_PASSWORD` albo stałe `denar-demo-2026`) i gospodarstwo „Dom (demo)” (PLN) z portfelami, kursami ręcznymi, kopertami z celami, kontrahentami z wzorcami, ~3 miesiącami transakcji, przydziałami (jedna koperta przekroczona w bieżącym miesiącu), subskrypcjami (w tym w USD, trial, anulowana, zmiana ceny, przegapiona płatność, płatności do dopasowania), botem i powiadomieniami z jobów.
- **FR-911** — IF gospodarstwo demo istnieje, THE polecenie SHALL odmówić; WHERE podano `--reset`, SHALL je usunąć i utworzyć od nowa.
- **FR-912** — THE polecenie SHALL wypisać URL, login, hasło, token bota i polecenie `claude mcp add`.
- **FR-913** — THE `task demo` SHALL zbudować binarium, wygenerować `.dev/master.key` (gdy brak), zaseedować `.dev/demo.db` z `--reset` i uruchomić serwer na :8080.

## Non-Goals

- Ochrona SSRF dla adresów konfigurowanych przez administratora (NBP, Telegram API, OIDC) poza tym, że Telegram korzysta z tego samego klienta.
- Allowlista domen; ochrona przed TOCTOU między walidacją a wysyłką poza kontrolą w dialerze (dialer ją zapewnia).
- Zmiana hasła istniejącego konta demo.
