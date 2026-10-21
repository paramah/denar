import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { readStored, storageKeys, writeStored } from '../lib/storage.ts'

export type ThemeChoice = 'system' | 'light' | 'dark'

type ThemeApi = { theme: ThemeChoice; setTheme: (theme: ThemeChoice) => void }

const ThemeContext = createContext<ThemeApi | null>(null)

function stored(): ThemeChoice {
  const value = readStored(storageKeys.theme)
  return value === 'light' || value === 'dark' ? value : 'system'
}

/** Motyw: systemowy albo wybrany; public/theme-init.js ustawia go przed pierwszym malowaniem. */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeChoice>(stored)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') {
      root.removeAttribute('data-theme')
    } else {
      root.setAttribute('data-theme', theme)
    }
  }, [theme])

  const setTheme = useCallback((next: ThemeChoice) => {
    writeStored(storageKeys.theme, next === 'system' ? null : next)
    setThemeState(next)
  }, [])

  const api = useMemo(() => ({ theme, setTheme }), [theme, setTheme])
  return <ThemeContext.Provider value={api}>{children}</ThemeContext.Provider>
}

export function useTheme(): ThemeApi {
  const api = useContext(ThemeContext)
  if (!api) {
    throw new Error('useTheme needs a ThemeProvider above it')
  }
  return api
}
