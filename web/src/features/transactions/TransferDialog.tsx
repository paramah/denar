import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useCreateTransfer } from '../../api/hooks/transactions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { TransactionStatus, TransferCreate } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { todayIn } from '../../lib/format.ts'
import { statusLabels } from '../../lib/labels.ts'
import { formatMoney, parseAmountInput } from '../../lib/money.ts'
import { envelopesEnabled, useEnvelopeOptions } from './envelopes.ts'
import { AmountField, EnvelopeField, parseTags, TextAreaField } from './fields.tsx'
import { pickError, useDecimals } from './money.ts'
import styles from './transactions.module.css'

type Props = {
  open: boolean
  onClose: () => void
  /** Portfel źródłowy zaproponowany na start (np. bieżący). */
  fromWalletId?: string
}

/** Przelew między portfelami; przy różnych walutach — także kwota docelowa. */
export function TransferDialog({ open, onClose, fromWalletId }: Props) {
  const { t } = useTranslation('transactions')
  return (
    <Dialog open={open} title={t('transfer.title')} onClose={onClose} size="md">
      <TransferForm onClose={onClose} fromWalletId={fromWalletId} />
    </Dialog>
  )
}

function TransferForm({ onClose, fromWalletId }: { onClose: () => void; fromWalletId?: string }) {
  const { t } = useTranslation('transactions')
  const { household } = useHousehold()
  const wallets = useWallets(household.id)
  const create = useCreateTransfer(household.id)
  const decimalsFor = useDecimals()
  const toast = useToast()

  const active = (wallets.data ?? []).filter((wallet) => !wallet.archived_at)
  const [from, setFrom] = useState(fromWalletId ?? '')
  const [to, setTo] = useState('')
  const [date, setDate] = useState(() => todayIn(household.timezone))
  const [amount, setAmount] = useState('')
  const [toAmount, setToAmount] = useState('')
  const [status, setStatus] = useState<TransactionStatus>('cleared')
  const [tags, setTags] = useState('')
  const [note, setNote] = useState('')
  const [envelopeId, setEnvelopeId] = useState('')
  const envelopeOptions = useEnvelopeOptions(household.id)
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})

  const fromId = active.some((w) => w.id === from) ? from : (active[0]?.id ?? '')
  const toId = active.some((w) => w.id === to && w.id !== fromId) ? to : (active.find((w) => w.id !== fromId)?.id ?? '')
  const source = active.find((w) => w.id === fromId)
  const target = active.find((w) => w.id === toId)
  const crossCurrency = !!source && !!target && source.currency !== target.currency
  // Koperta tylko przy przelewie między budżetem a portfelem poza nim (trafia na nogę budżetową).
  const crossBudget = envelopesEnabled() && !!source && !!target && source.on_budget !== target.on_budget
  const server = fieldErrors(create.error)
  const archived = create.error instanceof ApiError && create.error.reason === 'wallet-archived'

  const options = (exclude?: string) =>
    active.filter((w) => w.id !== exclude).map((w) => ({ value: w.id, label: `${w.name} (${w.currency})` }))

  const submit = async () => {
    const next: Record<string, string | undefined> = {}
    if (!source) {
      next.from = t('transfer.fromRequired')
    }
    if (!target) {
      next.to = t('transfer.toRequired')
    }
    const parsed = source ? parseAmountInput(amount, decimalsFor(source.currency)) : null
    if (parsed && !parsed.ok) {
      next.amount = parsed.error
    }
    const parsedTo = crossCurrency && target ? parseAmountInput(toAmount, decimalsFor(target.currency)) : null
    if (parsedTo && !parsedTo.ok) {
      next.to_amount = parsedTo.error
    }
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
      next.date = t('dialog.dateRequired')
    }
    setErrors(next)
    if (Object.values(next).some(Boolean) || !source || !target || !parsed?.ok) {
      return
    }
    const body: TransferCreate = {
      from_wallet_id: source.id,
      to_wallet_id: target.id,
      date,
      amount: { amount: parsed.value, currency: source.currency },
      ...(parsedTo?.ok ? { to_amount: { amount: parsedTo.value, currency: target.currency } } : {}),
      status,
      tags: parseTags(tags),
      note: note.trim(),
      ...(crossBudget && envelopeId ? { envelope_id: envelopeId } : {}),
    }
    try {
      await create.mutateAsync(body)
      toast.show({
        message: t('transfer.done', {
          amount: formatMoney(parsed.value, source.currency) ?? parsed.value,
          from: source.name,
          to: target.name,
        }),
      })
      onClose()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  if (wallets.isSuccess && active.length < 2) {
    return (
      <div className={styles.form}>
        <Callout>{t('transfer.needTwo')}</Callout>
        <div className={styles.footer}>
          <div className={styles.footerActions}>
            <Button onClick={onClose}>{t('transfer.close')}</Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <form
      noValidate
      aria-label={t('transfer.title')}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      {archived ? (
        <Callout tone="warn">{t('transfer.walletArchived')}</Callout>
      ) : (
        <FormAlert
          error={create.error}
          shownFields={['from_wallet_id', 'to_wallet_id', 'amount', 'amount.amount', 'to_amount', 'to_amount.amount', 'date', 'tags', 'note', 'envelope_id']}
        />
      )}
      <div className={styles.grid}>
        <SelectField
          label={t('transfer.from')}
          value={fromId}
          onChange={(event) => setFrom(event.target.value)}
          options={options()}
          error={errors.from ?? server.from_wallet_id}
        />
        <SelectField
          label={t('transfer.to')}
          value={toId}
          onChange={(event) => setTo(event.target.value)}
          options={options(fromId)}
          error={errors.to ?? server.to_wallet_id}
        />
        <AmountField
          label={crossCurrency ? t('transfer.amountSent', { currency: source.currency }) : t('dialog.amount')}
          value={amount}
          onChange={setAmount}
          currency={source?.currency ?? household.base_currency}
          error={errors.amount ?? pickError(server, 'amount', 'amount.amount')}
          data-autofocus
        />
        {crossCurrency ? (
          <AmountField
            label={t('transfer.amountReceived', { currency: target.currency })}
            value={toAmount}
            onChange={setToAmount}
            currency={target.currency}
            hint={t('transfer.amountReceivedHint')}
            error={errors.to_amount ?? pickError(server, 'to_amount', 'to_amount.amount')}
          />
        ) : (
          <TextField
            label={t('dialog.date')}
            type="date"
            value={date}
            onChange={(event) => setDate(event.target.value)}
            error={errors.date ?? server.date}
          />
        )}
        {crossCurrency && (
          <TextField
            label={t('dialog.date')}
            type="date"
            value={date}
            onChange={(event) => setDate(event.target.value)}
            error={errors.date ?? server.date}
          />
        )}
        {crossBudget && source && (
          <div className={styles.wide}>
            <EnvelopeField
              label={source.on_budget ? t('transfer.envelopeOut') : t('transfer.envelopeIn')}
              value={envelopeId}
              onChange={setEnvelopeId}
              options={envelopeOptions}
              error={server.envelope_id}
            />
            <p className={styles.note}>
              {source.on_budget ? t('transfer.envelopeOutNote') : t('transfer.envelopeInNote')}
            </p>
          </div>
        )}
        <SelectField
          label={t('dialog.status')}
          value={status}
          onChange={(event) => setStatus(event.target.value as TransactionStatus)}
          options={(['pending', 'cleared', 'reconciled'] as TransactionStatus[]).map((s) => ({
            value: s,
            label: statusLabels[s],
          }))}
        />
        <TextField
          label={t('dialog.tags')}
          value={tags}
          onChange={(event) => setTags(event.target.value)}
          error={server.tags}
          autoComplete="off"
          className={crossCurrency ? styles.wide : undefined}
        />
        <TextAreaField label={t('dialog.note')} value={note} onChange={setNote} maxLength={1000} error={server.note} className={styles.wide} />
      </div>
      <div className={styles.footer}>
        <div className={styles.footerActions}>
          <Button onClick={onClose}>{t('dialog.cancel')}</Button>
          <Button type="submit" variant="primary" icon="transfer" busy={create.isPending}>
            {t('transfer.submit')}
          </Button>
        </div>
      </div>
    </form>
  )
}
