import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink } from 'react-router'
import { useFxQuote } from '../../api/hooks/fx.ts'
import { useCharges, useVendors } from '../../api/hooks/subscriptions.ts'
import type { Charge, ChargeStatus, Money as MoneyValue, Subscription, SubscriptionStatus } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { convertAmount } from '../../lib/money.ts'
import { useDecimals } from '../transactions/money.ts'
import { todayIn } from '../../lib/format.ts'
import {
  chargeStatusDescriptions,
  chargeStatusLabels,
  monogram,
  subscriptionStatusLabels,
  vendorMetal,
} from '../../lib/subscriptions.ts'
import styles from './subscriptions.module.css'

type CoinProps = {
  /** Klucz dostawcy (stop monety). */
  vendor: string
  /** Nazwa, z której powstaje monogram. */
  name: string
  size?: 'sm' | 'md' | 'lg'
}

/**
 * Moneta dostawcy: monogram na krążku z karbowanym rantem, w stopie
 * wyznaczonym przez klucz. Bez zewnętrznych logo (CSP `img-src 'self'`).
 * Dekoracja — nazwę zawsze podaje tekst obok.
 */
export function VendorCoin({ vendor, name, size = 'md' }: CoinProps) {
  return (
    <span className={`${styles.coin} ${styles[`coin-${size}`]}`} data-metal={vendorMetal(vendor)} aria-hidden="true">
      <span>{monogram(name)}</span>
    </span>
  )
}

/** Nazwa na monetę: dostawca, a dla „Inne” — sama subskrypcja. */
export function coinName(vendor: string, vendorName: string, subscriptionName: string): string {
  return vendor === 'other' || !vendorName ? subscriptionName : vendorName
}

/** Nazwy dostawców po kluczu (ze słownika). */
export function useVendorNames(): Map<string, string> {
  const vendors = useVendors()
  return useMemo(() => new Map((vendors.data ?? []).map((v) => [v.key, v.name])), [vendors.data])
}

const statusTone: Record<SubscriptionStatus, 'positive' | 'brand' | 'warn' | 'neutral'> = {
  active: 'positive',
  trial: 'brand',
  paused: 'warn',
  cancelled: 'neutral',
}

export function SubscriptionStatusBadge({ subscription }: { subscription: Pick<Subscription, 'status'> }) {
  const status = subscription.status
  return (
    <Badge tone={statusTone[status]}>
      {status === 'paused' && <Icon name="pause" size={12} />}
      {status === 'trial' && <Icon name="sparkle" size={12} />}
      {subscriptionStatusLabels[status]}
    </Badge>
  )
}

const chargeTone: Record<ChargeStatus, 'positive' | 'brand' | 'warn' | 'neutral' | 'negative'> = {
  expected: 'neutral',
  matched: 'positive',
  posted: 'positive',
  missed: 'negative',
  skipped: 'neutral',
}

export function ChargeStatusBadge({ charge }: { charge: Pick<Charge, 'status'> }) {
  return (
    <Badge tone={chargeTone[charge.status]} title={chargeStatusDescriptions[charge.status]} struck={charge.status === 'skipped'}>
      {(charge.status === 'matched' || charge.status === 'posted') && <Icon name="check" size={12} />}
      {charge.status === 'missed' && <Icon name="alert" size={12} />}
      {chargeStatusLabels[charge.status]}
    </Badge>
  )
}

/** Dzisiejsza data gospodarstwa. */
export function useToday(): string {
  const { household } = useHousehold()
  return todayIn(household.timezone)
}

/** Obciążenia do rozstrzygnięcia: przegapione i oczekiwane z terminem do dziś. */
export function useOpenCharges(options: { enabled?: boolean } = {}) {
  const { household } = useHousehold()
  const today = useToday()
  return useCharges(household.id, { status: ['expected', 'missed'], to: today }, options)
}

export function SubscriptionTabs() {
  const { household } = useHousehold()
  const open = useOpenCharges()
  const pending = open.data?.pages.flatMap((page) => page.items).length ?? 0
  const more = open.hasNextPage
  const { t } = useTranslation('subscriptions')
  const tabs = [
    { path: 'subscriptions', label: t('tabs.overview'), end: true },
    { path: 'subscriptions/list', label: t('tabs.all'), end: true },
    { path: 'subscriptions/charges', label: t('tabs.charges'), end: true, count: pending },
  ]
  return (
    <nav aria-label={t('tabs.label')} className={styles.tabs}>
      <ul>
        {tabs.map((tab) => (
          <li key={tab.path}>
            <NavLink
              to={householdPath(household.id, tab.path)}
              end={tab.end}
              className={({ isActive }) => (isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab)}
            >
              {tab.label}
              {tab.count ? (
                <span className={styles.tabCount}>
                  <span className="visually-hidden">: </span>
                  {tab.count}
                  {more ? '+' : ''}
                </span>
              ) : null}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}

/**
 * Szacunek kwoty w walucie bazowej po dzisiejszym kursie („≈ 80,00 zł”);
 * nic dla waluty bazowej albo bez kursu.
 */
export function BaseEstimate({ value, suffix }: { value: MoneyValue; suffix?: string }) {
  const { household } = useHousehold()
  const today = useToday()
  const decimalsFor = useDecimals()
  const foreign = value.currency !== household.base_currency
  const quote = useFxQuote(household.id, foreign ? value.currency : '', today, { enabled: foreign })
  const rate = quote.data?.rate
  if (!foreign || !rate) {
    return null
  }
  const amount = convertAmount(value.amount, decimalsFor(value.currency), rate, decimalsFor(household.base_currency))
  if (amount === null) {
    return null
  }
  return (
    <span className={styles.estimate}>
      ≈ <Money amount={amount} currency={household.base_currency} size="sm" />
      {suffix}
    </span>
  )
}
