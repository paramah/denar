import type { Key, ReactNode } from 'react'
import { Button } from '../Button/Button.tsx'
import styles from './DataList.module.css'
import { tk } from '../../i18n/translate.ts'

type Props<T> = {
  items: T[]
  getKey: (item: T) => Key
  renderItem: (item: T) => ReactNode
  /** Dostępna nazwa listy. */
  label: string
  /** Co pokazać przy pustej liście (zwykle EmptyState). */
  empty?: ReactNode
  /** Stronicowanie: są kolejne strony. */
  hasMore?: boolean
  onLoadMore?: () => void
  loadingMore?: boolean
  loadMoreLabel?: string
  /** Wiersze jako karty z obramowaniem (domyślnie) albo lista z liniami. */
  variant?: 'rows' | 'cards'
}

/**
 * Lista danych z pustym stanem i „Pokaż więcej”. Rejestr transakcji
 * (część B) może dodać wirtualizację wewnątrz bez zmiany tego API.
 */
export function DataList<T>({
  items,
  getKey,
  renderItem,
  label,
  empty,
  hasMore,
  onLoadMore,
  loadingMore,
  loadMoreLabel = tk('common:components.loadMore'),
  variant = 'rows',
}: Props<T>) {
  if (items.length === 0 && empty) {
    return <>{empty}</>
  }
  return (
    <div className={styles.wrap}>
      <ul className={`${styles.list} ${styles[variant]}`} aria-label={label}>
        {items.map((item) => (
          <li key={getKey(item)} className={styles.item}>
            {renderItem(item)}
          </li>
        ))}
      </ul>
      {hasMore && onLoadMore && (
        <div className={styles.more}>
          <Button onClick={onLoadMore} busy={loadingMore}>
            {loadMoreLabel}
          </Button>
        </div>
      )}
    </div>
  )
}
