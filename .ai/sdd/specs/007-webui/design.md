# Design: Web UI — część A (007-webui)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md` (część A), `api/openapi.yaml` (etap 001), wzorzec `../para-board/web` (Straylight)
> Last Updated: 2026-10-21

Część A buduje SPA na kontrakcie etapu 001. **Części B** (portfele, transakcje, budżet — po 002/003) i **C** (subskrypcje, powiadomienia, pulpit z danymi — po 004–006) dopiszą tu własne sekcje i zadania; ten dokument opisuje fundament, na którym mają stanąć.

## 1. Stack i build

- React 19, Vite 8 (rolldown), TypeScript 5.9 strict, `react-router` 8 (data router), `@tanstack/react-query` 5, `openapi-fetch` + typy `openapi-typescript` w `src/api/schema.d.ts` (generowane, commitowane, sprawdzane `task web:types:check`).
- Testy: Vitest 5 + jsdom + Testing Library + MSW 3; każda odpowiedź mocka walidowana względem `api/openapi.yaml` (ajv 2020, kopia `openapiValidator.ts` ze Straylight). E2E: Playwright 1.64 + `@axe-core/playwright` na zbudowanym binarium.
- Wersje bibliotek skopiowane z `para-board/web/package.json` (znany, działający zestaw); doszły tylko fonty `@fontsource-variable/public-sans` i `@fontsource-variable/bricolage-grotesque`.
- `vite.config.ts`: proxy `/api` → `http://localhost:8080`, plugin odtwarzający `dist/.gitkeep` po buildzie (wymaga go `web/embed.go`).
- `web/Taskfile.yml` dołączony w root jako przestrzeń `web:` (`install`, `types`, `types:check`, `build`, `test` = eslint + tsc + vitest, `e2e`, `e2e:install`, `dev`). Root `verify` woła `web:build` i `web:test`; root `build` zależy od `web:build`. `postinstall` tworzy `node_modules/go.mod`, żeby `go test ./...` pomijał `node_modules`.
- CSP serwera (`internal/adapter/spa`) zabrania skryptów inline i elementów `<style>`: motyw ustawia przed pierwszym malowaniem zewnętrzny `public/theme-init.js`, fonty są w paczce.

## 2. Struktura `src/`

```
main.tsx                    punkt wejścia: style + <App/>
app/                        App, router.tsx (wszystkie trasy), providers.tsx, RequireSession,
                            AppShell (nawigacja, przełącznik gospodarstwa, menu użytkownika),
                            HouseholdRoute (kontekst gospodarstwa z adresu), ThemeProvider, NotFoundPage
api/                        client.ts (openapi-fetch + middleware błędów/401), errors.ts (ApiError,
                            NetworkError, classify, fieldErrors), queryClient.ts, keys.ts, schema.d.ts,
                            hooks/<obszar>.ts — useQuery/useMutation per zasób
components/<Nazwa>/         komponenty bazowe: Nazwa.tsx + Nazwa.module.css (+ test)
features/<obszar>/          strony i ich części: auth, households, settings, placeholders
lib/                        money.ts, format.ts (daty), storage.ts, returnTo.ts, labels.ts, scopes.ts
styles/                     fonts.css, tokens.css (kolory, typografia, odstępy), global.css
test/                       setup.ts, render.tsx, dialogShim.ts, openapiValidator.ts, msw/{server,handlers,fixtures}.ts
```

### Konwencje dla części B/C

- **Nowa strona:** komponent w `features/<obszar>/<Nazwa>Page.tsx`, trasa w `app/router.tsx` pod `/h/:householdId/<ścieżka>`, pozycja menu w `app/nav.ts`. Strona dostaje gospodarstwo z `useHousehold()` (`{household, role}`), tytuł przez `<PageHeader title=…/>` (ustawia też `document.title`).
- **Zapytania:** jeden plik `api/hooks/<zasób>.ts` z `use<Zasób>()`/`use<Akcja>()`; klucze wyłącznie z `api/keys.ts` (prefiks `['household', hid, …]`, więc unieważnienie gospodarstwa czyści wszystko pod nim). Wywołania przez `api.GET/POST…` + `unwrap()`; klient rzuca `ApiError`/`NetworkError`, nigdy nie zwraca `error`.
- **Mutacje:** `onSuccess` unieważnia/aktualizuje klucze; komunikat sukcesu przez `useToast().show({message})`; błąd 422 do formularza przez `fieldErrors(error)`, reszta przez `<FormAlert error/>` lub toast.
- **Kwoty:** zawsze `<Money amount="12.34" currency="PLN"/>` lub `formatMoney()`; nigdy `Number(amount)` ani arytmetyka na floatach. Sumy liczy serwer; gdyby UI musiało coś zsumować — `lib/money.ts` ma `addDecimal` na `bigint` jednostek minimalnych.
- **Listy:** `<DataList>` (stronicowanie „Pokaż więcej” na `useInfiniteQuery` z `next_cursor`); długie rejestry transakcji w części B mogą dodać wirtualizację wewnątrz `DataList` bez zmiany jego API.

## 3. Projekt wizualny („Mennica”)

Denar to historyczna srebrna moneta — kierunek wizualny: chłodne srebro i patyna, spokojny „skarbiec” zamiast krzykliwej fintechowej estetyki.

| Token | Jasny | Ciemny | Rola |
|---|---|---|---|
| `--paper` | `#EEF2F0` | `#121C1B` | tło strony (chłodna szałwia, nie krem) |
| `--surface` | `#F9FBFA` | `#1A2725` | panele, formularze |
| `--ink` | `#16302B` | `#E3EBE8` | tekst |
| `--muted` | `#506360` | `#9DB0AB` | tekst drugorzędny (≥4.5:1) |
| `--patina` | `#23695B` | `#72C4B0` | marka, akcje główne, focus |
| `--inflow` | `#1D6A4C` | `#7CD3A6` | wpływy |
| `--outflow` | `#9E2F40` | `#F3949F` | wydatki |
| `--rail` | `#16302B` | `#0D1615` | pasek nawigacji (tło) |

- **Typografia:** Bricolage Grotesque (variable, osie `wdth`/`opsz`) — nagłówki i duże kwoty, zwężona i ciasna; Public Sans (variable) — tekst i tabele, `font-variant-numeric: tabular-nums` w każdym miejscu z liczbami. Skala 1.25 (0.8 / 1 / 1.25 / 1.563 / 1.953 / 2.441 rem), interlinia tekstu 1.5, nagłówków 1.15.
- **Układ:** desktop — stały ciemny pasek „rail” po lewej (znak monety z rantem, przełącznik gospodarstwa, nawigacja, menu użytkownika na dole), treść wyrównana do lewej, maks. 72rem. Telefon (<48rem) — górny pasek z przyciskiem menu; nawigacja jako panel wysuwany (natywny `<dialog>`), formularze w jednej kolumnie, tabele zamieniają się w listy kart.
- **Jedyny „głośny” element:** znak monety (SVG z rantem) i nazwa gospodarstwa w railu; reszta cicha: cienkie linie, jeden promień zaokrąglenia na kontrolki (6px), panele bez cieni, ruch tylko w reakcji na akcję (otwarcie dialogu, toast), `prefers-reduced-motion` respektowane.
- **Kwoty:** wydatek — znak minus (U+2212) i `--outflow`; wpływ — `+` i `--inflow`; kolor nigdy nie jest jedynym nośnikiem (znak zawsze widoczny).

## 4. Sesja, trasy, gospodarstwo

```
/login                         LoginPage (publiczna)
/invite#<token>                InvitePage (publiczna; token z fragmentu albo sessionStorage)
/  (RequireSession)
  index                        HomeRedirect: ostatnie gospodarstwo → /h/:id, brak → Welcome (pusty stan)
  households/new               CreateHouseholdPage
  h/:householdId (HouseholdRoute + AppShell)
    index                      DashboardPage (powitanie, fakty o gospodarstwie, następne kroki)
    wallets|budget|subscriptions|notifications   ComingSoonPage (część B/C)
    settings (SettingsLayout, zakładki)
      general|members|invitations|bots|audit
*                              NotFoundPage
```

- `useMe()` (`GET /me`, `staleTime: Infinity`) jest źródłem użytkownika i listy gospodarstw z rolami. `RequireSession` czeka na `/me`; 401 → `Navigate` na `/login?return_to=…`.
- 401 z dowolnego innego wywołania: middleware klienta woła handler zarejestrowany w `App` → `queryClient.clear()` i `router.navigate('/login?return_to=…')`. Wyjątki: `POST /auth/login` i `GET /me` na stronach publicznych.
- `return_to` przechodzi przez `safeReturnTo()` (tylko ścieżki względne zaczynające się od `/`, bez `//`, bez `/api`).
- Zaproszenie: token czytany z `location.hash`, zapisywany w `sessionStorage` (`denar:invite`), a fragment usuwany z paska adresu. Przejście do logowania używa `return_to=/invite` — token nigdy nie trafia do zapytania HTTP poza body `/invitations/*` (także przy OIDC, którego `return_to` serwer widzi).
- Ostatnio wybrane gospodarstwo w `localStorage` (`denar:household`); przełącznik zachowuje bieżącą sekcję (`/h/A/settings/bots` → `/h/B/settings/bots`).

## 5. Komponenty bazowe

| Komponent | Plik | Opis |
|---|---|---|
| `Money` | `components/Money` | `amount` (decimal string) + `currency`; `tone="auto"|"neutral"`, `signed`, `size`. Formatuje przez `Intl.NumberFormat('pl-PL', {style:'currency'})` na stringu (NumberFormat v3 — dokładny dziesiętny, bez floatu); niepoprawny string → `—` + `title`. |
| `formatMoney`, `parseDecimal`, `addDecimal` | `lib/money.ts` | czyste funkcje; `addDecimal` na `bigint` jednostek minimalnych |
| `Button`, `IconButton` | `components/Button` | warianty `primary|secondary|ghost|danger`, `size`, `busy` (aria-busy, blokada) |
| `Field`, `TextInput`, `SelectInput`, `Checkbox` | `components/Field` | etykieta, podpowiedź, błąd z `aria-describedby`/`aria-invalid` |
| `FormAlert` | `components/Form` | błąd ogólny formularza z `ApiError` (role=alert) |
| `fieldErrors(err)` | `api/errors.ts` | `errors[]` problem+json → `Record<field, message>` |
| `Dialog`, `ConfirmDialog` | `components/Dialog` | natywny `<dialog>` (focus trap, Escape, powrót focusu); `ConfirmDialog` z opcjonalnym wpisaniem frazy |
| `ToastProvider`, `useToast` | `components/Toast` | region `role=status`, błędy zostają do zamknięcia |
| `EmptyState` | `components/EmptyState` | ikona, tytuł, opis, akcja |
| `Skeleton` | `components/Skeleton` | bloki/wiersze ładowania (`aria-hidden`, kontener `aria-busy`) |
| `QueryState` | `components/QueryState` | ładowanie/błąd/ponów dla `UseQueryResult` |
| `DataList` | `components/DataList` | lista z pustym stanem i „Pokaż więcej” |
| `CopyField` | `components/CopyField` | wartość tylko do odczytu + kopiowanie (Clipboard API, fallback zaznaczenia) |
| `Badge` | `components/Badge` | stany (aktywny, wyłączony, oczekuje…) |
| `PageHeader` | `components/PageHeader` | `h1`, opis, akcje; ustawia `document.title` |
| `Icon`, `CoinMark` | `components/Icon` | zestaw ikon SVG (inline, `aria-hidden`) i znak marki |
| `SkipLink` | `components/SkipLink` | „Przejdź do treści” |

## 6. Ustawienia gospodarstwa — uprawnienia w UI

UI tylko ukrywa i blokuje to, czego serwer i tak odmówi (autoryzacja jest w use case'ach):

| Zakładka | owner | admin | member | viewer |
|---|---|---|---|---|
| Gospodarstwo | edycja + usunięcie | edycja | odczyt | odczyt |
| Członkowie | wszystkie role | bez ruszania ownerów i nadawania owner | odczyt + opuszczenie | odczyt + opuszczenie |
| Zaproszenia | tak (też owner) | tak (admin/member/viewer) | — | — |
| Boty | wszystkie | wszystkie | własne | — |
| Dziennik | tak | tak | — | — |

Scope'y botów w grupach (wiersz = obszar, kolumny „Odczyt”/„Zapis”): Portfele, Transakcje, Budżet, Subskrypcje (read+write), Raporty, Powiadomienia (tylko odczyt). Zaznaczenie zapisu zaznacza odczyt. Gotowe zestawy „Tylko odczyt” i „Pełny dostęp”. Ostrzeżenie, gdy twórca jest viewerem (efektywne tylko `:read`). Wygaśnięcie: domyślnie dziś + 90 dni (koniec dnia lokalnie → ISO UTC), checkbox „Bez wygaśnięcia”. Ograniczenie do portfeli — w części B (API portfeli z etapu 002).

Audit: `useInfiniteQuery` po `cursor`, filtry `actor_kind` i `resource_type` w kluczu zapytania; aktorzy rozwiązywani przez listy członków i botów (fallback: skrócone ID); akcje tłumaczone w `lib/labels.ts` (nieznane — surowy kod).

## 7. Przypadki brzegowe

- Brak gospodarstw → Welcome z akcją; `/h/:id` spoza `me.households` → strona „Nie znaleziono gospodarstwa” z linkiem do listy.
- Usunięcie gospodarstwa / opuszczenie → `invalidate(me)`, czyszczenie `denar:household`, przejście na `/`.
- Token bota i link zaproszenia trzymane tylko w stanie dialogu (nie w cache react-query, nie w storage); zamknięcie dialogu je gubi.
- 429 logowania → komunikat z `Retry-After`; 409 `duplicate-email` przy rejestracji → podpowiedź „zaloguj się i przyjmij”.
- Strefa czasowa: lista `Intl.supportedValuesOf('timeZone')` z domyślną strefą przeglądarki.

## 8. Strategia testów

- **Vitest** (MSW z walidacją kontraktu): `lib/money` (tabelarycznie: znaki, waluty 0/2/3 miejsc, duże liczby bez utraty precyzji), `Money`, `client` (401 → handler, wyjątki), `RequireSession`, `LoginPage` (sukces + return_to, 401, 429, OIDC), `InvitePage` (rejestracja, przyjęcie zalogowanym, wygasłe), `CreateHouseholdPage` (422 przy polu), `BotsTab` (grupy scope'ów, tworzenie → jednorazowy token, rotacja, usuwanie), `AuditTab` (filtry, „Pokaż starsze”), `MembersTab` (uprawnienia), `Dialog`/`ConfirmDialog`, `Toast`.
- **Playwright** (`web/e2e`, Chromium jasny/ciemny + telefon 390px): binarium `../denar serve` na bazie w katalogu tymczasowym, `DENAR_AUTO_MIGRATE=true`, konto startowe z env, gotowość `/healthz`. Scenariusz: logowanie → utworzenie gospodarstwa → zaproszenie (link) → rejestracja z zaproszenia w drugim kontekście → bot (token, wywołanie `/api/me` Bearerem) → dziennik z wpisami; axe na każdym ekranie (bez naruszeń serious/critical).

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-A01 | CSS Modules + tokeny CSS, bez biblioteki UI | Zgodność z CSP, mały bundel, pełna kontrola dostępności; jak Straylight |
| AD-A02 | Fonty Bricolage Grotesque + Public Sans z `@fontsource-variable` | CSP `font-src 'self'`; Public Sans ma cyfry tabelaryczne i pełne polskie znaki |
| AD-A03 | Gospodarstwo w adresie `/h/:householdId` | Linki działają między kartami; brak globalnego stanu „bieżącego gospodarstwa” |
| AD-A04 | Paginacja „Pokaż więcej” zamiast wirtualizacji w części A | Audit jest stronicowany kursorem; wirtualizację doda część B dla rejestru, gdy będzie potrzebna |
| AD-A05 | Formatowanie kwot przez `Intl.NumberFormat.format(string)` | NumberFormat v3 formatuje dziesiętny string dokładnie; brak floatu po drodze |
| AD-A06 | Token zaproszenia w `sessionStorage`, `return_to=/invite` | Token nie trafia do adresów wysyłanych do serwera (OIDC `return_to`) |
| AD-A07 | Brak systemu i18n — teksty po polsku w komponentach, etykiety domenowe w `lib/labels.ts` | Jeden język w MVP; prostota |
| AD-A08 | Bez wyboru portfeli przy tworzeniu bota | Brak API portfeli (etap 002); pole dojdzie w części B |
| AD-A09 | `task web:e2e` poza `task verify` | Wymaga przeglądarek Playwright; uruchamiane osobno (wynik w tasks.md) |
| AD-A10 | Root `Taskfile.yml`: zadania `web:*` z `web/Taskfile.yml` (include) zamiast warunkowych skryptów | `web/package.json` istnieje od tego etapu |
| AD-A11 | Wpisy `PLAN.md`/`INDEX.md` dopisuje orkiestrator przy scaleniu | Równolegle na `main` pracuje etap 002; worktree dotyka tylko `web/`, `Taskfile.yml` i tej specyfikacji |

## Ograniczenia

- Pulpit, Portfele, Budżet, Subskrypcje, Powiadomienia to puste stany do czasu części B/C.
- Brak zmiany hasła i profilu (brak API).
- E2E tylko w Chromium (jasny, ciemny, telefon).
