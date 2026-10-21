import type { ReactNode } from 'react'
import { Icon, type IconName } from '../Icon/Icon.tsx'
import styles from './EmptyState.module.css'

type Props = {
  icon: IconName
  title: string
  children?: ReactNode
  /** Przycisk lub link prowadzący do pierwszego kroku. */
  action?: ReactNode
  /** h2 domyślnie; h1, gdy pusty stan jest całą stroną. */
  headingLevel?: 1 | 2 | 3
  /** Mała plakietka nad tytułem, np. „Wkrótce”. */
  badge?: ReactNode
}

/** Pusty widok jako zaproszenie do działania. */
export function EmptyState({ icon, title, children, action, headingLevel = 2, badge }: Props) {
  const Heading = `h${headingLevel}` as const
  return (
    <section className={styles.empty} aria-label={title}>
      <div className={styles.coin} aria-hidden="true">
        <Icon name={icon} size={28} />
      </div>
      {badge}
      <Heading className={styles.title}>{title}</Heading>
      {children && <div className={styles.text}>{children}</div>}
      {action && <div className={styles.action}>{action}</div>}
    </section>
  )
}
