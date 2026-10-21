import { Link } from 'react-router'
import { useDeleteTransaction, useUpdateTransaction } from '../../api/hooks/transactions.ts'
import type { Transaction, TransactionStatus, Wallet } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { IconButton } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDay } from '../../lib/format.ts'
import { sourceLabels, statusDescriptions, statusLabels } from '../../lib/labels.ts'
import { toneOf } from '../../lib/money.ts'
import styles from './transactions.module.css'

/** Nazwa wiersza: kontrahent, opis przelewu albo rodzaj. */
export function transactionTitle(transaction: Transaction, wallets: Map<string, Wallet>): string {
  if (transaction.kind === 'transfer') {
    const other = transaction.transfer_wallet_id ? wallets.get(transaction.transfer_wallet_id)?.name : undefined
    const outgoing = toneOf(transaction.amount.amount) === 'outflow'
    if (!other) {
      return outgoing ? 'Przelew wychodzący' : 'Przelew przychodzący'
    }
    return outgoing ? `Przelew do: ${other}` : `Przelew z: ${other}`
  }
  if (transaction.payee_name) {
    return transaction.payee_name
  }
  if (transaction.note) {
    return transaction.note.length > 60 ? `${transaction.note.slice(0, 60)}…` : transaction.note
  }
  return transaction.kind === 'income' ? 'Wpływ' : 'Wydatek'
}

type Props = {
  transaction: Transaction
  wallets: Map<string, Wallet>
  /** Pokazuj nazwę portfela (rejestr globalny). */
  showWallet: boolean
  canWrite: boolean
  onEdit: () => void
  onDelete: () => void
  /** Nazwy kopert po id (część C1); bez nich wiersz nie pokazuje koperty. */
  envelopes?: Map<string, string>
}

export function TransactionRow({ transaction, wallets, showWallet, canWrite, onEdit, onDelete, envelopes }: Props) {
  const { household } = useHousehold()
  const title = transactionTitle(transaction, wallets)
  const wallet = wallets.get(transaction.wallet_id)
  const foreign = transaction.amount.currency !== household.base_currency
  const showNote = transaction.note && title !== transaction.note && !title.startsWith(transaction.note.slice(0, 20))

  return (
    <article className={styles.row} aria-label={`${title}, ${formatDay(transaction.date)}`}>
      <span className={styles.date}>
        <time dateTime={transaction.date}>{formatDay(transaction.date)}</time>
      </span>
      <div className={styles.main}>
        {canWrite ? (
          <button type="button" className={styles.title} onClick={onEdit} aria-label={`Edytuj: ${title}`}>
            {transaction.kind === 'transfer' && <Icon name="transfer" size={16} />}
            {title}
          </button>
        ) : (
          <span className={styles.title}>
            {transaction.kind === 'transfer' && <Icon name="transfer" size={16} />}
            {title}
          </span>
        )}
        <span className={styles.meta}>
          {showWallet && wallet && (
            <Link to={householdPath(household.id, `wallets/${wallet.id}`)}>{wallet.name}</Link>
          )}
          {showNote && <span>{transaction.note}</span>}
          {transaction.source !== 'manual' && <Badge>{sourceLabels[transaction.source]}</Badge>}
          {transaction.envelope_id && envelopes?.has(transaction.envelope_id) && (
            <Badge tone="brand" title="Koperta">
              <Icon name="budget" size={12} />
              {envelopes.get(transaction.envelope_id)}
            </Badge>
          )}
          {transaction.splits.length > 0 && <Badge>Podział: {transaction.splits.length}</Badge>}
          {transaction.tags.length > 0 && (
            <ul className={styles.tags} aria-label="Tagi">
              {transaction.tags.map((tag) => (
                <li key={tag}>#{tag}</li>
              ))}
            </ul>
          )}
          {transaction.rate_pending && <RatePendingBadge transaction={transaction} />}
        </span>
      </div>
      <div className={styles.statusCell}>
        <StatusToggle transaction={transaction} canWrite={canWrite} />
      </div>
      <div className={styles.amountCell}>
        <Money amount={transaction.amount.amount} currency={transaction.amount.currency} tone="auto" />
        {foreign && transaction.base_amount && (
          <span className={styles.base}>
            <span className="visually-hidden">W walucie bazowej: </span>
            <Money amount={transaction.base_amount.amount} currency={transaction.base_amount.currency} size="sm" signed />
          </span>
        )}
      </div>
      <div className={styles.actions}>
        {canWrite && (
          <IconButton icon="trash" label={`Usuń: ${title}`} onClick={onDelete} />
        )}
      </div>
    </article>
  )
}

/**
 * Oznaczenie transakcji bez kwoty bazowej (brak kursu). Owner/admin dostaje
 * link do ręcznego kursu na ten dzień.
 */
export function RatePendingBadge({ transaction }: { transaction: Transaction }) {
  const { household, role } = useHousehold()
  const hint = `Kwota w ${household.base_currency} pojawi się sama, gdy będzie dostępny kurs ${transaction.amount.currency} z ${formatDay(transaction.date)}.`
  return (
    <span className={styles.pending}>
      <Badge tone="warn" title={hint}>
        <Icon name="clock" size={14} />
        Czeka na kurs
      </Badge>
      <span className="visually-hidden">{hint}</span>
      {canManage(role) && (
        <Link
          to={`${householdPath(household.id, 'settings/rates')}?currency=${transaction.amount.currency}&date=${transaction.date}`}
        >
          Ustaw kurs ręcznie
        </Link>
      )}
    </span>
  )
}

const nextStatus: Record<Exclude<TransactionStatus, 'reconciled'>, TransactionStatus> = {
  pending: 'cleared',
  cleared: 'pending',
}

/** Jednym kliknięciem: oczekująca ⇄ rozliczona. Uzgodnioną zmienia się w edycji. */
export function StatusToggle({ transaction, canWrite }: { transaction: Transaction; canWrite: boolean }) {
  const { household } = useHousehold()
  const update = useUpdateTransaction(household.id)
  const toast = useToast()
  const status = transaction.status
  if (status === 'reconciled' || !canWrite) {
    return (
      <Badge tone={status === 'pending' ? 'neutral' : 'brand'} title={statusDescriptions[status]}>
        {status === 'reconciled' && <Icon name="lock" size={12} />}
        {statusLabels[status]}
      </Badge>
    )
  }
  const next = nextStatus[status]
  return (
    <button
      type="button"
      className={styles.status}
      data-status={status}
      disabled={update.isPending}
      aria-label={`${statusLabels[status]} — oznacz jako ${statusLabels[next].toLowerCase()}`}
      title={statusDescriptions[status]}
      onClick={() =>
        update.mutate(
          { transactionId: transaction.id, body: { status: next } },
          { onError: () => toast.show({ message: 'Nie udało się zmienić statusu.', kind: 'error' }) },
        )
      }
    >
      <Icon name={status === 'cleared' ? 'checkCircle' : 'circle'} size={16} />
      {statusLabels[status]}
    </button>
  )
}

/** Potwierdzenie usunięcia; przelew znika w całości. */
export function DeleteTransactionDialog({ transaction, title, onDone }: { transaction: Transaction; title: string; onDone: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteTransaction(household.id)
  const toast = useToast()
  const transfer = transaction.kind === 'transfer'
  return (
    <ConfirmDialog
      open
      title={transfer ? 'Usunąć przelew?' : 'Usunąć transakcję?'}
      confirmLabel={transfer ? 'Usuń przelew' : 'Usuń transakcję'}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(transaction.id, {
          onSuccess: () => {
            toast.show({ message: transfer ? 'Usunięto przelew.' : 'Usunięto transakcję.' })
            onDone()
          },
        })
      }
    >
      <p>
        <strong>{title}</strong>, {formatDay(transaction.date)}:{' '}
        <Money amount={transaction.amount.amount} currency={transaction.amount.currency} tone="auto" />
      </p>
      <p>{transfer ? 'Zniknie cały przelew — z obu portfeli.' : 'Saldo portfela zmieni się od razu.'}</p>
    </ConfirmDialog>
  )
}
