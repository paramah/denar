import type { Goal, GoalInput, GoalType } from '../../api/types.ts'
import { tk } from '../../i18n/translate.ts'
import { formatDay } from '../../lib/format.ts'
import { amountForInput, formatMoney, parseAmountInput } from '../../lib/money.ts'

export type GoalChoice = 'none' | GoalType

export type GoalDraft = { type: GoalChoice; amount: string; targetDate: string }

export function goalDraft(goal: Goal | null | undefined): GoalDraft {
  if (!goal) {
    return { type: 'none', amount: '', targetDate: '' }
  }
  return { type: goal.type, amount: amountForInput(goal.amount.amount), targetDate: goal.target_date ?? '' }
}

export type GoalCheck =
  | { ok: true; goal: GoalInput | null }
  | { ok: false; errors: { amount?: string; target_date?: string } }

/**
 * Cel z formularza → GoalInput (003 FR-206): kwota > 0 w walucie bazowej;
 * target_by_date wymaga daty, monthly jej nie wysyła.
 */
export function checkGoal(draft: GoalDraft, currency: string, decimals: number, today?: string): GoalCheck {
  if (draft.type === 'none') {
    return { ok: true, goal: null }
  }
  const errors: { amount?: string; target_date?: string } = {}
  const amount = parseAmountInput(draft.amount, decimals)
  if (!amount.ok) {
    errors.amount = amount.error
  }
  if (draft.type === 'target_by_date') {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(draft.targetDate)) {
      errors.target_date = tk('budget:goal.dateMissing')
    } else if (today && draft.targetDate < today) {
      errors.target_date = tk('budget:goal.datePast')
    }
  }
  if (errors.amount || errors.target_date || !amount.ok) {
    return { ok: false, errors }
  }
  return {
    ok: true,
    goal: {
      type: draft.type,
      amount: { amount: amount.value, currency },
      ...(draft.type === 'target_by_date' ? { target_date: draft.targetDate } : {}),
    },
  }
}

/** „800,00 zł co miesiąc” / „1200,00 zł do 28 lut 2027”. */
export function describeGoal(goal: Goal): string {
  const amount = formatMoney(goal.amount.amount, goal.amount.currency) ?? goal.amount.amount
  return goal.type === 'monthly'
    ? tk('budget:goal.monthly', { amount })
    : tk('budget:goal.byDate', { amount, date: goal.target_date ? formatDay(goal.target_date) : '—' })
}

/** Nazwa rodzaju celu w aktywnym języku. */
export function goalTypeLabel(type: GoalChoice): string {
  return tk(`budget:goal.type.${type}`)
}
