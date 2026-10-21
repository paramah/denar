import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { TransferCreate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const konto = f.wallet()
const gotowka = f.wallet({ id: 'w-cash', name: 'Gotówka', type: 'cash' })
const dolary = f.wallet({
  id: 'w-usd',
  name: 'Dolary',
  currency: 'USD',
  balance: f.money('0.00', 'USD'),
  cleared_balance: f.money('0.00', 'USD'),
  base_balance: f.money('0.00'),
})

describe('TransferDialog', () => {
  let sent: TransferCreate | undefined

  beforeEach(() => {
    sent = undefined
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [dolary, gotowka, konto] })),
      http.get(apiUrl('/households/:hid/wallets/:wid'), () => HttpResponse.json(konto)),
      http.post(apiUrl('/households/:hid/transfers'), async ({ request }) => {
        sent = (await request.json()) as TransferCreate
        if (sent.note === 'serwer') {
          return problemResponse(422, 'validation', { errors: [{ field: 'to_amount.amount', message: 'za mało' }] })
        }
        const from = f.transaction({ id: 't-from', kind: 'transfer', payee_id: null, payee_name: null, amount: f.money(`-${sent.amount.amount}`, sent.amount.currency), transfer_pair_id: 'p', transfer_wallet_id: sent.to_wallet_id })
        const toMoney = sent.to_amount ?? sent.amount
        const to = f.transaction({ id: 't-to', wallet_id: sent.to_wallet_id, kind: 'transfer', payee_id: null, payee_name: null, amount: f.money(toMoney.amount, toMoney.currency), transfer_pair_id: 'p', transfer_wallet_id: sent.from_wallet_id })
        return HttpResponse.json({ from, to }, { status: 201 })
      }),
    )
  })

  it('asks for the received amount between currencies and sends both', async () => {
    const { user } = renderApp('/h/h-dom/wallets/w-konto')
    await user.click(await screen.findByRole('button', { name: 'Przelew' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przelew między portfelami' }))
    await within(await dialog.findByRole('combobox', { name: 'Do portfela' })).findByRole('option', { name: 'Dolary (USD)' })
    expect(dialog.getByRole('combobox', { name: 'Z portfela' })).toHaveValue('w-konto')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do portfela' }), 'w-cash')
    expect(dialog.queryByRole('textbox', { name: /Kwota otrzymana/ })).not.toBeInTheDocument()

    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do portfela' }), 'w-usd')
    expect(dialog.getByRole('textbox', { name: 'Kwota wysłana (PLN)' })).toBeInTheDocument()
    await user.type(dialog.getByRole('textbox', { name: 'Kwota wysłana (PLN)' }), '400')
    await user.click(dialog.getByRole('button', { name: 'Przelej' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota otrzymana (USD)' })).toHaveAccessibleDescription(/Podaj kwotę/)
    expect(sent).toBeUndefined()

    await user.type(dialog.getByRole('textbox', { name: 'Kwota otrzymana (USD)' }), '100,25')
    await user.click(dialog.getByRole('button', { name: 'Przelej' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(sent).toMatchObject({
      from_wallet_id: 'w-konto',
      to_wallet_id: 'w-usd',
      amount: { amount: '400.00', currency: 'PLN' },
      to_amount: { amount: '100.25', currency: 'USD' },
      status: 'cleared',
    })
    expect(await screen.findByText(/Przelano .* z Konto do Dolary\./)).toBeInTheDocument()
  })

  it('sends no target amount between wallets of one currency', async () => {
    const { user } = renderApp('/h/h-dom/wallets/w-konto')
    await user.click(await screen.findByRole('button', { name: 'Przelew' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przelew między portfelami' }))
    await within(await dialog.findByRole('combobox', { name: 'Do portfela' })).findByRole('option', { name: 'Gotówka (PLN)' })
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do portfela' }), 'w-cash')
    // Portfel źródłowy nie występuje wśród docelowych.
    expect(within(dialog.getByRole('combobox', { name: 'Do portfela' })).queryByRole('option', { name: 'Konto (PLN)' })).toBeNull()
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '50')
    await user.click(dialog.getByRole('button', { name: 'Przelej' }))
    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).not.toHaveProperty('to_amount')
    expect(sent?.amount).toEqual({ amount: '50.00', currency: 'PLN' })
  })

  it('shows a server error at the target amount', async () => {
    const { user } = renderApp('/h/h-dom/wallets/w-konto')
    await user.click(await screen.findByRole('button', { name: 'Przelew' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przelew między portfelami' }))
    await within(await dialog.findByRole('combobox', { name: 'Do portfela' })).findByRole('option', { name: 'Dolary (USD)' })
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do portfela' }), 'w-usd')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota wysłana (PLN)' }), '1')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota otrzymana (USD)' }), '1')
    await user.type(dialog.getByRole('textbox', { name: 'Notatka' }), 'serwer')
    await user.click(dialog.getByRole('button', { name: 'Przelej' }))
    expect(await dialog.findByText('za mało')).toBeInTheDocument()
    expect(dialog.getByRole('textbox', { name: 'Kwota otrzymana (USD)' })).toHaveAccessibleDescription(/za mało/)
  })

  it('needs two active wallets', async () => {
    server.use(http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto] })))
    const { user } = renderApp('/h/h-dom/wallets')
    await user.click(await screen.findByRole('button', { name: 'Przelew' }))
    expect(await screen.findByText(/co najmniej dwóch aktywnych portfeli/)).toBeInTheDocument()
  })
})
