# Brief dla agentów (przebieg autonomiczny)

> Last Updated: 2026-10-21

Jesteś wykonawcą jednego etapu projektu **Denar** (`/Users/paramah/Projects/eiac.dev/tools/denar`). Użytkownik śpi — **nie zadajesz pytań**; rozstrzygasz sam i dokumentujesz.

## Przeczytaj najpierw

1. `AGENTS.md`, `.ai/steering/*.md`, `.ai/sdd/PLAN.md` (całość — model domeny, etapy, tryb autonomiczny).
2. Specyfikacje wcześniejszych etapów w `.ai/sdd/specs/` (przynajmniej `design.md`, żeby znać istniejące porty, tabele i konwencje).
3. Wzorcową implementację **Straylight**: `/Users/paramah/Projects/eiac.dev/tools/para-board` (moduł `git.eiac.dev/tools/straylight`). Kopiuj i adaptuj sprawdzone fragmenty (config, sqlite, tx, migracje, rest/problem/middleware/auth, oidc, mcp, spa, platform/*, web/ setup, Taskfile, Dockerfile) zamiast pisać od zera. Zmieniaj nazwy: `straylight`→`denar`, `STRAYLIGHT_`→`DENAR_`, `slg_`→`dnr_`, moduł → `github.com/paramah/denar`.

## Proces etapu

1. Jeśli brak `requirements.md` — napisz go (format jak `001-core/requirements.md`, EARS, zwięźle), ustaw `.status` na `requirements:approved # delegated`.
2. Napisz `design.md` (zwięźle, ~200–500 linii: architektura, domena, porty, schemat SQL, endpointy, narzędzia, przypadki brzegowe, strategia testów, sekcja **Decyzje autonomiczne**). Status `design:approved # delegated`.
3. Napisz `tasks.md` (lista zadań T-NNN z plikami, kryteriami, weryfikacją; checkboxy). Status `tasks:approved # delegated`, potem `implementation:in-progress`.
4. Implementuj TDD, zadanie po zadaniu; odhaczaj w `tasks.md` po zielonych testach.
5. Zakończ: `task verify` zielone → status `implementation:done`, dopisz wiersze do tabeli w `PLAN.md` (sekcja „Tryb autonomiczny”) i zaktualizuj `.ai/sdd/INDEX.md`.
6. Commituj po każdym spójnym kawałku (konwencja: `feat(scope): …`, `docs(sdd): …`, `test: …`), komunikat po polsku lub angielsku, zakończony linią:
   `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
   Nigdy `git push`. Nie commituj binariów, `.db`, `node_modules`, `.env`.

## Twarde reguły

- Reguła zależności `adapter → usecase → domain`; `internal/arch_test.go` musi być zielony.
- Pieniądze: `int64` jednostek minimalnych + waluta; zero `float64` na kwotach (również w JSON — kwoty jako string dziesiętny + `currency`, ewentualnie `minor` jako integer).
- Autoryzacja w use case'ach; ukryte → `ErrNotFound`; bot = scope ∩ uprawnienia twórcy ∩ lista portfeli; boty bez operacji administracyjnych.
- Audit log w tej samej transakcji co zmiana.
- Kontrakt `api/openapi.yaml` aktualizujesz w swoim etapie (jesteś jedynym piszącym backend w danej chwili); po zmianie regeneruj kod (`go generate ./...`) i — jeśli istnieje `web/` — typy TS (`task web:types`).
- Migracje: nowy plik `NNNNN_nazwa.sql`, nigdy nie edytuj migracji z wcześniejszych etapów.
- Wersje bibliotek: preferuj te z `para-board/go.mod`/`web/package.json` (znane, działające). Przy wątpliwościach co do API użyj Context7.
- Nie dotykaj plików spoza swojego etapu, chyba że to konieczne do integracji (wtedy minimalnie i opisz w design.md).
- Nie zostawiaj czerwonych testów. Jeśli coś nie wychodzi — uprość zakres, zapisz to w „Decyzje autonomiczne” / „Ograniczenia” i dowieź działającą całość.

## Raport końcowy (Twoja ostatnia wiadomość)

Krótko: co zrobione, hashe commitów, wynik `task verify` (ostatnie linie), decyzje autonomiczne, znane ograniczenia, co następny etap musi wiedzieć (porty, tabele, endpointy).
