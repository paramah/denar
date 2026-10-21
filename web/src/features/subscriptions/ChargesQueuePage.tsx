import { useState } from 'react'
import { Link } from 'react-router'
import type { Charge } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDay } from '../../lib/format.ts'
import { relativeDay } from '../../lib/subscriptions.ts'
import { ChargeDialogs, SuggestionList, type ChargeModal } from './charges.tsx'
import { ChargeStatusBadge, coinName, useOpenCharges, useToday, useVendorNames, VendorCoin } from './parts.tsx'
import { useSubscriptionsLayout } from './SubscriptionsLayout.tsx'
import styles from './subscriptions.module.css'

/**
 * Kolejka dopasowań: przegapione i oczekiwane płatności z terminem do dziś,
 * każda z najlepszymi sugestiami transakcji i szybkim potwierdzeniem.
 */
export function ChargesQueuePage() {
  const charges = useOpenCharges()
  const { canWrite } = useSubscriptionsLayout()
  const [modal, setModal] = useState<ChargeModal>(null)
  return (
    <>
      <p className={styles.lead}>
        Płatności, których termin już minął, a w rejestrze nie ma jeszcze powiązanej transakcji. Potwierdź sugestię, zaksięguj wydatek
        albo pomiń termin.
      </p>
      <QueryState query={charges} errorTitle="Nie udało się wczytać płatności" loading={<Skeleton lines={4} />}>
        {(data) => {
          const items = data.pages.flatMap((page) => page.items)
          if (items.length === 0) {
            return (
              <EmptyState icon="checkCircle" title="Wszystko dopasowane" headingLevel={2}>
                <p>Każda płatność z minionym terminem ma swoją transakcję albo została pominięta.</p>
              </EmptyState>
            )
          }
          // Najpierw przegapione, potem od najstarszego terminu.
          const sorted = [...items].sort(
            (a, b) => Number(b.status === 'missed') - Number(a.status === 'missed') || a.due_date.localeCompare(b.due_date),
          )
          return (
            <>
              <ul className={styles.queue} aria-label="Płatności do dopasowania">
                {sorted.map((charge) => (
                  <li key={charge.id}>
                    <QueueItem charge={charge} canWrite={canWrite} onAction={setModal} />
                  </li>
                ))}
              </ul>
              {charges.hasNextPage && (
                <Button onClick={() => void charges.fetchNextPage()} busy={charges.isFetchingNextPage}>
                  Pokaż więcej
                </Button>
              )}
            </>
          )
        }}
      </QueryState>
      <ChargeDialogs modal={modal} onClose={() => setModal(null)} onChange={setModal} />
    </>
  )
}

function QueueItem({ charge, canWrite, onAction }: { charge: Charge; canWrite: boolean; onAction: (m: ChargeModal) => void }) {
  const { household } = useHousehold()
  const today = useToday()
  const vendors = useVendorNames()
  const name = coinName(charge.vendor, vendors.get(charge.vendor) ?? '', charge.subscription_name)
  return (
    <article className={styles.queueItem} data-status={charge.status} aria-label={`${charge.subscription_name}, płatność ${formatDay(charge.due_date)}`}>
      <header className={styles.queueHead}>
        <VendorCoin vendor={charge.vendor} name={name} size="md" />
        <div className={styles.queueTitle}>
          <h2>
            <Link to={householdPath(household.id, `subscriptions/${charge.subscription_id}`)}>{charge.subscription_name}</Link>
          </h2>
          <span className={styles.muted}>
            termin {formatDay(charge.due_date)} ({relativeDay(charge.due_date, today)})
          </span>
        </div>
        <ChargeStatusBadge charge={charge} />
        <Money amount={charge.amount.amount} currency={charge.amount.currency} size="lg" />
      </header>
      {canWrite ? (
        <>
          <SuggestionList charge={charge} limit={3} compact />
          <div className={styles.queueActions}>
            <Button size="sm" onClick={() => onAction({ kind: 'post', charge })}>
              Zaksięguj
            </Button>
            <Button size="sm" variant="ghost" onClick={() => onAction({ kind: 'skip', charge })}>
              Pomiń
            </Button>
          </div>
        </>
      ) : null}
    </article>
  )
}
