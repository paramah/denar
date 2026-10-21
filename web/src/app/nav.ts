import type { IconName } from '../components/Icon/Icon.tsx'

export type NavItem = {
  /** Ścieżka względem /h/:householdId ('' = pulpit). */
  path: string
  label: string
  icon: IconName
}

/** Główna nawigacja gospodarstwa; części B/C wypełniają te strony treścią. */
export const navItems: NavItem[] = [
  { path: '', label: 'Pulpit', icon: 'dashboard' },
  { path: 'wallets', label: 'Portfele', icon: 'wallet' },
  { path: 'budget', label: 'Budżet', icon: 'budget' },
  { path: 'subscriptions', label: 'Subskrypcje', icon: 'repeat' },
  { path: 'notifications', label: 'Powiadomienia', icon: 'bell' },
  { path: 'settings', label: 'Ustawienia', icon: 'settings' },
]

export const householdPath = (hid: string, path = '') => (path ? `/h/${hid}/${path}` : `/h/${hid}`)

/** Ten sam fragment ścieżki w innym gospodarstwie (przełącznik). */
export function switchHouseholdPath(pathname: string, toHid: string): string {
  const match = /^\/h\/[^/]+(\/.*)?$/.exec(pathname)
  return `/h/${toHid}${match?.[1] ?? ''}`
}
