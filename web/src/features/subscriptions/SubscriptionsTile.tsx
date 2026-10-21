import { Link } from 'react-router'
import { useSubscriptionSummary } from '../../api/hooks/subscriptions.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Money } from '../../components/Money/Money.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { RenewalCalendar } from './OverviewPage.tsx'
import styles from './subscriptions.module.css'

/** Pulpit: miesięczny koszt subskrypcji i najbliższe odnowienia. */
export function SubscriptionsTile() {
  const { household } = useHousehold()
  const summary = useSubscriptionSummary(household.id)
  if (summary.isError) {
    return null
  }
  if (!summary.data) {
    return <Skeleton lines={2} />
  }
  const data = summary.data
  const total = data.counts.active + data.counts.trial + data.counts.paused + data.counts.cancelled
  const link = householdPath(household.id, 'subscriptions')
  if (total === 0) {
    return (
      <p className={styles.dashEmpty}>
        Płacisz za Netflixa albo ChatGPT? <Link to={link}>Dodaj subskrypcje</Link>, a pokażemy ich koszt i najbliższe odnowienia.
      </p>
    )
  }
  return (
    <section className={styles.dashTile} aria-label="Subskrypcje">
      <div className={styles.dashCost}>
        <h2>Subskrypcje — koszt/mies.</h2>
        <Money amount={data.monthly.amount} currency={data.currency} size="xl" />
        <span className={styles.muted}>
          rocznie <Money amount={data.yearly.amount} currency={data.currency} size="sm" />
          {data.incomplete && ` (bez ${data.missing_currencies.join(', ')})`}
        </span>
        <Link to={link}>Przegląd subskrypcji</Link>
      </div>
      <div className={styles.dashRenewals}>
        <h2>Najbliższe odnowienia</h2>
        <RenewalCalendar upcoming={data.upcoming} compact />
      </div>
    </section>
  )
}
