import { useEffect, useId, useRef, useState } from 'react'
import { Button } from '../Button/Button.tsx'
import { inputClass } from '../Field/Field.tsx'
import styles from './CopyField.module.css'

type Copied = 'ok' | 'manual' | null

/** Kopiowanie do schowka z informacją zwrotną; bez Clipboard API — `select()` do ręcznego skopiowania. */
function useCopy(value: string, select: () => void) {
  const [copied, setCopied] = useState<Copied>(null)

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
      select()
      setCopied('manual')
    }
  }
  return { copied, copy }
}

function CopyStatus({ copied }: { copied: Copied }) {
  return (
    <p className="visually-hidden" aria-live="polite">
      {copied === 'ok' ? 'Skopiowano do schowka' : copied === 'manual' ? 'Zaznaczono — skopiuj skrótem klawiszowym' : ''}
    </p>
  )
}

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
  const { copied, copy } = useCopy(value, () => inputRef.current?.select())

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
          <span className="visually-hidden">: {label}</span>
        </Button>
      </div>
      {hint && (
        <p id={`${id}-hint`} className={styles.hint}>
          {hint}
        </p>
      )}
      <CopyStatus copied={copied} />
    </div>
  )
}

/** Wielowierszowy fragment (polecenie, konfiguracja) z przyciskiem kopiowania. */
export function CopyBlock({ label, value, hint }: { label: string; value: string; hint?: string }) {
  const id = useId()
  const preRef = useRef<HTMLPreElement>(null)
  const { copied, copy } = useCopy(value, () => {
    const pre = preRef.current
    const selection = window.getSelection()
    if (pre && selection) {
      selection.selectAllChildren(pre)
    }
  })

  return (
    <div className={styles.field}>
      <div className={styles.blockHead}>
        <span id={`${id}-label`} className={styles.label}>
          {label}
        </span>
        <Button size="sm" icon={copied === 'ok' ? 'check' : 'copy'} onClick={() => void copy()}>
          {copied === 'ok' ? 'Skopiowano' : 'Kopiuj'}
          <span className="visually-hidden">: {label}</span>
        </Button>
      </div>
      {/* Przewijalny region musi być osiągalny klawiaturą (axe: scrollable-region-focusable). */}
      {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex */}
      <pre ref={preRef} className={styles.block} tabIndex={0} aria-labelledby={`${id}-label`} aria-describedby={hint ? `${id}-hint` : undefined}>
        <code>{value}</code>
      </pre>
      {hint && (
        <p id={`${id}-hint`} className={styles.hint}>
          {hint}
        </p>
      )}
      <CopyStatus copied={copied} />
    </div>
  )
}
