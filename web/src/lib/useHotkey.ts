import { useEffect, useLayoutEffect, useRef } from 'react'

function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

/**
 * Skrót jednego klawisza (np. „n” — nowa transakcja). Nie działa w polach
 * edycji, z modyfikatorami ani przy otwartym oknie modalnym.
 */
export function useHotkey(key: string, handler: () => void, enabled = true) {
  const latestRef = useRef(handler)
  useLayoutEffect(() => {
    latestRef.current = handler
  })
  useEffect(() => {
    if (!enabled) {
      return undefined
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== key || event.ctrlKey || event.metaKey || event.altKey || event.repeat) {
        return
      }
      if (isEditable(event.target) || document.querySelector('dialog[open]')) {
        return
      }
      event.preventDefault()
      latestRef.current()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [key, enabled])
}
