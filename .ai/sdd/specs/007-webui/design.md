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
