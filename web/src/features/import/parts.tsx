import { useTranslation } from 'react-i18next'
import { translateFieldMessage } from '../../api/messages.ts'
import type { ImportBatch, ImportRowError } from '../../api/types.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { rowFieldLabel } from './model.ts'
import styles from './import.module.css'

/** Liczniki partii: utworzone, duplikaty, błędy, dopasowane do subskrypcji (i cofnięte). */
export function BatchCounters({ batch }: { batch: ImportBatch }) {
  const { t } = useTranslation('import')
  const items: [string, number, string?][] = [
    [t('counters.rows'), batch.total_rows],
    [t('counters.created'), batch.created_count, styles.counterPositive],
    [t('counters.duplicates'), batch.duplicate_count],
    [t('counters.errors'), batch.error_count, batch.error_count > 0 ? styles.counterWarn : undefined],
    [t('counters.matched'), batch.matched_count],
  ]
  if (batch.status === 'reverted') {
    items.push([t('counters.reverted'), batch.reverted_count])
  }
  return (
    <dl className={styles.counters} aria-label={t('counters.label')}>
      {items.map(([label, value, tone]) => (
        <div key={label} className={tone}>
          <dt>{label}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  )
}

export function BatchStatus({ batch }: { batch: ImportBatch }) {
  const { t } = useTranslation('import')
  return batch.status === 'reverted' ? (
    <Badge tone="warn">{t('status.reverted')}</Badge>
  ) : (
    <Badge tone="positive">{t('status.completed')}</Badge>
  )
}

/** Błędy wierszy: linia pliku, pole i komunikat w aktywnym języku. */
export function RowErrors({ errors, total, label }: { errors: ImportRowError[]; total: number; label?: string }) {
  const { t } = useTranslation('import')
  const title = label ?? t('rowErrors.label')
  if (errors.length === 0) {
    return null
  }
  return (
    <section className={styles.errors} aria-label={title}>
      <h3 className={styles.subTitle}>{t('rowErrors.heading', { label: title, total })}</h3>
      <ul className={styles.errorList}>
        {errors.map((error, index) => (
          <li key={`${error.row}:${error.field}:${index}`}>
            <span className={styles.errorRow}>{error.row > 0 ? t('rowErrors.line', { row: error.row }) : t('rowErrors.wholeFile')}</span>
            <span>
              {t('rowErrors.message', { field: rowFieldLabel(error.field), message: translateFieldMessage(error.message) })}
            </span>
          </li>
        ))}
      </ul>
      {total > errors.length && (
        <p className={styles.muted}>{t('rowErrors.shownFirst', { shown: errors.length, total })}</p>
      )}
    </section>
  )
}
