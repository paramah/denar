import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '../i18n/locale.ts'
import { formatMonth } from './budgetMonth.ts'
import { formatDate, formatDateTime, formatDay, formatRelative } from './format.ts'
import { formatFileSize } from '../features/import/model.ts'
import { amountForInput, formatMoney, formatRate, parseAmountInput } from './money.ts'

// FR-1305: kwoty, daty i liczby według aktywnego języka; po polsku bez zmian.

describe('formatting follows the active language', () => {
  afterEach(() => setLocale('pl'))

  it('formats money in pl-PL and en-GB', () => {
    expect(formatMoney('-1234.5', 'PLN')!.replace(/\s/g, ' ')).toBe('−1234,50 zł')
    setLocale('en')
    expect(formatMoney('-1234.5', 'PLN')!.replace(/\s/g, ' ')).toBe('−PLN 1,234.50')
    expect(formatMoney('12', 'EUR', { signed: true })).toBe('+€12.00')
    setLocale('pl')
    expect(formatMoney('-1234.5', 'PLN')!.replace(/\s/g, ' ')).toBe('−1234,50 zł')
  })

  it('formats dates, months and relative time', () => {
    const date = new Date(2026, 9, 21, 14, 5)
    expect(formatDate(date)).toBe('21 paź 2026')
    expect(formatMonth('2026-10')).toBe('październik 2026')
    setLocale('en')
    expect(formatDate(date)).toBe('21 Oct 2026')
    expect(formatDateTime(date)).toBe('21 Oct 2026, 14:05')
    expect(formatDay('2026-10-21')).toBe('21 Oct 2026')
    expect(formatMonth('2026-10')).toBe('October 2026')
    expect(formatRelative(new Date(2026, 9, 21, 14, 0), date)).toBe('5 minutes ago')
    expect(formatRelative(date, date)).toBe('now')
  })

  it('uses the decimal separator of the language for input and rates', () => {
    expect(amountForInput('-12.50')).toBe('12,50')
    expect(formatRate('4.25120000')).toBe('4,2512')
    expect(formatFileSize(1536)).toBe('1,5 KiB')
    setLocale('en')
    expect(amountForInput('-12.50')).toBe('12.50')
    expect(formatRate('4.25120000')).toBe('4.2512')
    expect(formatFileSize(1536)).toBe('1.5 KiB')
    // Wpis przyjmuje oba separatory w każdym języku.
    expect(parseAmountInput('12,5', 2)).toEqual({ ok: true, value: '12.50' })
    expect(parseAmountInput('', 2)).toEqual({ ok: false, error: 'Enter an amount.' })
  })
})
