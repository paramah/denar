import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useCurrencies } from '../../api/hooks/households.ts'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { timeZones } from '../../lib/format.ts'
import { perLocale } from '../../lib/intl.ts'
import styles from './households.module.css'

export type HouseholdValues = {
  name: string
  base_currency: string
  timezone: string
  month_start_day: string
}

type Props = {
  values: HouseholdValues
  onChange: (values: HouseholdValues) => void
  errors: Record<string, string | undefined>
  /** Waluty bazowej nie można zmienić po założeniu. */
  currencyLocked?: boolean
  readOnly?: boolean
}

const currencyNames = perLocale((locale) => {
  try {
    return new Intl.DisplayNames(locale, { type: 'currency' })
  } catch {
    return null
  }
})

const days = Array.from({ length: 28 }, (_, i) => String(i + 1))

/** Pola gospodarstwa wspólne dla zakładania i ustawień. */
export function HouseholdFields({ values, onChange, errors, currencyLocked, readOnly }: Props) {
  const { t } = useTranslation('households')
  const currencies = useCurrencies()
  const zones = useMemo(() => {
    const list = timeZones()
    return list.includes(values.timezone) ? list : [values.timezone, ...list]
  }, [values.timezone])
  const set = (patch: Partial<HouseholdValues>) => onChange({ ...values, ...patch })

  const currencyOptions = (currencies.data ?? [{ code: values.base_currency, decimals: 2 }]).map((c) => ({
    value: c.code,
    label: t('fields.currencyOption', { code: c.code, name: currencyNames()?.of(c.code) ?? c.code }),
  }))

  return (
    <div className={styles.fields}>
      <TextField
        label={t('fields.name')}
        value={values.name}
        onChange={(event) => set({ name: event.target.value })}
        error={errors.name}
        hint={t('fields.nameHint')}
        maxLength={100}
        readOnly={readOnly}
        required
      />
      {currencyLocked ? (
        <TextField
          label={t('fields.baseCurrency')}
          value={currencyOptions.find((o) => o.value === values.base_currency)?.label ?? values.base_currency}
          readOnly
          hint={t('fields.currencyLockedHint')}
        />
      ) : (
        <SelectField
          label={t('fields.baseCurrency')}
          value={values.base_currency}
          onChange={(event) => set({ base_currency: event.target.value })}
          options={currencyOptions}
          error={errors.base_currency}
          hint={t('fields.currencyHint')}
          disabled={currencies.isPending}
        />
      )}
      <div className={styles.pair}>
        <SelectField
          label={t('fields.timezone')}
          value={values.timezone}
          onChange={(event) => set({ timezone: event.target.value })}
          options={zones.map((zone) => ({ value: zone, label: zone.replaceAll('_', ' ') }))}
          error={errors.timezone}
          disabled={readOnly}
        />
        <SelectField
          label={t('fields.monthStart')}
          value={values.month_start_day}
          onChange={(event) => set({ month_start_day: event.target.value })}
          options={days.map((day) => ({ value: day, label: t('fields.monthStartDay', { day }) }))}
          error={errors.month_start_day}
          hint={t('fields.monthStartHint')}
          disabled={readOnly}
        />
      </div>
    </div>
  )
}
