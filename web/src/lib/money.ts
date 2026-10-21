// Kwoty z API przychodzą jako dziesiętny string ("-12.34") z kodem waluty.
// Nic tu nie zamienia ich na liczbę zmiennoprzecinkową: formatowanie idzie
// przez Intl.NumberFormat na stringu (dokładny dziesiętny), a arytmetyka —
// gdyby UI jej potrzebowało — na bigint jednostek minimalnych.

import { intlLocale } from '../i18n/locale.ts'
import { tk } from '../i18n/translate.ts'
import { decimalSeparator } from './intl.ts'

const decimalPattern = /^([+-]?)(\d+)(?:\.(\d+))?$/

export type ParsedDecimal = { negative: boolean; whole: string; fraction: string }

/** Rozkłada dziesiętny string; null, gdy to nie jest liczba w tym formacie. */
export function parseDecimal(value: string): ParsedDecimal | null {
  const match = decimalPattern.exec(value.trim())
  if (!match) {
    return null
  }
  const [, sign = '', whole = '0', fraction = ''] = match
  const isZero = /^0+$/.test(whole) && /^0*$/.test(fraction)
  return { negative: sign === '-' && !isZero, whole, fraction }
}

export type Tone = 'inflow' | 'outflow' | 'zero'

export function toneOf(value: string): Tone {
  const parsed = parseDecimal(value)
  if (!parsed || (/^0+$/.test(parsed.whole) && /^0*$/.test(parsed.fraction))) {
    return 'zero'
  }
  return parsed.negative ? 'outflow' : 'inflow'
}

const formatters = new Map<string, Intl.NumberFormat>()

function formatter(currency: string, signed: boolean): Intl.NumberFormat {
  const locale = intlLocale()
  const key = `${locale}|${currency}|${signed}`
  let nf = formatters.get(key)
  if (!nf) {
    nf = new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
      currencyDisplay: 'symbol',
      signDisplay: signed ? 'exceptZero' : 'auto',
    })
    formatters.set(key, nf)
  }
  return nf
}

/** Liczba miejsc dziesiętnych waluty według Intl (np. PLN 2, JPY 0). */
export function currencyDecimals(currency: string): number {
  try {
    return formatter(currency, false).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

/**
 * Formatuje kwotę w języku interfejsu, np. "-1234.5" PLN → "−1234,50 zł"
 * (po angielsku, en-GB: "−PLN 1,234.50").
 * Minus to U+2212 (czytelniejszy w cyfrach tabelarycznych). Niepoprawny
 * string zwraca null — wywołujący pokazuje wtedy surową wartość.
 */
export function formatMoney(amount: string, currency: string, options: { signed?: boolean } = {}): string | null {
  const parsed = parseDecimal(amount)
  if (!parsed) {
    return null
  }
  const exact = `${parsed.negative ? '-' : ''}${parsed.whole}${parsed.fraction ? `.${parsed.fraction}` : ''}`
  try {
    // NumberFormat v3: string jest formatowany jako dokładny dziesiętny.
    return formatter(currency, options.signed ?? false)
      .format(exact as Intl.StringNumericLiteral)
      .replace('-', '−')
  } catch {
    return `${exact.replace('-', '−')} ${currency}`
  }
}

/** Kwota w jednostkach minimalnych; null dla niepoprawnego stringa lub zbyt wielu miejsc. */
export function toMinor(amount: string, decimals: number): bigint | null {
  const parsed = parseDecimal(amount)
  if (!parsed || parsed.fraction.replace(/0+$/, '').length > decimals) {
    return null
  }
  const digits = parsed.whole + parsed.fraction.padEnd(decimals, '0').slice(0, decimals)
  const minor = BigInt(digits)
  return parsed.negative ? -minor : minor
}

/** Odwrotność toMinor: bigint jednostek minimalnych → dziesiętny string. */
export function fromMinor(minor: bigint, decimals: number): string {
  const negative = minor < 0n
  const digits = (negative ? -minor : minor).toString().padStart(decimals + 1, '0')
  const whole = decimals > 0 ? digits.slice(0, -decimals) : digits
  const fraction = decimals > 0 ? `.${digits.slice(-decimals)}` : ''
  return `${negative ? '-' : ''}${whole}${fraction}`
}

/** Suma dziesiętnych stringów tej samej waluty, bez floatów. */
export function addDecimal(amounts: string[], decimals: number): string | null {
  let sum = 0n
  for (const amount of amounts) {
    const minor = toMinor(amount, decimals)
    if (minor === null) {
      return null
    }
    sum += minor
  }
  return fromMinor(sum, decimals)
}

export type AmountInput = { ok: true; value: string } | { ok: false; error: string }

/**
 * Kwota wpisana przez człowieka („1 234,5”, „12.30”) → kanoniczny dziesiętny
 * z pełną liczbą miejsc waluty („1234.50”). Spacje (też twarde) są pomijane,
 * przecinek znaczy to samo co kropka. Bez znaku, chyba że allowNegative.
 */
export function parseAmountInput(
  text: string,
  decimals: number,
  options: { allowNegative?: boolean; allowZero?: boolean } = {},
): AmountInput {
  const cleaned = text.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.').replace('\u2212', '-')
  if (cleaned === '') {
    return { ok: false, error: tk('common:amount.required') }
  }
  const match = /^(-?)(\d+)(?:\.(\d*))?$/.exec(cleaned)
  if (!match || (match[1] === '-' && !options.allowNegative)) {
    return { ok: false, error: tk(options.allowNegative ? 'common:amount.number' : 'common:amount.unsigned') }
  }
  const [, sign = '', whole = '0', fraction = ''] = match
  if (fraction.length > decimals) {
    return {
      ok: false,
      error: decimals === 0 ? tk('common:amount.noFraction') : tk('common:amount.maxDecimals', { n: decimals }),
    }
  }
  const minor = toMinor(`${sign}${whole}${fraction ? `.${fraction}` : ''}`, decimals)
  if (minor === null) {
    return { ok: false, error: tk('common:amount.invalid') }
  }
  if (minor === 0n && !options.allowZero) {
    return { ok: false, error: tk('common:amount.positive') }
  }
  // Granica int64 jednostek minimalnych po stronie serwera.
  if (minor > 9_223_372_036_854_775_807n || minor < -9_223_372_036_854_775_807n) {
    return { ok: false, error: tk('common:amount.tooLarge') }
  }
  return { ok: true, value: fromMinor(minor, decimals) }
}

/** Zmiana znaku dziesiętnego stringa, bez liczb zmiennoprzecinkowych. */
export function negateDecimal(value: string): string {
  const parsed = parseDecimal(value)
  if (!parsed) {
    return value
  }
  const body = `${parsed.whole}${parsed.fraction ? `.${parsed.fraction}` : ''}`
  const isZero = /^0+$/.test(parsed.whole) && /^0*$/.test(parsed.fraction)
  return parsed.negative || isZero ? body : `-${body}`
}

/** Wartość bezwzględna dziesiętnego stringa. */
export function absDecimal(value: string): string {
  const parsed = parseDecimal(value)
  return parsed ? `${parsed.whole}${parsed.fraction ? `.${parsed.fraction}` : ''}` : value
}

/** Kwota dla pola formularza: bez znaku, z separatorem języka („-12.50” → „12,50” / „12.50”). */
export function amountForInput(value: string): string {
  return absDecimal(value).replace('.', decimalSeparator())
}

/**
 * Suma kwot (stringów) jednej waluty; liczba miejsc z najdłuższej części
 * ułamkowej. Dla pustej listy — "0".
 */
export function sumAmounts(amounts: string[], decimals?: number): string | null {
  const places =
    decimals ?? amounts.reduce((max, amount) => Math.max(max, parseDecimal(amount)?.fraction.length ?? 0), 0)
  return addDecimal(amounts, places)
}

/** Liczba miejsc waluty: z listy /currencies, inaczej z Intl. */
export function decimalsOf(currency: string, currencies?: { code: string; decimals: number }[]): number {
  return currencies?.find((c) => c.code === currency)?.decimals ?? currencyDecimals(currency)
}

export type RateInput = { ok: true; value: string } | { ok: false; error: string }

/** Kurs wpisany przez człowieka: dodatni dziesiętny, do 12 miejsc. */
export function parseRateInput(text: string): RateInput {
  const cleaned = text.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.')
  if (cleaned === '') {
    return { ok: false, error: tk('common:rate.required') }
  }
  const match = /^(\d+)(?:\.(\d{1,12}))?$/.exec(cleaned)
  if (!match) {
    return { ok: false, error: tk('common:rate.invalid') }
  }
  const [, whole = '0', fraction = ''] = match
  if (/^0+$/.test(whole) && /^0*$/.test(fraction)) {
    return { ok: false, error: tk('common:rate.positive') }
  }
  const trimmedWhole = whole.replace(/^0+(?=\d)/, '')
  return { ok: true, value: fraction ? `${trimmedWhole}.${fraction}` : trimmedWhole }
}

/** Kurs do wyświetlenia: separator języka, bez końcowych zer („4.25120000” → „4,2512” / „4.2512”). */
export function formatRate(rate: string): string {
  const parsed = parseDecimal(rate)
  if (!parsed) {
    return rate
  }
  const fraction = parsed.fraction.replace(/0+$/, '')
  return `${parsed.whole}${fraction ? `${decimalSeparator()}${fraction}` : ''}`
}

/**
 * Przeliczenie kwoty kursem (1 waluta kwoty = rate waluty docelowej) na
 * bigint, z zaokrągleniem half-even do miejsc waluty docelowej — szacunek
 * w UI (np. „≈ 80,00 zł”). null dla niepoprawnych danych.
 */
export function convertAmount(amount: string, fromDecimals: number, rate: string, toDecimals: number): string | null {
  const minor = toMinor(amount, fromDecimals)
  const parsedRate = parseDecimal(rate)
  if (minor === null || !parsedRate || parsedRate.negative) {
    return null
  }
  const rateScale = parsedRate.fraction.length
  const rateMinor = BigInt(parsedRate.whole + parsedRate.fraction)
  const numerator = minor * rateMinor * 10n ** BigInt(toDecimals)
  const denominator = 10n ** BigInt(fromDecimals + rateScale)
  const negative = numerator < 0n
  const abs = negative ? -numerator : numerator
  let quotient = abs / denominator
  const remainder = abs % denominator
  const twice = remainder * 2n
  if (twice > denominator || (twice === denominator && quotient % 2n === 1n)) {
    quotient += 1n
  }
  return fromMinor(negative ? -quotient : quotient, toDecimals)
}
