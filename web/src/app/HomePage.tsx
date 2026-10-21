import { Navigate } from 'react-router'
import { useSessionUser } from '../api/hooks/session.ts'
import { LinkButton } from '../components/Button/Button.tsx'
import { EmptyState } from '../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../components/PageHeader/PageHeader.tsx'
import { readStored, storageKeys } from '../lib/storage.ts'
import { PlainLayout } from './AppShell.tsx'
import { householdPath } from './nav.ts'

/** Start: ostatnie gospodarstwo, pierwsze z listy albo powitanie bez gospodarstw. */
export function HomePage() {
  const { households } = useSessionUser()
  const last = readStored(storageKeys.household)
  const target = households.find((h) => h.id === last) ?? households[0]
  if (target) {
    return <Navigate to={householdPath(target.id)} replace />
  }
  return <Welcome />
}

function Welcome() {
  const { user } = useSessionUser()
  useDocumentTitle('Witaj')
  return (
    <PlainLayout>
      <EmptyState
        icon="home"
        headingLevel={1}
        title={`Witaj, ${user.name}`}
        action={
          <LinkButton to="/households/new" variant="primary" icon="plus">
            Załóż gospodarstwo
          </LinkButton>
        }
      >
        <p>
          Nie należysz jeszcze do żadnego gospodarstwa. Załóż własne — z walutą, w której prowadzisz budżet — albo
          poproś domownika o link z zaproszeniem.
        </p>
      </EmptyState>
    </PlainLayout>
  )
}
