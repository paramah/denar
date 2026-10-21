import { Navigate, NavLink, Outlet, useLocation } from 'react-router'
import type { Role } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import styles from './settings.module.css'

type Tab = { path: string; label: string; visible: (role: Role) => boolean }

export const settingsTabs: Tab[] = [
  { path: 'general', label: 'Gospodarstwo', visible: () => true },
  { path: 'members', label: 'Członkowie', visible: () => true },
  { path: 'invitations', label: 'Zaproszenia', visible: canManage },
  { path: 'bots', label: 'Boty', visible: (role) => role !== 'viewer' },
  { path: 'payees', label: 'Kontrahenci', visible: () => true },
  { path: 'rates', label: 'Kursy walut', visible: () => true },
  { path: 'notifications', label: 'Powiadomienia', visible: () => true },
  { path: 'audit', label: 'Dziennik zmian', visible: canManage },
]

export function SettingsLayout() {
  const { household, role } = useHousehold()
  const tabs = settingsTabs.filter((tab) => tab.visible(role))
  return (
    <>
      <PageHeader title="Ustawienia" description={`Gospodarstwo ${household.name}`} />
      <nav aria-label="Ustawienia" className={styles.tabs}>
        <ul>
          {tabs.map((tab) => (
            <li key={tab.path}>
              <NavLink
                to={householdPath(household.id, `settings/${tab.path}`)}
                className={({ isActive }) => (isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab)}
              >
                {tab.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <div className={styles.panel}>
        <Outlet />
      </div>
    </>
  )
}

/** Zakładka niedostępna dla roli: wraca na pierwszą zakładkę. */
export function RequireTab({ path, children }: { path: string; children: React.ReactNode }) {
  const { household, role } = useHousehold()
  const location = useLocation()
  const tab = settingsTabs.find((t) => t.path === path)
  if (tab && !tab.visible(role)) {
    return <Navigate to={householdPath(household.id, 'settings/general')} replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
