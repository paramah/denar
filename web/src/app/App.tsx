import { useEffect } from 'react'
import { RouterProvider } from 'react-router'
import { publicPaths, setUnauthenticatedHandler } from '../api/client.ts'
import { queryClient } from '../api/queryClient.ts'
import { loginUrl } from '../lib/returnTo.ts'
import { Providers } from './providers.tsx'
import { createAppRouter } from './router.tsx'

const router = createAppRouter()

/**
 * Utrata sesji (401 z dowolnego wywołania): czyści dane i przechodzi na
 * logowanie z powrotem na bieżący adres.
 */
export function handleUnauthenticated(
  navigate: (to: string) => unknown,
  location: { pathname: string; search: string } = window.location,
  clear: () => void = () => queryClient.clear(),
) {
  if (publicPaths.includes(location.pathname)) {
    return
  }
  clear()
  void navigate(loginUrl(location))
}

export function App() {
  useEffect(() => {
    setUnauthenticatedHandler(() => handleUnauthenticated((to) => router.navigate(to, { replace: true })))
    return () => setUnauthenticatedHandler(null)
  }, [])
  return (
    <Providers>
      <RouterProvider router={router} />
    </Providers>
  )
}
