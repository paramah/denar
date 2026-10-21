import { screen, waitFor } from '@testing-library/react'
import { http } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { readStored, storageKeys } from '../../lib/storage.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

describe('language switcher on the login page', () => {
  beforeEach(() => {
    server.use(http.get(apiUrl('/me'), () => problemResponse(401, 'unauthenticated')))
  })

  it('switches to English, saves the choice in the browser and keeps focus', async () => {
    const { user } = renderApp('/login')
    expect(await screen.findByRole('heading', { name: 'Zaloguj się' })).toBeInTheDocument()
    const polish = screen.getByRole('button', { name: 'Polski' })
    expect(polish).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: 'English' }))

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Language' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'E-mail' })).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByText('Household budget, wallets and subscriptions — on your own server.')).toBeInTheDocument()
    const english = screen.getByRole('button', { name: 'English' })
    expect(english).toHaveAttribute('aria-pressed', 'true')
    expect(english).toHaveAttribute('lang', 'en')
    await waitFor(() => expect(english).toHaveFocus())
    expect(document.documentElement.lang).toBe('en')
    expect(document.title).toBe('Sign in · Denar')
    expect(readStored(storageKeys.locale)).toBe('en')

    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(screen.getByRole('textbox', { name: 'E-mail' })).toHaveAccessibleDescription('Enter your e-mail address.')

    await user.click(screen.getByRole('button', { name: 'Polski' }))
    expect(await screen.findByRole('heading', { name: 'Zaloguj się' })).toBeInTheDocument()
    expect(readStored(storageKeys.locale)).toBe('pl')
  })
})
