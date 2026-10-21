import { useMemo } from 'react'
import { useEnvelopeActivity, useUpdateEnvelope } from '../../api/hooks/budget.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { BudgetEnvelope, BudgetMonth, Transaction } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatMonth } from '../../lib/budgetMonth.ts'
import { formatDay } from '../../lib/format.ts'
import { absDecimal, formatMoney, toneOf } from '../../lib/money.ts'
import { transactionTitle } from '../transactions/TransactionRow.tsx'
import { GoalProgress } from './BudgetGrid.tsx'
import { describeGoal } from './goal.ts'
import styles from './budget.module.css'

type Props = {
  envelope: BudgetEnvelope
  month: BudgetMonth
  canWrite: boolean
  onClose: () => void
  onEdit: () => void
  onMove: () => void
  onDelete: () => void
}

/** Szczegóły koperty: kwoty miesiąca, cel, akcje i transakcje koperty w miesiącu. */
export function EnvelopeDetails({ envelope, month, canWrite, onClose, onEdit, onMove, onDelete }: Props) {
  const { household } = useHousehold()
  const update = useUpdateEnvelope(household.id)
  const toast = useToast()
  const register = `${householdPath(household.id, 'transactions')}?envelope=${envelope.id}&from=${month.start}&to=${month.end}`

  return (
    <Dialog open title={envelope.name} onClose={onClose} size="lg">
      <div className={styles.form}>
        <dl className={styles.stats} aria-label={`Koperta ${envelope.name} — ${formatMonth(month.month)}`}>
          <div>
            <dt>Z poprzedniego miesiąca</dt>
            <dd>
              <Money amount={envelope.carried_over.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>Przydzielono</dt>
            <dd>
              <Money amount={envelope.assigned.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>Aktywność</dt>
            <dd>
              <Money amount={envelope.activity.amount} currency={month.currency} tone="auto" signed={false} />
            </dd>
          </div>
          <div>
            <dt>Dostępne</dt>
            <dd>
              <Money amount={envelope.available.amount} currency={month.currency} size="lg" />
            </dd>
          </div>
        </dl>
        {envelope.goal ? (
          <div className={styles.nameCell}>
            <p className={styles.muted}>
              Cel: {describeGoal(envelope.goal)}
              {envelope.goal_status && (
                <>
                  {' '}
                  · w tym miesiącu potrzeba{' '}
                  <Money amount={envelope.goal_status.needed.amount} currency={month.currency} size="sm" />
                </>
              )}
            </p>
            <GoalProgress envelope={envelope} currency={month.currency} />
          </div>
        ) : (
          <p className={styles.muted}>Koperta nie ma celu.</p>
        )}
        {canWrite && (
          <div className={styles.detailActions}>
            <Button icon="transfer" onClick={onMove}>
              {toneOf(envelope.available.amount) === 'outflow' ? 'Pokryj przekroczenie' : 'Przenieś środki'}
            </Button>
            <Button icon="edit" onClick={onEdit}>
              Edytuj
            </Button>
            <Button
              icon={envelope.hidden ? 'eye' : 'eyeOff'}
              busy={update.isPending}
              onClick={() =>
                update.mutate(
                  { envelopeId: envelope.id, body: { hidden: !envelope.hidden } },
                  {
                    onSuccess: () =>
                      toast.show({ message: envelope.hidden ? `Przywrócono kopertę ${envelope.name}.` : `Ukryto kopertę ${envelope.name}.` }),
                  },
                )
              }
            >
              {envelope.hidden ? 'Pokaż kopertę' : 'Ukryj'}
            </Button>
            <Button icon="trash" variant="danger" onClick={onDelete}>
              Usuń
            </Button>
          </div>
        )}
        <section className={styles.flows} aria-labelledby="envelope-activity">
          <div className={styles.toolbar}>
            <h2 id="envelope-activity">Transakcje w miesiącu</h2>
            <div className={styles.toolbarEnd}>
              <LinkButton to={register} size="sm" icon="list">
                Pokaż w rejestrze
              </LinkButton>
            </div>
          </div>
          <ActivityList envelopeId={envelope.id} month={month} />
        </section>
      </div>
    </Dialog>
  )
}

function ActivityList({ envelopeId, month }: { envelopeId: string; month: BudgetMonth }) {
  const { household } = useHousehold()
  const activity = useEnvelopeActivity(household.id, month.month, envelopeId)
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletMap = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w])), [wallets.data])

  return (
    <QueryState query={activity} errorTitle="Nie udało się wczytać transakcji koperty" loading={<Skeleton lines={3} />}>
      {(data) => (
        <DataList
          label="Transakcje koperty"
          items={data.pages.flatMap((page) => page.items)}
          getKey={(t) => t.id}
          hasMore={activity.hasNextPage}
          onLoadMore={() => void activity.fetchNextPage()}
          loadingMore={activity.isFetchingNextPage}
          empty={
            <EmptyState icon="list" title="Brak transakcji w tym miesiącu" headingLevel={3}>
              <p>Wydatki z tą kopertą pojawią się tutaj.</p>
            </EmptyState>
          }
          renderItem={(t) => (
            <ActivityItem transaction={t} envelopeId={envelopeId} title={transactionTitle(t, walletMap)} wallet={walletMap.get(t.wallet_id)?.name} />
          )}
        />
      )}
    </QueryState>
  )
}

function ActivityItem({
  transaction,
  envelopeId,
  title,
  wallet,
}: {
  transaction: Transaction
  envelopeId: string
  title: string
  wallet?: string
}) {
  // Przy splicie liczy się tylko część przypisana do tej koperty (kwota bazowa).
  const parts = transaction.splits.filter((split) => split.envelope_id === envelopeId)
  const amount = parts.length === 1 ? (parts[0]!.base_amount ?? parts[0]!.amount) : (transaction.base_amount ?? transaction.amount)
  return (
    <article className={styles.activityRow} aria-label={`${title}, ${formatDay(transaction.date)}`}>
      <time dateTime={transaction.date}>{formatDay(transaction.date)}</time>
      <span>
        <span className={styles.activityTitle}>{title}</span>
        <span className={styles.activityMeta}>
          {wallet}
          {parts.length > 0 &&
            ` · część transakcji na ${formatMoney(absDecimal(transaction.amount.amount), transaction.amount.currency) ?? transaction.amount.amount}`}
          {transaction.rate_pending && ' · czeka na kurs'}
        </span>
      </span>
      <Money amount={amount.amount} currency={amount.currency} tone="auto" />
    </article>
  )
}
