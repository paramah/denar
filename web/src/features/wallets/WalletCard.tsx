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
  return (
    <span className={styles.badges}>
      {!wallet.on_budget && <Badge>Poza budżetem</Badge>}
      {wallet.visibility === 'private' && (
        <Badge tone="brand" title="Widzą go tylko wybrane osoby">
          <Icon name="lock" size={12} />
          Prywatny
        </Badge>
      )}
      {wallet.archived_at && <Badge tone="warn">Zarchiwizowany</Badge>}
      {wallet.rate_pending && (
        <Badge tone="warn" title="Część transakcji czeka na kurs waluty; saldo bazowe jest częściowe">
          <Icon name="clock" size={12} />
          Czeka na kurs
        </Badge>
      )}
    </span>
  )
}

export function WalletCard({ wallet }: { wallet: Wallet }) {
  const { household } = useHousehold()
  const foreign = wallet.currency !== household.base_currency
  return (
    <article
      className={styles.card}
      data-on-budget={wallet.on_budget}
      data-archived={!!wallet.archived_at}
      aria-label={`Portfel ${wallet.name}`}
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
        <span className="visually-hidden">Saldo:</span>
        <Money amount={wallet.balance.amount} currency={wallet.currency} size="lg" />
        {foreign && (
          <span className={styles.tileNote}>
            ≈ <Money amount={wallet.base_balance.amount} currency={wallet.base_balance.currency} size="sm" />
          </span>
        )}
      </div>
      <dl className={styles.balances}>
        <div>
          <dt>Rozliczone</dt>
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
  const partial = totals.partial ? (
    <span className={styles.tileNote}>
      <Icon name="clock" size={12} /> Część kwot czeka na kurs — suma jest częściowa.
    </span>
  ) : null
  return (
    <section className={styles.summary} aria-label="Sumy portfeli">
      <div className={styles.tile}>
        <span className={styles.tileLabel}>Na portfelach budżetowych</span>
        <Money amount={totals.onBudget} currency={currency} size="xl" />
        {partial ?? <span className={styles.tileNote}>Pieniądze, które rozdzielisz na koperty.</span>}
      </div>
      <div className={styles.tile}>
        <span className={styles.tileLabel}>Poza budżetem</span>
        <Money amount={totals.offBudget} currency={currency} size="lg" />
        <span className={styles.tileNote}>Oszczędności, inwestycje, kredyty</span>
      </div>
      <div className={styles.tile}>
        <span className={styles.tileLabel}>Razem</span>
        <Money amount={totals.all} currency={currency} size="lg" />
        <span className={styles.tileNote}>
          {totals.count === 1 ? '1 aktywny portfel' : `Aktywne portfele: ${totals.count}`}
        </span>
      </div>
    </section>
  )
}
