import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { fieldErrors } from '../../api/errors.ts'
import { useMoveBetweenEnvelopes, useSetAssignment } from '../../api/hooks/budget.ts'
import type { BudgetMonth } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatMoney, parseAmountInput } from '../../lib/money.ts'
import { AmountField } from '../transactions/fields.tsx'
import { pickError } from '../transactions/money.ts'
import { envelopesOf, shiftAssigned } from './model.ts'
import styles from './budget.module.css'

/** Wartość pola „z/do” oznaczająca kwotę do przydzielenia. */
export const readyToAssign = 'rta'

export type MovePreset = { from?: string; to?: string; amount?: string }

type Props = {
  open: boolean
  onClose: () => void
  month: BudgetMonth
  decimals: number
  preset?: MovePreset
}

/**
 * Przeniesienie środków: koperta → koperta (POST moves), z kwoty do
 * przydzielenia do koperty albo z koperty z powrotem (zmiana przydziału).
 */
export function MoveDialog({ open, onClose, month, decimals, preset }: Props) {
  const { t } = useTranslation('budget')
  return (
    <Dialog open={open} title={t('move.title')} onClose={onClose} size="md">
      <MoveForm onClose={onClose} month={month} decimals={decimals} preset={preset} />
    </Dialog>
  )
}

function MoveForm({ onClose, month, decimals, preset = {} }: Omit<Props, 'open'>) {
  const { t } = useTranslation(['budget', 'common'])
  const { household } = useHousehold()
  const move = useMoveBetweenEnvelopes(household.id)
  const assign = useSetAssignment(household.id)
  const toast = useToast()
  const envelopes = envelopesOf(month)
  const [from, setFrom] = useState(preset.from ?? readyToAssign)
  const [to, setTo] = useState(preset.to ?? envelopes.find((e) => !e.hidden && e.id !== preset.from)?.id ?? readyToAssign)
  const [amount, setAmount] = useState(preset.amount ?? '')
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})

  const byId = new Map(envelopes.map((e) => [e.id, e]))
  const mutation = from === readyToAssign || to === readyToAssign ? assign : move
  const server = fieldErrors(mutation.error)
  const available = (id: string) => (id === readyToAssign ? month.ready_to_assign : byId.get(id)?.available)
  const nameOf = (id: string) => (id === readyToAssign ? t('move.rta') : (byId.get(id)?.name ?? t('move.envelopeFallback')))

  const options = (selected: string) => [
    {
      value: readyToAssign,
      label: t('move.rtaOption', { amount: formatMoney(month.ready_to_assign.amount, month.currency) ?? '' }),
    },
    ...envelopes
      .filter((e) => !e.hidden || e.id === selected)
      .map((e) => ({
        value: e.id,
        label: `${e.groupName} · ${e.name} (${formatMoney(e.available.amount, month.currency) ?? ''})`,
      })),
  ]

  const submit = async () => {
    const next: Record<string, string | undefined> = {}
    const parsed = parseAmountInput(amount, decimals)
    if (!parsed.ok) {
      next.amount = parsed.error
    }
    if (from === to) {
      next.to = t('move.sameTarget')
    }
    if (from === readyToAssign && to === readyToAssign) {
      next.to = t('move.pickEnvelope')
    }
    setErrors(next)
    if (Object.values(next).some(Boolean) || !parsed.ok) {
      return
    }
    try {
      if (from === readyToAssign || to === readyToAssign) {
        // Przeniesienie z/do kwoty do przydzielenia to zmiana przydziału koperty.
        const target = byId.get(from === readyToAssign ? to : from)!
        const value = shiftAssigned(target.assigned.amount, parsed.value, decimals, to === readyToAssign)
        if (value === null) {
          setErrors({ amount: t('common:amount.invalid') })
          return
        }
        await assign.mutateAsync({
          month: month.month,
          envelopeId: target.id,
          amount: { amount: value, currency: month.currency },
        })
      } else {
        await move.mutateAsync({
          month: month.month,
          body: {
            from_envelope_id: from,
            to_envelope_id: to,
            amount: { amount: parsed.value, currency: month.currency },
          },
        })
      }
      toast.show({
        message: t('move.done', {
          amount: formatMoney(parsed.value, month.currency) ?? parsed.value,
          from: nameOf(from),
          to: nameOf(to),
        }),
      })
      onClose()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  const source = available(from)
  return (
    <form
      noValidate
      aria-label={t('move.title')}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert error={mutation.error} shownFields={['amount', 'amount.amount', 'from_envelope_id', 'to_envelope_id']} />
      <div className={styles.pair}>
        <SelectField
          label={t('move.from')}
          value={from}
          onChange={(event) => setFrom(event.target.value)}
          options={options(from)}
          error={server.from_envelope_id}
        />
        <SelectField
          label={t('move.to')}
          value={to}
          onChange={(event) => setTo(event.target.value)}
          options={options(to)}
          error={errors.to ?? server.to_envelope_id}
        />
      </div>
      <AmountField
        label={t('move.amount')}
        value={amount}
        onChange={setAmount}
        currency={month.currency}
        large
        data-autofocus
        error={errors.amount ?? pickError(server, 'amount', 'amount.amount')}
      />
      {source && (
        <p className={styles.muted}>
          <Trans t={t} i18nKey="move.source" components={{ money: <Money amount={source.amount} currency={month.currency} /> }} />
        </p>
      )}
      <div className={styles.actions}>
        <Button onClick={onClose}>{t('actions.cancel')}</Button>
        <Button type="submit" variant="primary" icon="transfer" busy={mutation.isPending}>
          {t('move.submit')}
        </Button>
      </div>
    </form>
  )
}
