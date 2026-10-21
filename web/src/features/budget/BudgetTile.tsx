import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useBudgetMonth } from '../../api/hooks/budget.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Money } from '../../components/Money/Money.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { currentBudgetMonth, formatMonth } from '../../lib/budgetMonth.ts'
import { envelopesEnabled } from '../transactions/envelopes.ts'
import { ReadyToAssign } from './BudgetSummary.tsx'
import { overspent } from './model.ts'
import styles from './budget.module.css'

const overspentShown = 3

/** Pulpit: „Do przydzielenia” bieżącego miesiąca i koperty z przekroczeniem. */
export function BudgetTile() {
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const month = currentBudgetMonth(household.timezone, household.month_start_day)
  const enabled = envelopesEnabled()
  const budget = useBudgetMonth(household.id, month, { enabled })
  if (!enabled || budget.isError) {
    return null
  }
  if (!budget.data) {
    return <Skeleton lines={2} />
  }
  const over = overspent(budget.data)
  const link = householdPath(household.id, 'budget')
  return (
    <section className={styles.dashTile} aria-label={t('tile.label', { month: formatMonth(month) })}>
      <ReadyToAssign month={budget.data} />
      <div className={styles.dashOver}>
        <h2>{over.length > 0 ? t('tile.overspent') : t('tile.envelopes')}</h2>
        {over.length > 0 ? (
          <ul className={styles.overList} aria-label={t('tile.overspent')}>
            {over.slice(0, overspentShown).map((envelope) => (
              <li key={envelope.id}>
                <span>{envelope.name}</span>
                <Money amount={envelope.available.amount} currency={budget.data.currency} tone="auto" />
              </li>
            ))}
          </ul>
        ) : (
          <p className={styles.muted}>
            {budget.data.groups.length === 0
              ? t('tile.noEnvelopes')
              : t('tile.noneOverspent')}
          </p>
        )}
        <Link to={link}>
          {over.length > overspentShown ? t('tile.allOverspent', { n: over.length }) : t('tile.toBudget')}
        </Link>
      </div>
    </section>
  )
}
