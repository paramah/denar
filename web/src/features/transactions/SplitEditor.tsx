import { Button, IconButton } from '../../components/Button/Button.tsx'
import { TextField } from '../../components/Field/Field.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { fromMinor, parseAmountInput, toMinor } from '../../lib/money.ts'
import type { EnvelopeOption } from './envelopes.ts'
import { AmountField, EnvelopeField } from './fields.tsx'
import styles from './transactions.module.css'

export type SplitDraft = { key: string; envelopeId: string; amount: string; note: string }

let nextKey = 0
export function newSplitDraft(partial: Partial<Omit<SplitDraft, 'key'>> = {}): SplitDraft {
  nextKey += 1
  return { key: `split-${nextKey}`, envelopeId: '', amount: '', note: '', ...partial }
}

export type SplitCheck = {
  /** Suma poprawnych linii (bez znaku), w jednostkach minimalnych. */
  assigned: bigint
  /** Kwota transakcji minus suma linii; null, gdy kwota transakcji jest niepoprawna. */
  remaining: bigint | null
  /** Błędy kwot linii według klucza. */
  errors: Record<string, string>
}

/** Kontrola sumy splitu na bigint jednostek minimalnych — bez floatów. */
export function checkSplits(lines: SplitDraft[], total: string | null, decimals: number): SplitCheck {
  let assigned = 0n
  const errors: Record<string, string> = {}
  for (const line of lines) {
    const parsed = parseAmountInput(line.amount, decimals)
    if (parsed.ok) {
      assigned += toMinor(parsed.value, decimals) ?? 0n
    } else {
      errors[line.key] = parsed.error
    }
  }
  const totalMinor = total === null ? null : toMinor(total, decimals)
  return { assigned, remaining: totalMinor === null ? null : totalMinor - assigned, errors }
}

type Props = {
  lines: SplitDraft[]
  onChange: (lines: SplitDraft[]) => void
  currency: string
  decimals: number
  /** Kwota transakcji bez znaku (kanoniczna) albo null, gdy jeszcze niepoprawna. */
  total: string | null
  envelopes: EnvelopeOption[]
  /** Błędy pokazywane po próbie zapisu. */
  showErrors: boolean
  serverErrors?: Record<string, string>
}

/**
 * Podział transakcji na koperty: linie (koperta, kwota, notatka) i suma
 * kontrolna. Kwoty linii mają znak transakcji, więc wpisuje się je bez znaku.
 */
export function SplitEditor({ lines, onChange, currency, decimals, total, envelopes, showErrors, serverErrors = {} }: Props) {
  const check = checkSplits(lines, total, decimals)
  const balanced = check.remaining === 0n
  const set = (key: string, patch: Partial<SplitDraft>) =>
    onChange(lines.map((line) => (line.key === key ? { ...line, ...patch } : line)))

  return (
    <fieldset className={styles.splits}>
      <legend>Podział na koperty</legend>
      {lines.map((line, index) => (
        <div key={line.key} className={styles.splitLine} role="group" aria-label={`Linia ${index + 1}`}>
          <EnvelopeField
            label={`Koperta, linia ${index + 1}`}
            value={line.envelopeId}
            onChange={(envelopeId) => set(line.key, { envelopeId })}
            options={envelopes}
            error={serverErrors[`splits[${index}].envelope_id`]}
          />
          <AmountField
            label={`Kwota, linia ${index + 1}`}
            value={line.amount}
            onChange={(amount) => set(line.key, { amount })}
            currency={currency}
            error={(showErrors ? check.errors[line.key] : undefined) ?? serverErrors[`splits[${index}].amount`]}
          />
          <TextField
            label={`Notatka, linia ${index + 1}`}
            value={line.note}
            onChange={(event) => set(line.key, { note: event.target.value })}
            maxLength={1000}
          />
          <IconButton
            icon="trash"
            label={`Usuń linię ${index + 1}`}
            onClick={() => onChange(lines.filter((other) => other.key !== line.key))}
          />
        </div>
      ))}
      <div>
        <Button size="sm" icon="plus" onClick={() => onChange([...lines, newSplitDraft(remainingDraft(check, decimals))])}>
          Dodaj linię
        </Button>
      </div>
      <p className={styles.splitTotal} data-balanced={balanced} aria-live="polite">
        <span>
          Rozdzielono <Money amount={fromMinor(check.assigned, decimals)} currency={currency} />
          {total !== null && (
            <>
              {' '}
              z <Money amount={total} currency={currency} />
            </>
          )}
        </span>
        <span>
          {check.remaining === null
            ? 'Najpierw podaj kwotę transakcji.'
            : balanced
              ? 'Suma się zgadza.'
              : check.remaining > 0n
                ? (
                    <>
                      Pozostało do rozdzielenia: <Money amount={fromMinor(check.remaining, decimals)} currency={currency} />
                    </>
                  )
                : (
                    <>
                      Za dużo o <Money amount={fromMinor(-check.remaining, decimals)} currency={currency} />
                    </>
                  )}
        </span>
      </p>
    </fieldset>
  )
}

// Nowa linia dostaje to, czego jeszcze brakuje do kwoty transakcji.
function remainingDraft(check: SplitCheck, decimals: number): Partial<SplitDraft> {
  if (check.remaining === null || check.remaining <= 0n) {
    return {}
  }
  return { amount: fromMinor(check.remaining, decimals).replace('.', ',') }
}
