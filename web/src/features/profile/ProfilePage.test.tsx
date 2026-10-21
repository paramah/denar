import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Me, User } from '../../api/types.ts'
import { currentLocale } from '../../i18n/locale.ts'
import { readStored, storageKeys } from '../../lib/storage.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

/** /me z językiem zapisywanym przez PATCH /me (jak serwer). */
function profileApi(initial: User['locale'] = null) {
  let locale = initial
  const patches: unknown[] = []
  server.use(
    http.get(apiUrl('/me'), () => HttpResponse.json<Me>({ ...f.me(), user: { ...f.user, locale } })),
    http.patch(apiUrl('/me'), async ({ request }) => {
      const body = (await request.json()) as { locale: User['locale'] }
      patches.push(body)
      locale = body.locale ?? null
      return HttpResponse.json<User>({ ...f.user, locale })
    }),
  )
  return patches
}

describe('ProfilePage', () => {
  it('shows the account and the language choice', async () => {
    profileApi()
    renderApp('/profile')
    expect(await screen.findByRole('heading', { level: 1, name: 'Profil' })).toBeInTheDocument()
    expect(screen.getByText('ada@example.test')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Język interfejsu i powiadomień' })).toHaveValue('auto')
    expect(document.title).toBe('Profil · Denar')
  })

  it('switches to English without a reload, saves it and keeps focus', async () => {
    const patches = profileApi()
    const { user } = renderApp('/profile')
    const select = await screen.findByRole('combobox', { name: 'Język interfejsu i powiadomień' })
    await user.selectOptions(select, 'en')

    expect(await screen.findByRole('heading', { level: 1, name: 'Profile' })).toBeInTheDocument()
    expect(patches).toEqual([{ locale: 'en' }])
    expect(document.documentElement.lang).toBe('en')
    expect(currentLocale()).toBe('en')
    expect(readStored(storageKeys.locale)).toBe('en')
    const english = screen.getByRole('combobox', { name: 'Language of the interface and notifications' })
    expect(english).toHaveValue('en')
    await waitFor(() => expect(english).toHaveFocus())
    expect(await screen.findByText('Language saved.')).toBeInTheDocument()
    // Powłoka też po angielsku.
    await user.click(screen.getByRole('button', { name: /account menu/ }))
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Profile' })).toHaveAttribute('href', '/profile')
  })

  it('goes back to Polish and to automatic', async () => {
    const patches = profileApi('en')
    const { user } = renderApp('/profile')
    const select = await screen.findByRole('combobox', { name: 'Language of the interface and notifications' })
    await user.selectOptions(select, 'pl')
    expect(await screen.findByRole('heading', { level: 1, name: 'Profil' })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('pl')

    await user.selectOptions(screen.getByRole('combobox', { name: 'Język interfejsu i powiadomień' }), 'auto')
    await waitFor(() => expect(patches).toEqual([{ locale: 'pl' }, { locale: null }]))
    // Przeglądarka testów mówi po polsku; zapisany wybór znika.
    await waitFor(() => expect(readStored(storageKeys.locale)).toBeNull())
    expect(currentLocale()).toBe('pl')
  })

  it('applies the profile language after /me loads', async () => {
    profileApi('en')
    renderApp('/profile')
    expect(await screen.findByRole('heading', { level: 1, name: 'Profile' })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('en')
  })

  it('keeps the language when saving fails', async () => {
    profileApi()
    server.use(http.patch(apiUrl('/me'), () => problemResponse(500, 'internal')))
    const { user } = renderApp('/profile')
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Język interfejsu i powiadomień' }), 'en')
    expect(await screen.findByText('Serwer nie mógł wykonać operacji. Spróbuj ponownie za chwilę.')).toBeInTheDocument()
    expect(currentLocale()).toBe('pl')
  })

  it('is linked from the user menu', async () => {
    profileApi()
    const { user, router } = renderApp('/h/h-dom')
    const shell = await screen.findByRole('complementary', { name: 'Menu aplikacji' })
    await user.click(within(shell).getByRole('button', { name: /menu konta/ }))
    await user.click(within(shell).getByRole('link', { name: 'Profil' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/profile'))
  })
})
