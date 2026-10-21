import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { usePayees } from '../../api/hooks/payees.ts'
import { useCreateTransaction, useUpdateTransaction } from '../../api/hooks/transactions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type {
  SplitInput,
  Transaction,
  TransactionCreate,
  TransactionStatus,
  TransactionUpdate,
  Wallet,
} from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { Checkbox, SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { todayIn } from '../../lib/format.ts'
import { statusLabels } from '../../lib/labels.ts'
import { amountForInput, formatMoney, negateDecimal, parseAmountInput, toMinor, toneOf } from '../../lib/money.ts'
import { readStored, writeStored } from '../../lib/storage.ts'
import { envelopesEnabled, useEnvelopeOptions } from './envelopes.ts'
import { AmountField, EnvelopeField, findPayee, ModePicker, parseTags, PayeeField, TextAreaField, type EntryMode } from './fields.tsx'
import { lastWalletKey, pickError, useDecimals } from './money.ts'
import { checkSplits, newSplitDraft, SplitEditor, type SplitDraft } from './SplitEditor.tsx'
import styles from './transactions.module.css'

export function modeOf(transaction: Transaction): EntryMode {
  if (transaction.kind === 'income') {
    return 'income'
  }
  return toneOf(transaction.amount.amount) === 'inflow' ? 'refund' : 'expense'
}

// Funkcja, nie stała: etykiety czytane w chwili renderu, w bieżącym języku.
const statusOptions = () =>
  (['pending', 'cleared', 'reconciled'] as TransactionStatus[]).map((status) => ({
    value: status,
    label: statusLabels[status],
  }))

type Props = {
  open: boolean
  onClose: () => void
  /** Edytowana transakcja; bez niej — nowa. */
  transaction?: Transaction
  /** Portfel ustalony przez stronę (bez wyboru portfela). */
  walletId?: string
}

/** Szybkie dodawanie i edycja wydatku, wpływu lub zwrotu (oraz daty/statusu nogi przelewu). */
export function TransactionDialog({ open, onClose, transaction, walletId }: Props) {
  const { t } = useTranslation('transactions')
  const title = transaction
    ? transaction.kind === 'transfer'
      ? t('dialog.editTransfer')
      : t('dialog.editTransaction')
    : t('dialog.newTransaction')
  return (
    <Dialog open={open} title={title} onClose={onClose} size="md">
      <TransactionForm onClose={onClose} transaction={transaction} fixedWalletId={walletId} />
    </Dialog>
  )
}

function initialWallet(hid: string, transaction?: Transaction, fixed?: string): string {
  return transaction?.wallet_id ?? fixed ?? readStored(lastWalletKey(hid)) ?? ''
}

function TransactionForm({
  onClose,
  transaction,
  fixedWalletId,
}: {
  onClose: () => void
  transaction?: Transaction
  fixedWalletId?: string
}) {
  const { t } = useTranslation('transactions')
  const { household } = useHousehold()
  const hid = household.id
  const wallets = useWallets(hid)
  const payees = usePayees(hid)
  const decimalsFor = useDecimals()
  const envelopeOptions = useEnvelopeOptions(hid)
  const create = useCreateTransaction(hid)
  const update = useUpdateTransaction(hid)
  const toast = useToast()
  const amountRef = useRef<HTMLInputElement>(null)

  const editing = !!transaction
  const transfer = transaction?.kind === 'transfer'
  const showEnvelopes = envelopesEnabled() && !transfer

  const [mode, setMode] = useState<EntryMode>(() => (transaction ? modeOf(transaction) : 'expense'))
  const [amount, setAmount] = useState(() => (transaction ? amountForInput(transaction.amount.amount) : ''))
  const [payee, setPayee] = useState(transaction?.payee_name ?? '')
  const [date, setDate] = useState(() => transaction?.date ?? todayIn(household.timezone))
  const [chosenWallet, setChosenWallet] = useState(() => initialWallet(hid, transaction, fixedWalletId))
  const [status, setStatus] = useState<TransactionStatus>(transaction?.status ?? 'cleared')
  const [tags, setTags] = useState(transaction?.tags.join(', ') ?? '')
  const [note, setNote] = useState(transaction?.note ?? '')
  const [envelopeId, setEnvelopeId] = useState(transaction?.envelope_id ?? '')
  const [splits, setSplits] = useState<SplitDraft[]>(
    () =>
      transaction?.splits.map((split) =>
        newSplitDraft({
          envelopeId: split.envelope_id ?? '',
          amount: amountForInput(split.amount.amount),
          note: split.note,
        }),
      ) ?? [],
  )
  const [addAnother, setAddAnother] = useState(false)
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const [tried, setTried] = useState(false)

  const active = (wallets.data ?? []).filter((wallet) => !wallet.archived_at)
  const walletId =
    editing || fixedWalletId
      ? (transaction?.wallet_id ?? fixedWalletId ?? '')
      : active.some((wallet) => wallet.id === chosenWallet)
        ? chosenWallet
        : (active[0]?.id ?? '')
  const wallet: Wallet | undefined = (wallets.data ?? []).find((w) => w.id === walletId)
  const currency = wallet?.currency ?? transaction?.amount.currency ?? household.base_currency
  const decimals = decimalsFor(currency)
  const parsedAmount = parseAmountInput(amount, decimals)
  const splitting = showEnvelopes && mode !== 'income' && splits.length > 0

  const mutation = editing ? update : create
  const server = fieldErrors(mutation.error)
  const archived = mutation.error instanceof ApiError && mutation.error.reason === 'wallet-archived'

  const signed = (value: string) => (mode === 'expense' ? negateDecimal(value) : value)

  const payeeBody = (): Pick<TransactionCreate, 'payee_id' | 'payee_name'> | Pick<TransactionUpdate, 'payee_id' | 'payee_name'> => {
    const name = payee.trim()
    const known = findPayee(payees.data ?? [], name)
    if (!editing) {
      return known ? { payee_id: known.id } : name ? { payee_name: name } : {}
    }
    if (!name) {
      return transaction?.payee_id ? { payee_id: null } : {}
    }
    if (known) {
      return known.id === transaction?.payee_id ? {} : { payee_id: known.id }
    }
    return { payee_name: name }
  }

  const splitBody = (): SplitInput[] =>
    splits.map((line) => {
      const value = parseAmountInput(line.amount, decimals)
      return {
        amount: { amount: signed(value.ok ? value.value : '0'), currency },
        ...(line.envelopeId ? { envelope_id: line.envelopeId } : {}),
        ...(line.note.trim() ? { note: line.note.trim() } : {}),
      }
    })

  const reset = () => {
    setAmount('')
    setPayee('')
    setNote('')
    setTags('')
    setSplits([])
    setEnvelopeId('')
    setTried(false)
    setErrors({})
    create.reset()
    amountRef.current?.focus()
  }

  const submit = async () => {
    setTried(true)
    const next: Record<string, string | undefined> = {}
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
      next.date = t('dialog.dateRequired')
    }
    if (!transfer) {
      if (!parsedAmount.ok) {
        next.amount = parsedAmount.error
      }
      if (!walletId) {
        next.wallet_id = t('dialog.walletRequired')
      }
      if (splitting) {
        const check = checkSplits(splits, parsedAmount.ok ? parsedAmount.value : null, decimals)
        if (Object.keys(check.errors).length > 0 || check.remaining !== 0n) {
          next.splits = t('dialog.splitsMismatch')
        }
      }
    }
    setErrors(next)
    if (Object.values(next).some(Boolean)) {
      return
    }

    const common = { date, status, tags: parseTags(tags), note: note.trim() }
    try {
      if (transfer && transaction) {
        await update.mutateAsync({ transactionId: transaction.id, body: common })
        toast.show({ message: t('dialog.transferSaved') })
        onClose()
        return
      }
      const value = signed((parsedAmount as { value: string }).value)
      const envelopeFields = showEnvelopes
        ? splitting
          ? { splits: splitBody() }
          : mode === 'income'
            ? { splits: [] }
            : { envelope_id: envelopeId || null, splits: [] }
        : {}
      const kind = mode === 'income' ? 'income' : 'expense'
      if (transaction) {
        const body: TransactionUpdate = {
          ...common,
          amount: { amount: value, currency },
          kind,
          ...payeeBody(),
          ...editedEnvelopeFields(envelopeFields, transaction),
        }
        await update.mutateAsync({ transactionId: transaction.id, body })
        toast.show({ message: t('dialog.transactionSaved') })
        onClose()
        return
      }
      const { envelope_id: envelope, ...rest } = envelopeFields as { envelope_id?: string | null; splits?: SplitInput[] }
      const body: TransactionCreate = {
        ...common,
        wallet_id: walletId,
        amount: { amount: value, currency },
        kind,
        ...(payeeBody() as Pick<TransactionCreate, 'payee_id' | 'payee_name'>),
        ...(envelope ? { envelope_id: envelope } : {}),
        ...(rest.splits && rest.splits.length > 0 ? { splits: rest.splits } : {}),
      }
      await create.mutateAsync(body)
      writeStored(lastWalletKey(hid), walletId)
      toast.show({
        message: t(`dialog.created.${mode}`, { amount: formatMoney(value, currency, { signed: true }) ?? value }),
      })
      if (addAnother) {
        reset()
      } else {
        onClose()
      }
    } catch {
      // Błąd pokazuje formularz (mutation.error).
    }
  }

  const shownFields = ['amount', 'amount.amount', 'amount.currency', 'date', 'payee_id', 'payee_name', 'wallet_id', 'tags', 'note', 'status', 'kind', 'envelope_id', 'splits']

  return (
    <form
      noValidate
      aria-label={editing ? t('dialog.editTransaction') : t('dialog.newTransaction')}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      {archived ? (
        <Callout tone="warn">{t('dialog.walletArchived')}</Callout>
      ) : (
        <FormAlert error={mutation.error} shownFields={shownFields} />
      )}

      {transfer ? (
        <p className={styles.note}>{t('dialog.transferNote')}</p>
      ) : (
        <>
          <ModePicker value={mode} onChange={setMode} />
          <div className={styles.grid}>
            <AmountField
              label={t('dialog.amount')}
              large
              data-autofocus
              inputRef={amountRef}
              value={amount}
              onChange={setAmount}
              currency={currency}
              sign={mode === 'expense' ? '−' : '+'}
              error={errors.amount ?? pickError(server, 'amount', 'amount.amount', 'amount.currency')}
            />
            <PayeeField
              value={payee}
              onChange={setPayee}
              payees={payees.data ?? []}
              error={pickError(server, 'payee_name', 'payee_id')}
            />
          </div>
        </>
      )}

      <div className={styles.grid}>
        <TextField
          label={t('dialog.date')}
          type="date"
          value={date}
          onChange={(event) => setDate(event.target.value)}
          error={errors.date ?? server.date}
          required
        />
        {!editing && !fixedWalletId && (
          <SelectField
            label={t('dialog.wallet')}
            value={walletId}
            onChange={(event) => setChosenWallet(event.target.value)}
            error={errors.wallet_id ?? server.wallet_id}
            options={
              active.length > 0
                ? active.map((w) => ({ value: w.id, label: `${w.name} (${w.currency})` }))
                : [{ value: '', label: wallets.isPending ? t('dialog.loading') : t('dialog.noWallets') }]
            }
          />
        )}
        <SelectField
          label={t('dialog.status')}
          value={status}
          onChange={(event) => setStatus(event.target.value as TransactionStatus)}
          options={statusOptions()}
          error={server.status}
        />
        {showEnvelopes && mode !== 'income' && !splitting && (
          <EnvelopeField value={envelopeId} onChange={setEnvelopeId} options={envelopeOptions} error={server.envelope_id} />
        )}
        <TextField
          label={t('dialog.tags')}
          value={tags}
          onChange={(event) => setTags(event.target.value)}
          hint={t('dialog.tagsHint')}
          error={server.tags}
          autoComplete="off"
        />
        <TextAreaField label={t('dialog.note')} value={note} onChange={setNote} maxLength={1000} error={server.note} className={styles.wide} />
      </div>

      {showEnvelopes && mode !== 'income' && (
        <>
          {splitting ? (
            <SplitEditor
              lines={splits}
              onChange={setSplits}
              currency={currency}
              decimals={decimals}
              total={parsedAmount.ok ? parsedAmount.value : null}
              envelopes={envelopeOptions}
              showErrors={tried}
              serverErrors={server}
            />
          ) : (
            <div>
              <Button
                size="sm"
                icon="plus"
                onClick={() =>
                  setSplits([
                    newSplitDraft({ envelopeId, amount: parsedAmount.ok ? amount : '' }),
                    newSplitDraft(),
                  ])
                }
              >
                {t('dialog.split')}
              </Button>
            </div>
          )}
          {(errors.splits ?? server.splits) && (
            <p role="alert" className={styles.note}>
              {errors.splits ?? server.splits}
            </p>
          )}
        </>
      )}

      <div className={styles.footer}>
        {!editing ? (
          <Checkbox label={t('dialog.addAnother')} checked={addAnother} onChange={(event) => setAddAnother(event.target.checked)} />
        ) : (
          <span />
        )}
        <div className={styles.footerActions}>
          <Button onClick={onClose}>{t('dialog.cancel')}</Button>
          <Button type="submit" variant="primary" busy={mutation.isPending} disabled={splitting && !splitsBalanced(splits, parsedAmount, decimals)}>
            {editing ? t('dialog.saveChanges') : t('dialog.save')}
          </Button>
        </div>
      </div>
    </form>
  )
}

/**
 * Przy edycji wysyłamy kopertę i splity tylko, gdy się zmieniają — bez
 * zmian transakcja zostaje, jak była.
 */
function editedEnvelopeFields(
  fields: { envelope_id?: string | null; splits?: SplitInput[] },
  transaction: Transaction,
): { envelope_id?: string | null; splits?: SplitInput[] } {
  const out: { envelope_id?: string | null; splits?: SplitInput[] } = {}
  if ('envelope_id' in fields && (fields.envelope_id ?? null) !== (transaction.envelope_id ?? null)) {
    out.envelope_id = fields.envelope_id ?? null
  }
  if (fields.splits && (fields.splits.length > 0 || transaction.splits.length > 0)) {
    out.splits = fields.splits
  }
  return out
}

function splitsBalanced(splits: SplitDraft[], parsed: ReturnType<typeof parseAmountInput>, decimals: number): boolean {
  if (!parsed.ok) {
    return false
  }
  const check = checkSplits(splits, parsed.value, decimals)
  return check.remaining === 0n && Object.keys(check.errors).length === 0 && toMinor(parsed.value, decimals) !== null
}
