import type { ReactNode } from 'react'
import styles from './Badge.module.css'

type Props = {
  tone?: 'neutral' | 'positive' | 'negative' | 'warn' | 'brand'
  children: ReactNode
  /** Przekreślona (np. scope, którego rola twórcy już nie daje). */
  struck?: boolean
  title?: string
}

export function Badge({ tone = 'neutral', children, struck, title }: Props) {
  return (
    <span className={`${styles.badge} ${styles[tone]}`} data-struck={struck || undefined} title={title}>
      {children}
    </span>
  )
}
