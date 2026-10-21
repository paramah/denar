import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  const envelopes = useEnvelopeNames(household.id)
  const { detected } = preview
  const shownRows = preview.rows.length

  return (
    <div className={styles.preview}>
      <dl className={styles.detected} aria-label={t('preview.detected')}>
        <div>
          <dt>{t('preview.delimiter')}</dt>
          <dd>{delimiterLabel(detected.delimiter)}</dd>
        </div>
        <div>
          <dt>{t('preview.encoding')}</dt>
          <dd>{encodingLabel(detected.encoding)}</dd>
        </div>
        <div>
          <dt>{t('preview.header')}</dt>
          <dd>{detected.header_line ? t('preview.headerLine', { line: detected.header_line }) : t('preview.headerNone')}</dd>
        </div>
        {detected.headers.length > 0 && (
          <div className={styles.detectedWide}>
            <dt>{t('preview.columns')}</dt>
            <dd>
              <ul className={styles.headerChips}>
                {detected.headers.map((header, index) => (
                  <li key={`${index}:${header}`}>{header || t('preview.unnamedColumn', { n: index + 1 })}</li>
                ))}
              </ul>
            </dd>
          </div>
        )}
      </dl>

      <dl className={styles.counters} aria-label={t('preview.counters')}>
        <div>
          <dt>{t('preview.rows')}</dt>
          <dd>{preview.total_rows}</dd>
        </div>
        <div className={styles.counterPositive}>
          <dt>{t('preview.new')}</dt>
          <dd>{preview.new_count}</dd>
        </div>
        <div>
          <dt>{t('preview.duplicates')}</dt>
          <dd>{preview.duplicate_count}</dd>
        </div>
        <div className={preview.error_count > 0 ? styles.counterWarn : undefined}>
          <dt>{t('preview.errors')}</dt>
          <dd>{preview.error_count}</dd>
        </div>
      </dl>

      {shownRows > 0 && (
        // Przewijany poziomo na wąskim ekranie: focus pozwala przewijać klawiaturą.
        // eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex
        <div className={styles.tableWrap} role="region" aria-label={t('preview.rowsRegion')} tabIndex={0}>
          <table className={styles.table}>
            <caption className={styles.caption}>
              {shownRows < preview.total_rows - preview.error_count
                ? t('preview.captionFirst', { shown: shownRows, total: preview.total_rows - preview.error_count })
                : t('preview.captionAll')}
            </caption>
            <thead>
              <tr>
                <th scope="col">{t('preview.colLine')}</th>
                <th scope="col">{t('preview.colDate')}</th>
                <th scope="col">{t('preview.colRaw')}</th>
                <th scope="col">{t('preview.colPayee')}</th>
                <th scope="col">{t('preview.colEnvelope')}</th>
                <th scope="col" className={styles.num}>
                  {t('preview.colAmount')}
                </th>
                <th scope="col">{t('preview.colStatus')}</th>
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
                  <td>{row.payee_name ?? <span className={styles.muted}>{t('preview.unrecognized')}</span>}</td>
                  <td>{row.envelope_id ? (envelopes.get(row.envelope_id) ?? t('preview.envelope')) : <span className={styles.muted}>—</span>}</td>
                  <td className={styles.num}>
                    <Money amount={row.amount.amount} currency={row.amount.currency || currency} signed />
                  </td>
                  <td>
                    {row.status === 'duplicate' ? <Badge>{t('preview.duplicate')}</Badge> : <Badge tone="brand">{t('preview.newRow')}</Badge>}
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
