import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { IconButton } from '../Button/Button.tsx'
import { Icon } from '../Icon/Icon.tsx'
import styles from './Toast.module.css'

type ToastInput = {
  message: string
  /** Błąd zostaje do zamknięcia; reszta znika sama. */
  kind?: 'success' | 'error'
}

type Toast = ToastInput & { id: number }

type ToastApi = { show: (toast: ToastInput) => void }

const ToastContext = createContext<ToastApi | null>(null)

const visibleFor = 5000

/**
 * Krótkie komunikaty o tym, co się właśnie stało. Region jest na stronie od
 * początku (role=status), więc czytniki ogłaszają dodane komunikaty.
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const nextId = useRef(1)

  const dismiss = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id))
  }, [])

  const show = useCallback((input: ToastInput) => {
    setToasts((current) => {
      if (current.some((toast) => toast.message === input.message && toast.kind === input.kind)) {
        return current
      }
      return [...current, { ...input, id: nextId.current++ }]
    })
  }, [])

  const api = useMemo(() => ({ show }), [show])
  return (
    <ToastContext.Provider value={api}>
      {children}
      <div role="status" aria-live="polite" className={styles.region}>
        {toasts.length > 0 && (
          <ul className={styles.list}>
            {toasts.map((toast) => (
              <ToastItem key={toast.id} toast={toast} onDismiss={dismiss} />
            ))}
          </ul>
        )}
      </div>
    </ToastContext.Provider>
  )
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  const [paused, setPaused] = useState(false)
  const isError = toast.kind === 'error'

  useEffect(() => {
    if (isError || paused) {
      return undefined
    }
    const timer = setTimeout(() => onDismiss(toast.id), visibleFor)
    return () => clearTimeout(timer)
  }, [isError, paused, toast.id, onDismiss])

  return (
    <li
      className={styles.toast}
      data-kind={toast.kind ?? 'success'}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <Icon name={isError ? 'alert' : 'check'} className={styles.icon} />
      <span className={styles.message}>{toast.message}</span>
      <IconButton icon="close" label="Zamknij komunikat" onClick={() => onDismiss(toast.id)} />
    </li>
  )
}

export function useToast(): ToastApi {
  const api = useContext(ToastContext)
  if (!api) {
    throw new Error('useToast needs a ToastProvider above it')
  }
  return api
}
