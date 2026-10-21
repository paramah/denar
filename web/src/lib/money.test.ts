import { describe, expect, it } from 'vitest'
import {
  absDecimal,
  addDecimal,
  amountForInput,
  convertAmount,
  currencyDecimals,
  decimalsOf,
  formatMoney,
  formatRate,
  fromMinor,
  negateDecimal,
  parseAmountInput,
  parseDecimal,
  parseRateInput,
  sumAmounts,
  toMinor,
  toneOf,
} from './money.ts'

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

describe('parseAmountInput', () => {
  it.each([
    ['12,5', 2, '12.50'],
    ['12.30', 2, '12.30'],
    ['1 234,56', 2, '1234.56'],
    ['1 234,5', 2, '1234.50'],
    ['007', 2, '7.00'],
    ['1500', 0, '1500'],
    ['0,001', 3, '0.001'],
    ['92233720368547758.07', 2, '92233720368547758.07'],
  ])('%s with %i places → %s', (text, decimals, value) => {
    expect(parseAmountInput(text, decimals)).toEqual({ ok: true, value })
  })

  it.each([
    ['', 2, 'Podaj kwotę.'],
    ['abc', 2, 'Wpisz kwotę bez znaku, np. 12,50.'],
    ['-5', 2, 'Wpisz kwotę bez znaku, np. 12,50.'],
    ['12,345', 2, 'Najwyżej 2 miejsca po przecinku.'],
    ['1,5', 0, 'Ta waluta nie ma części ułamkowej.'],
    ['0', 2, 'Kwota musi być większa od zera.'],
    ['1e5', 2, 'Wpisz kwotę bez znaku, np. 12,50.'],
    ['92233720368547758.08', 2, 'Kwota jest za duża.'],
  ])('rejects %j (%i places)', (text, decimals, error) => {
    expect(parseAmountInput(text, decimals)).toEqual({ ok: false, error })
  })

  it('accepts a signed or zero amount when asked', () => {
    expect(parseAmountInput('−1200', 2, { allowNegative: true })).toEqual({ ok: true, value: '-1200.00' })
    expect(parseAmountInput('0', 2, { allowZero: true })).toEqual({ ok: true, value: '0.00' })
  })
})

describe('decimal helpers', () => {
  it.each([
    ['12.50', '-12.50'],
    ['-12.50', '12.50'],
    ['0.00', '0.00'],
  ])('negateDecimal(%s) = %s', (value, negated) => {
    expect(negateDecimal(value)).toBe(negated)
  })

  it('absDecimal and amountForInput drop the sign', () => {
    expect(absDecimal('-7.05')).toBe('7.05')
    expect(amountForInput('-1234.50')).toBe('1234,50')
  })

  it('sums large amounts exactly (beyond 2^53)', () => {
    expect(sumAmounts(['90071992547409.93', '0.01', '-0.02'])).toBe('90071992547409.92')
    expect(sumAmounts([], 2)).toBe('0.00')
    expect(sumAmounts(['1.5', '2.25'])).toBe('3.75')
  })

  it('decimalsOf prefers the currency list', () => {
    expect(decimalsOf('XYZ', [{ code: 'XYZ', decimals: 3 }])).toBe(3)
    expect(decimalsOf('JPY')).toBe(0)
  })
})

describe('rates', () => {
  it.each([
    ['4,2512', '4.2512'],
    ['04.10', '4.10'],
    ['0.000123456789', '0.000123456789'],
    ['3', '3'],
  ])('parseRateInput(%s) = %s', (text, value) => {
    expect(parseRateInput(text)).toEqual({ ok: true, value })
  })

  it.each(['', '0', '0,000', '-4', '4,1234567890123', 'abc'])('parseRateInput rejects %j', (text) => {
    expect(parseRateInput(text).ok).toBe(false)
  })

  it('formatRate trims trailing zeros and uses a comma', () => {
    expect(formatRate('4.25120000')).toBe('4,2512')
    expect(formatRate('1')).toBe('1')
    expect(formatRate('2.00000000')).toBe('2')
  })
})

describe('convertAmount', () => {
  it.each([
    ['20.00', 2, '4.0', 2, '80.00'],
    ['19.99', 2, '4.0123', 2, '80.21'],
    ['0.125', 3, '1', 2, '0.12'],
    ['0.135', 3, '1', 2, '0.14'],
    ['1000', 0, '0.0271', 2, '27.10'],
    ['-12.50', 2, '4.3', 2, '-53.75'],
    ['92233720368547758.07', 2, '4.5', 2, '415051741658464911.32'],
  ])('%s × %s → %s', (amount, from, rate, to, expected) => {
    expect(convertAmount(amount, from, rate, to)).toBe(expected)
  })

  it('rejects bad input', () => {
    expect(convertAmount('abc', 2, '4', 2)).toBeNull()
    expect(convertAmount('1.00', 2, '-4', 2)).toBeNull()
  })
})
