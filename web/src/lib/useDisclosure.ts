import { useCallback, useEffect, useState, type RefObject } from 'react'

/**
 * Stan rozwijanego panelu (menu, przełącznik): zamyka się Escape'em
 * (z powrotem focusu na przycisk) i kliknięciem poza kontenerem.
 */
export function useDisclosure(containerRef: RefObject<HTMLElement | null>, buttonRef: RefObject<HTMLElement | null>) {
  const [open, setOpen] = useState(false)

  const close = useCallback((returnFocus = false) => {
    setOpen(false)
    if (returnFocus) {
      buttonRef.current?.focus()
    }
  }, [buttonRef])

  useEffect(() => {
    if (!open) {
      return undefined
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        close(true)
      }
    }
    const onPointer = (event: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        close()
      }
    }
    const onFocus = (event: FocusEvent) => {
      if (containerRef.current && event.target instanceof Node && !containerRef.current.contains(event.target)) {
        close()
      }
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointer)
    document.addEventListener('focusin', onFocus)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointer)
      document.removeEventListener('focusin', onFocus)
    }
  }, [open, close, containerRef])

  return {
    open,
    setOpen,
    toggle: () => setOpen((value) => !value),
    close,
  }
}
