import { useEffect } from 'react'
import { isLocale, setLocale, type Locale } from './locale.ts'

/**
 * Preferencja z profilu (`me.user.locale`) wygrywa z wyborem w przeglądarce
 * (FR-1303). Zapisujemy ją też w przeglądarce, żeby ekran logowania po
 * wylogowaniu mówił tym samym językiem. Brak preferencji (null) — zostaje
 * wybór przeglądarki.
 */
export function useProfileLocale(locale: Locale | null | undefined): void {
  useEffect(() => {
    if (isLocale(locale)) {
      setLocale(locale, { persist: true })
    }
  }, [locale])
}
