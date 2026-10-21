import { todayIn } from './format.ts'
import { locale } from './money.ts'

// Miesiąc budżetowy (003 FR-200): etykieta YYYY-MM miesiąca kalendarzowego,
// w którym się zaczyna. Przy month_start_day = 10 dzień 2026-10-05 należy do
// 2026-09, a 2026-10-10 — do 2026-10.

const pattern = /^(\d{4})-(\d{2})$/

export const firstMonth = '2000-01'
export const lastMonth = '2100-12'

/** Poprawna etykieta w zakresie API (2000-01…2100-12). */
export function isMonth(value: string | undefined): value is string {
  const match = value ? pattern.exec(value) : null
  if (!match) {
    return false
  }
  const month = Number(match[2])
  return month >= 1 && month <= 12 && value! >= firstMonth && value! <= lastMonth
}

function label(year: number, month: number): string {
  return `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}`
}

/** Miesiąc przesunięty o delta (−1 poprzedni, +1 następny). */
export function shiftMonth(month: string, delta: number): string {
  const match = pattern.exec(month)
  if (!match) {
    return month
  }
  const index = Number(match[1]) * 12 + (Number(match[2]) - 1) + delta
  return label(Math.floor(index / 12), (index % 12) + 1)
}

/** Miesiąc budżetowy dnia (YYYY-MM-DD) przy danym dniu startu. */
export function monthOfDay(day: string, startDay: number): string {
  const [year = 2000, month = 1, date = 1] = day.split('-').map(Number)
  const own = label(year, month)
  return date < startDay ? shiftMonth(own, -1) : own
}

/** Bieżący miesiąc budżetowy gospodarstwa (dziś w jego strefie). */
export function currentBudgetMonth(timeZone: string, startDay: number, now: Date = new Date()): string {
  return monthOfDay(todayIn(timeZone, now), startDay)
}

const monthFormat = new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' })

/** „2026-10” → „październik 2026”. */
export function formatMonth(month: string): string {
  const match = pattern.exec(month)
  if (!match) {
    return month
  }
  return monthFormat.format(new Date(Number(match[1]), Number(match[2]) - 1, 1))
}
