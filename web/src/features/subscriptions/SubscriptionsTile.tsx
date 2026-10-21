import { Trans, useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('subscriptions')
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
        <Trans t={t} i18nKey="tile.empty" components={{ link: <Link to={link} /> }} />
      </p>
    )
  }
  return (
    <section className={styles.dashTile} aria-label={t('tile.label')}>
      <div className={styles.dashCost}>
        <h2>{t('tile.cost')}</h2>
        <Money amount={data.monthly.amount} currency={data.currency} size="xl" />
        <span className={styles.muted}>
          {t('tile.yearly')} <Money amount={data.yearly.amount} currency={data.currency} size="sm" />
          {data.incomplete && t('tile.without', { currencies: data.missing_currencies.join(', ') })}
        </span>
        <Link to={link}>{t('tile.overview')}</Link>
      </div>
      <div className={styles.dashRenewals}>
        <h2>{t('tile.renewals')}</h2>
        <RenewalCalendar upcoming={data.upcoming} compact />
      </div>
    </section>
  )
}
