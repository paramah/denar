// Kwoty z API przychodzą jako dziesiętny string ("-12.34") z kodem waluty.
// Nic tu nie zamienia ich na liczbę zmiennoprzecinkową: formatowanie idzie
// przez Intl.NumberFormat na stringu (dokładny dziesiętny), a arytmetyka —
// gdyby UI jej potrzebowało — na bigint jednostek minimalnych.

export const locale = 'pl-PL'

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
  const key = `${currency}|${signed}`
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
 * Formatuje kwotę w locale pl-PL, np. "-1234.5" PLN → "−1234,50 zł".
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
