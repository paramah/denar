import { useMemo } from 'react'
import { Trans, useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const update = useUpdateEnvelope(household.id)
  const toast = useToast()
  const register = `${householdPath(household.id, 'transactions')}?envelope=${envelope.id}&from=${month.start}&to=${month.end}`

  return (
    <Dialog open title={envelope.name} onClose={onClose} size="lg">
      <div className={styles.form}>
        <dl className={styles.stats} aria-label={t('details.statsLabel', { name: envelope.name, month: formatMonth(month.month) })}>
          <div>
            <dt>{t('details.carriedOver')}</dt>
            <dd>
              <Money amount={envelope.carried_over.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>{t('details.assigned')}</dt>
            <dd>
              <Money amount={envelope.assigned.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>{t('details.activity')}</dt>
            <dd>
              <Money amount={envelope.activity.amount} currency={month.currency} tone="auto" signed={false} />
            </dd>
          </div>
          <div>
            <dt>{t('details.available')}</dt>
            <dd>
              <Money amount={envelope.available.amount} currency={month.currency} size="lg" />
            </dd>
          </div>
        </dl>
        {envelope.goal ? (
          <div className={styles.nameCell}>
            <p className={styles.muted}>
              {envelope.goal_status ? (
                <Trans
                  t={t}
                  i18nKey="details.goalNeeded"
                  values={{ goal: describeGoal(envelope.goal) }}
                  components={{ money: <Money amount={envelope.goal_status.needed.amount} currency={month.currency} size="sm" /> }}
                />
              ) : (
                t('details.goal', { goal: describeGoal(envelope.goal) })
              )}
            </p>
            <GoalProgress envelope={envelope} currency={month.currency} />
          </div>
        ) : (
          <p className={styles.muted}>{t('details.noGoal')}</p>
        )}
        {canWrite && (
          <div className={styles.detailActions}>
            <Button icon="transfer" onClick={onMove}>
              {toneOf(envelope.available.amount) === 'outflow' ? t('details.cover') : t('details.move')}
            </Button>
            <Button icon="edit" onClick={onEdit}>
              {t('details.edit')}
            </Button>
            <Button
              icon={envelope.hidden ? 'eye' : 'eyeOff'}
              busy={update.isPending}
              onClick={() =>
                update.mutate(
                  { envelopeId: envelope.id, body: { hidden: !envelope.hidden } },
                  {
                    onSuccess: () =>
                      toast.show({
                        message: envelope.hidden
                          ? t('details.restored', { name: envelope.name })
                          : t('details.hidden', { name: envelope.name }),
                      }),
                  },
                )
              }
            >
              {envelope.hidden ? t('details.show') : t('details.hide')}
            </Button>
            <Button icon="trash" variant="danger" onClick={onDelete}>
              {t('details.delete')}
            </Button>
          </div>
        )}
        <section className={styles.flows} aria-labelledby="envelope-activity">
          <div className={styles.toolbar}>
            <h2 id="envelope-activity">{t('details.activityTitle')}</h2>
            <div className={styles.toolbarEnd}>
              <LinkButton to={register} size="sm" icon="list">
                {t('details.showInRegister')}
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
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const activity = useEnvelopeActivity(household.id, month.month, envelopeId)
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletMap = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w])), [wallets.data])

  return (
    <QueryState query={activity} errorTitle={t('details.activityLoadFailed')} loading={<Skeleton lines={3} />}>
      {(data) => (
        <DataList
          label={t('details.activityLabel')}
          items={data.pages.flatMap((page) => page.items)}
          getKey={(tx) => tx.id}
          hasMore={activity.hasNextPage}
          onLoadMore={() => void activity.fetchNextPage()}
          loadingMore={activity.isFetchingNextPage}
          empty={
            <EmptyState icon="list" title={t('details.activityEmptyTitle')} headingLevel={3}>
              <p>{t('details.activityEmptyBody')}</p>
            </EmptyState>
          }
          renderItem={(tx) => (
            <ActivityItem
              transaction={tx}
              envelopeId={envelopeId}
              title={transactionTitle(tx, walletMap)}
              wallet={walletMap.get(tx.wallet_id)?.name}
            />
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
  const { t } = useTranslation('budget')
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
            t('details.splitPart', {
              amount: formatMoney(absDecimal(transaction.amount.amount), transaction.amount.currency) ?? transaction.amount.amount,
            })}
          {transaction.rate_pending && t('details.ratePending')}
        </span>
      </span>
      <Money amount={amount.amount} currency={amount.currency} tone="auto" />
    </article>
  )
}
