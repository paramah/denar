// Formattery Intl dla aktywnego języka. Tworzenie Intl.*Format jest drogie,
// więc każdy powstaje raz na locale i czeka w pamięci podręcznej; zmiana języka
// po prostu sięga po inny wpis.

import { intlLocale } from '../i18n/locale.ts'

/**
 * Leniwa fabryka formattera per locale, np.
 * `const dateFormat = perLocale((l) => new Intl.DateTimeFormat(l, {...}))`,
 * potem `dateFormat().format(date)`.
 */
export function perLocale<T>(make: (locale: string) => T): (locale?: string) => T {
  const cache = new Map<string, T>()
  return (locale = intlLocale()) => {
    let value = cache.get(locale)
    if (value === undefined) {
      value = make(locale)
      cache.set(locale, value)
    }
    return value
  }
}

const decimalSeparators = perLocale(
  (locale) => new Intl.NumberFormat(locale).formatToParts(1.5).find((part) => part.type === 'decimal')?.value ?? '.',
)

/** Separator dziesiętny aktywnego języka („,” po polsku, „.” po angielsku). */
export function decimalSeparator(): string {
  return decimalSeparators()
}

/** Liczba w aktywnym języku, np. rozmiar pliku „1,5” / „1.5”. */
export function formatNumber(value: number, options: Intl.NumberFormatOptions = {}): string {
  return value.toLocaleString(intlLocale(), options)
}
