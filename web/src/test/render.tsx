import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router'
import { createQueryClient } from '../api/queryClient.ts'
import { routes } from '../app/router.tsx'
import { Providers } from '../app/providers.tsx'

/** Klient zapytań aplikacji bez czekania między ponowieniami. */
export function createTestQueryClient() {
  const client = createQueryClient()
  const defaults = client.getDefaultOptions()
  client.setDefaultOptions({ ...defaults, queries: { ...defaults.queries, retryDelay: 0, retry: false } })
  return client
}

function renderRouter(routeObjects: RouteObject[], route: string) {
  const queryClient = createTestQueryClient()
  const router = createMemoryRouter(routeObjects, { initialEntries: [route] })
  const user = userEvent.setup()
  const view = render(
    <Providers queryClient={queryClient}>
      <RouterProvider router={router} />
    </Providers>,
  )
  return { ...view, queryClient, router, user }
}

/** Renderuje całą aplikację (wszystkie trasy) od podanego adresu. Żądania idą do MSW. */
export function renderApp(route = '/') {
  return renderRouter(routes, route)
}

/** Renderuje pojedynczy element w routerze i z providerami. */
export function renderWithProviders(ui: ReactElement, { route = '/' }: { route?: string } = {}) {
  return renderRouter([{ path: '*', element: ui }], route)
}
