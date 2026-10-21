import type { ReactNode } from 'react'
import { useAuthConfig } from '../../api/hooks/session.ts'
import { CoinMark, MilledRing } from '../../components/Icon/Icon.tsx'
import { SkipLink } from '../../components/SkipLink/SkipLink.tsx'
import styles from './auth.module.css'

/** Ekrany przed zalogowaniem: awers z marką i nazwą instancji, rewers z formularzem. */
export function AuthLayout({ children }: { children: ReactNode }) {
  const config = useAuthConfig()
  const instance = config.data?.instance_name ?? 'Denar'
  return (
    <div className={styles.layout}>
      <SkipLink />
      <div className={styles.obverse}>
        <MilledRing className={styles.ring} />
        <div className={styles.mark}>
          <CoinMark size={88} />
        </div>
        <p className={styles.instance}>{instance}</p>
        <p className={styles.tagline}>Budżet domowy, portfele i subskrypcje — na Twoim serwerze.</p>
      </div>
      <main id="main" className={styles.reverse} tabIndex={-1}>
        <div className={styles.panel}>{children}</div>
        {config.data?.version && <p className={styles.version}>Denar {config.data.version}</p>}
      </main>
    </div>
  )
}
