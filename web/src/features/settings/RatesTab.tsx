import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { fieldErrors } from '../../api/errors.ts'
import { useFxOverrides, useFxQuote, useSetFxOverride } from '../../api/hooks/fx.ts'
import { useCurrencies } from '../../api/hooks/households.ts'
import type { FxQuote } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDateTime, formatDay, todayIn } from '../../lib/format.ts'
import { fxSourceLabels } from '../../lib/labels.ts'
import { formatRate, parseRateInput } from '../../lib/money.ts'
import styles from './settings.module.css'

export function RatesTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('rates.title'))
  const { household, role } = useHousehold()
  const overrides = useFxOverrides(household.id)

  return (
    <>
      <section className={styles.section} aria-labelledby="quote-heading">
        <div className={styles.sectionHead}>
          <div>
            <h2 id="quote-heading">{t('rates.title')}</h2>
            <p>{t('rates.lead', { base: household.base_currency })}</p>
          </div>
        </div>
        <QuoteChecker />
      </section>

      {canManage(role) && <SetRateForm />}

      <section className={styles.section} aria-labelledby="overrides-heading">
        <h2 id="overrides-heading">{t('rates.overrides')}</h2>
        <QueryState query={overrides} errorTitle={t('rates.loadFailed')}>
          {(items) => (
            <DataList
              label={t('rates.overrides')}
              items={items}
              getKey={(o) => `${o.currency}/${o.date}`}
              empty={
                <EmptyState icon="rate" title={t('rates.empty.title')} headingLevel={3}>
                  <p>{t('rates.empty.body')}</p>
                </EmptyState>
              }
              renderItem={(o) => (
                <article className={styles.row} aria-label={t('rates.rowLabel', { currency: o.currency, date: formatDay(o.date) })}>
                  <div className={styles.rowMain}>
                    <h3 className={styles.rowTitle}>
                      1 {o.currency} = {formatRate(o.rate)} {household.base_currency}
                    </h3>
                    <span className={styles.rowMeta}>
                      {t('rates.rowMeta', { date: formatDay(o.date), created: formatDateTime(o.created_at) })}
                    </span>
                  </div>
                </article>
              )}
            />
          )}
        </QueryState>
      </section>
    </>
  )
}

function currencyOptions(codes: string[]) {
  return codes.map((code) => ({ value: code, label: code }))
}

function QuoteChecker() {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const currencies = useCurrencies()
  const foreign = (currencies.data ?? []).map((c) => c.code).filter((c) => c !== household.base_currency)
  const [currency, setCurrency] = useState('')
  const [date, setDate] = useState(() => todayIn(household.timezone))
  const chosen = currency || (foreign.includes('EUR') ? 'EUR' : (foreign[0] ?? ''))
  const quote = useFxQuote(household.id, chosen, /^\d{4}-\d{2}-\d{2}$/.test(date) ? date : '')

  return (
    <div className={`${styles.card} ${styles.section}`}>
      <div className={styles.filters}>
        <SelectField
          label={t('rates.currency')}
          value={chosen}
          onChange={(event) => setCurrency(event.target.value)}
          options={currencyOptions(foreign)}
        />
        <TextField label={t('rates.day')} type="date" value={date} onChange={(event) => setDate(event.target.value)} />
      </div>
      <div aria-live="polite">
        {quote.isSuccess && <QuoteResult quote={quote.data} />}
        {quote.isError && <FormAlert error={quote.error} />}
      </div>
    </div>
  )
}

function QuoteResult({ quote }: { quote: FxQuote }) {
  const { t } = useTranslation('settings')
  if (quote.rate === null || quote.source === 'unavailable') {
    return (
      <Callout tone="warn" title={t('rates.quote.missingTitle')}>
        {t('rates.quote.missingBody', { currency: quote.currency, date: formatDay(quote.date) })}
      </Callout>
    )
  }
  return (
    <p className={styles.quote}>
      <strong>
        1 {quote.currency} = {formatRate(quote.rate)} {quote.base_currency}
      </strong>{' '}
      <Badge tone={quote.source === 'manual' ? 'brand' : 'neutral'}>{fxSourceLabels[quote.source]}</Badge>
      {quote.rate_date && quote.rate_date !== quote.date && (
        <span className={styles.rowMeta}>{t('rates.quote.ratedOn', { date: formatDay(quote.rate_date) })}</span>
      )}
    </p>
  )
}

function SetRateForm() {
  const { household } = useHousehold()
  const currencies = useCurrencies()
  const [params] = useSearchParams()
  const { t } = useTranslation('settings')
  const set = useSetFxOverride(household.id)
  const toast = useToast()
  const foreign = (currencies.data ?? []).map((c) => c.code).filter((c) => c !== household.base_currency)
  const [currency, setCurrency] = useState(params.get('currency') ?? '')
  const [date, setDate] = useState(() => params.get('date') ?? todayIn(household.timezone))
  const [rate, setRate] = useState('')
  const [error, setError] = useState<string>()
  const chosen = currency && foreign.includes(currency) ? currency : (foreign.includes('USD') ? 'USD' : (foreign[0] ?? ''))
  const server = fieldErrors(set.error)

  const submit = async () => {
    const parsed = parseRateInput(rate)
    if (!parsed.ok) {
      setError(parsed.error)
      return
    }
    setError(undefined)
    try {
      const saved = await set.mutateAsync({ currency: chosen, date, rate: parsed.value })
      toast.show({ message: t('rates.set.done', { currency: saved.currency, date: formatDay(saved.date) }) })
      setRate('')
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <section className={styles.section} aria-labelledby="set-rate-heading">
      <h2 id="set-rate-heading">{t('rates.set.title')}</h2>
      <p className={styles.rowMeta}>{t('rates.set.lead', { base: household.base_currency })}</p>
      <form
        noValidate
        aria-label={t('rates.set.title')}
        className={`${styles.card} ${styles.section}`}
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <FormAlert error={set.error} shownFields={['currency', 'date', 'rate']} />
        <div className={styles.filters}>
          <SelectField
            label={t('rates.currency')}
            value={chosen}
            onChange={(event) => setCurrency(event.target.value)}
            options={currencyOptions(foreign)}
            error={server.currency}
          />
          <TextField label={t('rates.set.date')} type="date" value={date} onChange={(event) => setDate(event.target.value)} error={server.date} />
          <TextField
            label={t('rates.set.rate', { base: household.base_currency, currency: chosen })}
            value={rate}
            onChange={(event) => setRate(event.target.value)}
            inputMode="decimal"
            autoComplete="off"
            placeholder={t('rates.set.placeholder')}
            error={error ?? server.rate}
          />
        </div>
        <div className={styles.rowActions}>
          <Button type="submit" variant="primary" icon="check" busy={set.isPending}>
            {t('rates.set.submit')}
          </Button>
        </div>
      </form>
    </section>
  )
}
