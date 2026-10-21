import { afterEach, describe, expect, it } from 'vitest'
import { i18n } from './index.ts'
import { currentLocale, intlLocale, matchLanguage, resolveLocale, setLocale, storedLocale, applyBrowserLocale } from './locale.ts'
import { readStored, storageKeys, writeStored } from '../lib/storage.ts'

describe('resolveLocale', () => {
  it('prefers the profile, then the browser choice, then navigator.languages, then pl', () => {
    expect(resolveLocale('en', 'pl', ['pl-PL'])).toBe('en')
    expect(resolveLocale(null, 'en', ['pl-PL'])).toBe('en')
    expect(resolveLocale(undefined, null, ['de-DE', 'en-US', 'pl'])).toBe('en')
    expect(resolveLocale(null, null, ['de-DE', 'fr'])).toBe('pl')
    expect(resolveLocale(null, null, [])).toBe('pl')
  })

  it('ignores unknown stored values', () => {
    expect(resolveLocale(null, 'de', ['en-GB'])).toBe('en')
    expect(resolveLocale(null, '', [])).toBe('pl')
  })

  it('matches language tags by their base', () => {
    expect(matchLanguage('en-US')).toBe('en')
    expect(matchLanguage('PL')).toBe('pl')
    expect(matchLanguage('pl_PL')).toBe('pl')
    expect(matchLanguage('de')).toBeNull()
  })
})

describe('setLocale', () => {
  afterEach(() => setLocale('pl'))

  it('switches i18next, <html lang> and the Intl locale', () => {
    expect(currentLocale()).toBe('pl')
    expect(intlLocale()).toBe('pl-PL')
    setLocale('en')
    expect(currentLocale()).toBe('en')
    expect(i18n.t('nav.wallets')).toBe('Wallets')
    expect(document.documentElement.lang).toBe('en')
    expect(intlLocale()).toBe('en-GB')
  })

  it('saves the choice in the browser only when asked', () => {
    setLocale('en')
    expect(storedLocale()).toBeNull()
    setLocale('en', { persist: true })
    expect(readStored(storageKeys.locale)).toBe('en')
    setLocale('pl', { persist: 'clear' })
    expect(readStored(storageKeys.locale)).toBeNull()
  })

  it('goes back to the browser language for “automatic”', () => {
    writeStored(storageKeys.locale, 'en')
    setLocale('en')
    applyBrowserLocale() // the test browser speaks Polish (test/polish.ts)
    expect(currentLocale()).toBe('pl')
    expect(storedLocale()).toBeNull()
  })
})
