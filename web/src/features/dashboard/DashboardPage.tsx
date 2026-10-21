import { useMemo, useState } from 'react'
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
import placeholder from '../placeholders/placeholders.module.css'
import styles from './dashboard.module.css'

function greeting(now = new Date()): string {
  const hour = now.getHours()
  if (hour < 5) return 'Dobry wieczór'
  if (hour < 18) return 'Dzień dobry'
  return 'Dobry wieczór'
}

type Step = { to: string; icon: IconName; title: string; text: string }

const walletsShown = 6
const recentShown = 5

/**
 * Pulpit: salda portfeli w walucie bazowej, portfele i ostatnie transakcje.
 * Budżet (część C1): kafelek „Do przydzielenia” i przekroczone koperty.
 * Subskrypcje (część C2): koszt miesięczny i najbliższe odnowienia.
 */
export function DashboardPage() {
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
            title: 'Zaproś domowników',
            text: 'Wyślij link z zaproszeniem i wybierz, co każdy może robić.',
          },
        ]
      : []),
    ...(role !== 'viewer'
      ? [
          {
            to: householdPath(household.id, 'settings/bots'),
            icon: 'bot' as const,
            title: 'Podłącz asystenta AI',
            text: 'Claude lub inny asystent przez MCP: utwórz bota i skopiuj gotowe polecenie podłączenia.',
          },
        ]
      : []),
    {
      to: householdPath(household.id, 'settings'),
      icon: 'settings',
      title: 'Sprawdź ustawienia',
      text: 'Strefa czasowa i dzień, od którego liczysz miesiąc budżetowy.',
    },
  ]

  return (
    <>
      <PageHeader
        title={`${greeting()}, ${firstName}`}
        documentTitle="Pulpit"
        actions={
          canWrite &&
          hasWallets && (
            <Button variant="primary" icon="plus" onClick={() => setAdding(true)}>
              Dodaj transakcję
            </Button>
          )
        }
      />
      <div className={styles.layout}>
        <QueryState query={wallets} errorTitle="Nie udało się wczytać portfeli" loading={<Skeleton lines={3} />}>
          {(items) =>
            items.length === 0 ? (
              <>
                <WalletSummary
                  totals={walletTotals([], decimalsFor(household.base_currency))}
                  currency={household.base_currency}
                />
                <EmptyState
                  icon="wallet"
                  title="Dodaj pierwszy portfel"
                  action={
                    canWrite && (
                      <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                        Dodaj portfel
                      </Button>
                    )
                  }
                >
                  <p>Konto, gotówka albo karta — od portfela zaczyna się ewidencja wydatków i budżet.</p>
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
                      <h2 id="dash-wallets">Portfele</h2>
                      <Link to={householdPath(household.id, 'wallets')}>Wszystkie portfele</Link>
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
                              {!wallet.on_budget && ' · poza budżetem'}
                            </span>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  </section>
                  <section className={styles.panel} aria-labelledby="dash-recent">
                    <div className={styles.panelHead}>
                      <h2 id="dash-recent">Ostatnie transakcje</h2>
                      <Link to={householdPath(household.id, 'transactions')}>Cały rejestr</Link>
                    </div>
                    <QueryState query={recent} errorTitle="Nie udało się wczytać transakcji" loading={<Skeleton lines={3} />}>
                      {(transactions) =>
                        transactions.length === 0 ? (
                          <p className={styles.walletMeta}>Nie ma jeszcze transakcji.</p>
                        ) : (
                          <ul className={styles.recent} aria-label="Ostatnie transakcje">
                            {transactions.map((t) => (
                              <li key={t.id}>
                                <span className={styles.recentTitle}>{transactionTitle(t, walletMap)}</span>
                                <Money amount={t.amount.amount} currency={t.amount.currency} tone="auto" />
                                <span className={styles.walletMeta}>
                                  {formatDay(t.date)} · {walletMap.get(t.wallet_id)?.name ?? 'portfel'}
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

        <dl className={styles.facts} aria-label="Gospodarstwo">
          <div>
            <dt>Twoja rola:</dt>
            <dd>{roleLabels[role]}</dd>
          </div>
          <div>
            <dt>Waluta bazowa:</dt>
            <dd>{household.base_currency}</dd>
          </div>
          <div>
            <dt>Miesiąc budżetowy od:</dt>
            <dd>{household.month_start_day}. dnia</dd>
          </div>
          <div>
            <dt>Strefa:</dt>
            <dd>{household.timezone}</dd>
          </div>
        </dl>

        <section className={placeholder.steps} aria-labelledby="next-steps">
          <h2 id="next-steps">Następne kroki</h2>
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
