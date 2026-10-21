import { describe, expect, it } from 'vitest'
import { addDecimal, currencyDecimals, formatMoney, fromMinor, parseDecimal, toMinor, toneOf } from './money.ts'

// Intl wstawia twardą spację (U+00A0) albo wąską (U+202F); w testach obie są spacją.
const plain = (value: string | null) => value?.replace(/[\u00a0\u202f]/g, ' ')

describe('formatMoney', () => {
  it.each([
    ['1234.50', 'PLN', {}, '1234,50 zł'],
    ['12345.5', 'PLN', {}, '12 345,50 zł'],
    ['-12.34', 'PLN', {}, '−12,34 zł'],
    ['0', 'PLN', {}, '0,00 zł'],
    ['-0.00', 'PLN', {}, '0,00 zł'],
    ['10', 'EUR', {}, '10,00 €'],
    ['1500', 'JPY', {}, '1500 JPY'],
    ['1.234', 'KWD', {}, '1,234 KWD'],
    ['12.34', 'PLN', { signed: true }, '+12,34 zł'],
    ['-12.34', 'PLN', { signed: true }, '−12,34 zł'],
    ['0', 'PLN', { signed: true }, '0,00 zł'],
  ])('%s %s %o → %s', (amount, currency, options, expected) => {
    expect(plain(formatMoney(amount, currency, options))).toBe(expected)
  })

  it('keeps every digit of amounts beyond float precision', () => {
    expect(plain(formatMoney('90071992547409931.17', 'PLN'))).toBe('90 071 992 547 409 931,17 zł')
  })

  it('returns null for what is not a decimal', () => {
    expect(formatMoney('12,5', 'PLN')).toBeNull()
    expect(formatMoney('1e3', 'PLN')).toBeNull()
    expect(formatMoney('', 'PLN')).toBeNull()
  })

  it('falls back to the code for an unknown currency', () => {
    expect(formatMoney('5', 'XX')).toBe('5 XX')
  })
})

describe('parseDecimal and toneOf', () => {
  it('parses sign, whole and fraction', () => {
    expect(parseDecimal('-0012.30')).toEqual({ negative: true, whole: '0012', fraction: '30' })
    expect(parseDecimal('+7')).toEqual({ negative: false, whole: '7', fraction: '' })
  })

  it.each([
    ['12.00', 'inflow'],
    ['-0.01', 'outflow'],
    ['0.00', 'zero'],
    ['-0', 'zero'],
    ['abc', 'zero'],
  ])('%s → %s', (amount, tone) => {
    expect(toneOf(amount)).toBe(tone)
  })
})

describe('minor units', () => {
  it('converts both ways without loss', () => {
    expect(toMinor('-12.3', 2)).toBe(-1230n)
    expect(toMinor('12.345', 2)).toBeNull()
    expect(toMinor('12.340', 2)).toBe(1234n)
    expect(fromMinor(-5n, 2)).toBe('-0.05')
    expect(fromMinor(1500n, 0)).toBe('1500')
  })

  it('adds decimals exactly', () => {
    expect(addDecimal(['0.1', '0.2'], 2)).toBe('0.30')
    expect(addDecimal(['90071992547409931.17', '0.01'], 2)).toBe('90071992547409931.18')
    expect(addDecimal(['1', 'x'], 2)).toBeNull()
  })

  it('knows the decimals of a currency', () => {
    expect(currencyDecimals('PLN')).toBe(2)
    expect(currencyDecimals('JPY')).toBe(0)
  })
})
