# Design: i18n (013)

> Status: Approved (użytkownik, przez zatwierdzenie planu 2026-10-21)
> Source: `requirements.md`
> Last Updated: 2026-10-21

## Backend (część A)

- `internal/domain/user`: `type Locale string` (`LocalePL = "pl"`, `LocaleEN = "en"`), `ParseLocale(string) (Locale, error)` (ValidationError `must be pl or en`), pole `User.Locale *Locale`, `EffectiveLocale(*Locale) Locale` (nil → `pl`).
- Migracja `00007_user_locale.sql`: `ALTER TABLE users ADD COLUMN locale TEXT NULL CHECK (locale IN ('pl','en'))`. `sqlite/user.go` (kolumny, `Update`), fake, suita `portstest`.
- Use case `UpdateMe` (`internal/usecase/update_me.go`): tylko człowiek (bot → `ErrForbidden`), walidacja, `Users.Update`, audyt `user.updated` w tej samej transakcji (audyt bez gospodarstwa — jeśli port audytu wymaga gospodarstwa, wpis pomijamy i zapisujemy decyzję w AD).
- REST: `PATCH /api/me` (kontrakt już w `api/openapi.yaml`, operacja `updateMe`), mapowanie `User.locale`, przypadki konformancji 200/403/422.
- Powiadomienia: `notification.Render(locale, event)` z szablonami PL (dotychczasowe, bez zmian brzmienia) i EN; helpery `when`/`date`/`money` per język (PL `02.01.2006`, `1 234,56 zł`; EN `2006-01-02`, `PLN 1,234.56` lub zgodnie z istniejącym helperem — ważna jest spójność). `notification_dispatch.go` renderuje w pętli po odbiorcach.
- Zaproszenie (`invitations.go`), stopka SMTP, test kanału (`channels.go`): język użytkownika wywołującego / właściciela kanału (`EffectiveLocale`).
- Błędy dostarczania pokazywane w UI (`last_error`, wynik testu): serwer zapisuje stabilne angielskie komunikaty (np. `HTTP 500`, `connection refused`, `target not allowed (private network)`), a Web UI tłumaczy je regułami w `messages.ts`. Polskie komunikaty w adapterach zamieniamy na angielskie + regułę tłumaczenia w web (koordynacja z częścią B: lista komunikatów w raporcie).

## Web (część B — fundament)

- `i18next` + `react-i18next`, inicjalizacja synchroniczna z zasobami w paczce (bez backendu HTTP), `web/src/i18n/{index.ts,locale.ts,resources.ts}`, `web/src/i18n/{pl,en}/<obszar>.json`. Typowanie przez `declare module 'i18next' { interface CustomTypeOptions { resources: typeof pl } }`.
- Klucze: `obszar.komponent.znaczenie` w camelCase (np. `wallets.list.empty`); liczba mnoga przez sufiksy i18next (`_one/_few/_many/_other`).
- `locale.ts`: `resolveLocale(me, stored, navigatorLanguages)`, `setLocale()` (i18next, `document.documentElement.lang`, zapis w `localStorage` przez `lib/storage.ts`), `intlLocale()` → `pl-PL`/`en-GB`.
- Formattery (`lib/money.ts`, `format.ts`, `budgetMonth.ts`, `subscriptions.ts` i miejsca z `pl-PL` na sztywno) — tworzone leniwie i cache'owane per locale.
- `lib/labels.ts` → funkcje oparte na `t()`; `api/messages.ts` → reguły zwracają klucz + parametry.
- Profil: `features/profile/ProfilePage.tsx` (trasa `/profile`, link w menu użytkownika), wybór języka (Automatycznie / Polski / English); ekran logowania — przełącznik języka.
- Testy: domyślny język testów `pl` (setup Vitest), test zgodności kluczy i parametrów, testy `resolveLocale`, formatowania EN, przełączenia języka; `e2e/i18n.spec.ts`.

## Web (część C — ekstrakcja)

Po fundamencie: każdy obszar `features/<obszar>` przenosi napisy do `{pl,en}/<obszar>.json` bez zmiany polskiego brzmienia. Obszary rozdzielone między wykonawców tak, by nie dzielili plików JSON.

## Decyzje

- AD-1301: zapisane powiadomienia nie są ponownie renderowane (treść jest w bazie).
- AD-1302: brak preferencji po stronie serwera = `pl` (serwer nie zna przeglądarki odbiorcy).
- AD-1303: angielskie formatowanie dat i liczb w UI wg `en-GB` (dzień przed miesiącem, poniedziałek pierwszym dniem tygodnia — bliżej przyzwyczajeń użytkowników w Polsce).

### Decyzje — backend (część A)

- AD-1304: `PATCH /me` nie zapisuje wpisu audytu — `audit_log.household_id` jest `NOT NULL`, a język to ustawienie osobiste (jak kanały i preferencje, AD-507). Zmiana trafia do logu (`user preferences updated`, `user_id`, `locale`).
- AD-1305: bot na `PATCH /me` dostaje 403 z powodem `human_only` (jak inne operacje tylko dla ludzi); brak pola `locale` (`{}`) → 422 `locale: is required`; `null` czyści preferencję; nieznana wartość → 422 `locale: must be pl or en` (dekoder nie waliduje enumu, robi to `user.ParseLocale`).
- AD-1306: `User.Locale *Locale`; wartość spoza `pl|en` (niemożliwa przez `CHECK` w bazie) traktowana jak brak preferencji (`pl`). Use case czyta konto z repozytorium w transakcji, więc nieaktualny principal nie nadpisuje innych pól.
- AD-1307: kwoty w powiadomieniach w obu językach bez zmian: dziesiętny string z payloadu + kod waluty (`43.00 PLN`) — spójnie z dotychczasowym helperem i bez formatowania liczb w domenie. Daty: PL `02.01.2006` (bez zmian), EN ISO `2006-01-02`. Względne terminy: `dziś/jutro/za N dni` ↔ `today/tomorrow/in N days`.
- AD-1308: port `OutgoingMessage` dostał pole `Locale user.Locale` (zmiana `ports.go`, minimalna). Stałe teksty kanałów (linia „Gospodarstwo:/Household:” w SMTP, ntfy, Telegram, Matrix; stopka e-maila; wiadomość testowa) są w domenie (`notification.HouseholdLine`, `EmailFooter`, `TestMessage`), adaptery wołają je z `msg.Locale`. Worker dostaw czyta aktualny język odbiorcy w chwili wysyłki (treść i tytuł wyrenderowane przy dispatchu, zwykle sekundy wcześniej). Payload webhooka bez zmian.
- AD-1309: e-mail z zaproszeniem w języku zapraszającego (języka adresata serwer nie zna); `membership.InvitationEmail`. Termin ważności zawsze w UTC (wcześniej formatowany w strefie zegara — w praktyce UTC).
- AD-1310: komunikaty błędów dostarczania i testu kanału (`last_error`, `ChannelTestResult.error`, `email_error`) oraz komunikat 422 SSRF (`config.url`) są stałe, po angielsku; Web UI tłumaczy je regułami w `messages.ts` (lista dla części B w raporcie T-1304). Błędy przekazywane z zewnątrz (np. `Telegram: <description>`, `Matrix: HTTP 403 M_FORBIDDEN`, `HTTP 500`, kody SMTP) zostają jak są.

### Web (część B; numeracja od AD-1321, żeby nie zderzyć się z backendem)

- AD-1321: i18next 26.5.3 + react-i18next 17.1.0, klucze tekstowe (bez `enableSelector`), namespace = obszar = para plików `web/src/i18n/{pl,en}/<obszar>.json`; typy z polskich plików (`i18next.d.ts`), zgodność angielskich pilnuje `i18n.test.ts` (klucze, parametry, formy liczby mnogiej). Konwencja w `web/src/i18n/README.md`.
- AD-1322: zmiana języka montuje drzewo pod `QueryClientProvider`/`ToastProvider` od nowa (`LocaleBoundary`, klucz = język). Dzięki temu teksty spoza `useTranslation` (etykiety, formattery, komunikaty) zmieniają się bez przeładowania; koszt — utrata lokalnego stanu formularzy przy zmianie języka; fokus wraca na element o tym samym `data-focus-key`/`id`.
- AD-1323: `lib/labels.ts` zachowuje kształt `Record<klucz, string>` — właściwości to gettery czytające `labels.json`; kropki w kodach (`household.created`) zapisane w JSON jako `/`.
- AD-1324: preferencja z `/me` (nie-null) jest też zapisywana w przeglądarce, żeby ekran logowania po wylogowaniu mówił tym samym językiem; „Automatycznie” czyści zapis i wraca do `navigator.languages`.
- AD-1325: porównania danych użytkownika (wyszukiwanie kontrahenta/dostawcy po nazwie, inicjały subskrypcji) zostają przy regułach `pl` niezależnie od języka UI — to dane, nie interfejs.
- AD-1326: separator dziesiętny w polach kwot i kursów (`amountForInput`, `formatRate`) według języka; parser przyjmuje przecinek i kropkę w obu językach.
- AD-1327: błędy dostarczania (`last_error`, wynik testu kanału, `email_error`) tłumaczy `translateDeliveryError` według listy uzgodnionej z backendem; po polsku dawne brzmienie, po angielsku tekst serwera; polskie teksty starszego serwera i odpowiedzi zewnętrzne (`HTTP 500`, `Telegram: …`) przechodzą bez zmian.
- AD-1328: komponenty wspólne (`components/`) i powłoka (`app/`) przetłumaczone w części B (`common.json`), żeby ekstrakcja obszarów nie dzieliła plików.
