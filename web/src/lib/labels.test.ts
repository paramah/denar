import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '../i18n/locale.ts'
import * as labels from './labels.ts'

afterEach(() => setLocale('pl'))

const maps = Object.entries(labels).filter(
  (entry): entry is [string, Record<string, string>] => typeof entry[1] === 'object' && !Array.isArray(entry[1]),
)

describe('labels', () => {
  it('translates every key of every map in both languages', () => {
    expect(maps.length).toBeGreaterThan(10)
    for (const locale of ['pl', 'en'] as const) {
      setLocale(locale)
      for (const [name, map] of maps) {
        for (const [key, value] of Object.entries(map)) {
          expect({ name, key, missing: value.startsWith('labels:') }).toEqual({ name, key, missing: false })
        }
      }
    }
  })

  it('follows the active language without changing the call', () => {
    expect(labels.roleLabels.owner).toBe('Właściciel')
    expect(labels.actionLabel('household.created')).toBe('Założenie gospodarstwa')
    expect(labels.fieldLabel('splits[2].amount')).toBe('podział 3: kwota')
    setLocale('en')
    expect(labels.roleLabels.owner).toBe('Owner')
    expect(labels.actionLabel('household.created')).toBe('Household created')
    expect(labels.fieldLabel('goal.amount')).toBe('goal amount')
    expect(labels.fieldLabel('splits[0].note')).toBe('split 1: note')
  })

  it('keeps unknown codes as they are', () => {
    expect(labels.actionLabel('future.thing')).toBe('future.thing')
    expect(labels.resourceTypeLabel('toString')).toBe('toString')
    expect(labels.fieldLabel('mystery')).toBe('mystery')
    expect(labels.walletTypeLabels['nope' as 'cash']).toBeUndefined()
  })
})
