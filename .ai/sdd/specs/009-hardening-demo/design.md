# Design: Utwardzenie powiadomień (SSRF) i dane demo (009-hardening-demo)

> Status: Approved (delegated — orkiestrator z delegacji użytkownika, 2026-10-21)
> Source: `requirements.md`, `005-notifications/design.md`
> Last Updated: 2026-10-21

## 1. SSRF w kanałach powiadomień

```
internal/domain/notification/target.go   TargetPolicy{AllowPrivate, Allowed []netip.Prefix}, Private(addr),
                                         CheckAddr, CheckHost, CheckTargets(config), ErrTargetForbidden
internal/usecase/deps.go                 Deps.NotifyTargets notification.TargetPolicy (zero = tylko publiczne)
internal/usecase/channels.go             Create/UpdateChannel: NormalizeConfig → NotifyTargets.CheckTargets
internal/adapter/notify/notifyhttp       NewClient(policy), DefaultClient = NewClient(zero), Do(client, log, req), Snippet
internal/adapter/notify/{webhook,ntfy,telegram,matrix}   pole Log *slog.Logger; Client z composition root
internal/platform/config                 DENAR_NOTIFY_ALLOW_PRIVATE (bool), DENAR_NOTIFY_ALLOWED_CIDRS (lista CIDR/adresów)
cmd/denar/app.go                         wireNotifications: polityka → Deps + jeden klient dla 4 notifierów
```

### Zakresy (`notification.Private`)
`0.0.0.0/8`, `10/8`, `100.64/10`, `127/8`, `169.254/16`, `172.16/12`, `192.0.0/24`, `192.168/16`, `198.18/15`, `224/4`, `240/4` (z 255.255.255.255), `::`, `::1`, `::/96`, `100::/64`, `fc00::/7`, `fe80::/10`, `ff00::/8`, `2001:db8::/32`, `64:ff9b:1::/48`. Adres jest najpierw odzoniony i rozpakowany (`::ffff:a.b.c.d` → v4, `64:ff9b::/96` → osadzony v4).

### Polityka (`TargetPolicy.CheckAddr`)
1. Adres w `Allowed` (sprawdzany v4 po rozpakowaniu i oryginał) → dozwolony.
2. Publiczny → dozwolony.
3. `AllowPrivate` → dozwolony, **z wyjątkiem link-local** (`169.254/16`, `fe80::/10` — metadane chmury); te tylko przez `Allowed`.
4. W przeciwnym razie `ErrTargetForbidden`.

### Klient (`notifyhttp.NewClient`)
`net.Dialer{Control}` — `Control` dostaje rzeczywisty `ip:port` po rozwiązaniu DNS, przed `connect()`; nieparsowalny adres = odmowa. Działa na każdą próbę z happy eyeballs i każde nowe połączenie, więc DNS rebinding nie omija kontroli. `Transport.Proxy = nil` (proxy łączyłoby się za nas, poza kontrolą), `CheckRedirect = ErrUseLastResponse` (bez zmian), timeout 10 s.

### Błędy
`Do`: status spoza 2xx → `HTTP <kod>`; ciało (≤ 64 KiB) zwracane notifierowi, do logu `Warn` z `host` (bez ścieżki i zapytania — Telegram ma token w ścieżce) i `Snippet` (≤ 200 znaków). Odmowa dialera → `ErrTargetForbidden` (czytelny komunikat). Matrix dokleja tylko `errcode`, jeśli pasuje do `^M_[A-Z_]{1,64}$` (np. `Matrix: HTTP 403 M_FORBIDDEN`) — tekst `error` z serwera użytkownika nie wraca. Telegram zostawia `description`, bo host (`DENAR_TELEGRAM_API_URL`) ustawia administrator, a opis jest pomocny („chat not found”).

### Walidacja przy zapisie
`CheckTargets` sprawdza klucze `url` (webhook), `server` (ntfy), `homeserver` (Matrix) po `NormalizeConfig`: literalny IP → `CheckAddr`, `localhost`/`*.localhost` (także z kropką na końcu, dowolna wielkość liter) → jak `127.0.0.1`. Inne nazwy — dopiero dialer. Błąd: `ValidationError{Field: "config.url", Message: ErrTargetForbidden}` → 422.

### Konfiguracja administratora (poza zakresem)
`DENAR_NBP_URL`, `DENAR_TELEGRAM_API_URL`, `DENAR_OIDC_ISSUER`, `DENAR_SMTP_HOST` ustawia administrator, nie użytkownik — nie są celem SSRF. NBP i OIDC mają własne klienty bez polityki (mogą wskazywać na lokalny mirror). Telegram używa klienta powiadomień, więc lokalny serwer Bot API wymaga `DENAR_NOTIFY_ALLOWED_CIDRS`.

### Testy
Tabela zakresów (v4, v6, mapped, NAT64, strefa), polityka (strict/opt-in/allowlista/link-local), `CheckTargets` (localhost, `[::1]`, mapped metadane, ntfy, Matrix, opt-in); dialer: `httptest` na 127.0.0.1 odrzucony domyślnie (także przez nazwę `localhost`), przepuszczony z `AllowPrivate` i z allowlistą `127.0.0.0/8`, odrzucony z allowlistą innej sieci; brak proxy; przekierowanie nieśledzone; błąd bez ciała, ciało w logu bez ścieżki. Use case: 422 dla webhook/ntfy/Matrix i przy edycji; opt-in przepuszcza. E2E: 422 przez REST bez zgody; „Wyślij test” na 500 z ciałem → `HTTP 500`, `last_error` bez ciała. Istniejące testy adapterów dostają klienta `NewClient(AllowPrivate)`, e2e powiadomień `DENAR_NOTIFY_ALLOWED_CIDRS=127.0.0.1`, Playwright `DENAR_NOTIFY_ALLOW_PRIVATE=true`.

## 2. Dane demo

```
cmd/denar/demo.go          denar demo [--reset] [--password]: migracja, seedDemo, wydruk
cmd/denar/e2e_demo_test.go seed na bazie tymczasowej (dwie daty), liczby, inwariant, idempotencja, komenda
Taskfile.yml               task demo, task dev
```

### Mechanika
`seedDemo(ctx, db, log, now, password, reset)` buduje `useCaseDeps` z **zegarem demo** (`demoClock`) i bez dostawcy kursów, potem idzie dzień po dniu od `today − 92` do dziś (12:00 Warszawy; ostatni dzień — prawdziwe `now`). Każdy dzień: kursy ręczne USD/EUR na ten dzień (`SetFxOverride`), przydziały 1. dnia miesiąca, codzienne transakcje, subskrypcje (tworzenie, zmiana ceny, anulowanie, płatności i dopasowania), `RunSubscriptionJobs`, (dziś) `CheckBudgets`, `RunNotificationJobs`; 20 dni temu `MarkAllNotificationsRead`. Dzięki temu obciążenia, przypomnienia, przegapiona płatność i powiadomienia powstają w jobach z realnymi datami, audit log ma daty historii. Wszystko przez use case'y — zero SQL.

### Zawartość
- Konto `demo@denar.local` (`BootstrapAdmin` — admin instancji), gospodarstwo „Dom (demo)”, PLN, Europe/Warsaw, start miesiąca 1.
- Portfele: Konto osobiste (8000 PLN), Karta kredytowa, Gotówka (400), Konto USD (300 USD), **Gotówka EUR**, Oszczędności (15 000, poza budżetem, prywatne).
- Kursy: USD ≈ 3,98, EUR ≈ 4,30 (łagodna fala), od `start − 7` do `today + 14`.
- Grupy/koperty: Stałe (Czynsz, Media, Internet i telefon), Codzienne (Jedzenie, Transport, Zdrowie), Subskrypcje (Streaming, AI i narzędzia, Chmura), Cele (Wakacje — 6000 do `today + 250`, Poduszka finansowa — 1000/mies.), Przyjemności (Restauracje, Hobby).
- Kontrahenci z wzorcami i kopertą domyślną: Biedronka, Lidl, Orlen, PGNiG, Orange, Netflix, Spotify, OpenAI, Anthropic, Apple, Google, Adobe, Wynajmujący, Apteka, Empik, cztery restauracje.
- Transakcje: pensja 9500 (10.), czynsz 2400 (2.), media i internet (15.), przelew 1000 na Oszczędności z kopertą Poduszka (11.), spłata karty (25.), zakupy co 3 dni, paliwo co 9 dni, restauracje co 14 dni, apteka (20.), Empik (5.), wymiana 2000 PLN → 500 USD, 860 PLN → 200 EUR, kolacja 64,50 EUR (Wakacje); w bieżącym miesiącu dodatkowa kolacja = dostępne Restauracje + 85,37 → koperta na minusie (przydział bieżącego miesiąca obniżony do 100).
- Subskrypcje: Netflix 49 PLN, Spotify 23,99 → 26,99 PLN (zmiana ceny przy płatności 25–45 dni temu), ChatGPT Plus 20 USD, Claude Pro 20 USD (przypomnienia 3 i 1 dzień — następna płatność za 3 dni), iCloud+ 11,99 PLN (płatność 5–20 dni temu nie przyszła → `missed`), Adobe 51,49 PLN (anulowana 45 dni temu), YouTube Premium 25,99 PLN w trialu do `today + 4` (przypomnienia 7/3/1). Płatności starsze niż 2 dni potwierdzone (`ConfirmChargeMatch`), nowsze zostawione do dopasowania.
- Bot „Asystent AI”: `wallets:read, transactions:read, transactions:write, budget:read, subscriptions:read, reports:read, notifications:read`.

### Idempotencja
Konto istnieje → hasło musi pasować (inaczej błąd z podpowiedzią). Gospodarstwo o nazwie „Dom (demo)” istnieje → bez `--reset` błąd, z `--reset` `DeleteHousehold` (kaskada) i od nowa.

### `task demo`
`deps: [build]` (Web UI + binarium), env: `DENAR_DB=.dev/demo.db`, `DENAR_ADDR=:8080`, `DENAR_BASE_URL=http://localhost:8080`, `DENAR_MASTER_KEY_FILE=.dev/master.key` (generowany `openssl rand -base64 32`, 0600), `DENAR_AUTO_MIGRATE=true`, `DENAR_NOTIFY_ALLOW_PRIVATE=true` (lokalny n8n/ntfy), `DENAR_DEV_FAKE_CONNECTOR=true`, zmienne konta startowego i OIDC wyczyszczone; `./denar demo --reset && ./denar serve`. Port/URL: `DEMO_ADDR`, `DEMO_BASE_URL`. `task dev` = `dev:api` + `web:dev` równolegle.

### Test (`e2e_demo_test.go`)
Dla `2026-10-21 22:30` i `2026-11-01 08:00` (pierwszy dzień miesiąca): 6 portfeli bez `BasePending`, 7 subskrypcji (5 active, 1 trial, 1 cancelled), podsumowanie subskrypcji nie-`incomplete`, 1 obciążenie `missed`, ≥ 10 `matched`, w bieżącym miesiącu dokładnie Restauracje < 0, `Available + ReadyToAssign + AssignedInFuture` = saldo bazowe portfeli on-budget, RTA > 0, token bota uwierzytelnia; drugi seed bez `--reset` → błąd, z innym hasłem → błąd, `--reset` → jedno gospodarstwo i stary token nieważny. `TestDemoCommand` uruchamia `run([]string{"demo"})` i sprawdza wydruk.

## Decyzje autonomiczne

| ID | Decyzja | Uzasadnienie |
|----|---------|--------------|
| AD-901 | Polityka celów w domenie (`notification.TargetPolicy`), kontrola dwa razy: walidacja przy zapisie i dialer | Walidacja daje czytelny 422 od razu; dialer jest jedyną kontrolą odporną na DNS i rebinding |
| AD-902 | `DENAR_NOTIFY_ALLOW_PRIVATE` nie otwiera link-local (169.254/16, fe80::/10); otwiera je tylko jawny CIDR | Typowy self-hosting (ntfy/n8n w LAN) nie potrzebuje metadanych chmury; najgroźniejszy cel zostaje zamknięty |
| AD-903 | Klient powiadomień bez proxy ze środowiska | Proxy łączyłoby się z celem za nas, omijając `Control`; proxy dla powiadomień można dodać później jawną zmienną |
| AD-904 | Błąd dla użytkownika: tylko `HTTP <kod>`; Matrix — także `errcode` wg wzorca `M_*`; Telegram — `description` | Ciało odpowiedzi to kanał wycieku; errcode Matrixa jest ograniczonym tokenem; host Telegrama ustawia administrator |
| AD-905 | Nazwy hostów (poza `localhost`) nie są rozwiązywane przy zapisie kanału | Rozwiązanie przy zapisie i tak nie chroni (rebinding), a spowalnia i zależy od sieci; dialer to łapie |
| AD-906 | Konto demo przez `BootstrapAdmin` (admin instancji) | Jedyny use case tworzący konto z hasłem bez zaproszenia; demo i tak jest lokalne |
| AD-907 | Szósty portfel „Gotówka EUR” | Transakcja ma walutę portfela; „jedna transakcja w EUR” wymaga portfela w EUR |
| AD-908 | Stałe hasło `denar-demo-2026` (nadpisywalne `--password`/`DENAR_DEMO_PASSWORD`), ostrzeżenie w README i na wydruku | Łatwe do wpisania rano; demo tylko lokalnie |
| AD-909 | Historia generowana zegarem demo dzień po dniu z prawdziwymi jobami | Obciążenia, przypomnienia, `missed` i powiadomienia wychodzą z tych samych reguł co w produkcji, z realnymi datami |
| AD-910 | Powiadomienia starsze niż 20 dni oznaczone jako przeczytane | Dzwonek pokazuje kilka świeżych zamiast kilkudziesięciu historycznych |
| AD-911 | `task demo` zawsze `--reset` na `.dev/demo.db`, token bota się zmienia | Przewidywalny stan przy każdym uruchomieniu |
| AD-912 | `DENAR_NOTIFY_ALLOW_PRIVATE=true` w `task demo` i `.env.dev.example` | Lokalnie wygodnie testować webhook do n8n na tym samym komputerze; produkcyjny domyślny pozostaje zamknięty |
| AD-913 | Przenumerowanie przyszłych etapów: PSD2 → 010, OpenAI costs → 011, Apple/e-mail receipts → 012 | 009 zajęło utwardzenie i demo (PLAN.md) |

## Ograniczenia

- Adresy publiczne, które przekierowują ruch dalej (np. publiczny reverse proxy do sieci wewnętrznej), są poza zasięgiem kontroli adresu.
- `DENAR_NOTIFY_ALLOW_PRIVATE` dotyczy wszystkich użytkowników instancji — w README zalecana węższa `DENAR_NOTIFY_ALLOWED_CIDRS`.
- `--reset` nie zmienia hasła istniejącego konta demo.
- Dane demo zależą od dnia uruchomienia (daty względne); test sprawdza dwa dni, w tym początek miesiąca.
