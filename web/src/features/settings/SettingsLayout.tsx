import { useTranslation } from 'react-i18next'
import { Navigate, NavLink, Outlet, useLocation } from 'react-router'
import type { Role } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import styles from './settings.module.css'

type TabPath = 'general' | 'members' | 'invitations' | 'bots' | 'payees' | 'rates' | 'notifications' | 'connections' | 'audit'

/** Zakładka ustawień; nazwa to klucz `settings:tabs.<path>`. */
type Tab = { path: TabPath; visible: (role: Role) => boolean }

export const settingsTabs: Tab[] = [
  { path: 'general', visible: () => true },
  { path: 'members', visible: () => true },
  { path: 'invitations', visible: canManage },
  { path: 'bots', visible: (role) => role !== 'viewer' },
  { path: 'payees', visible: () => true },
  { path: 'rates', visible: () => true },
  { path: 'notifications', visible: () => true },
  { path: 'connections', visible: canManage },
  { path: 'audit', visible: canManage },
]

export function SettingsLayout() {
  const { t } = useTranslation('settings')
  const { household, role } = useHousehold()
  const tabs = settingsTabs.filter((tab) => tab.visible(role))
  return (
    <>
      <PageHeader title={t('layout.title')} description={t('layout.description', { name: household.name })} />
      <nav aria-label={t('layout.title')} className={styles.tabs}>
        <ul>
          {tabs.map((tab) => (
            <li key={tab.path}>
              <NavLink
                to={householdPath(household.id, `settings/${tab.path}`)}
                className={({ isActive }) => (isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab)}
              >
                {t(`tabs.${tab.path}`)}
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
  const tab = settingsTabs.find((tab) => tab.path === path)
  if (tab && !tab.visible(role)) {
    return <Navigate to={householdPath(household.id, 'settings/general')} replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
