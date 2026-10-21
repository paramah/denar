import { useTranslation } from 'react-i18next'
import { locales, setLocale } from '../../i18n/locale.ts'
import styles from './auth.module.css'

/**
 * Język przed zalogowaniem (FR-1304): wybór zapisany w przeglądarce. Nazwy
 * języków zawsze we własnym języku, z atrybutem lang dla czytników ekranu.
 */
export function LanguageSwitcher() {
  const { t, i18n } = useTranslation()
  const active = i18n.resolvedLanguage
  return (
    <div role="group" aria-label={t('language.label')} className={styles.languages}>
      {locales.map((locale) => (
        <button
          key={locale}
          data-focus-key={`language-${locale}`}
          type="button"
          lang={locale}
          aria-pressed={active === locale}
          className={styles.languageButton}
          onClick={() => setLocale(locale, { persist: true })}
        >
          {t(`language.${locale}`)}
        </button>
      ))}
    </div>
  )
}
