# Product Steering

> Status: Draft
> Last Updated: 2026-10-21

## Product Vision

`denar` to self-hosted aplikacja do zarządzania wydatkami i budżetem domowym. Główny nacisk: **subskrypcje** (koszty cykliczne, odnowienia, zmiany cen), **wiele portfeli** w wielu walutach i **serwer MCP**, przez który asystenci AI analizują finanse i wprowadzają dane. Dostęp do danych jest kontrolowany osobno dla ludzi i botów. Architektura jest gotowa na późniejsze integracje z API banków i dostawców usług (OpenAI, Apple…).

## Problem We Solve

- Subskrypcje „rozpływają się”: różne waluty, cykle, triale i podwyżki cen sprawiają, że trudno wiedzieć, ile naprawdę kosztują i co się odnawia.
- Popularne aplikacje budżetowe są chmurowe i nie dają kontroli nad danymi ani dostępu dla własnych automatyzacji/agentów.
- Brak bezpiecznego, granularnego dostępu dla botów (MCP/API) — zwykle albo pełny dostęp, albo żaden.

## Target Users / Personas

### Członek gospodarstwa (owner / admin / member / viewer)

- **Who:** Osoba logująca się hasłem lub przez OIDC (np. Pocket ID).
- **Needs:** Ewidencja wydatków w portfelach, budżet kopertowy, kontrola subskrypcji, przypomnienia.
- **Goals:** Wiedzieć, na co idą pieniądze, i nie przegapić odnowień.

### Bot / asystent AI

- **Who:** Klient MCP lub REST z tokenem `dnr_…` utworzonym przez członka gospodarstwa.
- **Needs:** Odczyt analiz i dodawanie danych w zakresie scope'ów i portfeli nadanych przez twórcę.
- **Goals:** Odpowiadać na pytania („ile wydaję na subskrypcje?”) i wprowadzać wydatki z rozmowy/paragonu.

## Value Proposition

Jedno binarium self-hosted: budżet kopertowy + subskrypcje jako byt pierwszej klasy + MCP i REST dla botów z uprawnieniami nie szerszymi niż ich twórcy.

## Core Use Cases

| Use Case | Primary Persona | Value | Notes |
|----------|-----------------|-------|-------|
| Ewidencja transakcji w portfelach | Człowiek, bot | Pełny obraz wydatków | Wiele walut, przelewy, splity |
| Budżet kopertowy | Człowiek | Plan wydatków | YNAB-style, waluta bazowa gospodarstwa |
| Śledzenie subskrypcji | Człowiek | Kontrola kosztów cyklicznych | Cykle, historia cen, dopasowanie do transakcji |
| Przypomnienia | Człowiek | Brak niespodzianek | In-app, SMTP, webhook/ntfy, Telegram/Matrix |
| Analizy przez MCP | Bot | Pytania w języku naturalnym | Scope `reports:read` |

## Product Boundaries

### In Scope (MVP)

- Wiele gospodarstw na instancję, członkostwa z rolami, prywatne portfele.
- Portfele, transakcje, przelewy, splity, kontrahenci z regułami, wiele walut z kursami NBP.
- Budżet kopertowy, cele kopert.
- Subskrypcje: cykle, triale, historia cen, oczekiwane obciążenia, dopasowanie do transakcji.
- Powiadomienia: in-app, SMTP, webhook, ntfy, Telegram, Matrix.
- REST, MCP (remote HTTP), Web UI; boty z scope'ami i ograniczeniem portfeli; audit log.
- Import CSV oraz porty pod przyszłe konektory bankowe i billingowe.

### Out of Scope (MVP)

- Połączenia PSD2 z bankami, API OpenAI/Apple (przyszłe specyfikacje 010+).
- Aplikacja mobilna natywna.
- Inwestycje (wycena portfela papierów), podatki.
- SaaS multi-tenant.

## Success Metrics

| Metric | Target / Signal | Why It Matters |
|--------|-----------------|----------------|
| Bot nie widzi danych spoza swoich scope'ów i portfeli ani spoza uprawnień twórcy | Macierz testów autoryzacji zielona | Bezpieczeństwo danych |
| Suma kopert + Ready-to-Assign = saldo portfeli on-budget (w walucie bazowej) | Test inwariantu zielony | Poprawność budżetu |
| Każde odnowienie subskrypcji generuje przypomnienie w zadanym terminie | Test joba + e2e | Główna wartość produktu |
| Claude odpowiada przez MCP „co odnawia się w tym tygodniu?” | Ręczny e2e | Wartość MCP |

## Risks / Assumptions

| Risk or Assumption | Impact | Validation / Mitigation |
|--------------------|--------|-------------------------|
| Model kopert w wielu walutach komplikuje inwarianty | High | Koperty tylko w walucie bazowej; snapshot `BaseAmount` na transakcji |
| API NBP niedostępne / brak kursu w weekend | Medium | Cache + fallback na ostatni dzień roboczy |
| Integracje bankowe wymagają licencji/agregatora | Medium | Tylko porty w MVP; agregator w spec 010 |

## Domain Glossary

- **Household (gospodarstwo):** przestrzeń danych z walutą bazową i członkami.
- **Wallet (portfel):** konto/gotówka/karta z walutą i saldem.
- **Envelope (koperta):** kategoria budżetowa z przydziałami na miesiąc.
- **Ready-to-Assign:** pieniądze on-budget jeszcze nieprzydzielone do kopert.
- **Subscription:** cykliczny koszt z cyklem, ceną i statusem.
- **Charge:** oczekiwane obciążenie wynikające z subskrypcji.
- **Bot:** nie-ludzki principal z tokenem, scope'ami i listą portfeli.
- **Scope:** uprawnienie bota, np. `transactions:write`.

## Open Questions

- Brak poza założeniami w PLAN.md.
