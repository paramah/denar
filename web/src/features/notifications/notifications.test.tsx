import { focusManager } from '@tanstack/react-query'
import { act, screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Notification } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

/** Serwer powiadomień w pamięci: lista, licznik, read i read-all. */
function inbox(initial: Notification[]) {
  let items = [...initial]
  const requests: { url: URL; method: string; body?: unknown }[] = []
  server.use(
    http.get(apiUrl('/me/notifications'), ({ request }) => {
      const url = new URL(request.url)
      requests.push({ url, method: 'GET' })
      const unread = url.searchParams.get('unread') === 'true'
      const household = url.searchParams.get('household_id')
      const limit = Number(url.searchParams.get('limit') ?? 50)
      const cursor = url.searchParams.get('cursor')
      let list = items.filter((n) => (!unread || n.read_at === null) && (!household || n.household_id === household))
      const start = cursor ? list.findIndex((n) => n.id === cursor) + 1 : 0
      list = list.slice(start)
      const page = list.slice(0, limit)
      return HttpResponse.json({ items: page, next_cursor: list.length > limit ? page.at(-1)!.id : null })
    }),
    http.get(apiUrl('/me/notifications/unread-count'), () =>
      HttpResponse.json({ count: items.filter((n) => n.read_at === null).length }),
    ),
    http.post(apiUrl('/me/notifications/:id/read'), ({ params, request }) => {
      requests.push({ url: new URL(request.url), method: 'POST' })
      items = items.map((n) => (n.id === params.id ? { ...n, read_at: n.read_at ?? '2026-10-21T12:00:00Z' } : n))
      return HttpResponse.json(items.find((n) => n.id === params.id)!)
    }),
    http.post(apiUrl('/me/notifications/read-all'), async ({ request }) => {
      const body = await request.json()
      requests.push({ url: new URL(request.url), method: 'POST', body })
      const before = items.filter((n) => n.read_at === null).length
      items = items.map((n) => ({ ...n, read_at: n.read_at ?? '2026-10-21T12:00:00Z' }))
      return HttpResponse.json({ updated: before })
    }),
  )
  return requests
}

const renewal = f.notification({ id: 'n-3', title: 'Odnowienie: Netflix za 3 dni', link: '/h/h-dom/budget/2026-10' })
const overspent = f.notification({
  id: 'n-2',
  type: 'envelope.overspent',
  title: 'Przekroczona koperta: Jedzenie',
  body: 'Koperta Jedzenie jest na minusie w miesiącu 2026-10: -150.00 PLN.',
  link: '/h/h-dom/budget/2026-10',
  resource_type: 'envelope',
  resource_id: 'e-1',
  created_at: '2026-10-20T08:00:00Z',
})
const read = f.notification({ id: 'n-1', type: 'subscription.price_changed', title: 'Zmiana ceny: Spotify', read_at: '2026-10-19T10:00:00Z', created_at: '2026-10-19T08:00:00Z' })

const bellButton = async () => (await screen.findAllByRole('button', { name: /^Powiadomienia, / }))[0]!

describe('NotificationBell', () => {
  it('shows the unread count and the latest notifications; a click opens the link and marks it read', async () => {
    const requests = inbox([renewal, overspent, read])
    const { user, router } = renderApp('/h/h-dom')
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, 2 nieprzeczytane'))
    await user.click(await bellButton())
    const panel = await screen.findByRole('region', { name: 'Powiadomienia' })
    const list = await within(panel).findByRole('list', { name: 'Ostatnie powiadomienia' })
    expect(within(list).getAllByRole('article').map((a) => a.getAttribute('aria-label'))).toEqual([
      'Odnowienie: Netflix za 3 dni',
      'Przekroczona koperta: Jedzenie',
      'Zmiana ceny: Spotify',
    ])
    await user.click(within(panel).getByRole('link', { name: 'Nieprzeczytane: Przekroczona koperta: Jedzenie' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/budget/2026-10'))
    expect(screen.queryByRole('region', { name: 'Powiadomienia' })).not.toBeInTheDocument()
    await waitFor(() => expect(requests.some((r) => r.method === 'POST' && r.url.pathname.endsWith('/n-2/read'))).toBe(true))
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, 1 nieprzeczytane'))
  })

  it('marks everything read', async () => {
    const requests = inbox([renewal, overspent])
    const { user } = renderApp('/h/h-dom')
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, 2 nieprzeczytane'))
    await user.click(await bellButton())
    const panel = await screen.findByRole('region', { name: 'Powiadomienia' })
    await user.click(within(panel).getByRole('button', { name: 'Oznacz wszystkie jako przeczytane' }))
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, brak nieprzeczytanych'))
    expect(requests.find((r) => r.url.pathname.endsWith('/read-all'))?.body).toEqual({})
    expect(within(panel).getByRole('button', { name: 'Oznacz wszystkie jako przeczytane' })).toBeDisabled()
    expect(within(panel).queryByRole('link', { name: /^Nieprzeczytane:/ })).not.toBeInTheDocument()
  })

  it('asks for the count again when the window regains focus', async () => {
    let count = 1
    server.use(http.get(apiUrl('/me/notifications/unread-count'), () => HttpResponse.json({ count })))
    renderApp('/h/h-dom')
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, 1 nieprzeczytane'))
    count = 5
    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })
    await waitFor(async () => expect(await bellButton()).toHaveAccessibleName('Powiadomienia, 5 nieprzeczytanych'))
    focusManager.setFocused(undefined)
  })

  it('closes with Escape and returns the focus', async () => {
    inbox([])
    const { user } = renderApp('/h/h-dom')
    await user.click(await bellButton())
    expect(await screen.findByText(/Nie masz powiadomień/)).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('region', { name: 'Powiadomienia' })).not.toBeInTheDocument()
    expect(await bellButton()).toHaveFocus()
  })
})

describe('NotificationsPage', () => {
  it('groups by day and marks one as read without leaving the page', async () => {
    const requests = inbox([renewal, overspent, read])
    const { user, router } = renderApp('/h/h-dom/notifications')
    const days = await screen.findAllByRole('list', { name: /^Powiadomienia — / })
    expect(days).toHaveLength(3)
    expect(within(days[1]!).getByRole('article', { name: 'Przekroczona koperta: Jedzenie' })).toHaveTextContent(
      'Koperta Jedzenie jest na minusie',
    )
    await user.click(screen.getByRole('button', { name: 'Oznacz jako przeczytane: Przekroczona koperta: Jedzenie' }))
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Oznacz jako przeczytane: Przekroczona koperta: Jedzenie' })).not.toBeInTheDocument(),
    )
    expect(router.state.location.pathname).toBe('/h/h-dom/notifications')
    expect(requests.some((r) => r.url.pathname.endsWith('/n-2/read'))).toBe(true)
  })

  it('filters unread ones and pages with the cursor', async () => {
    const many = Array.from({ length: 35 }, (_, i) =>
      f.notification({ id: `n-${100 - i}`, title: `Odnowienie ${i}`, created_at: '2026-10-21T08:00:00Z' }),
    )
    const requests = inbox([...many, read])
    const { user } = renderApp('/h/h-dom/notifications')
    expect(await screen.findByRole('article', { name: 'Odnowienie 0' })).toBeInTheDocument()
    expect(screen.queryByRole('article', { name: 'Odnowienie 34' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Pokaż starsze' }))
    expect(await screen.findByRole('article', { name: 'Odnowienie 34' })).toBeInTheDocument()
    expect(screen.getByRole('article', { name: 'Zmiana ceny: Spotify' })).toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: 'Tylko nieprzeczytane' }))
    await waitFor(() => expect(screen.queryByRole('article', { name: 'Zmiana ceny: Spotify' })).not.toBeInTheDocument())
    expect(requests.some((r) => r.url.searchParams.get('unread') === 'true')).toBe(true)
    expect(requests.some((r) => r.url.searchParams.get('cursor') === 'n-71')).toBe(true)
  })

  it('shows an empty state that leads to the settings', async () => {
    inbox([read])
    const { user } = renderApp('/h/h-dom/notifications?unread=1')
    expect(await screen.findByRole('heading', { name: 'Wszystko przeczytane' })).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Tylko nieprzeczytane' }))
    expect(await screen.findByRole('article', { name: 'Zmiana ceny: Spotify' })).toBeInTheDocument()
  })

  it('names the household and filters by it when there are several', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household(), f.household({ id: 'h-b', name: 'Działka' })]))))
    const requests = inbox([renewal, f.notification({ id: 'n-9', household_id: 'h-b', title: 'Odnowienie: Prąd' })])
    const { user } = renderApp('/h/h-dom/notifications')
    expect(await screen.findByRole('article', { name: 'Odnowienie: Prąd' })).toHaveTextContent('Działka')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Gospodarstwo' }), 'h-b')
    await waitFor(() => expect(screen.queryByRole('article', { name: 'Odnowienie: Netflix za 3 dni' })).not.toBeInTheDocument())
    expect(requests.some((r) => r.url.searchParams.get('household_id') === 'h-b')).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Oznacz wszystkie jako przeczytane' }))
    await waitFor(() => expect(requests.find((r) => r.url.pathname.endsWith('/read-all'))?.body).toEqual({ household_id: 'h-b' }))
  })
})

describe('dashboard', () => {
  it('shows the latest notifications of the household', async () => {
    inbox([renewal, overspent])
    renderApp('/h/h-dom')
    const tile = await screen.findByRole('list', { name: 'Ostatnie powiadomienia w gospodarstwie' })
    expect(within(tile).getAllByRole('article')).toHaveLength(2)
  })
})
