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
  return (
    <Dialog open title="Powiąż z subskrypcją" onClose={onClose} size="md">
      <div className={styles.dialogBody}>
        <p className={styles.chargeSummary}>
          <strong>{title}</strong>, {formatDay(transaction.date)}:{' '}
          <Money amount={transaction.amount.amount} currency={transaction.amount.currency} tone="auto" />
        </p>
        <QueryState query={suggestions} loading={<Skeleton lines={2} />} errorTitle="Nie udało się wczytać płatności">
          {(items) =>
            items.length === 0 ? (
              <p className={styles.muted}>
                Żadna otwarta płatność subskrypcji z terminem ±15 dni nie pasuje do tej transakcji.{' '}
                <Link to={householdPath(household.id, 'subscriptions/list')}>Wszystkie subskrypcje</Link>
              </p>
            ) : (
              <ul className={styles.suggestions} aria-label="Pasujące płatności">
                {items.map(({ charge, score }) => (
                  <li key={charge.id} className={styles.suggestion}>
                    <div className={styles.suggestionText}>
                      <span className={styles.suggestionTitle}>
                        <VendorCoin vendor={charge.vendor} name={coinName(charge.vendor, vendors.get(charge.vendor) ?? '', charge.subscription_name)} size="sm" />
                        {charge.subscription_name}
                      </span>
                      <span className={styles.muted}>termin {formatDay(charge.due_date)}</span>
                    </div>
                    <Money amount={charge.amount.amount} currency={charge.amount.currency} />
                    <ScoreMeter score={score} />
                    <Button
                      size="sm"
                      variant="primary"
                      icon="link"
                      disabled={match.isPending}
                      busy={match.isPending && match.variables?.chargeId === charge.id}
                      aria-label={`Powiąż: ${charge.subscription_name}, ${formatDay(charge.due_date)}`}
                      onClick={() =>
                        match.mutate(
                          { chargeId: charge.id, transactionId: transaction.id },
                          {
                            onSuccess: () => {
                              toast.show({ message: `Powiązano z subskrypcją ${charge.subscription_name}.` })
                              onClose()
                            },
                            onError: () => toast.show({ message: 'Nie udało się powiązać transakcji.', kind: 'error' }),
                          },
                        )
                      }
                    >
                      Powiąż
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
