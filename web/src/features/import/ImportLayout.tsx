import { useTranslation } from 'react-i18next'
import { NavLink, Outlet } from 'react-router'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import styles from './import.module.css'

const tabs = [
  { path: 'import', label: 'tabNew' },
  { path: 'import/history', label: 'tabHistory' },
] as const

/** Nagłówek i zakładki importu: kreator i historia partii. */
export function ImportLayout() {
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  return (
    <>
      <PageHeader
        title={t('layout.title')}
        description={t('layout.description')}
      />
      <nav aria-label={t('layout.nav')} className={styles.tabs}>
        <ul>
          {tabs.map((tab) => (
            <li key={tab.path}>
              <NavLink
                to={householdPath(household.id, tab.path)}
                end
                className={({ isActive }) => (isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab)}
              >
                {t(`layout.${tab.label}`)}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <div className={styles.page}>
        <Outlet />
      </div>
    </>
  )
}
