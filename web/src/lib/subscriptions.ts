import type { ChargeStatus, Cycle, CycleUnit, SubscriptionStatus } from '../api/types.ts'
import { perLocale } from './intl.ts'
import { toMinor } from './money.ts'

// Wielkość liter w nazwach (dane użytkownika, nie tekst interfejsu) — zawsze
// według reguł polskich, niezależnie od języka interfejsu.
const dataLocale = 'pl'

// Czyste funkcje subskrypcji: cykl po polsku, kolejne terminy (ta sama
// arytmetyka co serwer — 004 FR-302), monogram i odcień dostawcy, dni do terminu.

export const subscriptionStatusLabels: Record<SubscriptionStatus, string> = {
  trial: 'Okres próbny',
  active: 'Aktywna',
  paused: 'Wstrzymana',
  cancelled: 'Anulowana',
}

export const subscriptionStatusOrder: SubscriptionStatus[] = ['active', 'trial', 'paused', 'cancelled']

export const chargeStatusLabels: Record<ChargeStatus, string> = {
  expected: 'Oczekiwana',
  matched: 'Dopasowana',
  missed: 'Przegapiona',
  skipped: 'Pominięta',
  posted: 'Zaksięgowana',
}

export const chargeStatusDescriptions: Record<ChargeStatus, string> = {
  expected: 'Płatność jeszcze nie znaleziona w rejestrze',
  matched: 'Powiązana z transakcją z rejestru',
  missed: 'Termin i okno dopasowania minęły bez płatności',
  skipped: 'Oznaczona jako nieopłacana w tym terminie',
  posted: 'Wydatek zapisany z tej płatności',
}

export const cycleUnitLabels: Record<CycleUnit, string> = {
  day: 'dni',
  week: 'tygodnie',
  month: 'miesiące',
  year: 'lata',
}

/** Polska forma liczebnika: 1 → one, 2–4 (bez 12–14) → few, reszta → many. */
export function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) {
    return one
  }
  const last = n % 10
  const lastTwo = n % 100
  return last >= 2 && last <= 4 && !(lastTwo >= 12 && lastTwo <= 14) ? few : many
}

const unitWords: Record<CycleUnit, { every1: string; one: string; few: string; many: string }> = {
  day: { every1: 'codziennie', one: 'dzień', few: 'dni', many: 'dni' },
  week: { every1: 'co tydzień', one: 'tydzień', few: 'tygodnie', many: 'tygodni' },
  month: { every1: 'co miesiąc', one: 'miesiąc', few: 'miesiące', many: 'miesięcy' },
  year: { every1: 'co rok', one: 'rok', few: 'lata', many: 'lat' },
}

const weekdayOn = ['w niedzielę', 'w poniedziałek', 'we wtorek', 'w środę', 'w czwartek', 'w piątek', 'w sobotę']
const dayMonth = perLocale((locale) => new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'long' }))

type Day = { y: number; m: number; d: number }

function parseDay(value: string): Day | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  return match ? { y: Number(match[1]), m: Number(match[2]), d: Number(match[3]) } : null
}

const pad = (n: number) => String(n).padStart(2, '0')
const dayString = ({ y, m, d }: Day) => `${y}-${pad(m)}-${pad(d)}`
const daysInMonth = (y: number, m: number) => new Date(Date.UTC(y, m, 0)).getUTCDate()
const utc = ({ y, m, d }: Day) => Date.UTC(y, m - 1, d)
const fromUtc = (ms: number): Day => {
  const date = new Date(ms)
  return { y: date.getUTCFullYear(), m: date.getUTCMonth() + 1, d: date.getUTCDate() }
}

/** Liczba dni od a do b (dni kalendarzowe, b − a). */
export function daysBetween(a: string, b: string): number {
  const x = parseDay(a)
  const y = parseDay(b)
  return x && y ? Math.round((utc(y) - utc(x)) / 86_400_000) : 0
}

export function addDaysTo(day: string, days: number): string {
  const parsed = parseDay(day)
  return parsed ? dayString(fromUtc(utc(parsed) + days * 86_400_000)) : day
}

/**
 * Cykl czytelnie: „co miesiąc, 31.”, „co 2 tygodnie, w środę”, „co rok, 14 lutego”,
 * „codziennie”, „co 3 dni”.
 */
export function cycleLabel(cycle: Pick<Cycle, 'unit' | 'every' | 'anchor'>): string {
  const words = unitWords[cycle.unit]
  const every = Math.max(1, cycle.every)
  const head = every === 1 ? words.every1 : `co ${every} ${plural(every, words.one, words.few, words.many)}`
  const anchor = parseDay(cycle.anchor)
  if (!anchor) {
    return head
  }
  switch (cycle.unit) {
    case 'week':
      return `${head}, ${weekdayOn[new Date(utc(anchor)).getUTCDay()]}`
    case 'month':
      return `${head}, ${anchor.d}.`
    case 'year':
      return `${head}, ${dayMonth().format(new Date(anchor.y, anchor.m - 1, anchor.d))}`
    default:
      return head
  }
}

/** Dopisek dla dnia > 28: krótsze miesiące płacą w ostatnim dniu. */
export function cycleNote(cycle: Pick<Cycle, 'unit' | 'anchor'>): string | null {
  const anchor = parseDay(cycle.anchor)
  if (!anchor || anchor.d <= 28 || (cycle.unit !== 'month' && cycle.unit !== 'year')) {
    return null
  }
  if (cycle.unit === 'year') {
    return anchor.m === 2 && anchor.d === 29 ? 'W latach nieprzestępnych — 28 lutego.' : null
  }
  return `W krótszych miesiącach — ostatniego dnia miesiąca.`
}

/** k-ty termin cyklu liczony zawsze od anchor (bez dryfu). */
export function occurrence(cycle: Pick<Cycle, 'unit' | 'every' | 'anchor'>, k: number): string {
  const anchor = parseDay(cycle.anchor)
  if (!anchor) {
    return cycle.anchor
  }
  const every = Math.max(1, cycle.every)
  switch (cycle.unit) {
    case 'day':
      return dayString(fromUtc(utc(anchor) + k * every * 86_400_000))
    case 'week':
      return dayString(fromUtc(utc(anchor) + k * every * 7 * 86_400_000))
    default: {
      const months = k * every * (cycle.unit === 'year' ? 12 : 1)
      const index = anchor.y * 12 + (anchor.m - 1) + months
      const y = Math.floor(index / 12)
      const m = (index % 12) + 1
      return dayString({ y, m, d: Math.min(anchor.d, daysInMonth(y, m)) })
    }
  }
}

/** Najbliższe `count` terminów w dniu `from` lub później. */
export function nextDues(cycle: Pick<Cycle, 'unit' | 'every' | 'anchor'>, from: string, count: number): string[] {
  if (!parseDay(cycle.anchor) || !parseDay(from)) {
    return []
  }
  let k = 0
  if (from > cycle.anchor) {
    // Szacunek k z odległości, potem korekta w dół i w górę.
    const days = daysBetween(cycle.anchor, from)
    const every = Math.max(1, cycle.every)
    const step = { day: 1, week: 7, month: 30.44, year: 365.25 }[cycle.unit] * every
    k = Math.max(0, Math.floor(days / step) - 1)
    while (k > 0 && occurrence(cycle, k) >= from) {
      k -= 1
    }
    while (occurrence(cycle, k) < from) {
      k += 1
    }
  }
  return Array.from({ length: count }, (_, i) => occurrence(cycle, k + i))
}

/** „dziś”, „jutro”, „za 3 dni”, „wczoraj”, „3 dni temu”. */
export function relativeDay(day: string, today: string): string {
  const n = daysBetween(today, day)
  if (n === 0) return 'dziś'
  if (n === 1) return 'jutro'
  if (n === -1) return 'wczoraj'
  if (n > 0) return `za ${n} ${plural(n, 'dzień', 'dni', 'dni')}`
  return `${-n} ${plural(-n, 'dzień', 'dni', 'dni')} temu`
}

/** Monogram dostawcy lub nazwy: „OpenAI” → „OA”, „HBO Max” → „HM”, „iCloud” → „iC”. */
export function monogram(name: string): string {
  const words = name
    .replace(/[^\p{L}\p{N}\s+]/gu, ' ')
    .split(/\s+/)
    .filter(Boolean)
  if (words.length === 0) {
    return '?'
  }
  if (words.length >= 2) {
    return (words[0]![0]! + words[1]![0]!).toLocaleUpperCase(dataLocale)
  }
  const word = words[0]!
  // Wielbłądzi zapis: pierwsza litera i kolejna wielka (OpenAI, YouTube, GitHub, iCloud).
  const capital = [...word.slice(1)].find((ch) => ch !== ch.toLocaleLowerCase(dataLocale) && ch === ch.toLocaleUpperCase(dataLocale))
  if (capital) {
    return `${word[0]}${capital}`
  }
  return word.slice(0, 2).replace(/^./, (ch) => ch.toLocaleUpperCase(dataLocale))
}

/** Stop monety (1–6) dla dostawcy: stały dla klucza, bez zewnętrznych logo (CSP). */
export function vendorMetal(key: string): number {
  let hash = 0
  for (const ch of key) {
    hash = (hash * 31 + ch.codePointAt(0)!) >>> 0
  }
  return (hash % 6) + 1
}

/** Udział kwoty w maksimum, 0–100 (do szerokości słupka; liczone na bigint). */
export function share(amount: string, max: string, decimals: number): number {
  const a = toMinor(amount, decimals)
  const m = toMinor(max, decimals)
  if (a === null || m === null || m <= 0n || a <= 0n) {
    return 0
  }
  const permille = Number((a * 1000n) / m)
  return Math.max(2, Math.min(100, permille / 10))
}

/** Domyślne i proponowane offsety przypomnień (dni przed terminem). */
export const reminderChoices = [0, 1, 3, 7, 14, 30]
export const maxReminders = 5

export function reminderLabel(days: number): string {
  if (days === 0) return 'w dniu płatności'
  if (days === 7) return 'tydzień przed'
  if (days === 14) return '2 tygodnie przed'
  if (days === 30) return 'miesiąc przed'
  return `${days} ${plural(days, 'dzień', 'dni', 'dni')} przed`
}

/** Czy adres zarządzania jest http(s). */
export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
