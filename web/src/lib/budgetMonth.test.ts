import { describe, expect, it } from 'vitest'
import { currentBudgetMonth, formatMonth, isMonth, monthOfDay, shiftMonth } from './budgetMonth.ts'

describe('budget month', () => {
  it.each([
    ['2026-10', true],
    ['2000-01', true],
    ['2100-12', true],
    ['1999-12', false],
    ['2101-01', false],
    ['2026-13', false],
    ['2026-00', false],
    ['2026-1', false],
    ['', false],
  ])('isMonth(%s) = %s', (value, expected) => {
    expect(isMonth(value)).toBe(expected)
  })

  it.each([
    ['2026-10', 1, '2026-11'],
    ['2026-12', 1, '2027-01'],
    ['2026-01', -1, '2025-12'],
    ['2026-10', -14, '2025-08'],
  ])('shiftMonth(%s, %i) = %s', (month, delta, expected) => {
    expect(shiftMonth(month, delta)).toBe(expected)
  })

  it.each([
    ['2026-10-05', 1, '2026-10'],
    ['2026-10-05', 10, '2026-09'],
    ['2026-10-10', 10, '2026-10'],
    ['2026-01-09', 10, '2025-12'],
    ['2026-02-28', 28, '2026-02'],
  ])('monthOfDay(%s, start %i) = %s', (day, start, expected) => {
    expect(monthOfDay(day, start)).toBe(expected)
  })

  it('takes today in the household time zone', () => {
    // 2026-10-31 23:30 UTC to już 1 listopada w Warszawie.
    const now = new Date('2026-10-31T23:30:00Z')
    expect(currentBudgetMonth('Europe/Warsaw', 1, now)).toBe('2026-11')
    expect(currentBudgetMonth('UTC', 1, now)).toBe('2026-10')
    expect(currentBudgetMonth('Europe/Warsaw', 2, now)).toBe('2026-10')
  })

  it('names months in Polish', () => {
    expect(formatMonth('2026-10')).toBe('październik 2026')
    expect(formatMonth('2027-01')).toBe('styczeń 2027')
  })
})
