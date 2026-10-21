import { Link, useSearchParams } from 'react-router'
import { useSubscriptions, useVendors } from '../../api/hooks/subscriptions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { SubscriptionFilters } from '../../api/keys.ts'
import type { Subscription, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDay } from '../../lib/format.ts'
import { cycleLabel, relativeDay, subscriptionStatusLabels, subscriptionStatusOrder } from '../../lib/subscriptions.ts'
import { BaseEstimate, coinName, SubscriptionStatusBadge, useToday, VendorCoin } from './parts.tsx'
import { useSubscriptionsLayout } from './SubscriptionsLayout.tsx'
import styles from './subscriptions.module.css'

/** Lista subskrypcji z filtrami w adresie (status, dostawca, portfel). */
export function ListPage() {
  const { household } = useHousehold()
  const [params, setParams] = useSearchParams()
  const filters: SubscriptionFilters = {
    status: params.get('status') ?? undefined,
    vendor: params.get('vendor') ?? undefined,
    wallet_id: params.get('wallet') ?? undefined,
  }
  const subscriptions = useSubscriptions(household.id, filters)
  const vendors = useVendors()
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletMap = new Map((wallets.data ?? []).map((w) => [w.id, w]))
  const { create, canWrite } = useSubscriptionsLayout()
  const filtered = Object.values(filters).some(Boolean)

  const set = (name: string, value: string) =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (value) next.set(name, value)
        else next.delete(name)
        return next
      },
      { replace: true },
    )

  return (
    <>
      <form role="search" aria-label="Filtry subskrypcji" className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <SelectField
          label="Status"
          value={params.get('status') ?? ''}
          onChange={(event) => set('status', event.target.value)}
          options={[
            { value: '', label: 'Każdy' },
            ...subscriptionStatusOrder.map((status) => ({ value: status, label: subscriptionStatusLabels[status] })),
          ]}
        />
        <SelectField
          label="Dostawca"
          value={params.get('vendor') ?? ''}
          onChange={(event) => set('vendor', event.target.value)}
          options={[{ value: '', label: 'Każdy' }, ...(vendors.data ?? []).map((v) => ({ value: v.key, label: v.name }))]}
        />
        <SelectField
          label="Portfel"
          value={params.get('wallet') ?? ''}
          onChange={(event) => set('wallet', event.target.value)}
          options={[{ value: '', label: 'Każdy' }, ...(wallets.data ?? []).map((w) => ({ value: w.id, label: w.name }))]}
        />
        {filtered && (
          <Button
            variant="ghost"
            size="sm"
            icon="close"
            className={styles.filtersClear}
            onClick={() => setParams(new URLSearchParams(), { replace: true })}
          >
            Wyczyść filtry
          </Button>
        )}
      </form>
      <QueryState query={subscriptions} errorTitle="Nie udało się wczytać subskrypcji" loading={<Skeleton lines={5} />}>
        {(items) =>
          items.length === 0 ? (
            filtered ? (
              <EmptyState icon="search" title="Brak subskrypcji dla tych filtrów" headingLevel={2}>
                <p>Zmień albo wyczyść filtry.</p>
              </EmptyState>
            ) : (
              <EmptyState
                icon="repeat"
                title="Nie ma jeszcze subskrypcji"
                action={
                  canWrite && (
                    <Button variant="primary" icon="plus" onClick={create}>
                      Dodaj subskrypcję
                    </Button>
                  )
                }
              >
                <p>Netflix, ChatGPT, chmura na zdjęcia — zapisz, za co płacisz co miesiąc lub co rok.</p>
              </EmptyState>
            )
          ) : (
            <ul className={styles.cards} aria-label="Subskrypcje">
              {items.map((s) => (
                <li key={s.id}>
                  <SubscriptionCard subscription={s} wallet={s.wallet_id ? walletMap.get(s.wallet_id) : undefined} />
                </li>
              ))}
            </ul>
          )
        }
      </QueryState>
    </>
  )
}

export function SubscriptionCard({ subscription: s, wallet }: { subscription: Subscription; wallet?: Wallet }) {
  const { household } = useHousehold()
  const today = useToday()
  const inactive = s.status === 'paused' || s.status === 'cancelled'
  return (
    <article className={styles.card} data-inactive={inactive || undefined} aria-label={`Subskrypcja ${s.name}`}>
      <VendorCoin vendor={s.vendor} name={coinName(s.vendor, s.vendor_name, s.name)} size="lg" />
      <div className={styles.cardMain}>
        <Link to={householdPath(household.id, `subscriptions/${s.id}`)} className={styles.cardTitle}>
          {s.name}
        </Link>
        <span className={styles.muted}>
          {s.vendor !== 'other' ? `${s.vendor_name}, ` : ''}
          {cycleLabel(s.cycle)}
          {wallet ? `, z portfela ${wallet.name}` : ''}
        </span>
        <span className={styles.cardStatus}>
          <SubscriptionStatusBadge subscription={s} />
          {s.status === 'trial' && s.trial_ends_at && <span className={styles.muted}>do {formatDay(s.trial_ends_at)}</span>}
          {s.status === 'cancelled' && s.ends_at && <span className={styles.muted}>dostęp do {formatDay(s.ends_at)}</span>}
        </span>
      </div>
      <div className={styles.cardPrice}>
        <Money amount={s.price.amount} currency={s.price.currency} size="lg" />
        <BaseEstimate value={s.price} />
        <span className={styles.cardNext}>
          {s.next_due && !inactive ? (
            <>
              Następna: <time dateTime={s.next_due}>{formatDay(s.next_due)}</time>{' '}
              <span className={styles.muted}>({relativeDay(s.next_due, today)})</span>
            </>
          ) : (
            <span className={styles.muted}>Bez kolejnych płatności</span>
          )}
        </span>
      </div>
    </article>
  )
}
