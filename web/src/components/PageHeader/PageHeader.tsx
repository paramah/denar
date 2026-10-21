import { useEffect, type ReactNode } from 'react'
import styles from './PageHeader.module.css'

type Props = {
  title: string
  description?: ReactNode
  actions?: ReactNode
  /** Tekst pod nazwą aplikacji w karcie przeglądarki; domyślnie title. */
  documentTitle?: string
}

/** Nagłówek strony (jedyny h1) i tytuł karty przeglądarki. */
export function PageHeader({ title, description, actions, documentTitle }: Props) {
  useDocumentTitle(documentTitle ?? title)
  return (
    <header className={styles.header}>
      <div className={styles.text}>
        <h1 className={styles.title}>{title}</h1>
        {description && <div className={styles.description}>{description}</div>}
      </div>
      {actions && <div className={styles.actions}>{actions}</div>}
    </header>
  )
}

export function useDocumentTitle(title: string) {
  useEffect(() => {
    document.title = title ? `${title} · Denar` : 'Denar'
  }, [title])
}
