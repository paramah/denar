# Tasks: Web UI — część A (007-webui)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `design.md`
> Last Updated: 2026-10-21

Weryfikacja każdego zadania: `task web:test` (eslint, tsc, vitest); na końcu `task verify` i `task web:e2e`. Części B i C dopiszą zadania T-1xx i T-2xx.

## Część A

- [x] **T-001 Szkielet web/** — `package.json`, `package-lock.json`, `vite.config.ts`, `tsconfig*.json`, `eslint.config.js`, `index.html`, `public/{theme-init.js,favicon.svg}`, `web/Taskfile.yml`, include w root `Taskfile.yml`, `src/api/schema.d.ts`, `embed_test.go`. Kryterium: `task web:build` tworzy `dist/index.html` i zachowuje `dist/.gitkeep`; `task verify` zielone. NFR-A02.
- [x] **T-002 Warstwa API i testowa** — `api/{client,errors,queryClient,keys}.ts`, `test/{setup,render,dialogShim,openapiValidator}.ts`, `test/msw/*`. Testy: 401 → handler, wyjątki, walidacja kontraktu mocków. FR-A04.
- [x] **T-003 Style i komponenty bazowe** — tokeny, fonty, `Money` + `lib/money.ts`, `Button`, `Field`, `FormAlert`, `Dialog`, `ConfirmDialog`, `Toast`, `EmptyState`, `Skeleton`, `QueryState`, `DataList`, `CopyField`, `Badge`, `PageHeader`, `Icon`, `SkipLink`. Testy komponentów. FR-A40, FR-A41, NFR-A01.
- [x] **T-004 Sesja, trasy i powłoka** — `router.tsx`, `RequireSession`, `AppShell` (rail, menu mobilne, przełącznik gospodarstwa, menu użytkownika, motyw), `HouseholdRoute`, `NotFoundPage`, puste stany „wkrótce”, Pulpit. FR-A01…FR-A05.
- [x] **T-005 Logowanie i zaproszenia** — `LoginPage`, `InvitePage`. Testy MSW. FR-A10…FR-A13.
- [x] **T-006 Gospodarstwa** — Welcome, `CreateHouseholdPage`, `HouseholdForm`. Testy (422 przy polu). FR-A20…FR-A22.
- [x] **T-007 Ustawienia: gospodarstwo, członkowie, zaproszenia** — zakładki z uprawnieniami. Testy. FR-A30…FR-A32, FR-A35.
- [x] **T-008 Ustawienia: boty** — grupy scope'ów, wygaśnięcie, jednorazowy token, wyłączanie, rotacja, usuwanie. Testy. FR-A33.
- [x] **T-009 Ustawienia: dziennik** — filtry, stronicowanie. Testy. FR-A34.
- [x] **T-010 E2E** — `playwright.config.ts`, `e2e/*`: login → gospodarstwo → zaproszenie i rejestracja → bot i token → dziennik; axe na ekranach; telefon. NFR-A01, NFR-A03.

## Wynik części A

- T-001…T-010 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 100 testów w 13 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 43 testy Playwright zielone (chromium ciemny — pełny przepływ login → gospodarstwo → zaproszenie i rejestracja z linku → bot, token i wywołanie Bearer → dziennik; axe na każdym ekranie w motywie jasnym, ciemnym i na telefonie 390 px; nawigacja mobilna bez poziomego przewijania).
- Poprawka znaleziona przez e2e: po rejestracji z zaproszenia formularz odmontowywał się (zmiana `/me`) przed callbackiem `mutate`; rejestracja ustawia teraz `/me` z odpowiedzi i używa `mutateAsync`. Zasada dla B/C: nawigację po mutacji, która zmienia `/me`, robić przez `mutateAsync` albo w komponencie, który się nie odmontuje.

## Część B

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)

- [x] **T-101 Spec części B** — sekcje części B w `requirements.md`, `design.md`, `tasks.md`; `.status`.
- [x] **T-102 Kwoty i typy** — `lib/money.ts` (`parseAmountInput`, `negateDecimal`, `absDecimal`, `parseRateInput`, `formatRate`, `decimalsOf`), `lib/format.ts` (`todayIn`, `formatDay`), `lib/labels.ts` (typy portfeli, rodzaje, statusy, źródła, akcje audytu 002), `lib/features.ts`, `api/types.ts`, `api/keys.ts`. Testy tabelaryczne. FR-B16.
- [x] **T-103 Hooki API** — `api/hooks/{wallets,transactions,payees,fx}.ts`; fabryki MSW. FR-B01…FR-B41.
- [x] **T-104 Portfele** — `WalletsPage`, `WalletCard`, `WalletDialog`, `WalletPage`, trasy, menu. Testy. FR-B01…FR-B05.
- [x] **T-105 Transakcje** — `TransactionDialog`, `PayeeInput`, `EnvelopeField`, `SplitEditor`, `Register`, `TransactionRow`, `StatusToggle`, `RatePendingBadge`, `TransactionsPage`, skrót `n`. Testy (walidacja, 422/409, splity z flagą). FR-B10…FR-B19.
- [x] **T-106 Przelewy** — `TransferDialog`. Testy (wielowalutowy). FR-B20.
- [x] **T-107 Kontrahenci i kursy** — `PayeesTab`, `RatesTab`, zakładki ustawień. Testy. FR-B30, FR-B40, FR-B41.
- [x] **T-108 Boty — portfele** — `WalletScopePicker` w tworzeniu i dialog „Portfele”. Testy. FR-B50.
- [x] **T-109 Pulpit** — `features/dashboard/DashboardPage`. Testy. FR-B60.
- [x] **T-110 E2E** — stub NBP, `e2e/wallets.spec.ts`, nowe ekrany w `a11y.spec.ts`/`mobile.spec.ts`. NFR-B01, NFR-B02.

## Wynik części B

- T-101…T-110 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 177 testów w 19 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 61 testów Playwright zielonych (4 pominięte — testy telefonu w projektach desktopowych): nowy przepływ portfele PLN/USD → wydatek → przelew wielowalutowy → salda → rejestr → pulpit, ręczny kurs (CHF w gospodarstwie EUR), axe na nowych ekranach i dialogach w motywie jasnym, ciemnym i na telefonie, rejestr bez poziomego przewijania na 390 px. NBP zastępuje atrapa `e2e/support/nbp-stub.mjs` (drugi `webServer`, `DENAR_NBP_URL`) — testy nie wychodzą do sieci.
- Poprawki znalezione przez e2e: pulpit pokazuje teraz kilka kwot „0,00 €” (kafelki), więc test części A sprawdza region „Sumy portfeli”; przycisk „Edytuj” portfela trzeba odróżniać od „Edytuj: <transakcja>” w wierszach rejestru (`exact`).

## Część C1 — budżet kopertowy

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)

- [x] **T-201 Spec części C1** — sekcje C1 w `requirements.md`, `design.md`, `tasks.md`; `.status`.
- [x] **T-202 Warstwa API i miesiąc** — `api/hooks/budget.ts`, klucze `budget*` w `api/keys.ts`, typy, unieważnienia w hookach transakcji i portfeli, `lib/budgetMonth.ts` (+ testy), fabryki i domyślne handlery MSW budżetu. FR-C01, NFR-C01.
- [x] **T-203 Strona budżetu** — `BudgetPage`, `BudgetSummary`, `BudgetGrid` (edycja w miejscu, cele, stany), `BudgetFlows`, trasy `budget` i `budget/:month`, ikony. Testy. FR-C01…FR-C14, FR-C25.
- [x] **T-204 Operacje** — `MoveDialog`, auto-przydział z podsumowaniem, `StructureDialogs` (grupy, koperty, cele, 409), kolejność, `EnvelopeDetails`. Testy. FR-C20…FR-C24.
- [x] **T-205 Integracja z częścią B** — flaga `envelopes`, `useEnvelopeOptions`/`useEnvelopeNames`, edycja kopert/splitów tylko przy zmianie, koperta w przelewie on↔off, domyślna koperta kontrahenta, filtr i plakietka koperty w rejestrze, `BudgetTile` na pulpicie. Testy. FR-C30…FR-C35.
- [ ] **T-206 E2E** — `e2e/budget.spec.ts`, budżet w `a11y.spec.ts` i `mobile.spec.ts`. NFR-C02.
