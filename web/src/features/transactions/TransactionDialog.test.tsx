import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import type { Transaction, TransactionCreate, TransactionUpdate } from '../../api/types.ts'
import { features } from '../../lib/features.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const konto = f.wallet()
const dolary = f.wallet({ id: 'w-usd', name: 'Dolary', currency: 'USD', balance: f.money('10.00', 'USD'), cleared_balance: f.money('10.00', 'USD'), base_balance: f.money('40.00') })

describe('TransactionDialog', () => {
  let created: TransactionCreate[]
  let updated: TransactionUpdate | undefined
  let transactions: Transaction[]

  beforeEach(() => {
    created = []
    updated = undefined
    transactions = []
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto, dolary] })),
      http.get(apiUrl('/households/:hid/payees'), () => HttpResponse.json({ items: [f.payee()] })),
      http.get(apiUrl('/households/:hid/transactions'), () => HttpResponse.json({ items: transactions, next_cursor: null })),
      http.post(apiUrl('/households/:hid/transactions'), async ({ request }) => {
        const body = (await request.json()) as TransactionCreate
        created.push(body)
        if (body.note === 'archiwum') {
          return problemResponse(409, 'conflict', { reason: 'wallet-archived' })
        }
        if (body.note === 'zły') {
          return problemResponse(422, 'validation', { errors: [{ field: 'amount.amount', message: 'kwota poza zakresem' }] })
        }
        return HttpResponse.json(
          f.transaction({ id: `t-${created.length}`, amount: f.money(body.amount.amount, body.amount.currency) }),
          { status: 201 },
        )
      }),
      http.patch(apiUrl('/households/:hid/transactions/:tid'), async ({ request }) => {
        updated = (await request.json()) as TransactionUpdate
        return HttpResponse.json(transactions[0])
      }),
    )
  })

  afterEach(() => {
    features.envelopes = true
  })

  async function open() {
    const view = renderApp('/h/h-dom/transactions')
    await view.user.click(await screen.findByRole('button', { name: 'Dodaj transakcję' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa transakcja' }))
    await dialog.findByRole('option', { name: 'Konto (PLN)' })
    return { ...view, dialog }
  }

  it('records an expense with a new payee, keyboard first', async () => {
    const { user, dialog } = await open()
    const amount = dialog.getByRole('textbox', { name: 'Kwota' })
    expect(amount).toHaveFocus()
    await user.keyboard('12,5')
    await user.type(dialog.getByRole('combobox', { name: 'Kontrahent' }), 'Piekarnia')
    expect(dialog.getByText('Nowy kontrahent — zostanie dodany do listy.')).toBeInTheDocument()
    await user.type(dialog.getByRole('textbox', { name: 'Tagi' }), 'chleb, Dom ')
    await user.keyboard('{Enter}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(created[0]).toMatchObject({
      wallet_id: 'w-konto',
      amount: { amount: '-12.50', currency: 'PLN' },
      kind: 'expense',
      status: 'cleared',
      payee_name: 'Piekarnia',
      tags: ['chleb', 'Dom'],
      note: '',
    })
    expect(created[0]).not.toHaveProperty('payee_id')
    expect(created[0]?.date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(plain(await screen.findByText(/Zapisano: wydatek/).then((el) => el.textContent))).toBe('Zapisano: wydatek −12,50 zł.')
  })

  it('uses a known payee by id, signs refunds and income as inflow, and keeps the form for the next one', async () => {
    const { user, dialog } = await open()
    await user.click(dialog.getByRole('checkbox', { name: 'Dodaj kolejną' }))

    await user.click(dialog.getByRole('radio', { name: 'Zwrot' }))
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '5')
    await user.type(dialog.getByRole('combobox', { name: 'Kontrahent' }), 'sklep')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(created).toHaveLength(1))
    expect(created[0]).toMatchObject({ amount: { amount: '5.00' }, kind: 'expense', payee_id: 'p-sklep' })
    expect(created[0]).not.toHaveProperty('payee_name')

    // Formularz zostaje, kwota i kontrahent znikają, fokus wraca na kwotę.
    await waitFor(() => expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveValue(''))
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveFocus()
    expect(dialog.getByRole('combobox', { name: 'Kontrahent' })).toHaveValue('')

    await user.click(dialog.getByRole('radio', { name: 'Wpływ' }))
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Portfel' }), 'w-usd')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '1200')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Status' }), 'pending')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(created).toHaveLength(2))
    expect(created[1]).toMatchObject({
      wallet_id: 'w-usd',
      amount: { amount: '1200.00', currency: 'USD' },
      kind: 'income',
      status: 'pending',
    })
  })

  it('validates the amount before calling the API', async () => {
    const { user, dialog } = await open()
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveAccessibleDescription('Podaj kwotę.')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '1,234')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveAccessibleDescription('Najwyżej 2 miejsca po przecinku.')
    expect(created).toEqual([])
  })

  it('shows problem+json field errors at the field and a 409 above the form', async () => {
    const { user, dialog } = await open()
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '10')
    await user.type(dialog.getByRole('textbox', { name: 'Notatka' }), 'zły')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(await dialog.findByText('kwota poza zakresem')).toBeInTheDocument()
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveAccessibleDescription('kwota poza zakresem')

    await user.clear(dialog.getByRole('textbox', { name: 'Notatka' }))
    await user.type(dialog.getByRole('textbox', { name: 'Notatka' }), 'archiwum')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(await dialog.findByText(/Portfel jest zarchiwizowany/)).toBeInTheDocument()
  })

  it('edits only date, status, tags and note of a transfer leg', async () => {
    transactions = [
      f.transaction({
        id: 't-leg',
        kind: 'transfer',
        payee_id: null,
        payee_name: null,
        amount: f.money('-400.00'),
        transfer_pair_id: 'pair-1',
        transfer_wallet_id: 'w-usd',
      }),
    ]
    const { user } = renderApp('/h/h-dom/transactions')
    await user.click(await screen.findByRole('button', { name: 'Edytuj: Przelew do: Dolary' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja przelewu' }))
    expect(dialog.queryByRole('textbox', { name: 'Kwota' })).not.toBeInTheDocument()
    expect(dialog.queryByRole('combobox', { name: 'Kontrahent' })).not.toBeInTheDocument()
    await user.type(dialog.getByRole('textbox', { name: 'Notatka' }), 'oszczędności')
    await user.click(dialog.getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(updated).toBeDefined())
    expect(updated).toEqual({ date: '2026-10-20', status: 'cleared', tags: [], note: 'oszczędności' })
  })

  it('edits an expense and clears its payee', async () => {
    transactions = [f.transaction({ id: 't-edit', amount: f.money('-50.00'), note: 'zakupy' })]
    const { user } = renderApp('/h/h-dom/transactions')
    await user.click(await screen.findByRole('button', { name: 'Edytuj: Sklep' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja transakcji' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveValue('50,00')
    expect(dialog.getByRole('radio', { name: 'Wydatek' })).toBeChecked()
    await user.clear(dialog.getByRole('combobox', { name: 'Kontrahent' }))
    await user.clear(dialog.getByRole('textbox', { name: 'Kwota' }))
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '55')
    await user.click(dialog.getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(updated).toBeDefined())
    expect(updated).toMatchObject({ amount: { amount: '-55.00', currency: 'PLN' }, kind: 'expense', payee_id: null })
    expect(updated).not.toHaveProperty('splits')
    expect(updated).not.toHaveProperty('envelope_id')
  })

  it('hides envelopes and splits when the envelopes flag is off', async () => {
    features.envelopes = false
    const { dialog } = await open()
    expect(dialog.queryByRole('combobox', { name: 'Koperta' })).not.toBeInTheDocument()
    expect(dialog.queryByRole('button', { name: 'Podziel na koperty' })).not.toBeInTheDocument()
  })

  it('splits an expense into lines whose sum must match (flag on)', async () => {
    features.envelopes = true
    const { user, dialog } = await open()
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '100')
    expect(dialog.getByRole('combobox', { name: 'Koperta' })).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: 'Podziel na koperty' }))
    const first = dialog.getByRole('textbox', { name: 'Kwota, linia 1' })
    expect(first).toHaveValue('100')
    await user.clear(first)
    await user.type(first, '60')
    const total = dialog.getByText(/Rozdzielono/).closest('p')!
    expect(plain(total.textContent)).toContain('Pozostało do rozdzielenia: 40,00 zł')
    expect(dialog.getByRole('button', { name: 'Zapisz' })).toBeDisabled()

    await user.type(dialog.getByRole('textbox', { name: 'Kwota, linia 2' }), '50')
    expect(plain(total.textContent)).toContain('Za dużo o 10,00 zł')
    await user.clear(dialog.getByRole('textbox', { name: 'Kwota, linia 2' }))
    await user.type(dialog.getByRole('textbox', { name: 'Kwota, linia 2' }), '40')
    await user.type(dialog.getByRole('textbox', { name: 'Notatka, linia 2' }), 'chemia')
    expect(plain(total.textContent)).toContain('Suma się zgadza.')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))

    await waitFor(() => expect(created).toHaveLength(1))
    expect(created[0]?.amount).toEqual({ amount: '-100.00', currency: 'PLN' })
    expect(created[0]?.splits).toEqual([
      { amount: { amount: '-60.00', currency: 'PLN' } },
      { amount: { amount: '-40.00', currency: 'PLN' }, note: 'chemia' },
    ])
    expect(created[0]).not.toHaveProperty('envelope_id')
  })

  it('adds a remaining line and removes lines (flag on)', async () => {
    features.envelopes = true
    const { user, dialog } = await open()
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '30')
    await user.click(dialog.getByRole('button', { name: 'Podziel na koperty' }))
    await user.clear(dialog.getByRole('textbox', { name: 'Kwota, linia 1' }))
    await user.type(dialog.getByRole('textbox', { name: 'Kwota, linia 1' }), '10')
    await user.click(dialog.getByRole('button', { name: 'Usuń linię 2' }))
    await user.click(dialog.getByRole('button', { name: 'Dodaj linię' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota, linia 2' })).toHaveValue('20,00')
    await user.click(dialog.getByRole('button', { name: 'Usuń linię 2' }))
    await user.click(dialog.getByRole('button', { name: 'Usuń linię 1' }))
    // Bez linii — wraca zwykła koperta.
    expect(dialog.getByRole('combobox', { name: 'Koperta' })).toBeInTheDocument()
  })
})
