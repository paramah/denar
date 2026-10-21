# Feature: Web UI — część A (F07A, etap 7)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `.ai/sdd/PLAN.md` (sekcja WebUI), `api/openapi.yaml` etapu 001
> Last Updated: 2026-10-21

## Overview

Pierwsza część SPA Denara (`web/`, React 19 + Vite + TS), osadzonej w binarium. Obejmuje to, co da się zbudować na kontrakcie etapu 001: powłokę aplikacji, logowanie, zaproszenia, gospodarstwa, członków, boty i audit log, a także komponenty bazowe pod widoki finansowe. **Część B** (portfele, transakcje, budżet — po etapach 002–003) i **część C** (subskrypcje, powiadomienia, pulpit, MCP — po 004–006) dojdą jako kolejne przyrosty tej specyfikacji.

## Goals

- Działające SPA pod `/` serwowane przez `denar serve`, budowane i testowane w `task verify`.
- Pełna obsługa kont, gospodarstw, członków, zaproszeń, botów i audytu z UI.
- Komponenty bazowe (kwoty, listy, formularze, dialogi, toasty, puste stany, skeletony) gotowe dla części B/C.
- Dostępność (WCAG 2.2 AA w zakresie sprawdzanym przez axe), tryb jasny/ciemny, używalność na telefonie.

## Non-Goals

- Widoki portfeli, transakcji, budżetu, subskrypcji, powiadomień i pulpitu z danymi (części B/C); w części A są tylko dopracowane puste stany „wkrótce”.
- Zmiany backendu i kontraktu `api/openapi.yaml`.
- Zmiana własnego hasła, administracja kontami instancji (brak w API 001).

## Requirements (EARS)

### Powłoka i sesja

- **FR-A01** Aplikacja SHALL pokazywać nawigację: Pulpit, Portfele, Budżet, Subskrypcje, Powiadomienia, Ustawienia; na wąskim ekranie nawigacja SHALL otwierać się z przycisku menu.
- **FR-A02** WHEN użytkownik należy do więcej niż jednego gospodarstwa THE SYSTEM SHALL pozwalać przełączać gospodarstwo; wybór SHALL być częścią adresu (`/h/:householdId/…`) i zapamiętywany lokalnie.
- **FR-A03** Menu użytkownika SHALL pokazywać nazwę i e-mail, wybór motywu (systemowy/jasny/ciemny) i wylogowanie.
- **FR-A04** WHEN dowolne wywołanie API (poza logowaniem) zwróci 401 THE SYSTEM SHALL wyczyścić dane sesji i przejść na `/login?return_to=<bieżąca ścieżka>`.
- **FR-A05** Język interfejsu SHALL być polski; kwoty i daty formatowane przez `Intl` z locale `pl-PL`.

### Logowanie i zaproszenia

- **FR-A10** Ekran logowania SHALL pokazywać formularz e-mail/hasło, gdy `local_login`, i przycisk OIDC, gdy `oidc_enabled` (`GET /api/auth/login?return_to=…`).
- **FR-A11** WHEN logowanie się nie uda THE SYSTEM SHALL pokazać komunikat bez rozróżniania przyczyny; przy 429 — informację o czasie oczekiwania.
- **FR-A12** Strona `/invite#<token>` SHALL pobrać podgląd zaproszenia; dla nowego e-maila — formularz rejestracji (imię, hasło 12–128 znaków); dla istniejącego konta — logowanie z powrotem na zaproszenie; zalogowany SHALL móc przyjąć zaproszenie jednym przyciskiem. Token nie trafia do adresu wysyłanego do serwera.
- **FR-A13** Zaproszenia nieznane/odwołane, zużyte i wygasłe SHALL mieć osobne, zrozumiałe komunikaty.

### Gospodarstwa

- **FR-A20** WHEN użytkownik nie ma gospodarstw THE SYSTEM SHALL pokazać pusty stan z akcją utworzenia.
- **FR-A21** Formularz gospodarstwa SHALL mieć nazwę, walutę bazową (lista z `/api/currencies`), strefę czasową (domyślnie strefa przeglądarki) i dzień startu miesiąca 1–28.
- **FR-A22** Błędy walidacji problem+json (`errors[]`) SHALL być pokazywane przy właściwych polach; pozostałe — nad formularzem.

### Ustawienia gospodarstwa

- **FR-A30** Zakładka „Gospodarstwo”: edycja nazwy, strefy, dnia startu (owner/admin), waluta tylko do odczytu; usunięcie gospodarstwa (owner) z potwierdzeniem przez wpisanie nazwy.
- **FR-A31** Zakładka „Członkowie”: lista z rolami; zmiana roli i usunięcie zgodnie z uprawnieniami; opuszczenie gospodarstwa przez siebie.
- **FR-A32** Zakładka „Zaproszenia”: tworzenie (e-mail, rola), jednorazowe pokazanie linku z kopiowaniem, lista ze stanem, odwołanie oczekujących.
- **FR-A33** Zakładka „Boty”: tworzenie z wyborem scope'ów pogrupowanych według obszaru (odczyt/zapis), wygaśnięcie (domyślnie za 90 dni, można wyłączyć), jednorazowe pokazanie tokenu z kopiowaniem i ostrzeżeniem; lista z efektywnymi scope'ami, ostatnim użyciem i stanem; wyłączanie/włączanie, rotacja tokenu i usuwanie (z potwierdzeniem).
- **FR-A34** Zakładka „Dziennik”: wpisy audytu od najnowszych z filtrami (rodzaj aktora, typ zasobu) i stronicowaniem kursorem („Pokaż starsze”); czytelne nazwy akcji i aktorów.
- **FR-A35** Zakładki niedostępne dla roli SHALL być ukryte (viewer i member: Gospodarstwo w trybie odczytu, Członkowie, Boty — member tylko własne, viewer bez botów; Zaproszenia i Dziennik tylko owner/admin).

### Komponenty bazowe i dostępność

- **FR-A40** Komponenty: `Money`, `DataList` (stronicowanie „pokaż więcej”), pola formularza z błędami, `Dialog`/`ConfirmDialog`, toasty, `EmptyState`, `Skeleton`, `QueryState`.
- **FR-A41** `Money` SHALL formatować kwotę z decimal stringa bez konwersji na liczbę zmiennoprzecinkową, cyframi tabelarycznymi, ze znakiem i kolorem dla wpływu/wydatku.
- **NFR-A01** Każdy widok obsługiwalny klawiaturą, widoczny focus, link „Przejdź do treści”, semantyczne nagłówki i landmarki; e2e sprawdza axe (bez naruszeń serious/critical).
- **NFR-A02** Zgodność z CSP serwera (`script-src 'self'`, `style-src 'self'`, `font-src 'self'`): bez skryptów i elementów `<style>` inline, fonty z pakietów npm.
- **NFR-A03** Testy: Vitest (komponenty, hooki; MSW z walidacją odpowiedzi względem `api/openapi.yaml`), Playwright e2e na zbudowanym binarium.

---

# Część B — portfele, transakcje, przelewy, kontrahenci, kursy (F07B)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `api/openapi.yaml` etapu 002, `.ai/sdd/specs/002-wallets-transactions/`
> Last Updated: 2026-10-21

Część B wypełnia sekcję „Portfele”, dodaje rejestr transakcji, kontrahentów i kursy walut oraz pulpit z saldami. Stoi na kontrakcie etapu 002; budżet kopertowy (003) przychodzi w części C — tu pola kopert i splity są przygotowane, ale ukryte za flagą.

## Non-Goals (część B)

- Widoki kopert, „do przydzielenia”, subskrypcji i powiadomień (część C).
- Zmiany backendu i `api/openapi.yaml`.
- Import CSV (008), wirtualizacja długich list (wystarcza „Pokaż więcej”).

## Requirements (EARS)

### Portfele

- **FR-B01** Strona „Portfele” SHALL pokazywać portfele widoczne dla użytkownika z: nazwą, typem, walutą, saldem w walucie portfela, saldem uzgodnionym, saldem w walucie bazowej (gdy waluta ≠ bazowa), oznaczeniem „poza budżetem”, „prywatny” i „czeka na kurs”; nad listą — sumę sald portfeli budżetowych i wszystkich portfeli w walucie bazowej.
- **FR-B02** WHEN użytkownik zaznaczy „Pokaż zarchiwizowane” THE SYSTEM SHALL dociągnąć portfele zarchiwizowane (`include_archived=true`) i oznaczyć je; zarchiwizowane nie wchodzą do sum.
- **FR-B03** Member+ SHALL móc utworzyć portfel: nazwa, typ, waluta (lista z `/currencies`), saldo otwarcia (może być ujemne, np. kredyt) i jego data, „w budżecie”, widoczność współdzielony/prywatny z wyborem członków gospodarstwa (twórca zawsze zaznaczony i zablokowany).
- **FR-B04** Member+ SHALL móc zmienić nazwę, typ i „w budżecie” oraz zarchiwizować/przywrócić portfel; widoczność i członków SHALL móc zmienić tylko twórca, a dla portfela współdzielonego także owner/admin (pola ukryte dla pozostałych). Waluta jest tylko do odczytu.
- **FR-B05** Strona portfela (`/h/:id/wallets/:walletId`) SHALL pokazywać salda portfela i jego rejestr transakcji; portfel niewidoczny/nieistniejący — komunikat „Nie znaleziono portfela”.

### Rejestr transakcji

- **FR-B10** Rejestr (portfela i globalny `/h/:id/transactions`) SHALL listować transakcje od najnowszych ze stronicowaniem kursorem („Pokaż więcej”).
- **FR-B11** Rejestr SHALL mieć filtry: portfel (globalny), zakres dat, kontrahent, tekst, rodzaj, status, tag; filtry SHALL być w adresie strony (link odtwarza widok).
- **FR-B12** Wiersz SHALL pokazywać datę, kontrahenta lub opis przelewu („Przelew do/z <portfel>”), notatkę, tagi, portfel (globalny), kwotę ze znakiem i kolorem, kwotę bazową (gdy inna waluta), źródło inne niż ręczne (bot, import, subskrypcja) i status.
- **FR-B13** WHEN transakcja ma `rate_pending` THE SYSTEM SHALL pokazać oznaczenie „Czeka na kurs” z podpowiedzią, że kwota bazowa uzupełni się sama, a owner/admin może ustawić kurs ręcznie (link do ustawień dla owner/admin).
- **FR-B14** Member+ SHALL móc przełączyć status jednym kliknięciem (oczekująca ⇄ rozliczona); `reconciled` zmienia się w formularzu edycji.
- **FR-B15** Szybkie dodawanie (przycisk i skrót klawisza `n` poza polami tekstowymi) SHALL otwierać formularz z fokusem na kwocie; pola: rodzaj (wydatek / wpływ / zwrot), kwota, kontrahent z podpowiedziami z listy i autotworzeniem po nazwie, data (domyślnie dziś w strefie gospodarstwa), portfel (w rejestrze globalnym; domyślnie ostatnio używany), status, tagi, notatka; Enter zapisuje; opcja „dodaj kolejną” zostawia formularz otwarty.
- **FR-B16** Kwoty w formularzach SHALL być wpisywane jako liczba dodatnia (przecinek lub kropka, spacje ignorowane), z walidacją liczby miejsc waluty; znak wynika z rodzaju. Obliczenia (sumy, salda, kontrola splitów) SHALL odbywać się na `bigint` jednostek minimalnych.
- **FR-B17** Member+ SHALL móc edytować i usuwać transakcje (z potwierdzeniem); dla nogi przelewu formularz SHALL pozwalać zmienić tylko datę, status, tagi i notatkę, a usunięcie SHALL uprzedzać, że zniknie cały przelew.
- **FR-B18** Formularz SHALL mieć pole koperty i edytor splitów (linie: koperta, kwota, notatka; suma kontrolna „pozostało do rozdzielenia”, zapis zablokowany, dopóki suma ≠ kwocie) — widoczne tylko, gdy włączona jest flaga `envelopes` (część C, po API kopert z etapu 003).
- **FR-B19** Błędy problem+json SHALL trafiać do pól (`amount`, `date`, `payee_name`, `splits[i].amount`, …), 409 `wallet-archived` — komunikat nad formularzem.

### Przelewy

- **FR-B20** Member+ SHALL móc wykonać przelew między dwoma różnymi niezarchiwizowanymi portfelami: z, do, data, kwota; WHEN waluty portfeli się różnią THE SYSTEM SHALL wymagać kwoty docelowej w walucie portfela docelowego i pokazać obie waluty przy polach.

### Kontrahenci

- **FR-B30** Ustawienia → „Kontrahenci” SHALL listować kontrahentów z wzorcami; member+ SHALL móc dodać, zmienić nazwę i wzorce (lista podciągów) oraz usunąć (z uprzedzeniem, że transakcje zostaną bez kontrahenta).

### Kursy walut

- **FR-B40** Ustawienia → „Kursy walut” SHALL pozwalać sprawdzić kurs waluty do waluty bazowej na dzień (źródło: tożsamość, ręczny, NBP, brak) i listować kursy ustawione ręcznie.
- **FR-B41** Owner/admin SHALL móc ustawić kurs ręcznie (waluta ≠ bazowa, dzień, kurs dodatni do 12 miejsc); interfejs SHALL wyjaśnić, że uzupełni on transakcje czekające na kurs z tego dnia.

### Boty

- **FR-B50** Tworzenie bota SHALL pozwalać ograniczyć go do wybranych portfeli (domyślnie „wszystkie, które widzisz”); lista botów SHALL pokazywać ograniczenie, a przycisk „Portfele” SHALL je zmieniać (`PATCH wallet_ids`).

### Pulpit

- **FR-B60** Pulpit SHALL pokazywać sumę sald portfeli budżetowych w walucie bazowej (z oznaczeniem, gdy część kwot czeka na kurs), listę portfeli z saldami (link do portfela) i ostatnie transakcje (link do rejestru); bez portfeli — zachętę do dodania pierwszego.

### Jakość

- **NFR-B01** Każdy nowy ekran i dialog bez naruszeń axe serious/critical w motywie jasnym, ciemnym i na telefonie 390 px; rejestr na telefonie jako lista kart, bez poziomego przewijania.
- **NFR-B02** Testy: Vitest + MSW (walidacja formularzy, błędy problem+json, splity, przelew wielowalutowy, lib kwot), Playwright e2e na binarium (portfele PLN i USD → wydatek → przelew → salda) bez dostępu do sieci — NBP zastępuje lokalny stub.

---

# Część C1 — budżet kopertowy (F07C1)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `api/openapi.yaml` (sekcja budget, etap 003), `.ai/sdd/specs/003-envelope-budget/`
> Last Updated: 2026-10-21

Część C1 wypełnia sekcję „Budżet” i włącza koperty w formularzach części B. Stoi na kontrakcie etapu 003; nie zmienia backendu. Subskrypcje, powiadomienia i UI związane z MCP to część C2 (po etapach 004–006).

## Non-Goals (część C1)

- Zmiany backendu i `api/openapi.yaml`; przeciąganie myszą (kolejność zmieniają przyciski).
- Przenoszenie transakcji z usuwanej koperty na inną (003 AD-304), historia przydziałów, raporty.

## Requirements (EARS)

### Miesiąc i „Do przydzielenia”

- **FR-C01** Strona `/h/:id/budget/:month?` SHALL pokazywać miesiąc budżetowy `YYYY-MM`; bez miesiąca — bieżący miesiąc budżetowy gospodarstwa (dziś w strefie gospodarstwa i `month_start_day`); nagłówek SHALL mieć przejście do poprzedniego/następnego miesiąca i powrót do bieżącego; miesiąc spoza 2000-01…2100-12 — komunikat z linkiem do bieżącego.
- **FR-C02** Nagłówek SHALL pokazywać dużą kwotę „Do przydzielenia” (RTA) z opisem stanu: dodatnia — „rozdziel”, zero — „każda złotówka ma zadanie”, ujemna — ostrzeżenie z kwotą do zmniejszenia; obok — sumy miesiąca (przydzielono, aktywność, dostępne w kopertach).
- **FR-C03** WHEN miesiąc ma przekroczenia z poprzedniego miesiąca, przydziały przyszłe albo `incomplete` THE SYSTEM SHALL pokazać odpowiednią uwagę (kwota przekroczeń, kwota przydziałów przyszłych, liczba transakcji czekających na kurs z linkiem do rejestru).

### Siatka kopert

- **FR-C10** Siatka SHALL pokazywać grupy (z sumami przydzielono/aktywność/dostępne) i ich koperty: nazwa, przydzielono, aktywność, dostępne (stan: dodatnie, zero, ujemne = przekroczone, dodatnie z niedofinansowanym celem), pasek postępu celu i stan („Cel pokryty” / „Brakuje X”).
- **FR-C11** Member+ SHALL edytować przydział w miejscu: Enter zapisuje, Esc przywraca, Tab zapisuje i przechodzi do następnej koperty (Shift+Tab — do poprzedniej), wyjście z pola zapisuje; kwota może być ujemna, 0 usuwa przydział; błędna kwota — komunikat przy polu bez wywołania API; błąd serwera — toast i powrót do zapisanej wartości.
- **FR-C12** Koperty ukryte SHALL być poza siatką, dopóki użytkownik nie zaznaczy „Pokaż ukryte (n)”; grupa z samymi ukrytymi kopertami znika razem z nimi.
- **FR-C13** Pod siatką SHALL być sekcja „Poza kopertami”: wpływy, wydatki bez koperty, przelewy (z widoku miesiąca).
- **FR-C14** Na telefonie (390 px) siatka SHALL być listą kart bez poziomego przewijania strony.

### Operacje

- **FR-C20** Member+ SHALL przenosić środki (dialog: z, do, kwota) między kopertami (`moves`) oraz z „Do przydzielenia” do koperty i z koperty do „Do przydzielenia” (zmiana przydziału koperty o kwotę, liczona na `bigint`); walidacja: kwota > 0, źródło ≠ cel.
- **FR-C21** Member+ SHALL uruchomić auto-przydział wg celów jednym przyciskiem; wynik SHALL być pokazany (koperty i kwoty, pozostałe RTA, niedofinansowanie albo „nic do uzupełnienia”).
- **FR-C22** Member+ SHALL tworzyć grupy i koperty, zmieniać ich nazwy, przenosić kopertę do innej grupy, zmieniać kolejność przyciskami „w górę/w dół” (dostępnymi klawiaturą, tryb „Układ”), ukrywać/przywracać koperty i usuwać grupy/koperty z potwierdzeniem. 409 `envelope-in-use` SHALL dać komunikat z przyciskiem „Ukryj kopertę”, 409 `group-not-empty` — komunikat z propozycją ukrycia kopert grupy; 409 `duplicate-name` — błąd przy nazwie.
- **FR-C23** Formularz koperty SHALL mieć cel: brak, `monthly` (kwota co miesiąc), `target_by_date` (kwota i data); kwota > 0 w walucie bazowej, data wymagana dla `target_by_date` (nowa data nie może być w przeszłości); zmiana na „Bez celu” wysyła `goal: null`.
- **FR-C24** Szczegóły koperty (dialog z nazwy koperty) SHALL pokazywać kwoty miesiąca, cel, akcje (przenieś/pokryj przekroczenie, edytuj, ukryj, usuń) i transakcje koperty w miesiącu (`envelopes/{id}/transactions`, „Pokaż więcej”, przy splicie — kwota linii tej koperty) z linkiem do rejestru filtrowanego kopertą i datami miesiąca.
- **FR-C25** Viewer SHALL widzieć budżet bez akcji zapisu.

### Integracja z częścią B

- **FR-C30** Flaga `envelopes` SHALL być włączona; opcje kopert w formularzach pochodzą z `GET /budget/envelopes?include_hidden=true` i grup (etykieta „Grupa · Koperta”, ukryte z dopiskiem).
- **FR-C31** Pole koperty i edytor splitów SHALL działać w szybkim dodawaniu i edycji transakcji; edycja SHALL wysyłać kopertę/splity tylko, gdy się zmieniają.
- **FR-C32** Przelew między portfelem budżetowym a poza budżetem SHALL mieć opcjonalne pole koperty (`envelope_id` na nogę budżetową).
- **FR-C33** Kontrahent SHALL mieć „Domyślną kopertę” (tworzenie, edycja, wyczyszczenie = `null`); lista pokazuje ją przy kontrahencie.
- **FR-C34** Rejestr SHALL mieć filtr „Koperta” (`envelope=` w adresie → `envelope_id`) i pokazywać kopertę w wierszu.
- **FR-C35** Pulpit SHALL pokazywać kafelek „Do przydzielenia” bieżącego miesiąca i do 3 kopert z największym przekroczeniem z linkiem do budżetu.

### Jakość

- **NFR-C01** Kwoty wyłącznie przez `lib/money.ts` (`bigint`, stringi dziesiętne); porównania i sumy nigdy na `minor`.
- **NFR-C02** Testy: Vitest + MSW (edycja w miejscu, przeniesienia, auto-przydział, walidacja celów, 409, kolejność, integracja B), Playwright e2e na binarium (wpływ → przydziały → wydatek z kopertą → przekroczenie → następny miesiąc → RTA), axe na nowych ekranach w motywie jasnym, ciemnym i na telefonie.

---

# Część C2 — subskrypcje (F07C2)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `api/openapi.yaml` (sekcja subscriptions, etap 004), `.ai/sdd/specs/004-subscriptions/`
> Last Updated: 2026-10-21

Część C2 wypełnia sekcję „Subskrypcje” — główną funkcję produktu — i łączy ją z rejestrem transakcji i pulpitem. Stoi na kontrakcie etapu 004; nie zmienia backendu. Powiadomienia (005) i UI związane z MCP (006) to część C3.

## Non-Goals (część C2)

- Zmiany backendu i `api/openapi.yaml`; skrzynka powiadomień i preferencje (005); instrukcja podłączenia bota MCP (006).
- Zewnętrzne logo dostawców (CSP `img-src 'self'`), import CSV i auto-dopasowanie (008).
- Cofnięcie pominięcia płatności (brak w API).

## Requirements (EARS)

### Przegląd

- **FR-D01** Strona `/h/:id/subscriptions` SHALL pokazywać koszt miesięczny i roczny subskrypcji (aktywne + próbne) w walucie bazowej z `GET subscriptions/summary`, liczby wg statusu i — gdy `incomplete` — ostrzeżenie z brakującymi walutami i (owner/admin) linkiem do ręcznego kursu.
- **FR-D02** Przegląd SHALL mieć kalendarz najbliższych 30 dni: pasek dni z monetami dostawców w dniach płatności i listę odnowień (termin, „za N dni”, subskrypcja z linkiem, kwota w walucie ceny i szacunek w walucie bazowej).
- **FR-D03** Przegląd SHALL pokazywać rozbicie kosztu miesięcznego per dostawca i per koperta (słupki w CSS, długość liczona na `bigint`), kończące się okresy próbne (14 dni) i zmiany cen (90 dni, stara → nowa).
- **FR-D04** Bez subskrypcji przegląd SHALL być pustym stanem z akcją „Dodaj pierwszą subskrypcję” (member+).

### Lista

- **FR-D10** Zakładka „Wszystkie” (`subscriptions/list`) SHALL listować subskrypcje z filtrami status, dostawca i portfel (w adresie strony → `status`, `vendor`, `wallet_id`).
- **FR-D11** Karta subskrypcji SHALL mieć monogram dostawcy (bez zewnętrznych obrazków), nazwę (link), dostawcę, cykl po polsku („co miesiąc, 31.”, „co 2 tygodnie, w środę”, „co rok, 14 lutego”), portfel, status (próbna/aktywna/wstrzymana/anulowana, z końcem okresu próbnego lub dostępu), cenę w walucie oryginalnej, szacunek w walucie bazowej po dzisiejszym kursie i następną płatność („za N dni”).

### Formularz

- **FR-D20** Member+ SHALL tworzyć i edytować subskrypcję: dostawca ze słownika `/vendors` z autouzupełnianiem (nazwa subskrypcji podąża za dostawcą, dopóki jej nie zmieniono), nazwa, cena i waluta (tylko przy tworzeniu), cykl (co ile 1–365, jednostka, dzień płatności) z podglądem opisu i trzech najbliższych terminów, okres próbny, portfel płatności, koperta (`EnvelopeField`), przypomnienia (chipy dni przed, najwyżej 5), notatka i adres zarządzania (http/https).
- **FR-D21** Sekcja „Zaawansowane” SHALL zawierać regułę dopasowania: wzorzec kontrahenta, tolerancję kwoty 0–50 % i okno dat 0–15 dni.
- **FR-D22** Walidacja klienta (nazwa, znany dostawca, kwota i miejsca waluty, cykl, data końca triala, liczba przypomnień, tolerancja, okno, URL) SHALL blokować wysłanie; błędy 422 trafiają do pól, 409 `duplicate-name` — do nazwy. Edycja SHALL wysyłać tylko zmienione pola (`null` czyści portfel, kopertę, trial, URL).

### Szczegóły

- **FR-D30** Strona `subscriptions/:id` SHALL pokazywać cenę, cykl (z uwagą o krótszych miesiącach), następną płatność, koszt miesięczny i roczny (z szacunkiem w walucie bazowej), szczegóły (portfel, koperta, przypomnienia, dopasowanie, okres próbny, adres zarządzania, notatka); nieznana/ukryta — „Nie znaleziono subskrypcji”.
- **FR-D31** Member+ SHALL wstrzymać i wznowić subskrypcję, anulować ją (potwierdzenie; dostęp do końca opłaconego okresu) i usunąć (potwierdzenie; powrót do listy).
- **FR-D32** Historia cen SHALL listować punkty od najnowszego (obowiązująca, zaplanowana, różnica do poprzedniej w tej samej walucie); member+ SHALL zapisać zmianę ceny od dnia (domyślnie dziś, dowolna waluta).
- **FR-D33** Harmonogram płatności SHALL listować obciążenia subskrypcji ze statusem (oczekiwana/dopasowana/przegapiona/pominięta/zaksięgowana) i akcjami: dla otwartych — „Dopasuj” (sugestie z punktacją → potwierdzenie), „Zaksięguj” (portfel, kwota w walucie portfela — wymagana, gdy waluta portfela ≠ waluta płatności, data, kontrahent, notatka), „Pomiń” (potwierdzenie); dla powiązanych — „Cofnij dopasowanie”.

### Kolejka dopasowań

- **FR-D40** Zakładka „Do dopasowania” (`subscriptions/charges`) SHALL listować otwarte obciążenia z terminem do dziś (`expected` i `missed`), przegapione najpierw, każde z trzema najlepszymi sugestiami i przyciskiem „Potwierdź” oraz akcjami „Zaksięguj” i „Pomiń”; zakładka SHALL pokazywać ich liczbę.

### Integracja

- **FR-D50** Rejestr transakcji SHALL przy wydatku bez powiązania pokazywać akcję „Powiąż z subskrypcją” (dialog z `transactions/{id}/charge-suggestions` i potwierdzeniem), a przy transakcji powiązanej z obciążeniem — plakietkę „Subskrypcja: <nazwa>” z linkiem do subskrypcji.
- **FR-D51** Pulpit SHALL pokazywać kafelek „Subskrypcje — koszt/mies.” (z kosztem rocznym) i „Najbliższe odnowienia” (do 4 pozycji i pasek 30 dni); bez subskrypcji — jedno zdanie z linkiem.

### Jakość

- **NFR-D01** Kwoty wyłącznie przez `lib/money.ts` (`bigint`; szacunek w walucie bazowej przez `convertAmount` z half-even); cykle i terminy przez `lib/subscriptions.ts` (ta sama arytmetyka co 004 FR-302).
- **NFR-D02** Testy: Vitest + MSW (formularz cyklu i walidacja, edycja tylko zmian, zmiana ceny, akcje obciążeń, kolejka, rejestr, pulpit, lista z filtrami, `lib/subscriptions`, `convertAmount`), Playwright e2e na binarium z atrapą NBP (OpenAI USD, dzień 31. → obciążenia → wydatek → sugestia i potwierdzenie → zmiana ceny → koszt w PLN; kolejka → zaksięgowanie z kwotą PLN), axe w motywie jasnym, ciemnym i na telefonie, brak poziomego przewijania na 390 px.
