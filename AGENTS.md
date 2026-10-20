# Denar — instrukcje dla agentów

Self-hosted budżet domowy: portfele, transakcje, koperty (YNAB), subskrypcje. Serwer Go (Clean Architecture), SQLite, REST + MCP, React SPA.

## Zanim zaczniesz

- Przeczytaj `.ai/steering/` (produkt, stack, konwencje, zasady) i `.ai/sdd/PLAN.md`.
- Pracuj według specyfikacji etapu w `.ai/sdd/specs/NNN-*/`. Implementacja wymaga statusu `tasks:approved`; statusy `*:approved` ustawia tylko człowiek.

## Reguły

- Zależności tylko do środka: `adapter → usecase → domain`. Pilnuje tego `internal/arch_test.go`.
- Reguły biznesowe (salda, koperty, cykle subskrypcji, uprawnienia) należą do `internal/domain` i `internal/usecase`, nie do adapterów.
- Kwoty wyłącznie jako `int64` w jednostkach minimalnych + waluta. Żadnych `float64` dla pieniędzy.
- Każdy use case operujący na danych portfela filtruje przez widoczne portfele principala; ukryte zasoby zwracają `ErrNotFound`.
- Kontrakty `api/openapi.yaml`, `api/mcp-tools.md` i `internal/usecase/ports.go` zmienia tylko orkiestrator. Jeśli kontrakt Ci nie wystarcza, zgłoś to zamiast go zmieniać.
- Najpierw test, potem kod.

## Weryfikacja

```sh
task verify   # gofmt, go vet, go test -race, web build + test, go build
```
