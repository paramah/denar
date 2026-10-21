import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { storageKeys } from '../../lib/storage.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const preview = {
  household_name: 'Dom',
  email: 'jan@example.test',
  role: 'member',
  expires_at: '2026-10-28T10:00:00Z',
  account_exists: false,
}

function withToken(token: string) {
  window.history.replaceState(null, '', `/invite#${token}`)
}

describe('InvitePage', () => {
  let signedIn = false

  beforeEach(() => {
    signedIn = false
    server.use(
      http.get(apiUrl('/me'), () => (signedIn ? HttpResponse.json(f.me()) : problemResponse(401, 'unauthenticated'))),
      http.post(apiUrl('/invitations/preview'), async ({ request }) => {
        const { token } = (await request.json()) as { token: string }
        return token === 'dobry' ? HttpResponse.json(preview) : problemResponse(404, 'not_found')
      }),
    )
  })

  it('registers a new account and joins the household', async () => {
    withToken('dobry')
    let sent: unknown
    server.use(
      http.post(apiUrl('/invitations/register'), async ({ request }) => {
        sent = await request.json()
        signedIn = true
        return HttpResponse.json({ user: f.user, household: f.household({ role: 'member' }) }, { status: 201 })
      }),
    )
    const { user, router } = renderApp('/invite')
    expect(await screen.findByRole('heading', { name: 'Dołącz do gospodarstwa Dom' })).toBeInTheDocument()
    // Token leaves the address bar and waits in sessionStorage.
    expect(window.location.hash).toBe('')
    expect(window.sessionStorage.getItem(storageKeys.invite)).toBe('dobry')

    await user.type(screen.getByLabelText('Imię lub nazwa'), 'Jan')
    await user.type(screen.getByLabelText('Hasło'), 'krótkie')
    await user.type(screen.getByLabelText('Powtórz hasło'), 'krótkie')
    await user.click(screen.getByRole('button', { name: 'Załóż konto i dołącz' }))
    expect(screen.getByLabelText('Hasło')).toHaveAccessibleDescription(/co najmniej 12 znaków/)

    await user.clear(screen.getByLabelText('Hasło'))
    await user.type(screen.getByLabelText('Hasło'), 'bardzo-dlugie-haslo')
    await user.clear(screen.getByLabelText('Powtórz hasło'))
    await user.type(screen.getByLabelText('Powtórz hasło'), 'bardzo-dlugie-haslo')
    await user.click(screen.getByRole('button', { name: 'Załóż konto i dołącz' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom'))
    expect(sent).toEqual({ token: 'dobry', name: 'Jan', password: 'bardzo-dlugie-haslo' })
    expect(window.sessionStorage.getItem(storageKeys.invite)).toBeNull()
  })

  it('lets a signed-in user accept with one button', async () => {
    withToken('dobry')
    signedIn = true
    server.use(http.post(apiUrl('/invitations/accept'), () => HttpResponse.json(f.household({ role: 'member' }))))
    const { user, router } = renderApp('/invite')
    await user.click(await screen.findByRole('button', { name: 'Przyjmij zaproszenie' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom'))
  })

  it('sends an existing account to log in and back to the invitation', async () => {
    withToken('dobry')
    server.use(http.post(apiUrl('/invitations/preview'), () => HttpResponse.json({ ...preview, account_exists: true })))
    renderApp('/invite')
    expect(await screen.findByRole('link', { name: 'Zaloguj się, aby dołączyć' })).toHaveAttribute(
      'href',
      '/login?return_to=%2Finvite',
    )
  })

  it.each([
    [problemResponse(404, 'not_found'), 'Zaproszenie nie jest aktywne'],
    [problemResponse(409, 'conflict', { reason: 'invitation-expired' }), 'Zaproszenie wygasło'],
    [problemResponse(409, 'conflict', { reason: 'invitation-used' }), 'Zaproszenie zostało już użyte'],
  ])('explains an invitation that cannot be used (%#)', async (response, heading) => {
    withToken('inny')
    server.use(http.post(apiUrl('/invitations/preview'), () => response.clone()))
    renderApp('/invite')
    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
  })

  it('asks for the full link when there is no token', async () => {
    window.history.replaceState(null, '', '/invite')
    renderApp('/invite')
    expect(await screen.findByRole('heading', { name: 'Brak zaproszenia w linku' })).toBeInTheDocument()
  })
})
