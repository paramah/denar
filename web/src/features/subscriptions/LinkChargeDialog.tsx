import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useMatchCharge, useTransactionChargeSuggestions } from '../../api/hooks/subscriptions.ts'
import type { Transaction } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDay } from '../../lib/format.ts'
import { ScoreMeter } from './charges.tsx'
import { coinName, useVendorNames, VendorCoin } from './parts.tsx'
import styles from './subscriptions.module.css'

/** Rejestr: „Powiąż z subskrypcją” — otwarte płatności, które ta transakcja może opłacać. */
export function LinkChargeDialog({ transaction, title, onClose }: { transaction: Transaction; title: string; onClose: () => void }) {
  const { household } = useHousehold()
  const suggestions = useTransactionChargeSuggestions(household.id, transaction.id)
  const match = useMatchCharge(household.id)
  const vendors = useVendorNames()
  const toast = useToast()
  const { t } = useTranslation('subscriptions')
  return (
    <Dialog open title={t('link.title')} onClose={onClose} size="md">
      <div className={styles.dialogBody}>
        <p className={styles.chargeSummary}>
          <strong>{title}</strong>, {formatDay(transaction.date)}:{' '}
          <Money amount={transaction.amount.amount} currency={transaction.amount.currency} tone="auto" />
        </p>
        <QueryState query={suggestions} loading={<Skeleton lines={2} />} errorTitle={t('link.loadError')}>
          {(items) =>
            items.length === 0 ? (
              <p className={styles.muted}>
                {t('link.none')}{' '}
                <Link to={householdPath(household.id, 'subscriptions/list')}>{t('link.all')}</Link>
              </p>
            ) : (
              <ul className={styles.suggestions} aria-label={t('link.label')}>
                {items.map(({ charge, score }) => (
                  <li key={charge.id} className={styles.suggestion}>
                    <div className={styles.suggestionText}>
                      <span className={styles.suggestionTitle}>
                        <VendorCoin vendor={charge.vendor} name={coinName(charge.vendor, vendors.get(charge.vendor) ?? '', charge.subscription_name)} size="sm" />
                        {charge.subscription_name}
                      </span>
                      <span className={styles.muted}>{t('link.due', { date: formatDay(charge.due_date) })}</span>
                    </div>
                    <Money amount={charge.amount.amount} currency={charge.amount.currency} />
                    <ScoreMeter score={score} />
                    <Button
                      size="sm"
                      variant="primary"
                      icon="link"
                      disabled={match.isPending}
                      busy={match.isPending && match.variables?.chargeId === charge.id}
                      aria-label={t('link.actionLabel', { name: charge.subscription_name, date: formatDay(charge.due_date) })}
                      onClick={() =>
                        match.mutate(
                          { chargeId: charge.id, transactionId: transaction.id },
                          {
                            onSuccess: () => {
                              toast.show({ message: t('link.done', { name: charge.subscription_name }) })
                              onClose()
                            },
                            onError: () => toast.show({ message: t('link.failed'), kind: 'error' }),
                          },
                        )
                      }
                    >
                      {t('link.action')}
                    </Button>
                  </li>
                ))}
              </ul>
            )
          }
        </QueryState>
      </div>
    </Dialog>
  )
}
