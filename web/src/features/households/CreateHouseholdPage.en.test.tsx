import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

describe('CreateHouseholdPage in English', () => {
  it('shows the form and validates the name', async () => {
    setLocale('en')
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([]))))
    const { user } = renderApp('/households/new')
    expect(await screen.findByRole('heading', { level: 1, name: 'New household' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Name' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Base currency' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Time zone' })).toBeInTheDocument()
    const monthStart = screen.getByRole('combobox', { name: 'Budget month starts on' })
    expect(monthStart).toHaveDisplayValue('day 1')
    await user.click(screen.getByRole('button', { name: 'Create household' }))
    expect(await screen.findByText('Enter the household name.')).toBeInTheDocument()
    expect(document.title).toBe('New household · Denar')
  })
})
