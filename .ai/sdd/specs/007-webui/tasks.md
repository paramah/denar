# Tasks: Web UI (007-webui)

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
- [x] **T-206 E2E** — `e2e/budget.spec.ts`, budżet w `a11y.spec.ts` i `mobile.spec.ts`. NFR-C02.

## Wynik części C1

- T-201…T-206 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 218 testów w 22 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 67 testów Playwright zielonych (6 pominiętych — testy telefonu w projektach desktopowych): przepływ budżetu na binarium (wpływ 3000 zł z szybkiego dodawania → grupa i koperty z celem z UI → przydziały z klawiatury Enter/Tab → wydatek 650 zł z kopertą (skrót `n`) → przekroczenie −150 zł w siatce, szczegółach i na pulpicie → następny miesiąc: koperta wyzerowana, RTA 2150 zł z uwagą o przekroczeniu → przeniesienie z RTA → auto-przydział celu 800 zł → bieżący miesiąc widzi 850 zł przydziałów przyszłych), drugi test: kolejność przyciskami z powrotem fokusu, 409 `group-not-empty`, 409 `envelope-in-use` → ukrycie; axe na budżecie, dialogach, szczegółach i trybie układu w motywie jasnym, ciemnym i na telefonie; budżet i pulpit bez poziomego przewijania na 390 px.
- Lint wymaga ucieczek `\u00a0`/`\u202f` w wyrażeniach regularnych testów (narzędzia edycji potrafią je zamienić na znaki dosłowne — `no-irregular-whitespace`).

## Część C2 — subskrypcje

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)

- [x] **T-301 Spec części C2** — sekcje C2 w `requirements.md`, `design.md`, `tasks.md`; `.status`.
- [x] **T-302 Warstwa API i czyste funkcje** — `api/hooks/subscriptions.ts`, klucze `subscriptions*`/`charges`/`vendors` w `api/keys.ts`, typy, unieważnienie subskrypcji w `useTransactionsChanged`, `lib/subscriptions.ts` (+ testy), `convertAmount` w `lib/money.ts` (+ testy), fabryki i domyślne handlery MSW (subskrypcje, obciążenia, `fx-rates`), stopy `--metal-*` w `tokens.css`. NFR-D01.
- [x] **T-303 Przegląd i lista** — `SubscriptionsLayout`, `SubscriptionTabs`, `OverviewPage` (`RenewalCalendar`, słupki), `ListPage` (`SubscriptionCard`, filtry), `VendorCoin`, `BaseEstimate`, trasy. Testy. FR-D01…FR-D11.
- [x] **T-304 Formularz** — `SubscriptionDialog` (cykl z podglądem, trial, portfel, koperta, przypomnienia, zaawansowane, walidacja, PATCH tylko zmian). Testy. FR-D20…FR-D22.
- [x] **T-305 Szczegóły i obciążenia** — `SubscriptionPage` (statusy, anulowanie, usuwanie, historia cen, zmiana ceny, harmonogram), `charges.tsx` (dopasowanie, zaksięgowanie, pominięcie, cofnięcie). Testy. FR-D30…FR-D33.
- [x] **T-306 Kolejka i integracja** — `ChargesQueuePage`, `LinkChargeDialog`, plakietka i akcja w `TransactionRow`/`Register`, `SubscriptionsTile` na pulpicie. Testy. FR-D40, FR-D50, FR-D51.
- [x] **T-307 E2E** — `e2e/subscriptions.spec.ts`, subskrypcje w `a11y.spec.ts` i `mobile.spec.ts`. NFR-D02.

## Wynik części C2

- T-301…T-307 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 259 testów w 24 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 73 testy Playwright zielone (8 pominiętych — testy telefonu w projektach desktopowych): przepływ subskrypcji na binarium (OpenAI 20 USD, dzień 31. → obciążenia od razu po utworzeniu → wydatek 80,40 zł w rejestrze → „Dopasuj” → sugestia → potwierdzenie → plakietka w rejestrze prowadzi do subskrypcji → zmiana ceny na 25 USD → lista „≈ 100,00 zł” → przegląd 100,00 zł / 1200,00 zł rocznie → pulpit), kolejka dopasowań (płatność EUR z wczoraj → zaksięgowanie 42,17 zł z portfela PLN), axe na przeglądzie, liście, kolejce, stronie subskrypcji i dialogach w motywie jasnym, ciemnym i na telefonie; strony subskrypcji bez poziomego przewijania na 390 px.
- Obciążenia powstają przy tworzeniu subskrypcji (od dnia utworzenia minus okno dopasowania, 004 FR-304), więc e2e nie czeka na job `DENAR_SUBSCRIPTION_INTERVAL`; płatność „w przeszłości” uzyskuje się terminem wczoraj i oknem ≥ 1 dzień.

## Część C3a — asystent AI (MCP) i przegląd jakości

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)

- [x] **T-401 Spec części C3a** — sekcje C3a w `requirements.md`, `design.md`, `tasks.md`; `.status`.
- [x] **T-402 Podłączenie asystenta** — `AssistantConnect.tsx` (+ testy), `CopyBlock`/`useCopy`, `TokenDialog` (rozmiar `lg`), `assistantScopes` i przycisk zestawu, krok na pulpicie. FR-E01…FR-E05.
- [x] **T-403 Komunikaty serwera po polsku** — `api/messages.ts` (+ testy), `fieldErrors`/`errorMessage`, etykiety pól zagnieżdżonych. FR-E10.
- [x] **T-404 Dziennik** — akcje i zasoby 003/004, kwoty i cykle w zmianach. Testy. FR-E11.
- [x] **T-405 404 i focus** — `HouseholdNotFoundPage` w trasie `h/:householdId/*`, `lib/focusInvalid.ts` w `main.tsx`. Testy. FR-E12, FR-E13.
- [x] **T-406 Przegląd jakości** — skrypt Playwright poza repozytorium na binarium z atrapą NBP, zrzuty obejrzane (desktop ciemny/jasny, telefon, rola „Podgląd”); poprawki T-403…T-405.
- [x] **T-407 E2E** — `flow.spec.ts` (zestaw, sekcja podłączenia, `/mcp` z tokenem z dialogu), `a11y.spec.ts` (404, dialog tokenu). NFR-E01.

## Wynik części C3a

- T-401…T-407 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 283 testy w 27 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 79 testów Playwright zielonych (8 pominiętych — testy telefonu w projektach desktopowych): token z dialogu bota działa z `/mcp` (`tools/call whoami`), axe na dialogu tokenu z podłączeniem asystenta i na 404 w gospodarstwie w motywie jasnym, ciemnym i na telefonie.
- Przegląd jakości: bez błędów JS w konsoli na całej ścieżce nowego użytkownika; naprawione — angielskie komunikaty serwera, angielskie akcje/zasoby/cykle w dzienniku, 404 bez nawigacji, błąd walidacji poza widokiem w długim formularzu. Pozostałe — „Ograniczenia (część C3a)” w `design.md`.
- Dla C3b: `ComingSoonPage section="notifications"` nadal w trasie `notifications`; scope `notifications:read` jest w `scopeGroups` i w zestawie „Asystent AI”; nowe komunikaty walidacji backendu 005 dopisz do `api/messages.ts`, akcje audytu do `lib/labels.ts`.

## Część C3b — powiadomienia

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)

- [x] **T-501 Spec części C3b** — sekcje C3b w `requirements.md`, `design.md`, `tasks.md`; `.status`.
- [x] **T-502 Warstwa API** — `api/hooks/notifications.ts`, klucze `notifications*`/`channel*`/`notificationPreferences` w `api/keys.ts`, typy, etykiety w `lib/labels.ts`, reguły 005 w `api/messages.ts` (+ testy), fabryki i domyślne handlery MSW. FR-F31.
- [x] **T-503 Dzwonek i skrzynka** — `NotificationBell` (pasek i belka telefonu), `NotificationItem`, `NotificationsPage` (zamiast `ComingSoonPage`, usunięty), `model.ts`, `RecentNotificationsTile` na pulpicie, ikony `target`/`send`. Testy. FR-F01…FR-F12.
- [x] **T-504 Ustawienia → Powiadomienia** — `channelForm.ts` (+ testy), `ChannelDialog`, `ChannelsSection`, `PreferencesSection`, zakładka `settings/notifications`. Testy. FR-F20…FR-F23.
- [x] **T-505 Zaproszenia e-mailem** — checkbox `send_email`, wynik `email_sent`/`email_error`. Testy. FR-F30.
- [x] **T-506 E2E** — odbiornik webhooków w atrapie NBP, `DENAR_MASTER_KEY` i krótkie interwały w `playwright.config.ts`, `notifications.spec.ts`, nowe ekrany w `a11y.spec.ts`/`mobile.spec.ts`. NFR-F01.

## Wynik części C3b

- T-501…T-506 zakończone; `task verify` zielone: gofmt, go vet, web (eslint, tsc, vite build), Vitest — 332 testy w 30 plikach, `go test -race ./...`, build binarium z osadzonym SPA.
- `task web:e2e`: 88 testów Playwright zielonych (10 pominiętych — testy telefonu w projektach desktopowych): kanał webhook do atrapy → „Wyślij test” → podpis `X-Denar-Signature` zgodny z HMAC policzonym w teście → preferencja „odnowienie → webhook” → subskrypcja z przypomnieniem → powiadomienie w skrzynce i dzwonku → klik prowadzi do subskrypcji i oznacza przeczytane → dostawa na webhook z poprawnym podpisem; kanał z błędem 500 → „Test nieudany” i ostatni błąd → usunięcie sekretu przy edycji; axe na skrzynce, panelu dzwonka, zakładce i dialogach kanału w motywie jasnym, ciemnym i na telefonie; skrzynka i zakładka bez poziomego przewijania na 390 px, panel dzwonka w całości w oknie.
- Kanały są osobiste i konto e2e jest wspólne dla testów — testy kanałów zaczynają od `deleteAllChannels` (limit 20 na osobę).
