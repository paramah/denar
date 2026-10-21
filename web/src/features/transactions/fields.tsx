import { useId, type InputHTMLAttributes, type Ref } from 'react'
import type { Payee } from '../../api/types.ts'
import { Field, inputClass, SelectField } from '../../components/Field/Field.tsx'
import type { EnvelopeOption } from './envelopes.ts'
import styles from './transactions.module.css'

export type EntryMode = 'expense' | 'income' | 'refund'

const modes: { value: EntryMode; label: string; tone: 'outflow' | 'inflow' }[] = [
  { value: 'expense', label: 'Wydatek', tone: 'outflow' },
  { value: 'income', label: 'Wpływ', tone: 'inflow' },
  { value: 'refund', label: 'Zwrot', tone: 'inflow' },
]

/** Rodzaj wpisu: wydatek (−), wpływ (+) albo zwrot (+, wraca do koperty). */
export function ModePicker({ value, onChange }: { value: EntryMode; onChange: (mode: EntryMode) => void }) {
  const name = useId()
  return (
    <fieldset className={styles.segmented}>
      <legend>Rodzaj</legend>
      <div className={styles.segments}>
        {modes.map((mode) => (
          <label key={mode.value} data-tone={mode.tone}>
            <input
              type="radio"
              name={name}
              value={mode.value}
              checked={value === mode.value}
              onChange={() => onChange(mode.value)}
            />
            {mode.label}
          </label>
        ))}
      </div>
    </fieldset>
  )
}

type AmountFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id' | 'onChange' | 'value'> & {
  label: string
  value: string
  onChange: (value: string) => void
  currency: string
  error?: string
  hint?: string
  /** Znak pokazany przed polem (kwota wpisywana jest bez znaku). */
  sign?: '−' | '+'
  large?: boolean
  inputRef?: Ref<HTMLInputElement>
}

/** Pole kwoty: liczba bez znaku, waluta jako przyrostek, klawiatura dziesiętna na telefonie. */
export function AmountField({
  label,
  value,
  onChange,
  currency,
  error,
  hint,
  sign,
  large,
  inputRef,
  className,
  ...input
}: AmountFieldProps) {
  return (
    <Field label={label} error={error} hint={hint} className={className}>
      {(control) => (
        <div className={[styles.amount, large ? styles.amountLarge : ''].filter(Boolean).join(' ')}>
          {sign && (
            <span className={styles.sign} data-tone={sign === '−' ? 'outflow' : 'inflow'} aria-hidden="true">
              {sign}
            </span>
          )}
          <input
            ref={inputRef}
            className={inputClass}
            inputMode="decimal"
            autoComplete="off"
            value={value}
            onChange={(event) => onChange(event.target.value)}
            {...control}
            {...input}
          />
          <span className={styles.suffix} aria-hidden="true">
            {currency}
          </span>
        </div>
      )}
    </Field>
  )
}

/**
 * Kontrahent z podpowiedziami z listy gospodarstwa (natywny datalist). Nowa
 * nazwa trafia do API jako payee_name — serwer dopasuje wzorce albo utworzy
 * kontrahenta.
 */
export function PayeeField({
  value,
  onChange,
  payees,
  error,
  className,
}: {
  value: string
  onChange: (value: string) => void
  payees: Payee[]
  error?: string
  className?: string
}) {
  const listId = useId()
  const known = findPayee(payees, value)
  const hint = value.trim() && !known ? 'Nowy kontrahent — zostanie dodany do listy.' : undefined
  return (
    <Field label="Kontrahent" error={error} hint={hint} className={className}>
      {(control) => (
        <>
          <input
            className={inputClass}
            list={listId}
            value={value}
            onChange={(event) => onChange(event.target.value)}
            autoComplete="off"
            maxLength={100}
            {...control}
          />
          <datalist id={listId}>
            {payees.map((payee) => (
              <option key={payee.id} value={payee.name} />
            ))}
          </datalist>
        </>
      )}
    </Field>
  )
}

/** Kontrahent o tej nazwie (bez wielkości liter). */
export function findPayee(payees: Payee[], name: string): Payee | undefined {
  const wanted = name.trim().toLocaleLowerCase('pl')
  return wanted ? payees.find((payee) => payee.name.toLocaleLowerCase('pl') === wanted) : undefined
}

/** Wybór koperty (część C — gdy features.envelopes). */
export function EnvelopeField({
  value,
  onChange,
  options,
  error,
  label = 'Koperta',
  hideLabel,
}: {
  value: string
  onChange: (value: string) => void
  options: EnvelopeOption[]
  error?: string
  label?: string
  hideLabel?: boolean
}) {
  return (
    <SelectField
      label={label}
      hideLabel={hideLabel}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      error={error}
      options={[{ value: '', label: 'Bez koperty' }, ...options]}
    />
  )
}

/** Wielowierszowe pole tekstowe w stylu pól formularza. */
export function TextAreaField({
  label,
  value,
  onChange,
  error,
  hint,
  maxLength,
  className,
  rows = 2,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  hint?: string
  maxLength?: number
  className?: string
  rows?: number
}) {
  return (
    <Field label={label} error={error} hint={hint} className={className}>
      {(control) => (
        <textarea
          className={`${inputClass} ${styles.textarea}`}
          value={value}
          rows={rows}
          maxLength={maxLength}
          onChange={(event) => onChange(event.target.value)}
          {...control}
        />
      )}
    </Field>
  )
}

/** Tagi wpisane po przecinku → lista (serwer i tak normalizuje). */
export function parseTags(text: string): string[] {
  return text
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}
