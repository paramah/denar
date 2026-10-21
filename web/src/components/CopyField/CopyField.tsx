import { useEffect, useId, useRef, useState } from 'react'
import { Button } from '../Button/Button.tsx'
import { inputClass } from '../Field/Field.tsx'
import styles from './CopyField.module.css'

type Props = {
  label: string
  value: string
  hint?: string
  /** Ustawia focus na polu przy montowaniu (np. w dialogu z tokenem). */
  initialFocus?: boolean
}

/** Wartość tylko do odczytu z przyciskiem kopiowania. */
export function CopyField({ label, value, hint, initialFocus }: Props) {
  const id = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [copied, setCopied] = useState<'ok' | 'manual' | null>(null)

  useEffect(() => {
    if (!copied) {
      return undefined
    }
    const timer = setTimeout(() => setCopied(null), 4000)
    return () => clearTimeout(timer)
  }, [copied])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied('ok')
    } catch {
      // Bez Clipboard API (np. http poza localhost): zaznacz, żeby skopiować ręcznie.
      inputRef.current?.select()
      setCopied('manual')
    }
  }

  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {label}
      </label>
      <div className={styles.row}>
        <input
          ref={inputRef}
          id={id}
          className={`${inputClass} ${styles.value}`}
          value={value}
          readOnly
          spellCheck={false}
          onFocus={(event) => event.target.select()}
          aria-describedby={hint ? `${id}-hint` : undefined}
          data-autofocus={initialFocus || undefined}
        />
        <Button icon={copied === 'ok' ? 'check' : 'copy'} onClick={() => void copy()}>
          {copied === 'ok' ? 'Skopiowano' : 'Kopiuj'}
        </Button>
      </div>
      {hint && (
        <p id={`${id}-hint`} className={styles.hint}>
          {hint}
        </p>
      )}
      <p className="visually-hidden" aria-live="polite">
        {copied === 'ok' ? 'Skopiowano do schowka' : copied === 'manual' ? 'Zaznaczono — skopiuj skrótem klawiszowym' : ''}
      </p>
    </div>
  )
}
