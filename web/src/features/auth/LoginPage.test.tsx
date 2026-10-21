import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

describe('LoginPage', () => {
  let signedIn = false

  beforeEach(() => {
    signedIn = false
    server.use(
      http.get(apiUrl('/me'), () => (signedIn ? HttpResponse.json(f.me()) : problemResponse(401, 'unauthenticated'))),
    )
  })

  it('signs in and returns to return_to', async () => {
    server.use(
      http.post(apiUrl('/auth/login'), async ({ request }) => {
        const body = (await request.json()) as { email: string; password: string }
        if (body.email === 'ada@example.test' && body.password === 'dobre-haslo-123') {
          signedIn = true
          return HttpResponse.json(f.user)
        }
        return problemResponse(401, 'unauthenticated')
      }),
    )
    const { user, router } = renderApp('/login?return_to=%2Fh%2Fh-dom%2Fsettings')
    await user.type(await screen.findByRole('textbox', { name: 'E-mail' }), 'ada@example.test')
    await user.type(screen.getByLabelText('Hasło'), 'dobre-haslo-123')
    await user.click(screen.getByRole('button', { name: 'Zaloguj się' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/settings/general'))
  })

  it('says the credentials are wrong without telling which', async () => {
    server.use(http.post(apiUrl('/auth/login'), () => problemResponse(401, 'unauthenticated')))
    const { user } = renderApp('/login')
    await user.type(await screen.findByRole('textbox', { name: 'E-mail' }), 'ada@example.test')
    await user.type(screen.getByLabelText('Hasło'), 'zle')
    await user.click(screen.getByRole('button', { name: 'Zaloguj się' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Nieprawidłowy e-mail lub hasło.')
  })

  it('tells how long to wait after too many attempts', async () => {
    server.use(
      http.post(apiUrl('/auth/login'), () => problemResponse(429, 'rate_limited', {}, { 'Retry-After': '30' })),
    )
    const { user } = renderApp('/login')
    await user.type(await screen.findByRole('textbox', { name: 'E-mail' }), 'ada@example.test')
    await user.type(screen.getByLabelText('Hasło'), 'cokolwiek')
    await user.click(screen.getByRole('button', { name: 'Zaloguj się' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Spróbuj ponownie za 30 s.')
  })

  it('asks for missing fields before calling the API', async () => {
    const { user } = renderApp('/login')
    await user.click(await screen.findByRole('button', { name: 'Zaloguj się' }))
    expect(screen.getByRole('textbox', { name: 'E-mail' })).toHaveAccessibleDescription('Podaj adres e-mail.')
    expect(screen.getByLabelText('Hasło')).toHaveAttribute('aria-invalid', 'true')
  })

  it('offers SSO when OIDC is enabled', async () => {
    server.use(
      http.get(apiUrl('/auth/config'), () =>
        HttpResponse.json({ ...f.authConfig, oidc_enabled: true, local_login: false }),
      ),
    )
    renderApp('/login?return_to=%2Finvite')
    const sso = await screen.findByRole('link', { name: 'Zaloguj się przez SSO' })
    expect(sso).toHaveAttribute('href', '/api/auth/login?return_to=%2Finvite')
    expect(screen.queryByRole('textbox', { name: 'E-mail' })).not.toBeInTheDocument()
  })
})
