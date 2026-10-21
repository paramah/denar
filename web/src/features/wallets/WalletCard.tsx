import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { walletTypeLabels } from '../../lib/labels.ts'
import type { WalletTotals } from './totals.ts'
import styles from './wallets.module.css'

/** Plakietki stanu portfela: poza budżetem, prywatny, archiwum, czeka na kurs. */
export function WalletBadges({ wallet }: { wallet: Wallet }) {
  const { t } = useTranslation('wallets')
  return (
    <span className={styles.badges}>
      {!wallet.on_budget && <Badge>{t('badges.offBudget')}</Badge>}
      {wallet.visibility === 'private' && (
        <Badge tone="brand" title={t('badges.privateTitle')}>
          <Icon name="lock" size={12} />
          {t('badges.private')}
        </Badge>
      )}
      {wallet.archived_at && <Badge tone="warn">{t('badges.archived')}</Badge>}
      {wallet.rate_pending && (
        <Badge tone="warn" title={t('badges.ratePendingTitle')}>
          <Icon name="clock" size={12} />
          {t('badges.ratePending')}
        </Badge>
      )}
    </span>
  )
}

export function WalletCard({ wallet }: { wallet: Wallet }) {
  const { t } = useTranslation('wallets')
  const { household } = useHousehold()
  const foreign = wallet.currency !== household.base_currency
  return (
    <article
      className={styles.card}
      data-on-budget={wallet.on_budget}
      data-archived={!!wallet.archived_at}
      aria-label={t('card.label', { name: wallet.name })}
    >
      <div className={styles.cardHead}>
        <Link to={householdPath(household.id, `wallets/${wallet.id}`)} className={styles.cardName}>
          {wallet.name}
        </Link>
        <span className={styles.cardType}>
          {walletTypeLabels[wallet.type]} · {wallet.currency}
        </span>
      </div>
      <div className={styles.balanceMain}>
        <span className="visually-hidden">{t('card.balance')}</span>
        <Money amount={wallet.balance.amount} currency={wallet.currency} size="lg" />
        {foreign && (
          <span className={styles.tileNote}>
            ≈ <Money amount={wallet.base_balance.amount} currency={wallet.base_balance.currency} size="sm" />
          </span>
        )}
      </div>
      <dl className={styles.balances}>
        <div>
          <dt>{t('card.cleared')}</dt>
          <dd>
            <Money amount={wallet.cleared_balance.amount} currency={wallet.currency} size="sm" />
          </dd>
        </div>
      </dl>
      <WalletBadges wallet={wallet} />
    </article>
  )
}

/** Kafelki sum: portfele budżetowe, poza budżetem, wszystkie (w walucie bazowej). */
export function WalletSummary({ totals, currency }: { totals: WalletTotals; currency: string }) {
  const { t } = useTranslation('wallets')
  const partial = totals.partial ? (
    <span className={styles.tileNote}>
      <Icon name="clock" size={12} /> {t('summary.partial')}
    </span>
  ) : null
  return (
    <section className={styles.summary} aria-label={t('summary.label')}>
      <div className={styles.tile}>
        <span className={styles.tileLabel}>{t('summary.onBudget')}</span>
        <Money amount={totals.onBudget} currency={currency} size="xl" />
        {partial ?? <span className={styles.tileNote}>{t('summary.onBudgetNote')}</span>}
      </div>
      <div className={styles.tile}>
        <span className={styles.tileLabel}>{t('summary.offBudget')}</span>
        <Money amount={totals.offBudget} currency={currency} size="lg" />
        <span className={styles.tileNote}>{t('summary.offBudgetNote')}</span>
      </div>
      <div className={styles.tile}>
        <span className={styles.tileLabel}>{t('summary.all')}</span>
        <Money amount={totals.all} currency={currency} size="lg" />
        <span className={styles.tileNote}>{t('summary.count', { count: totals.count })}</span>
      </div>
    </section>
  )
}
