import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const konto = f.wallet()
const oszczednosci = f.wallet({ id: 'w-osz', name: 'Oszczędności', on_budget: false })

describe('transactions in English', () => {
  beforeEach(() => {
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto, oszczednosci] })),
      http.get(apiUrl('/households/:hid/payees'), () => HttpResponse.json({ items: [f.payee()] })),
      http.get(apiUrl('/households/:hid/transactions'), () =>
        HttpResponse.json({
          items: [
            f.transaction({ id: 't-1', status: 'pending', splits: [] }),
            f.transaction({
              id: 't-2',
              kind: 'transfer',
              payee_id: null,
              payee_name: null,
              transfer_wallet_id: 'w-osz',
              amount: f.money('-100.00'),
            }),
          ],
          next_cursor: null,
        }),
      ),
      http.post(apiUrl('/households/:hid/transactions'), () =>
        HttpResponse.json(f.transaction({ id: 't-new', amount: f.money('-12.50') }), { status: 201 }),
      ),
    )
  })

  it('shows the register and records an expense', async () => {
    setLocale('en')
    const { user } = renderApp('/h/h-dom/transactions')
    expect(await screen.findByRole('heading', { level: 1, name: 'Transactions' })).toBeInTheDocument()
    expect(screen.getByText(/All wallets in one register\. New transaction:/)).toBeInTheDocument()
    expect(screen.getByRole('search', { name: 'Transaction filters' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'More filters' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Edit: Transfer to: Oszczędności' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pending — mark as cleared' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete: Sklep' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Add transaction' }))
    const dialog = within(screen.getByRole('dialog', { name: 'New transaction' }))
    await dialog.findByRole('option', { name: 'Konto (PLN)' })
    expect(dialog.getByRole('radio', { name: 'Refund' })).toBeInTheDocument()
    expect(dialog.getByRole('combobox', { name: 'Payee' })).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: 'Save' }))
    expect(await dialog.findByText('Enter an amount.')).toBeInTheDocument()
    await user.type(dialog.getByRole('textbox', { name: 'Amount' }), '12.5')
    await user.click(dialog.getByRole('button', { name: 'Save' }))
    expect(plain((await screen.findByText(/Saved expense/)).textContent)).toBe('Saved expense −PLN 12.50.')
  })

  it('shows the transfer dialog', async () => {
    setLocale('en')
    const { user } = renderApp('/h/h-dom/transactions')
    await user.click(await screen.findByRole('button', { name: 'Transfer' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Transfer between wallets' }))
    expect(await dialog.findByRole('combobox', { name: 'From wallet' })).toBeInTheDocument()
    expect(dialog.getByRole('combobox', { name: 'To wallet' })).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Transfer' })).toBeInTheDocument()
  })
})
