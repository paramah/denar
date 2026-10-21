import { createContext, useContext, useEffect, type ReactNode } from 'react'
import { Outlet, useParams } from 'react-router'
import { useSessionUser } from '../api/hooks/session.ts'
import type { Household, Role } from '../api/types.ts'
import { LinkButton } from '../components/Button/Button.tsx'
import { EmptyState } from '../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../components/PageHeader/PageHeader.tsx'
import { storageKeys, writeStored } from '../lib/storage.ts'
import { AppShell } from './AppShell.tsx'

type HouseholdContextValue = { household: Household; role: Role }

const HouseholdContext = createContext<HouseholdContextValue | null>(null)

/** Gospodarstwo bieżącej trasy /h/:householdId i rola zalogowanego w nim. */
export function useHousehold(): HouseholdContextValue {
  const value = useContext(HouseholdContext)
  if (!value) {
    throw new Error('useHousehold outside /h/:householdId')
  }
  return value
}

export function HouseholdProvider({ value, children }: { value: HouseholdContextValue; children: ReactNode }) {
  return <HouseholdContext.Provider value={value}>{children}</HouseholdContext.Provider>
}

export const canManage = (role: Role) => role === 'owner' || role === 'admin'

/**
 * Rodzic tras gospodarstwa: wybiera gospodarstwo z adresu spośród tych
 * z /me i zapamiętuje je jako ostatnie. Obce lub nieistniejące — komunikat.
 */
export function HouseholdRoute() {
  const { householdId } = useParams()
  const { households } = useSessionUser()
  const household = households.find((h) => h.id === householdId)

  useEffect(() => {
    if (household) {
      writeStored(storageKeys.household, household.id)
    }
  }, [household])

  if (!household || !household.role) {
    return <MissingHousehold />
  }
  return (
    <HouseholdProvider value={{ household, role: household.role }}>
      <AppShell>
        <Outlet />
      </AppShell>
    </HouseholdProvider>
  )
}

function MissingHousehold() {
  useDocumentTitle('Nie znaleziono gospodarstwa')
  return (
    <main id="main" className="plain-main">
      <EmptyState
        icon="home"
        headingLevel={1}
        title="Nie znaleziono gospodarstwa"
        action={
          <LinkButton to="/" variant="primary">
            Wróć do swoich gospodarstw
          </LinkButton>
        }
      >
        <p>Gospodarstwo nie istnieje albo nie jesteś już jego członkiem.</p>
      </EmptyState>
    </main>
  )
}
