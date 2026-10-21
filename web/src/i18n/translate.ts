// t() poza komponentami (lib/, api/): globalna instancja i18next. Komponenty
// używają useTranslation — wtedy zmiana języka odświeża je same. Wynik t()
// tutaj jest aktualny w chwili wywołania; drzewo i tak montuje się od nowa po
// zmianie języka (LocaleBoundary), więc wartości liczone w renderze są świeże.

import i18next, { type TOptions } from 'i18next'

/**
 * t() dla klucza składanego w locie (np. `labels:role.${role}`), którego typy
 * nie sprawdzą. Brak klucza zwraca sam klucz — testy zgodności pl/en i
 * `hasKey` pilnują, by do tego nie doszło.
 */
export function tk(key: string, options?: TOptions): string {
  return (i18next.t as unknown as (key: string, options?: TOptions) => string)(key, options)
}

export function hasKey(key: string): boolean {
  return i18next.exists(key)
}
