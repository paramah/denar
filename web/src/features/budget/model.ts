import type { BudgetEnvelope, BudgetMonth } from '../../api/types.ts'
import { decimalSeparator } from '../../lib/intl.ts'
import { addDecimal, negateDecimal, toMinor, toneOf } from '../../lib/money.ts'

export type AvailableState = 'positive' | 'zero' | 'negative' | 'underfunded'

/** Stan kwoty dostępnej: przekroczona, zero, dodatnia, dodatnia z niedofinansowanym celem. */
export function availableState(envelope: BudgetEnvelope): AvailableState {
  const tone = toneOf(envelope.available.amount)
  if (tone === 'outflow') {
    return 'negative'
  }
  if (envelope.goal_status && !envelope.goal_status.funded) {
    return 'underfunded'
  }
  return tone === 'zero' ? 'zero' : 'positive'
}

export type EnvelopeInMonth = BudgetEnvelope & { groupName: string }

/** Koperty miesiąca w kolejności grup. */
export function envelopesOf(month: BudgetMonth): EnvelopeInMonth[] {
  return month.groups.flatMap((group) => group.envelopes.map((envelope) => ({ ...envelope, groupName: group.name })))
}

/** Kwota przydziału do pola edycji: ze znakiem, z separatorem języka („-12.50” → „-12,50” po polsku). */
export function assignedForInput(amount: string): string {
  return amount.replace('.', decimalSeparator())
}

/** Przydział po dodaniu/odjęciu kwoty — na bigint jednostek minimalnych. */
export function shiftAssigned(assigned: string, delta: string, decimals: number, subtract = false): string | null {
  return addDecimal([assigned, subtract ? negateDecimal(delta) : delta], decimals)
}

/** Koperty z przekroczeniem (available < 0), od największego. */
export function overspent(month: BudgetMonth): EnvelopeInMonth[] {
  return envelopesOf(month)
    .map((envelope) => ({ envelope, minor: toMinor(envelope.available.amount, 4) ?? 0n }))
    .filter(({ minor }) => minor < 0n)
    .sort((a, b) => (a.minor < b.minor ? -1 : a.minor > b.minor ? 1 : 0))
    .map(({ envelope }) => envelope)
}
