// Inicjalizacja i18next: synchroniczna, zasoby w paczce (bez pobierania), więc
// pierwszy render ma już właściwy język. Import tego modułu (main.tsx, setup
// testów) wystarcza; komponenty używają useTranslation z react-i18next.

import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import { browserLanguages, defaultLocale, locales, resolveLocale, storedLocale, syncDocumentLanguage } from './locale.ts'
import { namespaces, resources } from './resources.ts'

if (!i18next.isInitialized) {
  void i18next.use(initReactI18next).init({
    resources,
    lng: resolveLocale(null, storedLocale(), browserLanguages()),
    fallbackLng: defaultLocale,
    supportedLngs: [...locales],
    ns: namespaces,
    defaultNS: 'common',
    initAsync: false,
    // React sam escapuje tekst.
    interpolation: { escapeValue: false },
    returnEmptyString: false,
    react: { useSuspense: false },
  })
  syncDocumentLanguage()
}

export { i18next as i18n }
