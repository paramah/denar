import { NavLink, Outlet } from 'react-router'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import styles from './import.module.css'

const tabs = [
  { path: 'import', label: 'Nowy import' },
  { path: 'import/history', label: 'Historia' },
]

/** Nagłówek i zakładki importu: kreator i historia partii. */
export function ImportLayout() {
  const { household } = useHousehold()
  return (
    <>
      <PageHeader
        title="Import transakcji"
        description="Wyciąg CSV z banku trafia do portfela: duplikaty są pomijane, kontrahenci i koperty dopasowane regułami."
      />
      <nav aria-label="Import" className={styles.tabs}>
        <ul>
          {tabs.map((tab) => (
            <li key={tab.path}>
              <NavLink
                to={householdPath(household.id, tab.path)}
                end
                className={({ isActive }) => (isActive ? `${styles.tab} ${styles.activeTab}` : styles.tab)}
              >
                {tab.label}
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
