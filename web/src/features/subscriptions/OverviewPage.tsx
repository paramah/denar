import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useSubscriptionSummary } from '../../api/hooks/subscriptions.ts'
import type { SubscriptionSummary, UpcomingRenewal } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Callout } from '../../components/Form/Form.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDay } from '../../lib/format.ts'
import { addDaysTo, daysBetween, relativeDay, share } from '../../lib/subscriptions.ts'
import { useEnvelopeNames } from '../transactions/envelopes.ts'
import { useDecimals } from '../transactions/money.ts'
import { coinName, useToday, useVendorNames, VendorCoin } from './parts.tsx'
import { useSubscriptionsLayout } from './SubscriptionsLayout.tsx'
import styles from './subscriptions.module.css'

export const calendarDays = 30

/** Przegląd: koszt w walucie bazowej, kalendarz odnowień, rozbicie, triale, zmiany cen. */
export function OverviewPage() {
  const { household } = useHousehold()
  const summary = useSubscriptionSummary(household.id)
  const { create, canWrite } = useSubscriptionsLayout()
  const { t } = useTranslation('subscriptions')
  return (
    <QueryState query={summary} errorTitle={t('overview.loadError')} loading={<Skeleton lines={6} />}>
      {(data) => {
        const total = data.counts.active + data.counts.trial + data.counts.paused + data.counts.cancelled
        if (total === 0) {
          return (
            <EmptyState
              icon="repeat"
              title={t('overview.emptyTitle')}
              action={
                canWrite && (
                  <Button variant="primary" icon="plus" onClick={create}>
                    {t('overview.emptyAction')}
                  </Button>
                )
              }
            >
              <p>{t('overview.emptyBody')}</p>
            </EmptyState>
          )
        }
        return <Overview summary={data} />
      }}
    </QueryState>
  )
}

function Overview({ summary }: { summary: SubscriptionSummary }) {
  const { household, role } = useHousehold()
  const { t } = useTranslation('subscriptions')
  const counts = summary.counts
  const countParts = (['active', 'trial', 'paused', 'cancelled'] as const)
    .filter((status) => counts[status] > 0)
    .map((status, i) => {
      const label = t(`overview.counts.${status}`)
      return t('overview.count', { label: i === 0 ? label : label.toLocaleLowerCase(), value: counts[status] })
    })
  return (
    <>
      <section className={styles.hero} aria-labelledby="cost-heading">
        <div className={styles.heroCost}>
          <h2 id="cost-heading" className={styles.heroLabel}>
            {t('overview.costHeading')}
          </h2>
          <p className={styles.heroAmount}>
            <Money amount={summary.monthly.amount} currency={summary.currency} size="xl" />
            {summary.incomplete && <span className={styles.heroMark}>{t('overview.partial')}</span>}
          </p>
          <p className={styles.heroYear}>
            {t('overview.yearly')} <Money amount={summary.yearly.amount} currency={summary.currency} />
          </p>
          <p className={styles.muted}>{t('overview.countsNote', { counts: countParts.join(', ') })}</p>
          {summary.incomplete && (
            <Callout tone="warn" title={t('overview.incompleteTitle')}>
              <p>
                {t('overview.incompleteBody', { currencies: summary.missing_currencies.join(', '), base: summary.currency })}{' '}
                {canManage(role) && (
                  <Link to={`${householdPath(household.id, 'settings/rates')}?currency=${summary.missing_currencies[0] ?? ''}`}>
                    {t('overview.setRate')}
                  </Link>
                )}
              </p>
            </Callout>
          )}
        </div>
        <RenewalCalendar upcoming={summary.upcoming} />
      </section>

      <div className={styles.columns}>
        <VendorBreakdown summary={summary} />
        <EnvelopeBreakdown summary={summary} />
      </div>

      <div className={styles.columns}>
        <TrialsEnding summary={summary} />
        <PriceChanges summary={summary} />
      </div>
    </>
  )
}

/**
 * Kalendarz najbliższych 30 dni: pasek dni z monetami w dniach płatności
 * (dekoracja, aria-hidden) i lista odnowień, która niesie tę samą treść.
 */
export function RenewalCalendar({ upcoming, compact }: { upcoming: UpcomingRenewal[]; compact?: boolean }) {
  const { household } = useHousehold()
  const today = useToday()
  const vendors = useVendorNames()
  const { t } = useTranslation('subscriptions')
  const items = upcoming.filter((u) => {
    const offset = daysBetween(today, u.charge.due_date)
    return offset >= 0 && offset <= calendarDays
  })
  const stacks = new Map<number, number>()
  const markers = items.map((u) => {
    const offset = daysBetween(today, u.charge.due_date)
    const row = (stacks.get(offset) ?? 0) + 1
    stacks.set(offset, row)
    return { u, offset, row }
  })
  const days = Array.from({ length: calendarDays + 1 }, (_, i) => addDaysTo(today, i))
  const shown = compact ? items.slice(0, 4) : items

  return (
    <section className={styles.calendar} aria-labelledby={compact ? undefined : 'renewals-heading'} aria-label={compact ? t('calendar.label') : undefined}>
      {!compact && (
        <div className={styles.sectionHead}>
          <h2 id="renewals-heading">{t('calendar.label')}</h2>
          <span className={styles.muted}>
            {formatDay(today)} – {formatDay(days[days.length - 1]!)}
          </span>
        </div>
      )}
      <div className={styles.strip} aria-hidden="true">
        {days.map((day, i) => {
          const date = new Date(`${day}T12:00:00`)
          const weekday = date.getDay()
          return (
            <span
              key={day}
              className={styles.stripDay}
              data-weekend={weekday === 0 || weekday === 6 || undefined}
              data-today={i === 0 || undefined}
              ref={(el) => el?.style.setProperty('grid-column', String(i + 1))}
            >
              {(i === 0 || weekday === 1) && <span className={styles.stripLabel}>{i === 0 ? t('relative.today') : date.getDate()}</span>}
            </span>
          )
        })}
        {markers.map(({ u, offset, row }) => (
          <span
            key={u.charge.id}
            className={styles.stripMarker}
            ref={(el) => {
              el?.style.setProperty('grid-column', String(offset + 1))
              el?.style.setProperty('grid-row', String(row + 1))
            }}
          >
            <VendorCoin vendor={u.charge.vendor} name={coinName(u.charge.vendor, vendors.get(u.charge.vendor) ?? '', u.charge.subscription_name)} size="sm" />
          </span>
        ))}
      </div>
      {items.length === 0 ? (
        <p className={styles.muted}>{t('calendar.nothing', { days: calendarDays })}</p>
      ) : (
        <ol className={styles.renewals} aria-label={t('calendar.list', { days: calendarDays })}>
          {shown.map((u) => (
            <li key={u.charge.id} className={styles.renewal}>
              <span className={styles.renewalWhen}>
                <time dateTime={u.charge.due_date}>{formatDay(u.charge.due_date)}</time>
                <span className={styles.muted}>{relativeDay(u.charge.due_date, today)}</span>
              </span>
              <Link to={householdPath(household.id, `subscriptions/${u.charge.subscription_id}`)} className={styles.renewalName}>
                <VendorCoin vendor={u.charge.vendor} name={coinName(u.charge.vendor, vendors.get(u.charge.vendor) ?? '', u.charge.subscription_name)} size="sm" />
                {u.charge.subscription_name}
              </Link>
              <span className={styles.renewalAmount}>
                <Money amount={u.charge.amount.amount} currency={u.charge.amount.currency} />
                {u.base_amount && u.base_amount.currency !== u.charge.amount.currency && (
                  <span className={styles.muted}>
                    ≈ <Money amount={u.base_amount.amount} currency={u.base_amount.currency} size="sm" />
                  </span>
                )}
              </span>
            </li>
          ))}
        </ol>
      )}
      {compact && items.length > shown.length && (
        <p className={styles.muted}>
          {t('calendar.more', { count: items.length - shown.length })}
        </p>
      )}
    </section>
  )
}

type Bar = { key: string; label: string; vendor?: string; count: number; monthly: string }

function Bars({ title, bars, currency, empty }: { title: string; bars: Bar[]; currency: string; empty: string }) {
  const decimalsFor = useDecimals()
  const { t } = useTranslation('subscriptions')
  const decimals = decimalsFor(currency)
  const max = bars[0]?.monthly ?? '0'
  const id = `bars-${title.replace(/\s+/g, '-')}`
  return (
    <section className={styles.panel} aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      {bars.length === 0 ? (
        <p className={styles.muted}>{empty}</p>
      ) : (
        <ul className={styles.bars}>
          {bars.map((bar) => (
            <li key={bar.key} className={styles.bar}>
              <span className={styles.barLabel}>
                {bar.vendor && <VendorCoin vendor={bar.vendor} name={bar.label} size="sm" />}
                {bar.label}
                <span className={styles.muted}>
                  {t('bars.count', { count: bar.count })}
                </span>
              </span>
              <Money amount={bar.monthly} currency={currency} />
              <span className={styles.barTrack} aria-hidden="true">
                <span
                  className={styles.barFill}
                  data-metal={bar.vendor ? undefined : 'patina'}
                  ref={(el) => el?.style.setProperty('--share', `${share(bar.monthly, max, decimals)}%`)}
                />
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function VendorBreakdown({ summary }: { summary: SubscriptionSummary }) {
  const { t } = useTranslation('subscriptions')
  const bars: Bar[] = summary.by_vendor.map((v) => ({
    key: v.vendor,
    label: v.vendor_name,
    vendor: v.vendor,
    count: v.count,
    monthly: v.monthly.amount,
  }))
  return <Bars title={t('bars.byVendor')} bars={bars} currency={summary.currency} empty={t('bars.byVendorEmpty')} />
}

function EnvelopeBreakdown({ summary }: { summary: SubscriptionSummary }) {
  const { household } = useHousehold()
  const names = useEnvelopeNames(household.id)
  const { t } = useTranslation('subscriptions')
  const bars: Bar[] = summary.by_envelope.map((e) => ({
    key: e.envelope_id ?? 'none',
    label: e.envelope_id ? (names.get(e.envelope_id) ?? t('bars.envelope')) : t('bars.noEnvelope'),
    count: e.count,
    monthly: e.monthly.amount,
  }))
  return (
    <Bars
      title={t('bars.byEnvelope')}
      bars={bars}
      currency={summary.currency}
      empty={t('bars.byEnvelopeEmpty')}
    />
  )
}

function TrialsEnding({ summary }: { summary: SubscriptionSummary }) {
  const { household } = useHousehold()
  const { t } = useTranslation('subscriptions')
  const today = useToday()
  return (
    <section className={styles.panel} aria-labelledby="trials-heading">
      <h2 id="trials-heading">{t('trials.heading')}</h2>
      {summary.trials_ending.length === 0 ? (
        <p className={styles.muted}>{t('trials.empty')}</p>
      ) : (
        <ul className={styles.plainList}>
          {summary.trials_ending.map((trial) => (
            <li key={trial.subscription_id} className={styles.listRow}>
              <Link to={householdPath(household.id, `subscriptions/${trial.subscription_id}`)}>{trial.name}</Link>
              <span>
                {formatDay(trial.trial_ends_at)} <span className={styles.muted}>({relativeDay(trial.trial_ends_at, today)})</span>
              </span>
              <span className={styles.muted}>
                {t('trials.then')} <Money amount={trial.price.amount} currency={trial.price.currency} size="sm" />
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function PriceChanges({ summary }: { summary: SubscriptionSummary }) {
  const { household } = useHousehold()
  const { t } = useTranslation('subscriptions')
  return (
    <section className={styles.panel} aria-labelledby="prices-heading">
      <h2 id="prices-heading">{t('priceChanges.heading')}</h2>
      {summary.price_changes.length === 0 ? (
        <p className={styles.muted}>{t('priceChanges.empty')}</p>
      ) : (
        <ul className={styles.plainList}>
          {summary.price_changes.map((change) => (
            <li key={`${change.subscription_id}-${change.effective_from}`} className={styles.listRow}>
              <Link to={householdPath(household.id, `subscriptions/${change.subscription_id}`)}>{change.name}</Link>
              <span className={styles.priceShift}>
                <Money amount={change.old_price.amount} currency={change.old_price.currency} size="sm" />
                <span aria-hidden="true">→</span>
                <span className="visually-hidden">{t('priceChanges.changedTo')}</span>
                <Money amount={change.new_price.amount} currency={change.new_price.currency} size="sm" />
              </span>
              <span className={styles.muted}>{t('priceChanges.from', { date: formatDay(change.effective_from) })}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
