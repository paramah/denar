// Aktywny język interfejsu: wybór, zapis w przeglądarce, <html lang> i
// locale dla Intl. Moduł nie importuje inicjalizacji (index.ts), więc mogą go
// używać formattery w lib/ bez cyklu importów: czytają globalną instancję
// i18next, a przed inicjalizacją zachowują się jak po polsku.

import i18next from 'i18next'
import { readStored, storageKeys, writeStored } from '../lib/storage.ts'

export const locales = ['pl', 'en'] as const
export type Locale = (typeof locales)[number]
export const defaultLocale: Locale = 'pl'

/** Locale Intl dla języka (AD-1303: angielski według en-GB). */
const intlLocales: Record<Locale, string> = { pl: 'pl-PL', en: 'en-GB' }

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (locales as readonly string[]).includes(value)
}

/** Tag BCP 47 z przeglądarki („en-US”, „pl”) → obsługiwany język albo null. */
export function matchLanguage(tag: string): Locale | null {
  const base = tag.trim().toLowerCase().split(/[-_]/)[0]
  return isLocale(base) ? base : null
}

/**
 * Kolejność wyboru (FR-1303): preferencja z profilu → wybór zapisany w
 * przeglądarce → pierwszy obsługiwany język z `navigator.languages` → pl.
 */
export function resolveLocale(
  meLocale: Locale | null | undefined,
  stored: string | null | undefined,
  navigatorLanguages: readonly string[] = [],
): Locale {
  if (isLocale(meLocale)) {
    return meLocale
  }
  if (isLocale(stored)) {
    return stored
  }
  for (const tag of navigatorLanguages) {
    const match = matchLanguage(tag)
    if (match) {
      return match
    }
  }
  return defaultLocale
}

export function storedLocale(): Locale | null {
  const value = readStored(storageKeys.locale)
  return isLocale(value) ? value : null
}

export function browserLanguages(): readonly string[] {
  try {
    if (navigator.languages?.length) {
      return navigator.languages
    }
    return navigator.language ? [navigator.language] : []
  } catch {
    return []
  }
}

/** Język, którego interfejs używa teraz. */
export function currentLocale(): Locale {
  const language = i18next.resolvedLanguage ?? i18next.language
  return isLocale(language) ? language : defaultLocale
}

/** Locale dla Intl (`pl-PL`, `en-GB`); domyślnie bieżącego języka. */
export function intlLocale(locale: Locale = currentLocale()): string {
  return intlLocales[locale]
}

export function syncDocumentLanguage(locale: Locale = currentLocale()): void {
  try {
    document.documentElement.lang = locale
  } catch {
    // Poza przeglądarką nie ma dokumentu.
  }
}

let pendingFocusId: string | null = null

/**
 * Identyfikator elementu, który miał fokus w chwili zmiany języka. Zmiana
 * języka montuje drzewo od nowa (LocaleBoundary), więc fokus wraca na element
 * o tym samym `id` — przełączniki języka mają stałe identyfikatory.
 */
export function takePendingFocus(): string | null {
  const id = pendingFocusId
  pendingFocusId = null
  return id
}

export type SetLocaleOptions = {
  /** Zapis w przeglądarce: `true` zapisuje, `'clear'` usuwa zapisany wybór. Domyślnie bez zmian. */
  persist?: boolean | 'clear'
}

/** Przełącza język bez przeładowania strony i ustawia `<html lang>`. */
export function setLocale(locale: Locale, { persist = false }: SetLocaleOptions = {}): void {
  if (persist === 'clear') {
    writeStored(storageKeys.locale, null)
  } else if (persist) {
    writeStored(storageKeys.locale, locale)
  }
  if (locale !== currentLocale()) {
    try {
      const active = document.activeElement
      pendingFocusId = active instanceof HTMLElement && active.id ? active.id : null
    } catch {
      pendingFocusId = null
    }
    // Zasoby są w paczce, więc zmiana jest synchroniczna.
    void i18next.changeLanguage(locale)
  }
  syncDocumentLanguage(locale)
}

/** „Automatycznie”: bez zapisanego wyboru, według przeglądarki. */
export function applyBrowserLocale(): void {
  setLocale(resolveLocale(null, null, browserLanguages()), { persist: 'clear' })
}
