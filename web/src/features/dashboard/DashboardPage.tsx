import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useSessionUser } from '../../api/hooks/session.ts'
import { useRecentTransactions } from '../../api/hooks/transactions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Icon, type IconName } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDay } from '../../lib/format.ts'
import { roleLabels, walletTypeLabels } from '../../lib/labels.ts'
import { useDecimals } from '../transactions/money.ts'
import { TransactionDialog } from '../transactions/TransactionDialog.tsx'
import { transactionTitle } from '../transactions/TransactionRow.tsx'
import { walletTotals } from '../wallets/totals.ts'
import { WalletDialog } from '../wallets/WalletDialog.tsx'
import { WalletSummary } from '../wallets/WalletCard.tsx'
import { BudgetTile } from '../budget/BudgetTile.tsx'
import { SubscriptionsTile } from '../subscriptions/SubscriptionsTile.tsx'
import { RecentNotificationsTile } from '../notifications/RecentNotificationsTile.tsx'
import placeholder from '../placeholders/placeholders.module.css'
import styles from './dashboard.module.css'

function greeting(now = new Date()): 'evening' | 'day' {
  const hour = now.getHours()
  if (hour < 5) return 'evening'
  if (hour < 18) return 'day'
  return 'evening'
}

type Step = { to: string; icon: IconName; title: string; text: string }

const walletsShown = 6
const recentShown = 5

/**
 * Pulpit: salda portfeli w walucie bazowej, portfele i ostatnie transakcje.
 * Budżet (część C1): kafelek „Do przydzielenia” i przekroczone koperty.
 * Subskrypcje (część C2): koszt miesięczny i najbliższe odnowienia.
 * Powiadomienia (część C3b): trzy ostatnie z gospodarstwa.
 */
export function DashboardPage() {
  const { t } = useTranslation('dashboard')
  const { household, role } = useHousehold()
  const { user } = useSessionUser()
  const firstName = user.name.split(/\s+/)[0] ?? user.name
  const wallets = useWallets(household.id)
  const hasWallets = (wallets.data?.length ?? 0) > 0
  const recent = useRecentTransactions(household.id, recentShown, { enabled: hasWallets })
  const decimalsFor = useDecimals()
  const canWrite = role !== 'viewer'
  const [adding, setAdding] = useState(false)
  const [creating, setCreating] = useState(false)
  const walletMap = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w])), [wallets.data])

  const steps: Step[] = [
    ...(canManage(role)
      ? [
          {
            to: householdPath(household.id, 'settings/invitations'),
            icon: 'users' as const,
            title: t('steps.inviteTitle'),
            text: t('steps.inviteText'),
          },
        ]
      : []),
    ...(role !== 'viewer'
      ? [
          {
            to: householdPath(household.id, 'settings/bots'),
            icon: 'bot' as const,
            title: t('steps.botTitle'),
            text: t('steps.botText'),
          },
        ]
      : []),
    {
      to: householdPath(household.id, 'settings'),
      icon: 'settings',
      title: t('steps.settingsTitle'),
      text: t('steps.settingsText'),
    },
  ]

  return (
    <>
      <PageHeader
        title={t('title', { greeting: t(`greeting.${greeting()}`), name: firstName })}
        documentTitle={t('documentTitle')}
        actions={
          canWrite &&
          hasWallets && (
            <Button variant="primary" icon="plus" onClick={() => setAdding(true)}>
              {t('addTransaction')}
            </Button>
          )
        }
      />
      <div className={styles.layout}>
        <QueryState query={wallets} errorTitle={t('walletsLoadFailed')} loading={<Skeleton lines={3} />}>
          {(items) =>
            items.length === 0 ? (
              <>
                <WalletSummary
                  totals={walletTotals([], decimalsFor(household.base_currency))}
                  currency={household.base_currency}
                />
                <EmptyState
                  icon="wallet"
                  title={t('firstWallet.title')}
                  action={
                    canWrite && (
                      <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                        {t('firstWallet.action')}
                      </Button>
                    )
                  }
                >
                  <p>{t('firstWallet.body')}</p>
                </EmptyState>
              </>
            ) : (
              <>
                <WalletSummary
                  totals={walletTotals(items, decimalsFor(household.base_currency))}
                  currency={household.base_currency}
                />
                <BudgetTile />
                <SubscriptionsTile />
                <div className={styles.columns}>
                  <section className={styles.panel} aria-labelledby="dash-wallets">
                    <div className={styles.panelHead}>
                      <h2 id="dash-wallets">{t('wallets.title')}</h2>
                      <Link to={householdPath(household.id, 'wallets')}>{t('wallets.all')}</Link>
                    </div>
                    <ul className={styles.wallets}>
                      {items.slice(0, walletsShown).map((wallet) => (
                        <li key={wallet.id}>
                          <Link to={householdPath(household.id, `wallets/${wallet.id}`)} className={styles.walletLink}>
                            <span className={styles.walletName}>{wallet.name}</span>
                            <span className={styles.walletAmount}>
                              <Money amount={wallet.balance.amount} currency={wallet.currency} />
                              {wallet.currency !== household.base_currency && (
                                <span className={styles.walletMeta}>
                                  ≈ <Money amount={wallet.base_balance.amount} currency={wallet.base_balance.currency} size="sm" />
                                </span>
                              )}
                            </span>
                            <span className={styles.walletMeta}>
                              {walletTypeLabels[wallet.type]}
                              {!wallet.on_budget && t('wallets.offBudget')}
                            </span>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  </section>
                  <section className={styles.panel} aria-labelledby="dash-recent">
                    <div className={styles.panelHead}>
                      <h2 id="dash-recent">{t('recent.title')}</h2>
                      <Link to={householdPath(household.id, 'transactions')}>{t('recent.all')}</Link>
                    </div>
                    <QueryState query={recent} errorTitle={t('recent.loadFailed')} loading={<Skeleton lines={3} />}>
                      {(transactions) =>
                        transactions.length === 0 ? (
                          <p className={styles.walletMeta}>{t('recent.empty')}</p>
                        ) : (
                          <ul className={styles.recent} aria-label={t('recent.title')}>
                            {transactions.map((tx) => (
                              <li key={tx.id}>
                                <span className={styles.recentTitle}>{transactionTitle(tx, walletMap)}</span>
                                <Money amount={tx.amount.amount} currency={tx.amount.currency} tone="auto" />
                                <span className={styles.walletMeta}>
                                  {formatDay(tx.date)} · {walletMap.get(tx.wallet_id)?.name ?? t('recent.walletFallback')}
                                </span>
                              </li>
                            ))}
                          </ul>
                        )
                      }
                    </QueryState>
                  </section>
                </div>
              </>
            )
          }
        </QueryState>

        <RecentNotificationsTile />

        <dl className={styles.facts} aria-label={t('facts.label')}>
          <div>
            <dt>{t('facts.role')}</dt>
            <dd>{roleLabels[role]}</dd>
          </div>
          <div>
            <dt>{t('facts.currency')}</dt>
            <dd>{household.base_currency}</dd>
          </div>
          <div>
            <dt>{t('facts.monthStart')}</dt>
            <dd>{t('facts.monthStartDay', { day: household.month_start_day })}</dd>
          </div>
          <div>
            <dt>{t('facts.timezone')}</dt>
            <dd>{household.timezone}</dd>
          </div>
        </dl>

        <section className={placeholder.steps} aria-labelledby="next-steps">
          <h2 id="next-steps">{t('steps.title')}</h2>
          <ul>
            {steps.map((step) => (
              <li key={step.to}>
                <Link to={step.to} className={placeholder.step}>
                  <Icon name={step.icon} size={22} />
                  <span>
                    <span className={placeholder.stepTitle}>{step.title}</span>
                    <span className={placeholder.stepText}>{step.text}</span>
                  </span>
                  <Icon name="chevronRight" size={18} />
                </Link>
              </li>
            ))}
          </ul>
        </section>
      </div>
      <TransactionDialog open={adding} onClose={() => setAdding(false)} />
      <WalletDialog open={creating} onClose={() => setCreating(false)} />
    </>
  )
}
