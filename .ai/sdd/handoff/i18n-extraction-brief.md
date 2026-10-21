# Brief: ekstrakcja napisów (013-i18n, T-1307)

Jesteś jednym z kilku wykonawców przenoszących polskie napisy Web UI do plików tłumaczeń. Nie zadajesz pytań. Nigdy `git push`. Commity kończysz linią `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Przeczytaj
- `web/src/i18n/README.md` — konwencja (klucze, liczba mnoga, `tk()` poza komponentami, formattery, testy).
- Wzorce już przetłumaczone: `web/src/app/`, `web/src/components/`, `web/src/features/profile/`, `web/src/features/auth/LoginPage.tsx`, `web/src/lib/labels.ts`.

## Zasady
- Piszesz TYLKO w swoich plikach: `features/<twoje obszary>/**` oraz `web/src/i18n/{pl,en}/<twoje obszary>.json` (i plikach wymienionych w zadaniu). Nie edytuj `common.json`, `labels.json`, `errors.json` ani cudzych obszarów — jeśli potrzebujesz wspólnego napisu, użyj istniejącego klucza z `common`; jeśli go brak, dodaj kopię w swoim namespace.
- Polskie brzmienie kopiujesz znak w znak (cudzysłowy „”, półpauzy —, wielokropki …, kropki na końcu, spacje nierozdzielające). Istniejące testy (Vitest i Playwright) szukają elementów po polskim tekście i MUSZĄ przejść bez modyfikacji. Nie zmieniaj testów poza dodaniem nowych.
- Angielski: naturalny, zwięzły, spójny z `common.json`/`labels.json` (terminologia: wallet, envelope, Ready to Assign, subscription, charge, payee, household, bot, scope, channel).
- Liczba mnoga: `count` jako liczba; pl `_one/_few/_many/_other`, en `_one/_other`. Ręczne `plural()` w Twoich plikach zamień na i18next.
- Żadnych `'pl-PL'` na sztywno — `intlLocale()` / `perLocale()` / istniejące formattery.
- Atrybuty też tłumaczysz: `aria-label`, `title`, `placeholder`, `alt`, teksty w toastach, komunikaty walidacji po stronie klienta, `document.title` przez `PageHeader`.
- Komentarze w kodzie zostają po polsku.
- Dodaj w każdym obszarze przynajmniej jeden test Vitest renderujący kluczowy ekran po `setLocale('en')` i sprawdzający kilka angielskich tekstów.
- Po zakończeniu: `grep -nP '[ąćęłńóśźżĄĆĘŁŃÓŚŹŻ]' <twoje pliki .ts/.tsx bez testów>` może zwracać tylko komentarze i dane (np. porównania). Sprawdź też ręcznie napisy bez polskich znaków („Zapisz”, „Nazwa”).

## Weryfikacja
W `web/`: `npx tsc -b`, `npm run lint`, `npx vitest run`. Na koniec z katalogu głównego `task verify`. Jeśli masz czas, `task web:e2e` (wszystkie spec'e muszą przejść).

## Raport końcowy
Commity, wyniki testów, liczba kluczy dodanych, pliki pozostawione bez tłumaczenia (jeśli jakieś) i dlaczego.
