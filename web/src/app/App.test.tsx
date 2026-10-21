import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { storageKeys } from '../lib/storage.ts'
import * as f from '../test/msw/fixtures.ts'
import { problemResponse } from '../test/msw/handlers.ts'
import { server } from '../test/msw/server.ts'
import { apiUrl } from '../test/msw/url.ts'
import { renderApp } from '../test/render.tsx'
import { handleUnauthenticated } from './App.tsx'
import { switchHouseholdPath } from './nav.ts'

describe('session and households', () => {
  it('sends a visitor without a session to the login page, remembering the address', async () => {
    server.use(http.get(apiUrl('/me'), () => problemResponse(401, 'unauthenticated')))
    const { router } = renderApp('/h/h-dom/settings/bots')
    expect(await screen.findByRole('heading', { name: 'Zaloguj się' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toBe('?return_to=%2Fh%2Fh-dom%2Fsettings%2Fbots')
  })

  it('opens the last household from the start page', async () => {
    window.localStorage.setItem(storageKeys.household, 'h-b')
    server.use(
      http.get(apiUrl('/me'), () =>
        HttpResponse.json(f.me([f.household(), f.household({ id: 'h-b', name: 'Wakacje', base_currency: 'EUR' })])),
      ),
    )
    const { router } = renderApp('/')
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-b'))
    expect(await screen.findByRole('heading', { level: 1, name: /Ada/ })).toBeInTheDocument()
  })

  it('welcomes a user without households and offers to create one', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([]))))
    const { user, router } = renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Witaj, Ada Kowalska' })).toBeInTheDocument()
    await user.click(screen.getByRole('link', { name: 'Załóż gospodarstwo' }))
    expect(router.state.location.pathname).toBe('/households/new')
  })

  it('says so when the household of the address is not one of the user', async () => {
    renderApp('/h/obce')
    expect(await screen.findByRole('heading', { name: 'Nie znaleziono gospodarstwa' })).toBeInTheDocument()
  })

  it('shows the navigation of the household', async () => {
    renderApp('/h/h-dom')
    const nav = (await screen.findAllByRole('navigation', { name: 'Główna' }))[0]!
    for (const label of ['Pulpit', 'Portfele', 'Budżet', 'Subskrypcje', 'Powiadomienia', 'Ustawienia']) {
      expect(within(nav).getByRole('link', { name: label })).toBeInTheDocument()
    }
    expect(within(nav).getByRole('link', { name: 'Pulpit' })).toHaveAttribute('aria-current', 'page')
  })

  it('switches households keeping the section', async () => {
    server.use(
      http.get(apiUrl('/me'), () =>
        HttpResponse.json(f.me([f.household(), f.household({ id: 'h-b', name: 'Wakacje', base_currency: 'EUR' })])),
      ),
    )
    const { user, router } = renderApp('/h/h-dom/wallets')
    const switcher = (await screen.findAllByRole('button', { name: /zmień gospodarstwo/ }))[0]!
    await user.click(switcher)
    await user.click(screen.getByRole('link', { name: /Wakacje/ }))
    expect(router.state.location.pathname).toBe('/h/h-b/wallets')
    expect(switchHouseholdPath('/h/a/settings/bots', 'b')).toBe('/h/b/settings/bots')
  })

  it('logs out from the user menu', async () => {
    const { user, router } = renderApp('/h/h-dom')
    await user.click((await screen.findAllByRole('button', { name: /menu konta/ }))[0]!)
    server.use(http.get(apiUrl('/me'), () => problemResponse(401, 'unauthenticated')))
    await user.click(screen.getByRole('button', { name: 'Wyloguj się' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })

  it('shows coming soon pages for sections of later parts', async () => {
    renderApp('/h/h-dom/notifications')
    expect(await screen.findByRole('heading', { level: 1, name: 'Powiadomienia' })).toBeInTheDocument()
    expect(screen.getByText('Wkrótce')).toBeInTheDocument()
  })
})

describe('handleUnauthenticated', () => {
  it('clears the data and goes to the login page with return_to', () => {
    const navigate = vi.fn()
    const clear = vi.fn()
    handleUnauthenticated(navigate, { pathname: '/h/x/settings', search: '' }, clear)
    expect(clear).toHaveBeenCalled()
    expect(navigate).toHaveBeenCalledWith('/login?return_to=%2Fh%2Fx%2Fsettings')
  })

  it('does nothing on public pages', () => {
    const navigate = vi.fn()
    handleUnauthenticated(navigate, { pathname: '/invite', search: '' }, vi.fn())
    expect(navigate).not.toHaveBeenCalled()
  })
})
