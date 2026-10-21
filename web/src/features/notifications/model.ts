import type { EventType, Notification } from '../../api/types.ts'
import type { IconName } from '../../components/Icon/Icon.tsx'
import { perLocale } from '../../lib/intl.ts'
import { safeReturnTo } from '../../lib/returnTo.ts'

export const eventTypeIcons: Record<EventType, IconName> = {
  'subscription.renewal_upcoming': 'repeat',
  'subscription.trial_ending': 'clock',
  'subscription.price_changed': 'tag',
  'subscription.charge_missed': 'alert',
  'envelope.overspent': 'budget',
  'envelope.goal_due': 'target',
}

export function notificationIcon(type: string): IconName {
  return eventTypeIcons[type as EventType] ?? 'bell'
}

/** Kolor plakietki ikony: ostrzeżenia odróżniają się od informacji. */
export function notificationTone(type: string): 'warn' | 'negative' | 'brand' {
  if (type === 'subscription.charge_missed' || type === 'envelope.overspent') return 'negative'
  if (type === 'subscription.price_changed' || type === 'envelope.goal_due' || type === 'subscription.trial_ending') return 'warn'
  return 'brand'
}

/** Ścieżka SPA z powiadomienia; cokolwiek innego niż adres tej aplikacji prowadzi do gospodarstwa. */
export function notificationLink(n: Pick<Notification, 'link' | 'household_id'>): string {
  const safe = safeReturnTo(n.link)
  return safe === '/' ? `/h/${n.household_id}` : safe
}

function localDay(date: Date): string {
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

const weekdayFormat = perLocale((locale) => new Intl.DateTimeFormat(locale, { weekday: 'long', day: 'numeric', month: 'long' }))
const yearFormat = perLocale((locale) => new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'long', year: 'numeric' }))
const timeFormat = perLocale((locale) => new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' }))

/** „Dziś”, „Wczoraj”, „poniedziałek, 19 października”, w innym roku — z rokiem. */
export function dayLabel(day: string, now: Date = new Date()): string {
  const today = localDay(now)
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (day === today) return 'Dziś'
  if (day === localDay(yesterday)) return 'Wczoraj'
  const [y, m, d] = day.split('-').map(Number)
  const date = new Date(y!, m! - 1, d!)
  const text = y === now.getFullYear() ? weekdayFormat().format(date) : yearFormat().format(date)
  return text.charAt(0).toUpperCase() + text.slice(1)
}

export type DayGroup = { day: string; label: string; items: Notification[] }

/** Grupy po lokalnym dniu utworzenia, w kolejności listy (od najnowszych). */
export function groupByDay(items: Notification[], now: Date = new Date()): DayGroup[] {
  const groups: DayGroup[] = []
  for (const item of items) {
    const day = localDay(new Date(item.created_at))
    let group = groups.at(-1)
    if (!group || group.day !== day) {
      group = { day, label: dayLabel(day, now), items: [] }
      groups.push(group)
    }
    group.items.push(item)
  }
  return groups
}

export function formatTime(value: string): string {
  return timeFormat().format(new Date(value))
}
