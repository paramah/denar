import { createContext, use, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
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
  const [theme, setTheme] = useState<ThemeChoice>(stored)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') {
      root.removeAttribute('data-theme')
    } else {
      root.setAttribute('data-theme', theme)
    }
  }, [theme])

  const chooseTheme = useCallback((next: ThemeChoice) => {
    writeStored(storageKeys.theme, next === 'system' ? null : next)
    setTheme(next)
  }, [])

  const api = useMemo(() => ({ theme, setTheme: chooseTheme }), [theme, chooseTheme])
  return <ThemeContext value={api}>{children}</ThemeContext>
}

export function useTheme(): ThemeApi {
  const api = use(ThemeContext)
  if (!api) {
    throw new Error('useTheme needs a ThemeProvider above it')
  }
  return api
}
