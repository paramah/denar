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
