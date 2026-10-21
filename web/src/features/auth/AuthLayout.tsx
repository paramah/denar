import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthConfig } from '../../api/hooks/session.ts'
import { CoinMark, MilledRing } from '../../components/Icon/Icon.tsx'
import { SkipLink } from '../../components/SkipLink/SkipLink.tsx'
import styles from './auth.module.css'
import { LanguageSwitcher } from './LanguageSwitcher.tsx'

/** Ekrany przed zalogowaniem: awers z marką i nazwą instancji, rewers z formularzem. */
export function AuthLayout({ children }: { children: ReactNode }) {
  const config = useAuthConfig()
  const { t } = useTranslation(['auth', 'common'])
  const instance = config.data?.instance_name ?? t('common:appName')
  return (
    <div className={styles.layout}>
      <SkipLink />
      <div className={styles.obverse}>
        <MilledRing className={styles.ring} />
        <div className={styles.mark}>
          <CoinMark size={88} />
        </div>
        <p className={styles.instance}>{instance}</p>
        <p className={styles.tagline}>{t('layout.tagline')}</p>
      </div>
      <main id="main" className={styles.reverse} tabIndex={-1}>
        <LanguageSwitcher />
        <div className={styles.panel}>{children}</div>
        {config.data?.version && <p className={styles.version}>{t('layout.version', { version: config.data.version })}</p>}
      </main>
    </div>
  )
}
