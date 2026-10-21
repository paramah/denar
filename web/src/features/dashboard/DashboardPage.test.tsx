import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

describe('DashboardPage', () => {
  it('invites to add the first wallet', async () => {
    renderApp('/h/h-dom')
    expect(await screen.findByRole('heading', { name: 'Dodaj pierwszy portfel' })).toBeInTheDocument()
    expect(plain(screen.getByRole('region', { name: 'Sumy portfeli' }).textContent)).toContain('0,00 zł')
  })

  it('shows on-budget totals, wallets and recent transactions', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () =>
        HttpResponse.json({
          items: [
            f.wallet(),
            f.wallet({
              id: 'w-usd',
              name: 'Dolary',
              currency: 'USD',
              balance: f.money('100.00', 'USD'),
              cleared_balance: f.money('100.00', 'USD'),
              base_balance: f.money('400.00'),
              rate_pending: true,
            }),
            f.wallet({ id: 'w-lok', name: 'Lokata', on_budget: false, base_balance: f.money('50.00') }),
          ],
        }),
      ),
      http.get(apiUrl('/households/:hid/transactions'), () =>
        HttpResponse.json({ items: [f.transaction({ payee_name: 'Piekarnia' })], next_cursor: null }),
      ),
    )
    renderApp('/h/h-dom')
    const totals = await screen.findByRole('region', { name: 'Sumy portfeli' })
    expect(plain(totals.textContent)).toContain('Na portfelach budżetowych1400,00 zł')
    expect(totals).toHaveTextContent('Część kwot czeka na kurs')
    expect(screen.getByRole('link', { name: /Dolary/ })).toHaveAttribute('href', '/h/h-dom/wallets/w-usd')
    const recent = await screen.findByRole('list', { name: 'Ostatnie transakcje' })
    expect(within(recent).getByText('Piekarnia')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dodaj transakcję' })).toBeInTheDocument()
  })
})
