import { describe, expect, it } from 'vitest'
import { cycleLabel, cycleNote, daysBetween, monogram, nextDues, plural, relativeDay, share, vendorMetal } from './subscriptions.ts'

describe('cycleLabel', () => {
  it.each([
    [{ unit: 'month', every: 1, anchor: '2026-01-31' }, 'co miesiąc, 31.'],
    [{ unit: 'month', every: 3, anchor: '2026-01-15' }, 'co 3 miesiące, 15.'],
    [{ unit: 'month', every: 5, anchor: '2026-01-15' }, 'co 5 miesięcy, 15.'],
    [{ unit: 'week', every: 2, anchor: '2026-10-21' }, 'co 2 tygodnie, w środę'],
    [{ unit: 'week', every: 1, anchor: '2026-10-20' }, 'co tydzień, we wtorek'],
    [{ unit: 'year', every: 1, anchor: '2026-02-14' }, 'co rok, 14 lutego'],
    [{ unit: 'year', every: 2, anchor: '2026-02-14' }, 'co 2 lata, 14 lutego'],
    [{ unit: 'day', every: 1, anchor: '2026-10-01' }, 'codziennie'],
    [{ unit: 'day', every: 12, anchor: '2026-10-01' }, 'co 12 dni'],
    [{ unit: 'week', every: 22, anchor: '2026-10-25' }, 'co 22 tygodnie, w niedzielę'],
  ] as const)('%o → %s', (cycle, label) => {
    expect(cycleLabel(cycle)).toBe(label)
  })

  it('notes month-end clamping', () => {
    expect(cycleNote({ unit: 'month', anchor: '2026-01-31' })).toMatch(/ostatniego dnia/)
    expect(cycleNote({ unit: 'month', anchor: '2026-01-28' })).toBeNull()
    expect(cycleNote({ unit: 'year', anchor: '2024-02-29' })).toMatch(/28 lutego/)
  })
})

describe('nextDues', () => {
  it('clamps anchor 31 to short months without drifting', () => {
    expect(nextDues({ unit: 'month', every: 1, anchor: '2026-01-31' }, '2026-01-01', 4)).toEqual([
      '2026-01-31',
      '2026-02-28',
      '2026-03-31',
      '2026-04-30',
    ])
    expect(nextDues({ unit: 'month', every: 1, anchor: '2023-08-31' }, '2026-10-21', 3)).toEqual([
      '2026-10-31',
      '2026-11-30',
      '2026-12-31',
    ])
  })

  it('handles leap days, weeks and every N', () => {
    expect(nextDues({ unit: 'year', every: 1, anchor: '2024-02-29' }, '2024-03-01', 4)).toEqual([
      '2025-02-28',
      '2026-02-28',
      '2027-02-28',
      '2028-02-29',
    ])
    expect(nextDues({ unit: 'week', every: 2, anchor: '2026-10-07' }, '2026-10-21', 2)).toEqual(['2026-10-21', '2026-11-04'])
    expect(nextDues({ unit: 'month', every: 2, anchor: '2026-08-31' }, '2026-09-01', 3)).toEqual([
      '2026-10-31',
      '2026-12-31',
      '2027-02-28',
    ])
    expect(nextDues({ unit: 'day', every: 10, anchor: '2026-12-01' }, '2026-10-01', 1)).toEqual(['2026-12-01'])
  })
})

describe('helpers', () => {
  it('declines Polish numerals', () => {
    expect([1, 2, 5, 12, 22, 25].map((n) => plural(n, 'dzień', 'dni', 'dni'))).toEqual(['dzień', 'dni', 'dni', 'dni', 'dni', 'dni'])
    expect([1, 3, 13, 23].map((n) => plural(n, 'tydzień', 'tygodnie', 'tygodni'))).toEqual([
      'tydzień',
      'tygodnie',
      'tygodni',
      'tygodnie',
    ])
  })

  it('counts calendar days and names them', () => {
    expect(daysBetween('2026-10-21', '2026-11-01')).toBe(11)
    expect(relativeDay('2026-10-21', '2026-10-21')).toBe('dziś')
    expect(relativeDay('2026-10-22', '2026-10-21')).toBe('jutro')
    expect(relativeDay('2026-10-26', '2026-10-21')).toBe('za 5 dni')
    expect(relativeDay('2026-10-18', '2026-10-21')).toBe('3 dni temu')
  })

  it('builds monograms and stable metals', () => {
    expect(monogram('OpenAI')).toBe('OA')
    expect(monogram('HBO Max')).toBe('HM')
    expect(monogram('iCloud')).toBe('iC')
    expect(monogram('Netflix')).toBe('Ne')
    expect(monogram('Disney+')).toBe('Di')
    expect(vendorMetal('openai')).toBe(vendorMetal('openai'))
    expect(vendorMetal('openai')).toBeGreaterThanOrEqual(1)
    expect(vendorMetal('openai')).toBeLessThanOrEqual(6)
  })

  it('computes bar shares on bigint', () => {
    expect(share('50.00', '200.00', 2)).toBe(25)
    expect(share('200.00', '200.00', 2)).toBe(100)
    expect(share('0.01', '90000000000000000.00', 2)).toBe(2)
    expect(share('0.00', '200.00', 2)).toBe(0)
  })
})
