import { useEffect, useId, useLayoutEffect, useRef, useState, type FormEventHandler, type ReactNode } from 'react'
import { Button, IconButton } from '../Button/Button.tsx'
import { inputClass } from '../Field/Field.tsx'
import styles from './Dialog.module.css'
import { tk } from '../../i18n/translate.ts'

type Props = {
  open: boolean
  title: string
  /** Escape, przycisk zamknięcia i klik w tło; właściciel zamyka, ustawiając open=false. */
  onClose: () => void
  children: ReactNode
  /** Akcje w stopce. */
  footer?: ReactNode
  /** Owija treść i stopkę w formularz (Enter zatwierdza). */
  onSubmit?: FormEventHandler<HTMLFormElement>
  role?: 'dialog' | 'alertdialog'
  size?: 'sm' | 'md' | 'lg'
  /** Opis dla czytników (aria-describedby) — id elementu w treści. */
  describedBy?: string
}

const focusable =
  'input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'

/**
 * Okno modalne na natywnym <dialog>: przeglądarka więzi focus, wyłącza
 * stronę pod spodem i obsługuje Escape. Montowane tylko, gdy otwarte, więc
 * treść startuje od zera; po zamknięciu focus wraca do elementu, który je otworzył.
 */
export function Dialog(props: Props) {
  return props.open ? <OpenDialog {...props} /> : null
}

function OpenDialog({ title, onClose, children, footer, onSubmit, role, size = 'md', describedBy }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const titleId = useId()
  const close = useRef(onClose)
  useLayoutEffect(() => {
    close.current = onClose
  })

  useEffect(() => {
    const dialog = ref.current!
    const opener = document.activeElement
    dialog.showModal()
    const start =
      contentRef.current?.querySelector<HTMLElement>('[data-autofocus]') ??
      contentRef.current?.querySelector<HTMLElement>(focusable) ??
      dialog
    start.focus()

    const onCancel = (event: Event) => {
      event.preventDefault()
      close.current()
    }
    let pressedOnBackdrop = false
    const onPointerDown = (event: PointerEvent) => {
      pressedOnBackdrop = event.target === dialog
    }
    const onBackdrop = (event: MouseEvent) => {
      if (event.target === dialog && pressedOnBackdrop) {
        close.current()
      }
      pressedOnBackdrop = false
    }
    dialog.addEventListener('cancel', onCancel)
    dialog.addEventListener('pointerdown', onPointerDown)
    dialog.addEventListener('click', onBackdrop)
    return () => {
      dialog.removeEventListener('cancel', onCancel)
      dialog.removeEventListener('pointerdown', onPointerDown)
      dialog.removeEventListener('click', onBackdrop)
      dialog.close()
      if (opener instanceof HTMLElement && opener.isConnected) {
        opener.focus()
      }
    }
  }, [])

  const content = (
    <>
      <div className={styles.body}>{children}</div>
      {footer && <footer className={styles.footer}>{footer}</footer>}
    </>
  )
  return (
    <dialog
      ref={ref}
      role={role}
      tabIndex={-1}
      aria-labelledby={titleId}
      aria-describedby={describedBy}
      className={`${styles.dialog} ${styles[size]}`}
    >
      <header className={styles.header}>
        <h2 id={titleId} className={styles.title}>
          {title}
        </h2>
        <IconButton icon="close" label={tk('common:components.close')} onClick={onClose} />
      </header>
      <div ref={contentRef} className={styles.content}>
        {onSubmit ? (
          <form
            className={styles.content}
            noValidate
            onSubmit={(event) => {
              event.preventDefault()
              onSubmit(event)
            }}
          >
            {content}
          </form>
        ) : (
          content
        )}
      </div>
    </dialog>
  )
}

type ConfirmProps = {
  open: boolean
  title: string
  children: ReactNode
  confirmLabel: string
  onConfirm: () => void
  onCancel: () => void
  busy?: boolean
  danger?: boolean
  /** Fraza do przepisania przed potwierdzeniem (np. nazwa gospodarstwa). */
  requirePhrase?: string
  error?: ReactNode
}

/** Pytanie przed nieodwracalną operacją. */
export function ConfirmDialog({
  open,
  title,
  children,
  confirmLabel,
  onConfirm,
  onCancel,
  busy,
  danger = true,
  requirePhrase,
  error,
}: ConfirmProps) {
  return (
    <Dialog open={open} title={title} onClose={onCancel} role="alertdialog" size="sm">
      <ConfirmBody
        confirmLabel={confirmLabel}
        onConfirm={onConfirm}
        onCancel={onCancel}
        busy={busy}
        danger={danger}
        requirePhrase={requirePhrase}
        error={error}
      >
        {children}
      </ConfirmBody>
    </Dialog>
  )
}

function ConfirmBody({
  children,
  confirmLabel,
  onConfirm,
  onCancel,
  busy,
  danger,
  requirePhrase,
  error,
}: Omit<ConfirmProps, 'open' | 'title'>) {
  const [typed, setTyped] = useState('')
  const phraseId = useId()
  const ready = !requirePhrase || typed.trim() === requirePhrase
  return (
    <form
      className={styles.confirm}
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        if (ready) {
          onConfirm()
        }
      }}
    >
      <div className={styles.confirmText}>{children}</div>
      {requirePhrase && (
        <div className={styles.phrase}>
          <label htmlFor={phraseId}>
            {tk('common:components.typeToConfirmBefore')}
            <strong>{requirePhrase}</strong>
            {tk('common:components.typeToConfirmAfter')}
          </label>
          <input
            id={phraseId}
            className={inputClass}
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
            autoComplete="off"
            data-autofocus
          />
        </div>
      )}
      {error}
      <footer className={styles.footer}>
        <Button onClick={onCancel} data-autofocus={requirePhrase ? undefined : true}>
          Anuluj
        </Button>
        <Button type="submit" variant={danger ? 'danger' : 'primary'} busy={busy} disabled={!ready}>
          {confirmLabel}
        </Button>
      </footer>
    </form>
  )
}
