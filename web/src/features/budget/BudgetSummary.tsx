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
export function ReadyToAssign({ month, compact }: { month: BudgetMonth; compact?: boolean }) {
  const state = rtaState(month)
  const amount = month.ready_to_assign
  const text =
    state === 'negative'
      ? `Przydzielono więcej, niż masz. Zmniejsz przydziały o ${formatMoney(absDecimal(amount.amount), amount.currency) ?? ''}.`
      : state === 'zero'
        ? 'Każda złotówka ma zadanie.'
        : 'Rozdziel te pieniądze na koperty.'
  return (
    <div className={styles.rta} data-state={state} role="group" aria-label="Do przydzielenia">
      <span className={styles.rtaLabel}>Do przydzielenia</span>
      <Money amount={amount.amount} currency={amount.currency} size="xl" className={compact ? undefined : styles.rtaAmount} />
      <p className={styles.rtaText}>
        {state === 'negative' && <Icon name="alert" size={16} />} {text}
      </p>
    </div>
  )
}

/** Nagłówek budżetu: kwota do przydzielenia, sumy miesiąca i uwagi. */
export function BudgetSummary({ month }: { month: BudgetMonth }) {
  const { household } = useHousehold()
  const overspent = toneOf(month.overspent_last_month.amount) === 'outflow'
  const future = toneOf(month.assigned_in_future.amount) === 'inflow'
  return (
    <>
      <section className={styles.summary} aria-label="Podsumowanie miesiąca">
        <ReadyToAssign month={month} />
        <dl className={styles.totals}>
          <div>
            <dt>Przydzielono</dt>
            <dd>
              <Money amount={month.assigned.amount} currency={month.currency} />
            </dd>
          </div>
          <div>
            <dt>Aktywność</dt>
            <dd>
              <Money amount={month.activity.amount} currency={month.currency} tone="auto" signed={false} />
            </dd>
          </div>
          <div>
            <dt>Dostępne w kopertach</dt>
            <dd>
              <Money amount={month.available.amount} currency={month.currency} />
            </dd>
          </div>
        </dl>
      </section>
      {(overspent || future || month.incomplete) && (
        <ul className={styles.notices} aria-label="Uwagi do miesiąca">
          {overspent && (
            <li className={styles.notice} data-tone="warn">
              <Icon name="alert" size={16} />
              <span>
                Przekroczenia z miesiąca {formatMonth(shiftMonth(month.month, -1))}:{' '}
                <Money amount={month.overspent_last_month.amount} currency={month.currency} /> — pomniejszyły kwotę do
                przydzielenia.
              </span>
            </li>
          )}
          {future && (
            <li className={styles.notice}>
              <Icon name="clock" size={16} />
              <span>
                Przydzielono na kolejne miesiące: <Money amount={month.assigned_in_future.amount} currency={month.currency} />{' '}
                — tych pieniędzy nie ma już w kwocie do przydzielenia.
              </span>
            </li>
          )}
          {month.incomplete && (
            <li className={styles.notice} data-tone="warn">
              <Icon name="clock" size={16} />
              <span>
                {month.pending_transactions === 1
                  ? '1 transakcja czeka na kurs waluty'
                  : `Transakcje czekające na kurs waluty: ${month.pending_transactions}`}{' '}
                — nie są jeszcze liczone, kwoty uzupełnią się same.{' '}
                <Link to={`${householdPath(household.id, 'transactions')}?to=${month.end}`}>Zobacz transakcje</Link>
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
  return (
    <section className={styles.flows} aria-labelledby="budget-flows">
      <h2 id="budget-flows">Poza kopertami</h2>
      <dl className={styles.flowList}>
        <div>
          <dt>Wpływy</dt>
          <dd>
            <Money amount={month.income.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>Przychody i salda otwarcia portfeli budżetowych — zasilają kwotę do przydzielenia.</dd>
        </div>
        <div>
          <dt>Bez koperty</dt>
          <dd>
            <Money amount={month.uncategorized.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>Wydatki i zwroty bez koperty — obciążają kwotę do przydzielenia.</dd>
        </div>
        <div>
          <dt>Przelewy</dt>
          <dd>
            <Money amount={month.transfers.amount} currency={month.currency} tone="auto" />
          </dd>
          <dd className={styles.flowNote}>Przelewy poza budżet bez koperty i różnice kursowe między portfelami.</dd>
        </div>
      </dl>
    </section>
  )
}
