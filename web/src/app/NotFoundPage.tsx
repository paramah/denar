import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { LinkButton } from '../components/Button/Button.tsx'
import { EmptyState } from '../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../components/PageHeader/PageHeader.tsx'
import { householdPath } from './nav.ts'

/** Nieznany adres poza gospodarstwem: osobna strona bez nawigacji. */
export function NotFoundPage() {
  const { t } = useTranslation()
  const title = t('notFound.title')
  useDocumentTitle(title)
  return (
    <main id="main" className="plain-main">
      <EmptyState
        icon="alert"
        headingLevel={1}
        title={title}
        action={
          <LinkButton to="/" variant="primary">
            {t('notFound.toStart')}
          </LinkButton>
        }
      >
        <p>{t('notFound.explanation')}</p>
      </EmptyState>
    </main>
  )
}

/**
 * Nieznany adres w gospodarstwie (`/h/:id/…`): w powłoce aplikacji, żeby
 * nawigacja zostawała pod ręką, z powrotem na pulpit gospodarstwa.
 */
export function HouseholdNotFoundPage() {
  const { t } = useTranslation()
  const title = t('notFound.title')
  useDocumentTitle(title)
  const { householdId = '' } = useParams()
  return (
    <EmptyState
      icon="alert"
      headingLevel={1}
      title={title}
      action={
        <LinkButton to={householdPath(householdId)} variant="primary">
          {t('notFound.toDashboard')}
        </LinkButton>
      }
    >
      <p>{t('notFound.explanation')}</p>
    </EmptyState>
  )
}
