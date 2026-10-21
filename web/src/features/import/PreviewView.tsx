import type { ImportPreview } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { formatDay } from '../../lib/format.ts'
import { useEnvelopeNames } from '../transactions/envelopes.ts'
import { delimiterLabel, encodingLabel } from './model.ts'
import { RowErrors } from './parts.tsx'
import styles from './import.module.css'

/** Wynik podglądu: co parser wykrył, liczniki, pierwsze wiersze i błędy. */
export function PreviewView({ preview, currency }: { preview: ImportPreview; currency: string }) {
  const { household } = useHousehold()
  const envelopes = useEnvelopeNames(household.id)
  const { detected } = preview
  const shownRows = preview.rows.length

  return (
    <div className={styles.preview}>
      <dl className={styles.detected} aria-label="Wykryto w pliku">
        <div>
          <dt>Separator</dt>
          <dd>{delimiterLabel(detected.delimiter)}</dd>
        </div>
        <div>
          <dt>Kodowanie</dt>
          <dd>{encodingLabel(detected.encoding)}</dd>
        </div>
        <div>
          <dt>Nagłówek</dt>
          <dd>{detected.header_line ? `linia ${detected.header_line}` : 'brak'}</dd>
        </div>
        {detected.headers.length > 0 && (
          <div className={styles.detectedWide}>
            <dt>Kolumny</dt>
            <dd>
              <ul className={styles.headerChips}>
                {detected.headers.map((header, index) => (
                  <li key={`${index}:${header}`}>{header || `(kolumna ${index + 1})`}</li>
                ))}
              </ul>
            </dd>
          </div>
        )}
      </dl>

      <dl className={styles.counters} aria-label="Liczniki podglądu">
        <div>
          <dt>Wiersze</dt>
          <dd>{preview.total_rows}</dd>
        </div>
        <div className={styles.counterPositive}>
          <dt>Nowe</dt>
          <dd>{preview.new_count}</dd>
        </div>
        <div>
          <dt>Duplikaty</dt>
          <dd>{preview.duplicate_count}</dd>
        </div>
        <div className={preview.error_count > 0 ? styles.counterWarn : undefined}>
          <dt>Błędy</dt>
          <dd>{preview.error_count}</dd>
        </div>
      </dl>

      {shownRows > 0 && (
        // Przewijany poziomo na wąskim ekranie: focus pozwala przewijać klawiaturą.
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        <div className={styles.tableWrap} role="region" aria-label="Wiersze podglądu" tabIndex={0}>
          <table className={styles.table}>
            <caption className={styles.caption}>
              {shownRows < preview.total_rows - preview.error_count
                ? `Pierwsze ${shownRows} z ${preview.total_rows - preview.error_count} poprawnych wierszy`
                : 'Wszystkie poprawne wiersze'}
            </caption>
            <thead>
              <tr>
                <th scope="col">Linia</th>
                <th scope="col">Data</th>
                <th scope="col">Z pliku</th>
                <th scope="col">Kontrahent</th>
                <th scope="col">Koperta</th>
                <th scope="col" className={styles.num}>
                  Kwota
                </th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {preview.rows.map((row) => (
                <tr key={`${row.row}:${row.external_id}`} data-duplicate={row.status === 'duplicate' || undefined}>
                  <td className={styles.lineNo}>{row.row}</td>
                  <td className={styles.nowrap}>
                    <time dateTime={row.date}>{formatDay(row.date)}</time>
                  </td>
                  <td className={styles.raw}>
                    {row.payee_raw}
                    {row.memo && row.memo !== row.payee_raw && <span className={styles.memo}>{row.memo}</span>}
                  </td>
                  <td>{row.payee_name ?? <span className={styles.muted}>nie rozpoznano</span>}</td>
                  <td>{row.envelope_id ? (envelopes.get(row.envelope_id) ?? 'koperta') : <span className={styles.muted}>—</span>}</td>
                  <td className={styles.num}>
                    <Money amount={row.amount.amount} currency={row.amount.currency || currency} signed />
                  </td>
                  <td>
                    {row.status === 'duplicate' ? <Badge>Duplikat</Badge> : <Badge tone="brand">Nowa</Badge>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <RowErrors errors={preview.errors} total={preview.error_count} />
    </div>
  )
}
