import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useDeleteTransaction, useUpdateTransaction } from '../../api/hooks/transactions.ts'
import type { Charge, Transaction, TransactionStatus, Wallet } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { IconButton } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { tk } from '../../i18n/translate.ts'
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
      return outgoing ? tk('transactions:row.transferOut') : tk('transactions:row.transferIn')
    }
    return outgoing
      ? tk('transactions:row.transferTo', { name: other })
      : tk('transactions:row.transferFrom', { name: other })
  }
  if (transaction.payee_name) {
    return transaction.payee_name
  }
  if (transaction.note) {
    return transaction.note.length > 60 ? `${transaction.note.slice(0, 60)}…` : transaction.note
  }
  return transaction.kind === 'income' ? tk('transactions:mode.income') : tk('transactions:mode.expense')
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
  /** Obciążenie subskrypcji powiązane z transakcją (część C2) — plakietka z linkiem. */
  charge?: Charge
  /** „Powiąż z subskrypcją” dla wydatku bez powiązania (część C2). */
  onLinkCharge?: () => void
}

export function TransactionRow({ transaction, wallets, showWallet, canWrite, onEdit, onDelete, envelopes, charge, onLinkCharge }: Props) {
  const { t } = useTranslation('transactions')
  const { household } = useHousehold()
  const title = transactionTitle(transaction, wallets)
  const wallet = wallets.get(transaction.wallet_id)
  const foreign = transaction.amount.currency !== household.base_currency
  const showNote = transaction.note && title !== transaction.note && !title.startsWith(transaction.note.slice(0, 20))

  return (
    <article className={styles.row} aria-label={t('row.label', { title, date: formatDay(transaction.date) })}>
      <span className={styles.date}>
        <time dateTime={transaction.date}>{formatDay(transaction.date)}</time>
      </span>
      <div className={styles.main}>
        {canWrite ? (
          <button type="button" className={styles.title} onClick={onEdit} aria-label={t('row.edit', { title })}>
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
          {charge ? (
            <Badge tone="brand" title={t('row.subscriptionTitle')}>
              <Icon name="repeat" size={12} />
              <Link to={householdPath(household.id, `subscriptions/${charge.subscription_id}`)} className={styles.badgeLink}>
                {t('row.subscription', { name: charge.subscription_name })}
              </Link>
            </Badge>
          ) : (
            transaction.source !== 'manual' && <Badge>{sourceLabels[transaction.source]}</Badge>
          )}
          {transaction.envelope_id && envelopes?.has(transaction.envelope_id) && (
            <Badge tone="brand" title={t('row.envelope')}>
              <Icon name="budget" size={12} />
              {envelopes.get(transaction.envelope_id)}
            </Badge>
          )}
          {transaction.splits.length > 0 && <Badge>{t('row.splits', { n: transaction.splits.length })}</Badge>}
          {transaction.tags.length > 0 && (
            <ul className={styles.tags} aria-label={t('row.tags')}>
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
            <span className="visually-hidden">{t('row.inBase')}</span>
            <Money amount={transaction.base_amount.amount} currency={transaction.base_amount.currency} size="sm" signed />
          </span>
        )}
      </div>
      <div className={styles.actions}>
        {canWrite && onLinkCharge && !charge && linkable(transaction) && (
          <IconButton icon="repeat" label={t('row.linkCharge', { title })} onClick={onLinkCharge} />
        )}
        {canWrite && (
          <IconButton icon="trash" label={t('row.delete', { title })} onClick={onDelete} />
        )}
      </div>
    </article>
  )
}

/** Czy transakcję można powiązać z płatnością subskrypcji (wydatek, nie przelew). */
function linkable(transaction: Transaction): boolean {
  return transaction.kind === 'expense' && toneOf(transaction.amount.amount) === 'outflow'
}

/**
 * Oznaczenie transakcji bez kwoty bazowej (brak kursu). Owner/admin dostaje
 * link do ręcznego kursu na ten dzień.
 */
export function RatePendingBadge({ transaction }: { transaction: Transaction }) {
  const { t } = useTranslation('transactions')
  const { household, role } = useHousehold()
  const hint = t('row.rateHint', {
    base: household.base_currency,
    currency: transaction.amount.currency,
    date: formatDay(transaction.date),
  })
  return (
    <span className={styles.pending}>
      <Badge tone="warn" title={hint}>
        <Icon name="clock" size={14} />
        {t('row.ratePending')}
      </Badge>
      <span className="visually-hidden">{hint}</span>
      {canManage(role) && (
        <Link
          to={`${householdPath(household.id, 'settings/rates')}?currency=${transaction.amount.currency}&date=${transaction.date}`}
        >
          {t('row.setRate')}
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
  const { t } = useTranslation('transactions')
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
      aria-label={t('row.toggleStatus', { status: statusLabels[status], next: statusLabels[next].toLowerCase() })}
      title={statusDescriptions[status]}
      onClick={() =>
        update.mutate(
          { transactionId: transaction.id, body: { status: next } },
          { onError: () => toast.show({ message: t('row.statusFailed'), kind: 'error' }) },
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
  const { t } = useTranslation('transactions')
  const { household } = useHousehold()
  const remove = useDeleteTransaction(household.id)
  const toast = useToast()
  const transfer = transaction.kind === 'transfer'
  return (
    <ConfirmDialog
      open
      title={transfer ? t('row.deleteTransferTitle') : t('row.deleteTransactionTitle')}
      confirmLabel={transfer ? t('row.deleteTransfer') : t('row.deleteTransaction')}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(transaction.id, {
          onSuccess: () => {
            toast.show({ message: transfer ? t('row.transferDeleted') : t('row.transactionDeleted') })
            onDone()
          },
        })
      }
    >
      <p>
        <strong>{title}</strong>, {formatDay(transaction.date)}:{' '}
        <Money amount={transaction.amount.amount} currency={transaction.amount.currency} tone="auto" />
      </p>
      <p>{transfer ? t('row.deleteTransferBody') : t('row.deleteTransactionBody')}</p>
    </ConfirmDialog>
  )
}
