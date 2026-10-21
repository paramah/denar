import type { UseQueryResult } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { errorMessage } from '../../api/errors.ts'
import { Button } from '../Button/Button.tsx'
import { Icon } from '../Icon/Icon.tsx'
import { Skeleton } from '../Skeleton/Skeleton.tsx'
import styles from './QueryState.module.css'
import { tk } from '../../i18n/translate.ts'

type Props<T> = {
  query: Pick<UseQueryResult<T>, 'data' | 'error' | 'isPending' | 'isError' | 'refetch' | 'isFetching'>
  children: (data: T) => ReactNode
  /** Co pokazać w trakcie ładowania; domyślnie Skeleton. */
  loading?: ReactNode
  errorTitle?: string
}

/** Ładowanie, błąd z „Spróbuj ponownie” albo treść z danymi zapytania. */
export function QueryState<T>({ query, children, loading, errorTitle = tk('common:components.loadFailed') }: Props<T>) {
  if (query.isPending) {
    return <>{loading ?? <Skeleton />}</>
  }
  if (query.isError) {
    return <SectionError title={errorTitle} error={query.error} onRetry={() => void query.refetch()} busy={query.isFetching} />
  }
  return <>{children(query.data as T)}</>
}

export function SectionError({
  title,
  error,
  onRetry,
  busy,
}: {
  title: string
  error: unknown
  onRetry?: () => void
  busy?: boolean
}) {
  return (
    <div role="alert" className={styles.error}>
      <Icon name="alert" />
      <div className={styles.text}>
        <p className={styles.title}>{title}</p>
        <p>{errorMessage(error)}</p>
      </div>
      {onRetry && (
        <Button size="sm" icon="refresh" onClick={onRetry} busy={busy}>
          {tk('common:components.retry')}
        </Button>
      )}
    </div>
  )
}
