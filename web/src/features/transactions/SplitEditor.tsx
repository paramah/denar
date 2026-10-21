import { Trans, useTranslation } from 'react-i18next'
import { Button, IconButton } from '../../components/Button/Button.tsx'
import { TextField } from '../../components/Field/Field.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { decimalSeparator } from '../../lib/intl.ts'
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
  const { t } = useTranslation('transactions')
  const check = checkSplits(lines, total, decimals)
  const balanced = check.remaining === 0n
  const assigned = <Money amount={fromMinor(check.assigned, decimals)} currency={currency} />
  const set = (key: string, patch: Partial<SplitDraft>) =>
    onChange(lines.map((line) => (line.key === key ? { ...line, ...patch } : line)))

  return (
    <fieldset className={styles.splits}>
      <legend>{t('split.legend')}</legend>
      {lines.map((line, index) => (
        <div key={line.key} className={styles.splitLine} role="group" aria-label={t('split.line', { n: index + 1 })}>
          <EnvelopeField
            label={t('split.envelope', { n: index + 1 })}
            value={line.envelopeId}
            onChange={(envelopeId) => set(line.key, { envelopeId })}
            options={envelopes}
            error={serverErrors[`splits[${index}].envelope_id`]}
          />
          <AmountField
            label={t('split.amount', { n: index + 1 })}
            value={line.amount}
            onChange={(amount) => set(line.key, { amount })}
            currency={currency}
            error={(showErrors ? check.errors[line.key] : undefined) ?? serverErrors[`splits[${index}].amount`]}
          />
          <TextField
            label={t('split.note', { n: index + 1 })}
            value={line.note}
            onChange={(event) => set(line.key, { note: event.target.value })}
            maxLength={1000}
          />
          <IconButton
            icon="trash"
            label={t('split.remove', { n: index + 1 })}
            onClick={() => onChange(lines.filter((other) => other.key !== line.key))}
          />
        </div>
      ))}
      <div>
        <Button size="sm" icon="plus" onClick={() => onChange([...lines, newSplitDraft(remainingDraft(check, decimals))])}>
          {t('split.add')}
        </Button>
      </div>
      <p className={styles.splitTotal} data-balanced={balanced} aria-live="polite">
        <span>
          {total !== null ? (
            <Trans
              t={t}
              i18nKey="split.assignedOf"
              components={{ assigned: assigned, total: <Money amount={total} currency={currency} /> }}
            />
          ) : (
            <Trans t={t} i18nKey="split.assigned" components={{ assigned: assigned }} />
          )}
        </span>
        <span>
          {check.remaining === null
            ? t('split.amountFirst')
            : balanced
              ? t('split.balanced')
              : check.remaining > 0n
                ? (
                    <Trans
                      t={t}
                      i18nKey="split.remaining"
                      components={{ amount: <Money amount={fromMinor(check.remaining, decimals)} currency={currency} /> }}
                    />
                  )
                : (
                    <Trans
                      t={t}
                      i18nKey="split.over"
                      components={{ amount: <Money amount={fromMinor(-check.remaining, decimals)} currency={currency} /> }}
                    />
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
  return { amount: fromMinor(check.remaining, decimals).replace('.', decimalSeparator()) }
}
