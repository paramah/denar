import { locale } from './money.ts'

const dateFormat = new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short', year: 'numeric' })
const dateTimeFormat = new Intl.DateTimeFormat(locale, {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})
const relativeFormat = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })

function parse(value: string | Date): Date | null {
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

/** „21 paź 2026”. */
export function formatDate(value: string | Date): string {
  const date = parse(value)
  return date ? dateFormat.format(date) : String(value)
}

/** „21 paź 2026, 14:05”. */
export function formatDateTime(value: string | Date): string {
  const date = parse(value)
  return date ? dateTimeFormat.format(date) : String(value)
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

/** „5 minut temu”, „za 3 dni”; poniżej minuty — „teraz”. */
export function formatRelative(value: string | Date, now: Date = new Date()): string {
  const date = parse(value)
  if (!date) {
    return String(value)
  }
  const seconds = Math.round((date.getTime() - now.getTime()) / 1000)
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) {
      return relativeFormat.format(Math.trunc(seconds / size), unit)
    }
  }
  return 'teraz'
}

/** Wartość dla <input type="date"> (lokalna data). */
export function toDateInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** Koniec lokalnego dnia z <input type="date"> jako ISO UTC. */
export function endOfLocalDay(value: string): string | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) {
    return null
  }
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]), 23, 59, 59)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

export function addDays(date: Date, days: number): Date {
  const out = new Date(date)
  out.setDate(out.getDate() + days)
  return out
}

/** Strefy IANA wspierane przez przeglądarkę; strefa przeglądarki pierwsza. */
export function timeZones(): string[] {
  const own = Intl.DateTimeFormat().resolvedOptions().timeZone
  let all: string[] = []
  try {
    all = Intl.supportedValuesOf('timeZone')
  } catch {
    all = []
  }
  const list = all.includes('UTC') ? all : [...all, 'UTC']
  return own && !list.includes(own) ? [own, ...list] : list
}

export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Warsaw'
}

/** Dzisiejsza data (YYYY-MM-DD) w strefie IANA, np. strefie gospodarstwa. */
export function todayIn(timeZone: string, now: Date = new Date()): string {
  try {
    return new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(now)
  } catch {
    return toDateInput(now)
  }
}

/** Dzień kalendarzowy z API („2026-10-21”) → „21 paź 2026”, bez przesunięcia strefy. */
export function formatDay(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) {
    return value
  }
  return dateFormat.format(new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
}
