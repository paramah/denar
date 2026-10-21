# Tłumaczenia Web UI (013)

i18next + react-i18next, zasoby w paczce, inicjalizacja synchroniczna (`index.ts`). Języki: `pl` (domyślny, wzorzec typów) i `en`.

## Pliki i klucze

- Jeden **obszar = jeden namespace = para plików** `pl/<obszar>.json` i `en/<obszar>.json`: `common`, `auth`, `dashboard`, `wallets`, `transactions`, `budget`, `subscriptions`, `notifications`, `import`, `settings`, `households`, `profile`, `labels`, `errors`. Obszar `features/<obszar>` pisze tylko do swoich plików.
- Wewnątrz pliku klucze zagnieżdżone, camelCase: `komponent.znaczenie`, np. w `wallets.json` → `list.empty`, w kodzie `t('wallets:list.empty')` albo `useTranslation('wallets')` + `t('list.empty')`.
- `common` — teksty wspólne (powłoka, nawigacja, menu, przyciski typu „Anuluj”, kwoty, czas). `labels` — nazwy wartości z API (`lib/labels.ts`). `errors` — komunikaty serwera i błędy ogólne (`api/messages.ts`, `api/errors.ts`).
- Parametry: `{{name}}`, w obu językach te same (test `i18n.test.ts`).

## Liczba mnoga

Sufiksy i18next z `count`: po polsku `_one`, `_few`, `_many`, `_other` (wszystkie cztery; `_other` = forma `_many`, dotyczy ułamków), po angielsku `_one`, `_other`.

```json
"lengthMax_one": "Najwyżej {{count}} znak.",
"lengthMax_few": "Najwyżej {{count}} znaki.",
"lengthMax_many": "Najwyżej {{count}} znaków.",
"lengthMax_other": "Najwyżej {{count}} znaków."
```

`t('errors:field.lengthMax', { count: n })` — `count` musi być liczbą.

## Jak dodać napis

1. Dopisz klucz do `pl/<obszar>.json` z **dokładnie dotychczasowym polskim brzmieniem** (znak w znak: cudzysłowy „”, półpauzy —, wielokropek …, kropka na końcu). Polskich tekstów nie poprawiamy przy ekstrakcji — testy szukają elementów po polskim tekście.
2. Dopisz ten sam klucz z naturalnym angielskim do `en/<obszar>.json`.
3. W komponencie: `const { t } = useTranslation('<obszar>')`, potem `t('klucz')`. Klucze są typowane (z polskich plików), literówka to błąd `tsc`. Klucze z innego pliku: `useTranslation(['<obszar>', 'common'])` i `t('common:components.close')`. Klucz składany w locie (`t(\`nav.${item}\`)`) działa, gdy typ zmiennej to unia znanych kluczy; inaczej `tk()`.
4. Tekst z elementami w środku: `<Trans>` z react-i18next albo dwa klucze; nie skleja się zdań z kawałków, jeśli szyk w angielskim jest inny.

## Poza komponentami

Moduły bez Reacta (`lib/`, `api/`, `model.ts`) używają `tk('obszar:klucz', params)` z `translate.ts` w chwili wywołania (nie na poziomie modułu!) — stała `const label = tk(...)` na górze pliku zostałaby w języku startowym. Mapy etykiet w `lib/labels.ts` są gettery, więc `roleLabels[role]` działa jak dotąd.

## Formatowanie

Kwoty, daty, liczby: `lib/money.ts`, `lib/format.ts`, `lib/budgetMonth.ts`, `lib/intl.ts` (`formatNumber`, `decimalSeparator`, `perLocale`). Nie pisz `'pl-PL'` na sztywno: `intlLocale()` z `locale.ts` (`pl-PL` / `en-GB`), a własny formatter twórz przez `perLocale((locale) => new Intl.DateTimeFormat(locale, …))` i wywołuj `fmt().format(x)`. Porównania danych użytkownika (wyszukiwanie po nazwie, inicjały) zostają przy `'pl'` — to dane, nie interfejs.

## Zmiana języka

`setLocale(locale, { persist })` z `locale.ts`. `LocaleBoundary` montuje drzewo od nowa po zmianie, więc wszystko (też teksty spoza `useTranslation`) przechodzi na nowy język bez przeładowania; fokus wraca na element o tym samym `id`.

## Testy

Testy Vitest idą po polsku (`test/polish.ts` + `setLocale('pl')` po każdym teście) i szukają elementów po polskim tekście — po ekstrakcji muszą przechodzić bez zmian. Test innego języka: `setLocale('en')` w teście (setup przywraca `pl`). `i18n.test.ts` sprawdza zgodność kluczy, parametrów i form liczby mnogiej we wszystkich plikach.
