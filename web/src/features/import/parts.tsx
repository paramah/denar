import { translateFieldMessage } from '../../api/messages.ts'
import type { ImportBatch, ImportRowError } from '../../api/types.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { rowFieldLabel } from './model.ts'
import styles from './import.module.css'

/** Liczniki partii: utworzone, duplikaty, błędy, dopasowane do subskrypcji (i cofnięte). */
export function BatchCounters({ batch }: { batch: ImportBatch }) {
  const items: [string, number, string?][] = [
    ['Wiersze w pliku', batch.total_rows],
    ['Nowe transakcje', batch.created_count, styles.counterPositive],
    ['Duplikaty pominięte', batch.duplicate_count],
    ['Błędy', batch.error_count, batch.error_count > 0 ? styles.counterWarn : undefined],
    ['Dopasowane do subskrypcji', batch.matched_count],
  ]
  if (batch.status === 'reverted') {
    items.push(['Usunięte przy cofnięciu', batch.reverted_count])
  }
  return (
    <dl className={styles.counters} aria-label="Liczniki importu">
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
  return batch.status === 'reverted' ? <Badge tone="warn">Cofnięto</Badge> : <Badge tone="positive">Zaimportowano</Badge>
}

/** Błędy wierszy: linia pliku, pole i komunikat po polsku. */
export function RowErrors({ errors, total, label = 'Błędy wierszy' }: { errors: ImportRowError[]; total: number; label?: string }) {
  if (errors.length === 0) {
    return null
  }
  return (
    <section className={styles.errors} aria-label={label}>
      <h3 className={styles.subTitle}>
        {label} ({total})
      </h3>
      <ul className={styles.errorList}>
        {errors.map((error, index) => (
          <li key={`${error.row}:${error.field}:${index}`}>
            <span className={styles.errorRow}>{error.row > 0 ? `Linia ${error.row}` : 'Cały plik'}</span>
            <span>
              {rowFieldLabel(error.field)}: {translateFieldMessage(error.message)}
            </span>
          </li>
        ))}
      </ul>
      {total > errors.length && (
        <p className={styles.muted}>Pokazano pierwsze {errors.length} z {total} błędów.</p>
      )}
    </section>
  )
}
