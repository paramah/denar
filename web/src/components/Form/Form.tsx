import type { FormEventHandler, ReactNode } from 'react'
import { ApiError, errorMessage, fieldErrors } from '../../api/errors.ts'
import { fieldLabel } from '../../lib/labels.ts'
import { Icon } from '../Icon/Icon.tsx'
import styles from './Form.module.css'

type FormProps = {
  onSubmit: FormEventHandler<HTMLFormElement>
  children: ReactNode
  /** Dostępna nazwa formularza. */
  label?: string
  className?: string
}

/** Formularz bez walidacji przeglądarki: błędy pokazuje interfejs. */
export function Form({ onSubmit, children, label, className }: FormProps) {
  return (
    <form
      noValidate
      aria-label={label}
      className={[styles.form, className].filter(Boolean).join(' ')}
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit(event)
      }}
    >
      {children}
    </form>
  )
}

export function FormActions({ children }: { children: ReactNode }) {
  return <div className={styles.actions}>{children}</div>
}

type FormAlertProps = {
  error: unknown
  /** Pola wyświetlane przy kontrolkach; ich błędy nie trafiają tutaj. */
  shownFields?: string[]
  fallback?: string
}

/**
 * Błąd formularza, który nie należy do żadnego widocznego pola: ogólny
 * komunikat albo błędy pól, których formularz nie pokazuje.
 */
export function FormAlert({ error, shownFields = [], fallback }: FormAlertProps) {
  if (!error) {
    return null
  }
  const fields = fieldErrors(error)
  const other = Object.entries(fields).filter(([field]) => !shownFields.includes(field))
  const allShown = error instanceof ApiError && error.status === 422 && Object.keys(fields).length > 0 && other.length === 0
  if (allShown) {
    return (
      <p role="alert" className="visually-hidden">
        Popraw zaznaczone pola.
      </p>
    )
  }
  return (
    <div role="alert" className={styles.alert}>
      <Icon name="alert" size={18} />
      <div>
        <p>{other.length > 0 ? 'Popraw dane formularza.' : errorMessage(error, fallback)}</p>
        {other.length > 0 && (
          <ul>
            {other.map(([field, message]) => (
              <li key={field}>
                {fieldLabel(field)}: {message}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

type CalloutProps = { tone?: 'info' | 'warn'; title?: string; children: ReactNode }

/** Wyróżniona informacja w treści (nie komunikat o błędzie). */
export function Callout({ tone = 'info', title, children }: CalloutProps) {
  return (
    <div className={`${styles.callout} ${styles[tone]}`}>
      <Icon name={tone === 'warn' ? 'alert' : 'info'} size={18} />
      <div>
        {title && <p className={styles.calloutTitle}>{title}</p>}
        <div>{children}</div>
      </div>
    </div>
  )
}
