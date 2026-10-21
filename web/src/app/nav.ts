import type { IconName } from '../components/Icon/Icon.tsx'

export type NavItem = {
  /** Ścieżka względem /h/:householdId ('' = pulpit). */
  path: string
  /** Klucz w common.json → nav. */
  label: 'dashboard' | 'wallets' | 'transactions' | 'budget' | 'subscriptions' | 'notifications' | 'settings'
  icon: IconName
}

/** Główna nawigacja gospodarstwa; części B/C wypełniają te strony treścią. */
export const navItems: NavItem[] = [
  { path: '', label: 'dashboard', icon: 'dashboard' },
  { path: 'wallets', label: 'wallets', icon: 'wallet' },
  { path: 'transactions', label: 'transactions', icon: 'list' },
  { path: 'budget', label: 'budget', icon: 'budget' },
  { path: 'subscriptions', label: 'subscriptions', icon: 'repeat' },
  { path: 'notifications', label: 'notifications', icon: 'bell' },
  { path: 'settings', label: 'settings', icon: 'settings' },
]

export const householdPath = (hid: string, path = '') => (path ? `/h/${hid}/${path}` : `/h/${hid}`)

/** Ten sam fragment ścieżki w innym gospodarstwie (przełącznik). */
export function switchHouseholdPath(pathname: string, toHid: string): string {
  const match = /^\/h\/[^/]+(\/.*)?$/.exec(pathname)
  return `/h/${toHid}${match?.[1] ?? ''}`
}
