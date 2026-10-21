import { Trans, useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { BudgetMonth } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { formatMonth, shiftMonth } from '../../lib/budgetMonth.ts'
import { absDecimal, formatMoney, toneOf } from '../../lib/money.ts'
import styles from './budget.module.css'

type RtaState = 'positive' | 'zero' | 'negative'

export function rtaState(month: BudgetMonth): RtaState {
  const tone = toneOf(month.ready_to_assign.amount)
  return tone === 'outflow' ? 'negative' : tone === 'zero' ? 'zero' : 'positive'
}

/** Duża kwota „Do przydzielenia” z opisem stanu. */
export function ReadyToAssign({ month }: { month: BudgetMonth }) {
  const { t } = useTranslation('budget')
  const state = rtaState(month)
  const amount = month.ready_to_assign
  const text =
    state === 'negative'
      ? t('rta.negative', { amount: formatMoney(absDecimal(amount.amount), amount.currency) ?? '' })
      : state === 'zero'
        ? t('rta.zero')
        : t('rta.positive')
  return (
    <div className={styles.rta} data-state={state} role="group" aria-label={t('rta.label')}>
      <span className={styles.rtaLabel}>{t('rta.label')}</span>
      <Money amount={amount.amount} currency={amount.currency} size="xl" className={styles.rtaAmount} />
      <p className={styles.rtaText}>
        {state === 'negative' && <Icon name="alert" size={16} />} {text}
      </p>
    </div>
  )
}

/** Nagłówek budżetu: kwota do przydzielenia, sumy miesiąca i uwagi. */
export function BudgetSummary({ month }: { month: BudgetMonth }) {
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const overspent = toneOf(month.overspent_last_month.amount) === 'outflow'
  const future = toneOf(month.assigned_in_future.amount) === 'inflow'
  return (
    <>
      <section className={styles.summary} aria-label={t('summary.label')}>
        <ReadyToAssign month={month} />
        <dl className={styles.totals}>
          <div>
            <dt>{t('summary.assigned')}</dt>
            <dd>
              <Money amount={month.assigned.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>{t('summary.activity')}</dt>
            <dd>
              <Money amount={month.activity.amount} currency={month.currency} tone="auto" signed={false} />
            </dd>
          </div>
          <div>
            <dt>{t('summary.available')}</dt>
            <dd>
              <Money amount={month.available.amount} currency={month.currency} />
            </dd>
          </div>
        </dl>
      </section>
      {(overspent || future || month.incomplete) && (
        <ul className={styles.notices} aria-label={t('summary.noticesLabel')}>
          {overspent && (
            <li className={styles.notice} data-tone="warn">
              <Icon name="alert" size={16} />
              <span>
                <Trans
                  t={t}
                  i18nKey="summary.overspentLastMonth"
                  values={{ month: formatMonth(shiftMonth(month.month, -1)) }}
                  components={{ money: <Money amount={month.overspent_last_month.amount} currency={month.currency} /> }}
                />
              </span>
            </li>
          )}
          {future && (
            <li className={styles.notice}>
              <Icon name="clock" size={16} />
              <span>
                <Trans
                  t={t}
                  i18nKey="summary.assignedInFuture"
                  components={{ money: <Money amount={month.assigned_in_future.amount} currency={month.currency} /> }}
                />
              </span>
            </li>
          )}
          {month.incomplete && (
            <li className={styles.notice} data-tone="warn">
              <Icon name="clock" size={16} />
              <span>
                {t('summary.pending', { count: month.pending_transactions })} {t('summary.pendingNote')}{' '}
                <Link to={`${householdPath(household.id, 'transactions')}?to=${month.end}`}>{t('summary.viewTransactions')}</Link>
              </span>
            </li>
          )}
        </ul>
      )}
    </>
  )
}

/** Przepływy miesiąca poza kopertami: wpływy, wydatki bez koperty, przelewy. */
export function BudgetFlows({ month }: { month: BudgetMonth }) {
  const { t } = useTranslation('budget')
  return (
    <section className={styles.flows} aria-labelledby="budget-flows">
      <h2 id="budget-flows">{t('flows.title')}</h2>
      <dl className={styles.flowList}>
        <div>
          <dt>{t('flows.income')}</dt>
          <dd>
            <Money amount={month.income.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>{t('flows.incomeNote')}</dd>
        </div>
        <div>
          <dt>{t('flows.uncategorized')}</dt>
          <dd>
            <Money amount={month.uncategorized.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>{t('flows.uncategorizedNote')}</dd>
        </div>
        <div>
          <dt>{t('flows.transfers')}</dt>
          <dd>
            <Money amount={month.transfers.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>{t('flows.transfersNote')}</dd>
        </div>
      </dl>
    </section>
  )
}
