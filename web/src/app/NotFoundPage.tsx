import { useParams } from 'react-router'
import { LinkButton } from '../components/Button/Button.tsx'
import { EmptyState } from '../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../components/PageHeader/PageHeader.tsx'
import { householdPath } from './nav.ts'

const title = 'Nie ma takiej strony'
const explanation = 'Adres jest niepełny albo strona została przeniesiona. Sprawdź link albo wróć do aplikacji.'

/** Nieznany adres poza gospodarstwem: osobna strona bez nawigacji. */
export function NotFoundPage() {
  useDocumentTitle(title)
  return (
    <main id="main" className="plain-main">
      <EmptyState
        icon="alert"
        headingLevel={1}
        title={title}
        action={
          <LinkButton to="/" variant="primary">
            Przejdź na start
          </LinkButton>
        }
      >
        <p>{explanation}</p>
      </EmptyState>
    </main>
  )
}

/**
 * Nieznany adres w gospodarstwie (`/h/:id/…`): w powłoce aplikacji, żeby
 * nawigacja zostawała pod ręką, z powrotem na pulpit gospodarstwa.
 */
export function HouseholdNotFoundPage() {
  useDocumentTitle(title)
  const { householdId = '' } = useParams()
  return (
    <EmptyState
      icon="alert"
      headingLevel={1}
      title={title}
      action={
        <LinkButton to={householdPath(householdId)} variant="primary">
          Wróć na pulpit
        </LinkButton>
      }
    >
      <p>{explanation}</p>
    </EmptyState>
  )
}
