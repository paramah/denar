import { LinkButton } from '../components/Button/Button.tsx'
import { EmptyState } from '../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../components/PageHeader/PageHeader.tsx'

export function NotFoundPage() {
  useDocumentTitle('Nie ma takiej strony')
  return (
    <main id="main" className="plain-main">
      <EmptyState
        icon="alert"
        headingLevel={1}
        title="Nie ma takiej strony"
        action={
          <LinkButton to="/" variant="primary">
            Przejdź na start
          </LinkButton>
        }
      >
        <p>Adres jest niepełny albo strona została przeniesiona.</p>
      </EmptyState>
    </main>
  )
}
