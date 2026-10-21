import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

describe('CreateHouseholdPage', () => {
  it('creates a household with the chosen currency and opens it', async () => {
    let households = [] as ReturnType<typeof f.household>[]
    let sent: Record<string, unknown> | undefined
    server.use(
      http.get(apiUrl('/me'), () => HttpResponse.json(f.me(households))),
      http.post(apiUrl('/households'), async ({ request }) => {
        sent = (await request.json()) as Record<string, unknown>
        const created = f.household({ id: 'h-new', name: String(sent.name), base_currency: String(sent.base_currency) })
        households = [created]
        return HttpResponse.json(created, { status: 201 })
      }),
    )
    const { user, router } = renderApp('/households/new')
    await user.type(await screen.findByRole('textbox', { name: 'Nazwa' }), 'Mieszkanie')
    const currency = screen.getByRole('combobox', { name: 'Waluta bazowa' })
    await waitFor(() => expect(currency).toBeEnabled())
    await user.selectOptions(currency, 'EUR')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Miesiąc budżetowy zaczyna się' }), '10')
    await user.click(screen.getByRole('button', { name: 'Załóż gospodarstwo' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-new'))
    expect(sent).toMatchObject({ name: 'Mieszkanie', base_currency: 'EUR', month_start_day: 10 })
    expect(typeof sent?.timezone).toBe('string')
    expect(await screen.findByRole('status')).toHaveTextContent('Założono gospodarstwo Mieszkanie.')
  })

  it('shows validation errors of the API at their fields', async () => {
    server.use(
      http.post(apiUrl('/households'), () =>
        problemResponse(422, 'validation', {
          errors: [
            { field: 'timezone', message: 'nieznana strefa' },
            { field: 'name', message: 'nazwa jest za długa' },
          ],
        }),
      ),
    )
    const { user } = renderApp('/households/new')
    await user.type(await screen.findByRole('textbox', { name: 'Nazwa' }), 'X')
    await user.click(screen.getByRole('button', { name: 'Załóż gospodarstwo' }))
    expect(await screen.findByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription(/nazwa jest za długa/)
    expect(screen.getByRole('combobox', { name: 'Strefa czasowa' })).toHaveAccessibleDescription('nieznana strefa')
  })

  it('needs a name before calling the API', async () => {
    const { user } = renderApp('/households/new')
    await user.click(await screen.findByRole('button', { name: 'Załóż gospodarstwo' }))
    expect(screen.getByRole('textbox', { name: 'Nazwa' })).toHaveAttribute('aria-invalid', 'true')
  })
})
