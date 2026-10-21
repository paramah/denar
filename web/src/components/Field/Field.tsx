import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react'
import styles from './Field.module.css'

export type ControlProps = {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
}

type FieldProps = {
  label: ReactNode
  hint?: ReactNode
  /** Komunikat błędu (np. z problem+json `errors[]`). */
  error?: string
  /** Etykieta widoczna tylko dla czytników ekranu. */
  hideLabel?: boolean
  children: (control: ControlProps) => ReactNode
  className?: string
}

/** Etykieta, kontrolka, podpowiedź i błąd powiązane przez id i aria-describedby. */
export function Field({ label, hint, error, hideLabel, children, className }: FieldProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [errorId, hintId].filter(Boolean).join(' ') || undefined
  return (
    <div className={[styles.field, className].filter(Boolean).join(' ')}>
      <label htmlFor={id} className={hideLabel ? 'visually-hidden' : styles.label}>
        {label}
      </label>
      {children({ id, 'aria-describedby': describedBy, 'aria-invalid': error ? true : undefined })}
      {hint && (
        <p id={hintId} className={styles.hint}>
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className={styles.error}>
          {error}
        </p>
      )}
    </div>
  )
}

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: ReactNode
  hint?: ReactNode
  error?: string
  hideLabel?: boolean
}

export function TextField({ label, hint, error, hideLabel, className, ...input }: TextFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} hideLabel={hideLabel} className={className}>
      {(control) => <input className={styles.input} {...control} {...input} />}
    </Field>
  )
}

type SelectFieldProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> & {
  label: ReactNode
  hint?: ReactNode
  error?: string
  hideLabel?: boolean
  options: { value: string; label: string; disabled?: boolean }[]
}

export function SelectField({ label, hint, error, hideLabel, options, className, ...select }: SelectFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} hideLabel={hideLabel} className={className}>
      {(control) => (
        <select className={styles.select} {...control} {...select}>
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
      )}
    </Field>
  )
}

type CheckboxProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'id'> & {
  label: ReactNode
  hint?: ReactNode
}

export function Checkbox({ label, hint, className, ...input }: CheckboxProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  return (
    <div className={[styles.checkbox, className].filter(Boolean).join(' ')}>
      <input type="checkbox" id={id} aria-describedby={hintId} {...input} />
      <div className={styles.checkboxText}>
        <label htmlFor={id}>{label}</label>
        {hint && (
          <span id={hintId} className={styles.hint}>
            {hint}
          </span>
        )}
      </div>
    </div>
  )
}

/** Klasa pola tekstowego dla kontrolek składanych ręcznie przez Field. */
export const inputClass = styles.input
export const selectClass = styles.select
