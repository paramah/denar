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

- **Nowa strona:** komponent w `features/<obszar>/<Nazwa>Page.tsx`, trasa w `app/router.tsx` pod `/h/:householdId/<ścieżka>` (zastępuje `ComingSoonPage`), pozycja menu w `app/nav.ts`. Strona dostaje gospodarstwo z `useHousehold()` (`app/household.tsx`: `{household, role}`, `canManage(role)`), zalogowanego z `useSessionUser()` (kontekst podawany przez `RequireSession`), tytuł przez `<PageHeader title=…/>` (ustawia też `document.title`).
- **Nawigacja po mutacji zmieniającej `/me`:** `mutateAsync().then(navigate)` albo zapis `/me` przez `setQueryData` przed przejściem — callback `mutate(…, {onSuccess})` nie zadziała, jeśli komponent zdąży się odmontować.
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
| AD-A12 | `useSessionUser()` czyta kontekst `RequireSession`, nie cache `/me` | Po wylogowaniu (czyszczenie cache) komponenty w trakcie renderu nie rzucają błędu |

## Ograniczenia

- Pulpit, Portfele, Budżet, Subskrypcje, Powiadomienia to puste stany do czasu części B/C.
- Brak zmiany hasła i profilu (brak API).
- E2E tylko w Chromium (jasny, ciemny, telefon).

---

# Design: część B — portfele, transakcje, przelewy, kontrahenci, kursy

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md` (część B), `api/openapi.yaml` (etap 002), `002-wallets-transactions/design.md`
> Last Updated: 2026-10-21

Część B nie zmienia backendu ani kontraktu. Stoi na fundamencie z części A (sekcje 1–8) i trzyma się jego konwencji; poniżej tylko to, co dochodzi.

## B1. Trasy i nawigacja

```
h/:householdId
  index                       DashboardPage (features/dashboard) — salda, portfele, ostatnie transakcje
  wallets                     WalletsPage — lista z sumami, „Pokaż zarchiwizowane”, nowy portfel
  wallets/:walletId           WalletPage — salda, akcje (transakcja, przelew, edycja, archiwizacja), rejestr portfela
  transactions                TransactionsPage — rejestr globalny
  settings/payees             PayeesTab — kontrahenci
  settings/rates              RatesTab — podgląd kursu, ręczne kursy
```

Menu główne dostaje pozycję „Transakcje” (po „Portfele”). Zakładki ustawień: „Kontrahenci” (wszyscy; zapis member+) i „Kursy walut” (wszyscy; ręczny kurs owner/admin) — przed „Dziennikiem zmian”.

## B2. Warstwa API

| Plik | Hooki | Klucze (`api/keys.ts`) |
|---|---|---|
| `api/hooks/wallets.ts` | `useWallets(hid, {includeArchived})`, `useWallet(hid, id)`, `useCreateWallet`, `useUpdateWallet` | `wallets(hid, {archived})`, `wallet(hid, id)` |
| `api/hooks/transactions.ts` | `useTransactions(hid, filters)` (infinite, `limit 50`), `useRecentTransactions(hid, n)`, `useCreateTransaction`, `useUpdateTransaction`, `useDeleteTransaction`, `useCreateTransfer` | `transactions(hid, filters)` |
| `api/hooks/payees.ts` | `usePayees`, `useCreatePayee`, `useUpdatePayee`, `useDeletePayee` | `payees(hid)` |
| `api/hooks/fx.ts` | `useFxQuote(hid, currency, date)`, `useFxOverrides`, `useSetFxOverride` | `fxQuote(hid, c, d)`, `fxOverrides(hid)` |

Unieważnienia: każda zmiana transakcji/przelewu unieważnia prefiksy `wallets`, `wallet`, `transactions`, `payees` (autotworzenie kontrahenta) i `audit`; zmiana portfela — `wallets`, `wallet`, `transactions` (nazwy w rejestrze), `audit`; ręczny kurs — `fxOverrides`, `fxQuote`, `wallets`, `transactions` (uzupełnione kwoty bazowe); kontrahent — `payees`, `transactions`. Klucze `wallets` i `transactions` mają wspólny prefiks bez filtrów, więc `invalidateQueries({queryKey: keys.transactions(hid)})` odświeża każdy filtr.

`TransactionFilters` = `wallet_id, from, to, payee_id, q, kind, status, tag` (+ `envelope_id` dla części C) — znormalizowane jak filtry audytu (puste pola pominięte).

## B3. Kwoty (`lib/money.ts`)

Dochodzą czyste funkcje (testy tabelaryczne):

- `parseAmountInput(text, decimals)` → `{ok, value}` | `{ok:false, error}`: usuwa spacje (też twarde), przecinek → kropka, wymaga liczby nieujemnej `\d+(\.\d+)?`, odrzuca więcej miejsc niż waluta; zwraca kanoniczny string z pełną liczbą miejsc (`"12,5"` PLN → `"12.50"`). Opcja `allowNegative` (saldo otwarcia).
- `negateDecimal(s)`, `absDecimal(s)` — na stringu, bez liczb.
- `sumAmounts(amounts, decimals)` = `addDecimal`; `compareMinor`.
- `decimalsOf(currency, currencies?)` — z listy `/currencies`, inaczej z `Intl`.

Sumy w UI (kafelki pulpitu, nagłówek portfeli, kontrola splitów) liczone z `amount` (string) przez `toMinor` → `bigint`, nigdy z `minor` (JSON-owa liczba traci precyzję powyżej 2^53). Kurs (`fx_rate`, ręczny kurs) zostaje stringiem; wyświetlany z przecinkiem (`formatRate`), wysyłany jako kanoniczny dziesiętny.

## B4. Formularz transakcji

`features/transactions/TransactionDialog.tsx` — jeden komponent na dodawanie i edycję.

- **Rodzaj** — segmentowany wybór (radio): „Wydatek” (`expense`, kwota ujemna), „Wpływ” (`income`, dodatnia), „Zwrot” (`expense`, dodatnia — zwrot do koperty, model YNAB z 002 AD-114). Przy edycji rodzaj wynika z `kind` i znaku.
- **Kwota** (`data-autofocus`, `inputmode="decimal"`) — wartość bez znaku; waluta portfela jako przyrostek; `parseAmountInput` z miejscami waluty portfela.
- **Kontrahent** — `<input list>` + `<datalist>` z `usePayees` (natywne podpowiedzi, działają z klawiatury i czytnikami). Wysyłamy `payee_id`, gdy nazwa dokładnie (bez wielkości liter) pasuje do istniejącego, inaczej `payee_name` (serwer dopasuje wzorce albo utworzy kontrahenta). Pusty przy edycji → `payee_id: null`.
- **Data** — `<input type=date>`, domyślnie `todayIn(household.timezone)` (Intl, `en-CA`).
- **Portfel** — wybór tylko w rejestrze globalnym i pulpicie (niezarchiwizowane); domyślnie ostatnio użyty (`localStorage denar:wallet:<hid>`), inaczej pierwszy. Przy edycji niezmienny (002 AD-107).
- **Status** (domyślnie „Rozliczona”), **Tagi** (po przecinku → tablica, serwer normalizuje), **Notatka**.
- **Koperta i splity** — tylko przy `features.envelopes` (§B5).
- Enter zatwierdza (formularz w `Dialog onSubmit`); checkbox „Dodaj kolejną” (tylko przy dodawaniu) po sukcesie czyści kwotę, kontrahenta, notatkę i splity i wraca fokusem do kwoty; toast „Zapisano wydatek …”.
- Noga przelewu: edycja tylko daty, statusu, tagów, notatki (pozostałe pola nie są renderowane; informacja „Kwoty przelewu zmienisz, usuwając go i tworząc ponownie”).
- Błędy: walidacja klienta (kwota wymagana i > 0, data) przy polach; serwerowe 422 przez `fieldErrors` (`amount`, `amount.amount`, `date`, `payee_name`, `payee_id`, `tags`, `note`, `splits`, `splits[i].amount`); reszta i 409 (`wallet-archived`) — `FormAlert`.
- Skrót `n`: `useHotkey('n', open)` w rejestrze — ignorowany, gdy fokus jest w polu edycji, z modyfikatorem lub przy otwartym dialogu.

## B5. Punkt rozszerzenia: koperty (część C)

- `lib/features.ts`: `features = { envelopes: false }` (obiekt modułu; testy przełączają go w `beforeEach`). Część C ustawia `true`, gdy etap 003 doda API kopert.
- `features/transactions/envelopes.ts`: `useEnvelopeOptions(hid): {value, label}[]` — dziś pusta lista; część C podmienia implementację na `GET /envelopes` (bez zmian w formularzach).
- `EnvelopeField` (select z opcją „Bez koperty”) — w formularzu transakcji (wydatek/zwrot bez splitów), w przyszłości w przelewie on↔off-budget i w domyślnej kopercie kontrahenta.
- `SplitEditor` — linie `{envelope_id, amount, note}`; przycisk „Podziel na koperty” zamienia kopertę transakcji na linie; suma kontrolna na `bigint`: „Rozdzielono X z Y — pozostało Z” (`aria-live=polite`), zapis zablokowany, gdy Z ≠ 0; znak linii = znak transakcji; pusta lista przy edycji wysyła `splits: []` (usuwa split). Splity tylko dla wydatku/zwrotu (002 FR-131).
- Filtr rejestru `envelope_id` jest w typie filtrów i kluczu, nie w UI.

## B6. Rejestr

`features/transactions/Register.tsx` (używany przez `WalletPage` i `TransactionsPage`):
- Filtry w `useSearchParams` (`from, to, payee, q, kind, status, tag, wallet`); pole tekstowe z opóźnieniem 300 ms (`useDebouncedValue`); przycisk „Wyczyść filtry”. Na telefonie widać tylko wyszukiwanie, resztę rozwija przycisk „Więcej filtrów (n)” (`aria-expanded`); na komputerze wszystkie filtry są widoczne.
- `useInfiniteQuery` → `DataList` (wariant `rows`), pusty stan zależny od tego, czy są filtry.
- Wiersz (`TransactionRow`): siatka `data | opis (kontrahent/przelew, notatka, tagi, portfel, źródło, czeka na kurs) | status | kwota (+ bazowa) | akcje (edytuj, usuń)`; na telefonie kolumny łamią się w kartę. Opis jest przyciskiem otwierającym edycję (member+).
- `StatusToggle`: przycisk z ikoną i etykietą „Oznacz jako rozliczoną/oczekującą”, `PATCH {status}`; `reconciled` — plakietka „Uzgodniona” bez przełącznika.
- `RatePendingBadge`: plakietka „Czeka na kurs” z `title` i ukrytym opisem; owner/admin dostaje link „Ustaw kurs” do `settings/rates?currency=…&date=…` (formularz kursu czyta te parametry).
- Viewer: bez akcji zapisu (przycisków nie ma).

## B7. Portfele

- `WalletsPage`: nagłówek z akcjami „Nowy portfel”, „Przelew”; kafelki „Na portfelach budżetowych” i „Wszystkie portfele” (suma `base_balance` aktywnych portfeli; „częściowa”, gdy któryś `rate_pending`); lista kart (`WalletCard`: nazwa-link, typ, plakietki, saldo, uzgodnione, bazowe); checkbox „Pokaż zarchiwizowane”; pusty stan z akcją.
- `WalletDialog` (tworzenie/edycja): typ, waluta (tylko tworzenie, domyślnie bazowa), saldo otwarcia + data (tylko tworzenie; może być ujemne), „Portfel w budżecie” (z wyjaśnieniem), widoczność (radio) i członkowie (checkboxy z `useMembers`; twórca zaznaczony i zablokowany) — widoczne przy tworzeniu (bot nie dotyczy UI) i przy edycji dla twórcy albo owner/admin przy portfelu współdzielonym. Archiwizacja: przycisk na stronie portfela z potwierdzeniem; przywrócenie bez potwierdzenia.
- `WalletPage`: `useWallet`; 404 → pusty stan „Nie znaleziono portfela”; nagłówek z saldami, akcje i `Register` z ustalonym `wallet_id`. Portfel zarchiwizowany — komunikat i brak akcji dodawania.
- `TransferDialog`: z/do (domyślnie bieżący portfel jako źródło), data, kwota w walucie źródła; gdy waluty różne — pole „Kwota docelowa (USD)” wymagane i podpowiedź; ta sama waluta — `to_amount` nie jest wysyłane. Walidacja: portfele różne, kwoty > 0.

## B8. Kontrahenci i kursy

- `PayeesTab`: lista (`DataList`) z nazwą i wzorcami jako plakietkami; dialog dodawania/edycji (nazwa; wzorce w `textarea`, jeden w linii, z podglądem jako plakietki); usuwanie z `ConfirmDialog`. Domyślna koperta kontrahenta — część C (`EnvelopeField` za flagą).
- `RatesTab`: sekcja „Sprawdź kurs” (waluta, dzień → `useFxQuote`; wynik „1 USD = 4,0123 zł · NBP, notowanie z 20 paź 2026” albo „Brak kursu — transakcje w tej walucie poczekają na kurs”); lista kursów ręcznych; formularz „Ustaw kurs” (owner/admin; waluta z `/currencies` bez bazowej, dzień, kurs — `parseRateInput` do 12 miejsc).

## B9. Boty — portfele

`WalletScopePicker` (radio „Wszystkie portfele, które widzisz” / „Tylko wybrane” + checkboxy aktywnych portfeli) w formularzu tworzenia i w dialogu „Portfele” przy bocie (`PATCH {wallet_ids}`; pusta lista = wszystkie). Wiersz bota pokazuje „Portfele: wszystkie widoczne dla twórcy” albo nazwy (nieznane id — „portfel niewidoczny dla Ciebie”, zostaje w wyborze). Wybór „Tylko wybrane” bez portfeli — błąd walidacji.

## B10. Pulpit

`features/dashboard/DashboardPage.tsx` zastępuje pulpit części A (powitanie i „następne kroki” zostają): kafelek „Na portfelach budżetowych” (suma bazowa, oznaczenie „część kwot czeka na kurs”), kafelek „Poza budżetem”, lista portfeli (do 6, link „Wszystkie portfele”), „Ostatnie transakcje” (5, `useRecentTransactions`) z linkiem do rejestru, przycisk „Dodaj transakcję” (member+, gdy są portfele). Bez portfeli — pusty stan „Dodaj pierwszy portfel”.

## B11. Strategia testów (część B)

- **Vitest**: `lib/money` (parseAmountInput, negate/abs, sumy dużych kwot bez utraty precyzji, parseRateInput), `WalletsPage` (sumy w walucie bazowej, archiwalne, tworzenie z saldem otwarcia i prywatnością, 409 `duplicate-name` przy nazwie), `TransactionDialog` (walidacja kwoty i miejsc, znak wg rodzaju, `payee_id` vs `payee_name`, 422 przy polu, 409 `wallet-archived`, „dodaj kolejną”, noga przelewu), `SplitEditor` z włączoną flagą (suma kontrolna, blokada zapisu, wysłane linie), `TransferDialog` (wielowalutowy — wymagana i wysłana kwota docelowa; ta sama waluta bez `to_amount`), `Register` (filtry w adresie i zapytaniu, „Pokaż więcej”, przełącznik statusu, `rate_pending`, usuwanie przelewu), `PayeesTab`, `RatesTab`, `BotsTab` (wybór portfeli), `DashboardPage`. Odpowiedzi MSW walidowane kontraktem jak w części A; fabryki w `test/msw/fixtures.ts` (`wallet`, `transaction`, `payee`, `fxQuote`, `money`).
- **Playwright** (`e2e/wallets.spec.ts`): logowanie → portfel „Konto” PLN z saldem otwarcia 1000 → portfel „Dolary” USD → wydatek 50 zł z nowym kontrahentem (skrót `n`, Enter) → przelew 400 zł → 100 USD → salda (550 zł; 100 USD ≈ 400 zł po kursie stubu 4,0; suma budżetowa 950 zł) → rejestr globalny i filtr → status → kontrahent utworzony z nazwy → pulpit z sumą; drugi test: gospodarstwo w EUR, portfel CHF (kurs krzyżowy przez PLN z atrapy), ręczny kurs w ustawieniach; axe na każdym ekranie i dialogu. `mobile.spec.ts`: rejestr, portfel, lista i pulpit bez poziomego przewijania na 390 px. Brak sieci: `DENAR_NBP_URL` wskazuje lokalny stub NBP (`e2e/support/nbp-stub.mjs`, drugi `webServer` w `playwright.config.ts`, stały kurs 4.0 dla każdej waluty, port wybierany jak port serwera). `a11y.spec.ts` dostaje nowe ekrany (portfel, transakcje, kontrahenci, kursy) w trzech projektach.

## Decyzje autonomiczne (część B)

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-B01 | Kontrahent przez `<input list>` + `<datalist>` zamiast własnego comboboxa | Natywna dostępność i obsługa klawiatury, mało kodu; autotworzenie robi serwer (`payee_name`) |
| AD-B02 | Rodzaj „Zwrot” = `expense` z dodatnią kwotą; kwotę wpisuje się bez znaku | Model 002 (znak niezależny od rodzaju); użytkownik nie myśli o minusach |
| AD-B03 | Koperty i splity za flagą `features.envelopes` (domyślnie wyłączoną), opcje kopert z `useEnvelopeOptions` (dziś puste) | API kopert powstaje w 003; część C włącza flagę i podmienia jeden hook |
| AD-B04 | Filtry rejestru w adresie strony (`useSearchParams`) | Linki i „wstecz” odtwarzają widok; jeden stan filtrów = jeden klucz zapytania |
| AD-B05 | Sumy w UI z `amount` (string) przez `bigint`, nie z `minor` | `minor` w JSON jest liczbą JS — powyżej 2^53 traci precyzję |
| AD-B06 | Kontrahenci i kursy jako zakładki ustawień, nie osobne pozycje menu | Rzadko używane; menu zostaje krótkie |
| AD-B07 | Szybki przełącznik statusu tylko pending ⇄ cleared; `reconciled` w edycji | Uzgodnienie to świadoma decyzja; pojedyncze kliknięcie nie powinno go cofać |
| AD-B08 | Bez wirtualizacji rejestru (strony po 50, „Pokaż więcej”) | Wystarcza dla gospodarstwa domowego; API `DataList` pozwala dodać później |
| AD-B09 | E2E: stub NBP jako drugi `webServer` Playwrighta (stały kurs 4.0) | Brak zależności od sieci; ten sam mechanizm startu i gotowości co serwer |
| AD-B10 | Ostatnio używany portfel w `localStorage` (`denar:wallet:<hid>`) | Szybkie dodawanie bez wybierania portfela za każdym razem |
| AD-B11 | Przełącznik „Pokaż zarchiwizowane” robi osobne zapytanie (`include_archived=true`) z własnym kluczem | Zgodnie z API; lista domyślna zostaje lekka |
| AD-B12 | Pulpit części A przeniesiony do `features/dashboard/` i rozbudowany; `ComingSoonPage` zostaje dla budżetu, subskrypcji i powiadomień | Część C dopisze kafelki budżetu i odnowień w tym samym miejscu |

## Punkty rozszerzenia dla części C

- **Budżet/koperty:** `lib/features.ts` (`envelopes: true`), `features/transactions/envelopes.ts` (`useEnvelopeOptions` → API kopert), `EnvelopeField` w `TransactionDialog`/`TransferDialog` (on↔off-budget)/`PayeeDialog` (domyślna koperta), `SplitEditor` gotowy; filtr `envelope_id` w `TransactionFilters`; pulpit — miejsce na kafelek „Do przydzielenia” (`DashboardPage`, sekcja `summary`).
- **Subskrypcje:** wiersz rejestru pokazuje już `source = subscription` (plakietka „Subskrypcja”); część C może dodać link do subskrypcji, gdy API zwróci jej id.
- **Powiadomienia:** brak zależności; plakietka „Czeka na kurs” i toasty to jedyne sygnały w części B.

## Ograniczenia (część B)

- Pole koperty, splity i domyślna koperta kontrahenta ukryte do części C.
- Nie da się zmienić kwot nogi przelewu (ograniczenie API — usuń i utwórz ponownie).
- Podpowiedź kwoty docelowej przelewu wielowalutowego nie jest liczona (brak kursu krzyżowego w API); użytkownik wpisuje kwotę z wyciągu.
- Lista członków portfela prywatnego pochodzi z `GET /members`; viewer też może być członkiem portfela.

---

# Design: część C1 — budżet kopertowy

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md` (część C1), `api/openapi.yaml` (etap 003), `003-envelope-budget/design.md`
> Last Updated: 2026-10-21

Bez zmian backendu i kontraktu. Konwencje z części A/B (sekcje 1–8, B1–B11) obowiązują.

## C1-1. Trasy

```
h/:householdId
  budget            BudgetPage — bieżący miesiąc budżetowy (currentBudgetMonth)
  budget/:month     BudgetPage — miesiąc YYYY-MM (isMonth; inaczej pusty stan „Nie ma takiego miesiąca”)
```

`BudgetPage` → `BudgetMonthView key={month}` (zmiana miesiąca czyści stan dialogów i pól). `ComingSoonPage` zostaje dla subskrypcji i powiadomień (C2).

## C1-2. Warstwa API (`api/hooks/budget.ts`)

| Hook | Endpoint | Klucz |
|---|---|---|
| `useBudgetMonth(hid, month)` | `GET budget/months/{month}` | `budgetMonth(hid, month)` |
| `useEnvelopeGroups`, `useEnvelopes` (zawsze `include_hidden=true`) | `GET budget/envelope-groups`, `GET budget/envelopes` | `envelopeGroups(hid)`, `envelopes(hid)` |
| `useEnvelopeActivity(hid, month, id)` (infinite, 50) | `GET …/envelopes/{id}/transactions` | `envelopeActivity(hid, month, id)` |
| `useSetAssignment`, `useMoveBetweenEnvelopes`, `useAutoAssign` | `PUT assignments/{id}`, `POST moves`, `POST auto-assign` | wynik → `setQueryData(budgetMonth)`; pozostałe miesiące i `audit` unieważnione |
| `use{Create,Update,Delete}Envelope{Group,}` | CRUD grup i kopert | unieważnia prefiks `budget(hid)` i `audit` (usunięcie koperty — też `payees`) |

Wszystkie klucze budżetu leżą pod `['household', hid, 'budget']`. Zmiany transakcji (`useTransactionsChanged`) i portfeli unieważniają ten prefiks (aktywność, RTA, `on_budget`). `reorderPatches(items, from, to)` — nowa kolejność jako pozycje = indeksy, tylko zmienione (pozycje z API mogą się powtarzać).

## C1-3. Miesiąc (`lib/budgetMonth.ts`)

`isMonth`, `shiftMonth(m, ±n)`, `monthOfDay(day, startDay)` (dzień < startDay → poprzedni miesiąc, 003 FR-200), `currentBudgetMonth(tz, startDay, now)` (przez `todayIn`), `formatMonth` („październik 2026”, `Intl` pl-PL). Testy tabelaryczne (przełom roku, start 10/28, strefa).

## C1-4. Komponenty (`features/budget/`)

| Plik | Zawartość |
|---|---|
| `BudgetPage.tsx` | nawigacja miesiąca (`nav` „Miesiąc budżetu”, linki ←/→, „Bieżący miesiąc”), `BudgetSummary`, pasek akcji (`role=toolbar`: Przenieś środki, Uzupełnij cele + suma niedofinansowania, Pokaż ukryte, Układ `aria-pressed`, Nowa grupa), `BudgetGrid`, `BudgetFlows`, stan dialogów (jeden `Modal` union), `AutoAssignSummary` |
| `BudgetSummary.tsx` | `ReadyToAssign` (rola `group` „Do przydzielenia”, rant monety w kolorze stanu), sumy miesiąca, uwagi (przekroczenia, przyszłe, `incomplete`), `BudgetFlows` |
| `BudgetGrid.tsx` | grupy (`section` + `h3`, sumy, akcje), wiersze (`article` „Koperta X”), `AssignedInput`, `GoalProgress` (`role=progressbar` z `aria-valuetext`) |
| `MoveDialog.tsx` | z/do (wartość `rta` = Do przydzielenia), kwota; RTA↔koperta przez `setAssignment(assigned ± kwota)` (`shiftAssigned`, bigint) |
| `StructureDialogs.tsx` | `GroupDialog`, `EnvelopeDialog` (nazwa, grupa, cel, ukrycie), `DeleteEnvelopeDialog`/`DeleteGroupDialog` z obsługą 409 |
| `EnvelopeDetails.tsx` | dialog szczegółów: kwoty, cel, akcje, `ActivityList` (DataList), link do rejestru |
| `BudgetTile.tsx` | kafelek pulpitu: `ReadyToAssign` + do 3 przekroczonych kopert |
| `goal.ts`, `model.ts` | `checkGoal`/`goalDraft`/`describeGoal`; `availableState`, `envelopesOf`, `overspent` (sort po `bigint`), `shiftAssigned` |

### Edycja przydziału w miejscu

Zawsze widoczne pole (`inputMode=decimal`, `aria-label="Przydzielono: <koperta>"`, `data-assign`) zamiast przełączania tekst/pole — mniej kliknięć i naturalny porządek tabulacji. Stan: `draft` (edycja, lustro w `ref`, bo `blur` po Tab przychodzi przed renderem), `saving` (wartość pokazywana w trakcie zapisu, `aria-busy`), `error` (`aria-invalid` + `aria-describedby`). `parseAmountInput(…, {allowNegative, allowZero})`; brak zmiany (porównanie `toMinor`) — bez żądania. Tab/Shift+Tab: zapis, potem fokus na następne/poprzednie pole `[data-assign]` w kolejności strony (ostatnie — domyślny Tab). Wyjście z pola z błędną kwotą przywraca zapisaną wartość.

### Kolejność

Tryb „Układ” pokazuje przyciski ↑/↓ przy grupach i kopertach (oraz usuwanie grup) i ukryte koperty. Klik → `reorderPatches` → kolejne `PATCH {position}`; po odświeżeniu fokus wraca na ten sam przycisk (`data-reorder="<id>:up|down"`, a gdy zablokowany — na przeciwny). Nowa koperta/grupa dostaje pozycję na końcu; przeniesienie koperty do innej grupy — koniec tej grupy.

### Stany kwot

`Dostępne` — pigułka: `negative` (tło `--outflow-soft`, obramowanie, ukryte „(przekroczono)”), `underfunded` (dodatnie, cel niepokryty — `--warn-soft`), `positive`, `zero`. Znak minus zawsze widoczny. RTA: `positive` (kwota `--inflow`), `zero`, `negative` (tło `--outflow-soft` i komunikat z kwotą do zmniejszenia).

### Telefon

Poniżej 48rem: podsumowanie w jednej kolumnie, nagłówek kolumn ukryty, wiersz koperty to karta 2×2 (`name | available`, `assigned | activity`) z widocznymi etykietami komórek (`.cellLabel` — na komputerze ukryte jak `visually-hidden`), sumy grupy ograniczone do „dostępne”.

## C1-5. Integracja z częścią B

- `lib/features.ts`: `envelopes: true`. `useEnvelopeOptions` czyta `useEnvelopes` + `useEnvelopeGroups` (`envelopeOptions()` — kolejność grup, „Grupa · Koperta”, „(ukryta)”); `useEnvelopeNames` — mapa id → nazwa (wiersz rejestru, kontrahenci).
- `TransactionDialog`: edycja wysyła `envelope_id`/`splits` tylko przy zmianie (`editedEnvelopeFields`).
- `TransferDialog`: pole „Koperta (wypływ z budżetu | wpływ do budżetu)”, gdy dokładnie jeden portfel jest `on_budget`; `envelope_id` tylko wtedy.
- `PayeesTab`: „Domyślna koperta” (create: tylko ustawiona; update: tylko zmiana, pusta = `null`), wiersz „Domyślna koperta: X”.
- `Register`: parametr `envelope` → `envelope_id`, select „Koperta”; `TransactionRow` z plakietką koperty (`title="Koperta"`).
- `DashboardPage`: `BudgetTile` pod sumami portfeli (gdy są portfele).

## C1-6. Strategia testów

- **Vitest**: `lib/budgetMonth` (tabelarycznie), `BudgetPage.test.tsx` (RTA i uwagi, stany kopert, ukryte; edycja w miejscu Enter/Esc/Tab/Shift; błędna kwota i 422; przeniesienia koperta↔koperta i RTA→koperta z sumą na bigint; walidacja przeniesienia; pokrycie przekroczenia z szczegółów; auto-przydział z podsumowaniem; walidacja celów i 409 `duplicate-name`; usunięcie celu `goal:null`; 409 `envelope-in-use` → ukrycie; 409 `group-not-empty`; kolejność; viewer; zły miesiąc; pusty budżet), `integration.test.tsx` (opcje kopert, koperta i splity w szybkim dodawaniu, przelew on→off z kopertą, domyślna koperta kontrahenta, filtr rejestru, kafelek pulpitu, transakcje koperty ze splitem). Domyślne handlery MSW dla budżetu (puste) — każdy test części A/B dalej działa z włączoną flagą.
- **Playwright** `e2e/budget.spec.ts` (na binarium): wpływ 3000 zł → grupy i koperty z UI (cel miesięczny) → przydziały w siatce z klawiatury (Enter, Tab) → wydatek z kopertą z szybkiego dodawania → przekroczenie widoczne w siatce i na pulpicie → następny miesiąc (koperta wyzerowana, RTA pomniejszone o przekroczenie, uwaga o przekroczeniach) → przeniesienie i auto-przydział → kontrola RTA; axe na budżecie, dialogach i szczegółach. `a11y.spec.ts`: ekran budżetu z danymi; `mobile.spec.ts`: budżet bez poziomego przewijania na 390 px.

## Decyzje autonomiczne (część C1)

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-C01 | Przeniesienie z/do „Do przydzielenia” przez `setAssignment` (przydział ± kwota), a nie osobny endpoint | API `moves` łączy tylko koperty; zmiana przydziału to dokładnie ruch z/do RTA (003 FR-204) |
| AD-C02 | Pole przydziału zawsze widoczne (nie tryb „kliknij, by edytować”) | Najmniej kroków, natywna tabulacja i czytniki; Tab między kopertami jak w arkuszu |
| AD-C03 | Wyjście z pola zapisuje zmianę (jak Tab); błędna kwota przy wyjściu — powrót do zapisanej | Zgodne z oczekiwaniem z arkuszy; nie zostawiamy „wiszących” szkiców |
| AD-C04 | Kolejność przyciskami ↑/↓ w trybie „Układ”, pozycje przenumerowane od 0 przy każdej zmianie | Dostępne z klawiatury, bez biblioteki drag & drop; odporne na powtarzające się pozycje |
| AD-C05 | Bieżący miesiąc liczony w UI (`todayIn` strefy gospodarstwa + `month_start_day`) | API nie ma endpointu „bieżący miesiąc”; ta sama reguła co 003 FR-200 |
| AD-C06 | Ukryte koperty poza siatką domyślnie, przełącznik w `localStorage` (`denar:budget:hidden`); w trybie „Układ” widoczne | Porządek siatki; ukrytą da się przywrócić bez szukania |
| AD-C07 | Opcje kopert zawsze z ukrytymi (z dopiskiem) | Edycja starej transakcji z ukrytą kopertą nie może jej zgubić (003 AD-305) |
| AD-C08 | Po operacji na przydziałach odpowiedź (`BudgetMonth`) trafia do cache; inne miesiące unieważnione | Bez dodatkowego żądania i migotania siatki; przeniesienia zmieniają kolejne miesiące |
| AD-C09 | 409 `group-not-empty` proponuje ukrycie kopert grupy (grupy nie da się ukryć w API) | „Propozycja ukrycia” z zadania; grupa z samymi ukrytymi kopertami znika z siatki |
| AD-C10 | Nowa data celu `target_by_date` nie może być w przeszłości (klient); stara data po terminie zostaje edytowalna | Serwer akceptuje cel po terminie (003: n = 1), ale nowy cel w przeszłości to pomyłka |
| AD-C11 | Pasek postępu celu jako `div role=progressbar` z szerokością przez CSSOM (`style` z Reacta) | CSP `style-src 'self'` blokuje atrybut `style` w HTML, ale nie właściwości ustawiane skryptem |

## Ograniczenia (część C1)

- Brak przeciągania myszą; brak przeniesienia transakcji z usuwanej koperty (409 → ukrycie).
- Brak historii przydziałów i podglądu „ile przydzielono w poprzednich miesiącach” (brak API; można policzyć z kolejnych `GET months`).
- Kafelek pulpitu pokazuje tylko bieżący miesiąc.

## Punkty rozszerzenia dla części C2

- **Subskrypcje (004):** `EnvelopeField` + `useEnvelopeOptions` gotowe dla formularza subskrypcji (koperta i portfel płatności); cel `monthly` koperty może podpowiadać sumę miesięcznych subskrypcji (`EnvelopeDialog` → pole kwoty). `DeleteEnvelopeDialog` już obsługuje 409 `envelope-in-use` (004 doda powód „użyta przez subskrypcję” — ten sam kod). Wiersz rejestru ma plakietkę źródła `subscription`.
- **Powiadomienia (005):** `envelope.overspent`/`goal_due` mogą linkować do `/h/:id/budget/:month` (szczegóły koperty: stan `{kind:'details', envelopeId}` w `BudgetBody` — do wystawienia przez parametr adresu, np. `?envelope=`).
- **MCP (006):** scope'y `budget:read|write` już są w `BotsTab`; UI nie potrzebuje zmian poza ewentualnym opisem narzędzi.
- Pulpit: miejsce na kafelek „Najbliższe odnowienia” obok `BudgetTile` (`DashboardPage`).

---

# Design: część C2 — subskrypcje

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md` (część C2), `api/openapi.yaml` (etap 004), `004-subscriptions/design.md`
> Last Updated: 2026-10-21

Bez zmian backendu i kontraktu. Konwencje z części A/B/C1 obowiązują.

## C2-1. Trasy

```
h/:householdId
  subscriptions (SubscriptionsLayout: nagłówek, „Nowa subskrypcja”, zakładki)
    index            OverviewPage — koszt, kalendarz 30 dni, rozbicie, triale, zmiany cen
    list             ListPage — filtry status/dostawca/portfel (?status=&vendor=&wallet=)
    charges          ChargesQueuePage — kolejka dopasowań
  subscriptions/:subscriptionId   SubscriptionPage — szczegóły, ceny, harmonogram
```

`SubscriptionsLayout` przekazuje zakładkom `{create, canWrite}` przez `useOutletContext` (pusty stan otwiera ten sam dialog). Pozycja menu „Subskrypcje” już istnieje (`NavLink` bez `end` podświetla też podstrony).

## C2-2. Warstwa API (`api/hooks/subscriptions.ts`)

| Hook | Endpoint | Klucz |
|---|---|---|
| `useVendors` | `GET /vendors` | `vendors()` (globalny, `staleTime: Infinity`) |
| `useSubscriptions(hid, filters)`, `useSubscription`, `useSubscriptionPrices` | `GET subscriptions`, `…/{id}`, `…/{id}/prices` | `subscriptions(hid, filters)`, `subscription`, `subscriptionPrices` |
| `useSubscriptionSummary` | `GET subscriptions/summary` | `subscriptionSummary` |
| `useCharges(hid, filters)` (infinite, 50) | `GET charges` (`subscription_id, status[], from, to`) | `charges(hid, filters)` |
| `useLinkedCharges(hid, from, to)` | `GET charges?status=matched&status=posted&from&to&limit=200` | `charges(…)` |
| `useChargeSuggestions`, `useTransactionChargeSuggestions` | `GET charges/{id}/suggestions`, `transactions/{id}/charge-suggestions` | `chargeSuggestions`, `transactionChargeSuggestions` |
| `use{Create,Update,Delete}Subscription`, `useChangeSubscriptionPrice` | CRUD, `POST …/prices` | unieważniają prefiks `subscriptionsAll(hid)` i `audit` (usunięcie — bez strony usuniętej subskrypcji) |
| `useMatchCharge`, `useUnmatchCharge`, `usePostCharge`, `useSkipCharge` | `POST charges/{id}/match|unmatch|post|skip` | `useTransactionsChanged` (rejestry, salda, budżet, subskrypcje, audyt) |

Wszystkie klucze subskrypcji i obciążeń leżą pod `['household', hid, 'subscriptions']`. `useTransactionsChanged` unieważnia też ten prefiks (usunięcie transakcji zwalnia obciążenie, nowa transakcja zmienia sugestie).

## C2-3. Czyste funkcje (`lib/subscriptions.ts`, `lib/money.ts`)

- `cycleLabel(cycle)` — „codziennie”, „co 3 dni”, „co tydzień, we wtorek”, „co 2 tygodnie, w środę”, „co miesiąc, 31.”, „co 5 miesięcy, 15.”, „co rok, 14 lutego”, „co 2 lata, …” (`plural()` — polskie formy 1 / 2–4 / 5+, z wyjątkiem 12–14); `cycleNote` — uwaga o dniu > 28 i 29.02.
- `occurrence(cycle, k)`, `nextDues(cycle, from, n)` — terminy zawsze od `anchor` z obcięciem do końca miesiąca (jak `domain/subscription.Cycle`): podgląd w formularzu.
- `daysBetween`, `addDaysTo`, `relativeDay` („dziś”, „jutro”, „za 5 dni”, „3 dni temu”) na datach kalendarzowych UTC (bez przesunięć strefy).
- `monogram(name)` („OpenAI” → „OA”, „HBO Max” → „HM”, „iCloud” → „iC”), `vendorMetal(key)` — stały stop 1–6 z hasha klucza.
- `share(amount, max, decimals)` — szerokość słupka 0–100 % z ilorazu `bigint` (wynik to tylko geometria).
- `convertAmount(amount, fromDec, rate, toDec)` (`lib/money.ts`) — kwota × kurs na `bigint` z half-even; szacunek „≈ 80,00 zł” przy cenach w obcej walucie (`BaseEstimate` + `useFxQuote(currency, dziś)`).

## C2-4. Projekt wizualny

Kierunek „Mennica” z części A, rozwinięty dla subskrypcji: **każdy dostawca to moneta**. `VendorCoin` — krążek z karbowanym rantem (`repeating-conic-gradient`) i monogramem w Bricolage Grotesque zwężonym, w jednym z sześciu stopów (`--metal-N-bg/ink`: patyna, brąz, srebro, złoto, miedź, łupek; tekst ≥ 4.5:1 w obu motywach). To jedyny „głośny” element; reszta jak księga: cienkie linie, cyfry tabelaryczne, kolor tylko w stanie płatności (przegapiona — `--outflow`, dopasowana — `--inflow`).

- **Kalendarz odnowień** (element charakterystyczny): pasek 31 kolumn-dni (weekendy przyciemnione, dziś — linia patyny, etykiety poniedziałków), monety „wybite” w dniach płatności (kilka w jednym dniu — w stosie). Pasek jest dekoracją (`aria-hidden`); tę samą treść niesie lista `ol` „Odnowienia w 30 dni”. Na telefonie monety maleją do kropek w kolorze stopu.
- **Koszt**: panel z rantem monety wzdłuż górnej krawędzi (jak „Do przydzielenia” w C1), kwota miesięczna w `--text-3xl`, roczna pod nią; obok kalendarz.
- **Rozbicie**: słupki poziome (`--share` ustawiane przez CSSOM — CSP, AD-C11), monety przy dostawcach.
- **Formularz**: cykl w ramce z podglądem na tle patyny („Co miesiąc, 31.” + najbliższe terminy), przypomnienia jako chipy-checkboxy, reguła dopasowania w natywnym `<details>`.
- **Harmonogram i kolejka**: wiersze z datą i „za N dni”, plakietką statusu, kwotą i akcjami; w kolejce karta z lewą krawędzią w kolorze stanu i sugestiami (`ScoreMeter`: pasek + „Zgodność 90%”).

## C2-5. Komponenty (`features/subscriptions/`)

| Plik | Zawartość |
|---|---|
| `parts.tsx` | `VendorCoin`, `coinName`, `useVendorNames`, `SubscriptionStatusBadge`, `ChargeStatusBadge`, `useToday`, `useOpenCharges`, `SubscriptionTabs` (z liczbą płatności do dopasowania), `BaseEstimate` |
| `SubscriptionsLayout.tsx` | nagłówek, zakładki, dialog tworzenia, kontekst zakładek |
| `OverviewPage.tsx` | koszt, `RenewalCalendar` (też w kafelku pulpitu, `compact`), `Bars` (dostawca/koperta), triale, zmiany cen |
| `ListPage.tsx` | filtry w adresie, `SubscriptionCard` |
| `SubscriptionDialog.tsx` | formularz tworzenia/edycji, `findVendor`, `changedFields` (PATCH tylko zmian) |
| `SubscriptionPage.tsx` | szczegóły, statusy, `PriceHistory` + `PriceDialog`, `ChargeSchedule`, `CancelDialog`, `DeleteDialog`, `priceDelta` |
| `charges.tsx` | `ChargeRow`, `ChargeDialogs` (`MatchDialog`, `PostDialog`, `SkipDialog`), `SuggestionList`, `ScoreMeter` |
| `ChargesQueuePage.tsx` | kolejka dopasowań |
| `LinkChargeDialog.tsx` | rejestr: „Powiąż z subskrypcją” |
| `SubscriptionsTile.tsx` | pulpit |

Zaksięgowanie: kwota zawsze wysyłana jawnie w walucie wybranego portfela; gdy waluta portfela = waluta płatności, pole ma kwotę płatności, inaczej jest puste i wymagane (kwota z wyciągu po przewalutowaniu banku).

## C2-6. Integracja z rejestrem

API transakcji nie zwraca obciążenia (`ChargeRepository.ByTransaction` nie jest wystawione przez REST). `Register` pyta więc `GET charges?status=matched&status=posted&from=<najstarsza−31 dni>&to=<najnowsza+31 dni>&limit=200` dla dat wczytanych wierszy i buduje mapę `transaction_id → charge`. `TransactionRow` dostaje `charge` (plakietka „Subskrypcja: <nazwa>” z linkiem — zastępuje plakietkę źródła) i `onLinkCharge` (przycisk przy wydatku z ujemną kwotą bez powiązania).

## C2-7. Strategia testów

- **Vitest**: `lib/subscriptions.test.ts` (cykle po polsku, odmiana, `nextDues` z 31 i 29.02, dni, monogramy, `share`), `convertAmount` w `money.test.ts`, `features/subscriptions/subscriptions.test.tsx`: przegląd (koszt, `incomplete`, odnowienia „za 10 dni”, rozbicie, triale, zmiany cen), pusty stan, formularz (tworzenie USD z cyklem, przypomnieniami i regułą → body; opisy cykli; walidacja bez wywołania API; limit 5 przypomnień; 409 przy nazwie; edycja tylko zmian), strona subskrypcji (historia z różnicą, zmiana ceny; pauza i usunięcie; dopasowanie, zaksięgowanie z kwotą PLN dla płatności USD, pominięcie, cofnięcie; 404), lista (karty i filtry → zapytanie), kolejka (sugestie, szybkie potwierdzenie, licznik w zakładce, pusty stan), rejestr (plakietka z linkiem, „Powiąż z subskrypcją”), pulpit. Data przez `vi.useFakeTimers({toFake: ['Date']})`. Domyślne handlery MSW: `/vendors`, subskrypcje, podsumowanie, obciążenia, `fx-rates` (kurs 4,0).
- **Playwright** `e2e/subscriptions.spec.ts` (binarium, atrapa NBP 4,0): pusty przegląd → formularz (OpenAI, 20 USD, dzień 31.01.2025, portfel PLN, przypomnienie, zaawansowane) → strona subskrypcji z obciążeniami (powstają od razu przy tworzeniu — 004 FR-304, bez czekania na job `DENAR_SUBSCRIPTION_INTERVAL`) → wydatek 80,40 zł „OPENAI *CHATGPT” w dniu pierwszej płatności → „Dopasuj” → sugestia → potwierdzenie → plakietka w rejestrze prowadzi do subskrypcji → zmiana ceny na 25 USD od dziś → lista „≈ 100,00 zł” → przegląd 100,00 zł / 1200,00 zł rocznie, zmiana ceny 20 → 25 USD → pulpit; drugi test: płatność z wczoraj w EUR w kolejce → zaksięgowanie 42,17 zł → kolejka pusta → wiersz rejestru z plakietką. `a11y.spec.ts`: przegląd, lista, kolejka, strona subskrypcji, edycja, dopasowanie (jasny, ciemny, telefon); `mobile.spec.ts`: strony subskrypcji bez poziomego przewijania.

## Decyzje autonomiczne (część C2)

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-D01 | Monogram na „monecie” w stopie z hasha klucza zamiast logo dostawców | CSP `img-src 'self'`, brak praw do znaków towarowych; spójne z „Mennicą” |
| AD-D02 | Szacunek w walucie bazowej przy pojedynczej subskrypcji liczony w UI (`convertAmount`, kurs `fx-rates` z dziś) | API daje koszt w walucie ceny; sumy w walucie bazowej liczy serwer (summary) — UI nie sumuje walut |
| AD-D03 | Link z rejestru do subskrypcji przez `GET charges?status=matched,posted` w oknie dat widocznych wierszy (±31 dni, ≤ 200) | REST nie zwraca obciążenia przy transakcji; bez zmiany kontraktu |
| AD-D04 | Zaksięgowanie zawsze z jawną kwotą w walucie portfela; przy innej walucie pole puste i wymagane | Kwota z wyciągu jest prawdą (przewalutowanie banku), serwerowe przeliczenie kursem z dnia terminu byłoby tylko szacunkiem |
| AD-D05 | Kolejka = `expected` i `missed` z terminem ≤ dziś; przyszłe oczekiwane są tylko w harmonogramie | Do rozstrzygnięcia jest to, co już powinno się pojawić na wyciągu |
| AD-D06 | Pominięcie z potwierdzeniem, cofnięcie dopasowania bez | API nie cofa pominięcia; dopasowanie można powtórzyć |
| AD-D07 | Cena w formularzu tylko przy tworzeniu; zmiana — osobny dialog „Zmień cenę” z datą | Historia cen to jedyne źródło ceny (004 AD-404); edycja nie może jej nadpisać po cichu |
| AD-D08 | Edycja wysyła tylko zmienione pola (`changedFields`) | Zmiana np. notatki nie przelicza harmonogramu (cykl wysłany tylko przy zmianie) |
| AD-D09 | Wznowienie ustawia `trial`, gdy okres próbny jeszcze trwa, inaczej `active` | Zgodne z regułą statusu domyślnego z 004 |
| AD-D10 | Przypomnienia jako chipy {0, 1, 3, 7, 14, 30} + istniejące wartości subskrypcji, najwyżej 5 | Typowe wybory bez wpisywania liczb; zgodne z 004 FR-300 |
| AD-D11 | Kalendarz jako dekoracyjny pasek + lista `ol` z tą samą treścią | Dostępność bez opisywania wykresu; pasek czytelny na rzut oka |

## Ograniczenia (część C2)

- Plakietka subskrypcji w rejestrze tylko dla powiązań z obciążeniem o terminie w oknie ±31 dni od wczytanych wierszy (do 200 obciążeń); poza nim zostaje plakietka źródła.
- Szacunek „≈” przy subskrypcji używa dzisiejszego kursu; brak kursu — brak szacunku (sumy przeglądu mają wtedy `incomplete`).
- Brak edycji `external_ref` i `ends_at` anulowania w UI (domyślne z serwera); brak cofnięcia pominięcia (API).
- Pasek kalendarza pokazuje maks. kilka monet dziennie w stosie; pełna lista jest pod nim.

## Punkty rozszerzenia dla części C3

- **Powiadomienia (005):** zdarzenia `subscription.renewal_upcoming|trial_ending|price_changed|charge_missed` mają `subscription_id` i `charge_id` — linkuj do `/h/:id/subscriptions/:subscriptionId` (harmonogram na stronie) i do kolejki `/h/:id/subscriptions/charges`. Licznik `SubscriptionTabs` (`useOpenCharges`) można pokazać też przy pozycji menu. `ComingSoonPage section="notifications"` czeka na zastąpienie.
- **MCP (006):** scope'y `subscriptions:read|write` są w `BotsTab`; instrukcja podłączenia bota (URL MCP, nagłówek `Authorization: Bearer dnr_…`, przykładowe pytania „co odnawia się w tym tygodniu?”) pasuje do dialogu z jednorazowym tokenem (`BotsTab`) i kroku „Podłącz asystenta AI” na pulpicie.
- **Budżet:** `summary.by_envelope` (miesięczny koszt subskrypcji per koperta, 004 FR-330) może podpowiadać cel `monthly` w `EnvelopeDialog`.

---

# Design: część C3a — podłączenie asystenta AI (MCP) i przegląd jakości

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Last Updated: 2026-10-21

## C3a-1. Podłączenie asystenta

`features/settings/AssistantConnect.tsx` — sekcja w `TokenDialog` (`BotsTab`, rozmiar `lg`), widoczna przy tworzeniu bota i po rotacji tokenu. Czyste funkcje (testowane osobno): `mcpUrl(origin)`, `claudeCodeCommand(url, token)`, `remoteHttpConfig` (`{mcpServers: {denar: {type: 'http', url, headers}}}` — format `.mcp.json` Claude Code / Cursor), `mcpRemoteConfig` (`npx -y mcp-remote <url> --header "Authorization: Bearer …"`, jak w README), `examplePrompts(effective_scopes)`. Adres z `window.location.origin` (przeglądarka zna publiczny adres także za reverse proxy). Ostrzeżenie o braku HTTPS, gdy `http:` i host ≠ localhost/127.0.0.1/[::1].

Nowy komponent `CopyBlock` (obok `CopyField`): wielowierszowy `<pre>` (zawijanie, `tabIndex=0` dla przewijania klawiaturą) z przyciskiem „Kopiuj”; wspólny hook `useCopy` (Clipboard API, inaczej zaznaczenie do ręcznego skopiowania). Przyciski kopiowania mają nazwę dostępną „Kopiuj: <etykieta>” (dopisek `visually-hidden`), bo w dialogu jest ich kilka.

`lib/scopes.ts`: `assistantScopes` = `*:read` + `transactions:write`; przycisk zestawu w `CreateBotForm` (filtr `roleAllowsScope`). Pulpit: tekst kroku „Podłącz asystenta AI” wskazuje MCP.

## C3a-2. Komunikaty serwera po polsku

`api/messages.ts`: `translateFieldMessage(message)` — uporządkowana lista wyrażeń regularnych pokrywająca komunikaty `domain.ValidationError` z `internal/domain` i `internal/usecase` (długości, zakresy, kwoty z liczbą miejsc, waluty, daty, listy wartości, przelewy, splity, cele, subskrypcje); nieznany komunikat zaczynający się od angielskiego słowa kluczowego → „Nieprawidłowa wartość.”, inny (np. polski z testów) przechodzi bez zmian. `reasonMessages` — opisy 409/422 po ostatnim segmencie `type`. `fieldErrors` tłumaczy zawsze; `errorMessage` dla 409/422/400 zwraca pierwszy przetłumaczony błąd pola albo opis przyczyny, nigdy angielskie `detail`/`title`. `labels.ts`: etykiety pól zagnieżdżonych (`goal.amount.amount`, `match_rule.*`, `splits[i].amount` → „podział i: kwota”).

## C3a-3. Dziennik, 404, focus

- Dziennik: akcje 003/004 (`envelope_group.*`, `envelope.*`, `assignment.set`, `subscription.*`, `charge.*`, `household.deleted`) i zasoby (`envelope_group`, `envelope`, `assignment`, `subscription`, `charge`) — filtr „Czego dotyczy” bierze je z tej samej mapy. Wartości `{amount, currency}` → „49.00 PLN”; cykl zapisany przez serwer jako `"1 month from 2026-10-21"` → `cycleLabel` po polsku.
- `HouseholdNotFoundPage` jako dziecko `*` trasy `h/:householdId` (w `AppShell`); `NotFoundPage` zostaje dla adresów spoza gospodarstwa.
- `lib/focusInvalid.ts`: nasłuch `submit` w fazie przechwytywania na `document` (instalowany w `main.tsx`); w następnej klatce, gdy focus nie stoi już na błędnym polu, przenosi go na pierwsze `[aria-invalid="true"]` w formularzu (działa dla `Form` i surowych `<form>`).

## C3a-4. Przegląd jakości (exploratory QA)

Jednorazowy skrypt Playwright (poza repozytorium) na zbudowanym binarium z atrapą NBP: logowanie (błędne i poprawne) → gospodarstwo (pusta walidacja) → portfele (za dużo miejsc po przecinku, duplikat nazwy, USD) → transakcje (skrót `n`, pusta walidacja, przelew wielowalutowy) → budżet (grupa, koperta z celem, duplikat) → subskrypcje (pusta walidacja, Netflix, przegląd, lista, kolejka) → boty (zestaw „Asystent AI”, token, wywołanie `/mcp`) → wszystkie zakładki ustawień → trasy nieznane i nieistniejące zasoby → kolejność Tab → telefon 390 px w jasnym motywie, menu, rola „Podgląd” z zaproszenia. Zrzuty obejrzane ręcznie; brak błędów JS w konsoli (tylko oczekiwane 401/404/409 z API), brak poziomego przewijania.

Znalezione i naprawione: angielskie nazwy akcji/zasobów 003–004 w dzienniku i angielski cykl w zmianach; angielskie komunikaty serwera (detail/title w `FormAlert`, toastach i `QueryState`, komunikaty walidacji pól); 404 w gospodarstwie bez nawigacji; po wysłaniu długiego formularza (subskrypcja) z błędami komunikat zostawał poza widokiem.

## C3a-5. Strategia testów

Vitest: `api/messages.test.ts`, `features/settings/AssistantConnect.test.ts`, `BotsTab.test.tsx` (zestaw „Asystent AI” → scope'y, sekcja podłączenia z adresem, nagłówkiem, poleceniem, mcp-remote i pytaniami), `AuditTab.test.tsx` (akcje i wartości 003/004), `App.test.tsx` (404 w gospodarstwie i poza nim), `lib/focusInvalid.test.ts`. Playwright: `flow.spec.ts` (zestaw, sekcja podłączenia, `tools/call whoami` na `/mcp` z tokenem z dialogu, axe), `a11y.spec.ts` (404 w gospodarstwie, dialog tokenu z rozwiniętymi konfiguracjami — jasny, ciemny, telefon, bez poziomego przewijania).

## Decyzje autonomiczne (część C3a)

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-E01 | Instrukcja MCP tylko w dialogu jednorazowego tokenu, z tokenem wstawionym w polecenia | Token jest znany tylko w tej chwili; gotowe polecenie usuwa najczęstszy błąd (zły nagłówek) |
| AD-E02 | Konfiguracja „HTTP z nagłówkami” w formacie `mcpServers.denar.{type,url,headers}`; dla klientów bez nagłówków — `mcp-remote` jak w README | Najszerzej obsługiwane formaty; README pozostaje źródłem dla CLI |
| AD-E03 | Zestaw „Asystent AI” = odczyt wszystkiego + `transactions:write` | Zgodnie z nazwą („wprowadzanie wydatków”); zapis budżetu i subskrypcji wymaga świadomego zaznaczenia |
| AD-E04 | Tłumaczenie komunikatów serwera po stronie UI (regexy na znanych komunikatach) | Kontrakt i backend bez zmian; angielskie komunikaty zostają dla API/MCP (modele) |
| AD-E05 | Focus na pierwszym błędnym polu przez globalny nasłuch `submit` zamiast zmian w każdym formularzu | Jedno miejsce dla kilkunastu formularzy; nie zmienia zachowania poprawnych formularzy |
| AD-E06 | Profil użytkownika pominięty | API ma tylko `GET /me` (bez zmiany imienia/hasła); menu konta już pokazuje imię, e-mail, motyw i wylogowanie |

## Ograniczenia (część C3a)

- Brak strony profilu (zmiana imienia, hasła, e-maila) — brak endpointów; do dodania razem z API.
- Tłumaczenie komunikatów serwera pokrywa komunikaty istniejące w etapach 001–004 i 006; nowe komunikaty backendu bez reguły dają „Nieprawidłowa wartość.” — przy zmianach walidacji dopisz regułę w `api/messages.ts`.
- Pola daty (`<input type="date">`) wyświetlają format zależny od języka przeglądarki/systemu, nie od `lang` strony (np. `mm/dd/yyyy` w przeglądarce po angielsku).
- Pasek zakładek ustawień i subskrypcji na telefonie przewija się poziomo w swoim kontenerze (ostatnia zakładka bywa częściowo ucięta przy krawędzi).
- Wartości w zmianach dziennika poza kwotami i cyklami (np. scope'y `wallets:read`, `via: auto_assign`) pokazywane są jak zapisał je serwer.

---

# Design: część C3b — powiadomienia

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Last Updated: 2026-10-21

## C3b-1. Warstwa API

- `api/hooks/notifications.ts`: `useUnreadCount` (`refetchInterval` 60 s, `refetchOnWindowFocus: 'always'`), `useRecentNotifications(limit, {householdId})`, `useNotifications({householdId, unread})` (`useInfiniteQuery`, 30 na stronę), `useMarkNotificationRead`, `useMarkAllNotificationsRead`, `useChannelTypes`, `useChannels`, `useCreateChannel`, `useUpdateChannel`, `useDeleteChannel`, `useTestChannel`, `useNotificationPreferences(hid)`, `useSetNotificationPreferences(hid)`.
- Klucze (`api/keys.ts`): powiadomienia są osobiste, więc nie leżą pod `['household', hid]`: `['notifications']` (prefiks), `['notifications','list',filtry]`, `['notifications','unread-count']`, `['channel-types']`, `['channels']`; preferencje — `['household', hid, 'notification-preferences']`. Oznaczenie przeczytanego unieważnia cały prefiks `notifications`; zmiana licznika z odpytywania unieważnia listy (nowe powiadomienie pojawia się w otwartej skrzynce). Usunięcie kanału unieważnia preferencje wszystkich gospodarstw.
- Typy w `api/types.ts`, etykiety (`eventTypeLabels`/`Descriptions`/`Order`, `channelTypeLabels`, `unavailableReasonLabels`, pola `config.*`/`secrets.*`/`items.*`) w `lib/labels.ts`; reguły komunikatów 005 i przyczyny 409 w `api/messages.ts`. Etap 005 nie dodał akcji audytu (kanały i preferencje są osobiste, AD-507) — `actionLabels` bez zmian.

## C3b-2. Dzwonek i skrzynka (`features/notifications/`)

- `NotificationBell` (`placement: 'rail' | 'topbar'`): w pasku obok znaku Denara, na telefonie w górnej belce (`RailContent bell` — szuflada menu go nie powtarza). Przycisk z plakietką liczby (do „99+”) i ukrytym tekstem „Powiadomienia, N nieprzeczytane”; panel to `section` z nagłówkiem (rola `region`), `useDisclosure` (Escape z powrotem focusu, klik/focus poza). Panel jest `position: fixed` (pasek ma `overflow-y: auto` i przycinałby go): na komputerze obok paska, na telefonie pod belką na całą szerokość. Lista ładowana dopiero po otwarciu (8 ostatnich).
- `NotificationItem`: karta `article` z ikoną w kolorze tonu (ostrzeżenia: zmiana ceny, okres próbny, termin celu; negatywne: przegapiona płatność, przekroczona koperta), tytułem jako linkiem rozciągniętym na kartę (`::after`), treścią i czasem; nieprzeczytane — pogrubienie, kropka i ukryty prefiks „Nieprzeczytane:”. Klik linku oznacza przeczytane (`mutate` — mutacja kończy się po odmontowaniu). Na liście dodatkowy przycisk „Oznacz jako przeczytane”. `notificationLink` przepuszcza tylko ścieżki SPA (`safeReturnTo`), inaczej `/h/{household_id}`.
- `NotificationsPage`: filtry w adresie (`?unread=1`, `?household=`), `groupByDay` po lokalnym dniu, `PageHeader` z „Oznacz wszystkie” i „Ustawienia powiadomień”; puste stany „Wszystko przeczytane” i zaproszenie do ustawień.
- `RecentNotificationsTile` na pulpicie (3 ostatnie z gospodarstwa; brak — nic).

## C3b-3. Ustawienia → Powiadomienia

Zakładka `settings/notifications` (każda rola) = `ChannelsSection` + `PreferencesSection`.

- `channelForm.ts` (czyste funkcje, testy tabelaryczne): opisy pól per `typ.pole` (etykieta, podpowiedź, placeholder, typ inputu) z fallbackiem na nazwę pola z serwera, `channelGuides` (pomoc per typ), `emptyForm`/`formFromChannel`, `validateForm` (nazwa, wymagane `config`, wymagane sekrety przy tworzeniu i przy „Zmień”), `createBody` (bez pustych pól), `updateBody` (tylko zmiany; `config` w całości, bo PATCH go zastępuje; `secrets`: brak / obiekt / `null`), `canRemoveSecrets`, `secretsAvailable` (brak klucza głównego wnioskowany z `unavailable_reason: secrets_unavailable` typów z wymaganym sekretem), `channelSummary`.
- `ChannelDialog` (rozmiar `lg`): przy tworzeniu kafelki typów (radio; niedostępne zablokowane z powodem w nazwie dostępnej), przy edycji typ tylko do odczytu; pola `config_fields` (opcjonalne z dopiskiem „(opcjonalnie)”), sekrety jako `type="password"`, `autoComplete="new-password"`; przy kanale z sekretem — radio „Zostaw bez zmian / Zmień / Usuń”; „Włączony”; pomoc typu w `Callout`. Błędy 422 pól `config.*`/`secrets.*` przy polach, reszta w `FormAlert`.
- `ChannelsSection`: lista niedostępnych typów z powodem, karty kanałów z plakietkami i akcjami (nazwy dostępne „Wyślij test: X”, „Wyłącz: X”…), wynik testu w `role="status"` w karcie, limit 20 (przycisk „Dodaj kanał” wyłączony).
- `PreferencesSection`: `completePreferences` (6 typów w kolejności serwera, brakujące domyślne, nieznane z nowszego serwera zachowane), tabela w kontenerze przewijanym poziomo (checkboxy z nazwami „Typ: kanał”), wyłączone kanały z dopiskiem; zapis odfiltrowuje kanały usunięte w międzyczasie; formularz remontowany kluczem po zapisie.

## C3b-4. Zaproszenia

`InvitationsTab` czyta `useChannelTypes`; gdy `email.available` — checkbox „Wyślij link e-mailem” (domyślnie zaznaczony, przycisk „Utwórz i wyślij”), body `send_email: true`; dialog linku pokazuje `email_sent` (informacja) lub `email_error` (ostrzeżenie: zaproszenie powstało, przekaż link sam). Bez SMTP — opis „ten serwer nie ma skonfigurowanej poczty”.

## C3b-5. Strategia testów

- Vitest: `features/notifications/model.test.ts` (grupowanie po dniu, bezpieczne linki, odmiana „nieprzeczytane”, `createBody`/`updateBody`/`validateForm`, preferencje), `notifications.test.tsx` (dzwonek: licznik, panel, klik → nawigacja i read, „Oznacz wszystkie”, odświeżenie przy fokusie, Escape; skrzynka: dni, pojedyncze read, filtr, kursor, gospodarstwa; pulpit), `settings/NotificationsTab.test.tsx` (formularz z typów, niedostępne typy, sekrety przy edycji, test/wyłącz/usuń, brak klucza → 409 po polsku, macierz → PUT, viewer, zaproszenie e-mailem), `api/messages.test.ts`. Domyślne handlery MSW: licznik 0, pusta lista, typy (z kluczem, bez SMTP), brak kanałów, domyślne preferencje.
- Playwright: atrapa NBP (`e2e/support/nbp-stub.mjs`) przyjmuje też webhooki (`POST /webhook/<nazwa>` zapisuje nagłówki i surowe body, `/webhook-fail/<nazwa>` → 500, `GET /webhook/<nazwa>` zwraca zapisane). `playwright.config.ts`: `DENAR_MASTER_KEY` testowy, `DENAR_SUBSCRIPTION_INTERVAL` i `DENAR_NOTIFY_INTERVAL` = 2 s, bez SMTP. `notifications.spec.ts`: webhook z sekretem → „Wyślij test” → podpis HMAC policzony w teście zgadza się → preferencja → subskrypcja (płatność za 3 dni, przypomnienie 3 dni) → powiadomienie w skrzynce i dzwonku → klik → strona subskrypcji, przeczytane → dostawa na webhook z poprawnym podpisem i treścią; kanał zepsuty → „Test nieudany” i ostatni błąd → edycja „Usuń” sekret → usunięcie. `a11y.spec.ts` (zakładka, skrzynka z powiadomieniem, panel dzwonka, nowy kanał Matrix, edycja z sekretem), `mobile.spec.ts` (bez poziomego przewijania, dzwonek w belce, panel w całości w oknie).

## Decyzje autonomiczne (część C3b)

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-F01 | Dzwonek i skrzynka pokazują powiadomienia ze wszystkich gospodarstw (nazwa gospodarstwa przy kilku), skrzynka ma filtr gospodarstwa; pulpit — tylko bieżące | Powiadomienia są osobiste (`/me/…`), linki prowadzą do właściwego gospodarstwa; licznik per gospodarstwo chowałby ważne rzeczy |
| AD-F02 | Kanały i preferencje jako zakładka „Powiadomienia” w Ustawieniach gospodarstwa, widoczna dla każdej roli; kanały opisane jako wspólne dla gospodarstw, macierz — dla bieżącego | Nie ma strony profilu; preferencje i tak są per gospodarstwo |
| AD-F03 | Odpytywanie licznika co 60 s + przy fokusie; zmiana licznika unieważnia listy | 005 nie ma powiadomień na żywo; minuta to rozsądny koszt dla instancji domowej |
| AD-F04 | Panel dzwonka `position: fixed` zamiast natywnego `<dialog>`/popover | Pasek przewija się w pionie i przyciąłby panel; wzorzec disclosure jak przełącznik gospodarstwa |
| AD-F05 | Klik w powiadomienie oznacza je jako przeczytane; osobny przycisk tylko na liście | Typowe zachowanie skrzynek; panel ma być lekki |
| AD-F06 | Sekrety per kanał jako grupa: „Zostaw / Zmień / Usuń” dotyczy wszystkich sekretów kanału | Kontrakt: obiekt `secrets` zastępuje całość (AD-511); każdy typ ma najwyżej jeden sekret |
| AD-F07 | Brak klucza głównego wykrywany z `unavailable_reason: secrets_unavailable` typów z wymaganym sekretem; pola opcjonalnych sekretów (webhook, ntfy) wtedy zablokowane | Kontrakt nie ma osobnej flagi; zapis i tak dałby 409 |
| AD-F08 | „Wyślij link e-mailem” domyślnie zaznaczone, gdy SMTP jest dostępne | Skoro serwer umie wysłać, to oczekiwana ścieżka; link i tak widać po utworzeniu |
| AD-F09 | E2E: joby subskrypcji i rozsyłki co 2 s, webhook do atrapy NBP rozszerzonej o odbiornik zamiast osobnego serwera | Jeden dodatkowy proces mniej; atrapa i tak startuje jako `webServer` |

## Ograniczenia (część C3b)

- Licznik i nowe powiadomienia pojawiają się z opóźnieniem do minuty (lub od razu po powrocie do karty).
- Brak historii dostaw na kanał i ponowienia ręcznego — API daje tylko `verified_at`/`last_error`.
- Bez SMTP w e2e: ścieżka „Wyślij link e-mailem” i kanał e-mail są testowane tylko w Vitest (MSW).
- Kanały mają pomoc konfiguracji tylko po polsku i bez kreatora (np. pobierania `chat_id` przez Telegram API z poziomu UI).
