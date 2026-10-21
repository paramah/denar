import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { PayeeCreate, PayeeUpdate, TransactionCreate, TransferCreate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { envelopeOptions } from '../transactions/envelopes.ts'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const groups = [f.envelopeGroup(), f.envelopeGroup({ id: 'g-cele', name: 'Cele', position: 1 })]
const envelopes = [
  f.envelope(),
  f.envelope({ id: 'e-wakacje', group_id: 'g-cele', name: 'Wakacje' }),
  f.envelope({ id: 'e-stare', name: 'Stare', position: 1, hidden: true }),
]
const konto = f.wallet()
const lokata = f.wallet({ id: 'w-lokata', name: 'Lokata', type: 'savings', on_budget: false })

describe('envelopes in the forms of part B', () => {
  beforeEach(() => {
    server.use(
      http.get(apiUrl('/households/:hid/budget/envelope-groups'), () => HttpResponse.json({ items: groups })),
      http.get(apiUrl('/households/:hid/budget/envelopes'), ({ request }) => {
        expect(new URL(request.url).searchParams.get('include_hidden')).toBe('true')
        return HttpResponse.json({ items: envelopes })
      }),
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto, lokata] })),
      http.get(apiUrl('/households/:hid/wallets/:wid'), () => HttpResponse.json(konto)),
    )
  })

  it('labels options with the group, in group order, marking hidden ones', () => {
    expect(envelopeOptions([...envelopes].reverse(), groups)).toEqual([
      { value: 'e-stare', label: 'Codzienne · Stare (ukryta)' },
      { value: 'e-jedzenie', label: 'Codzienne · Jedzenie' },
      { value: 'e-wakacje', label: 'Cele · Wakacje' },
    ])
  })

  it('assigns an envelope in quick add and splits across envelopes', async () => {
    const created: TransactionCreate[] = []
    server.use(
      http.post(apiUrl('/households/:hid/transactions'), async ({ request }) => {
        created.push((await request.json()) as TransactionCreate)
        return HttpResponse.json(f.transaction(), { status: 201 })
      }),
    )
    const { user } = renderApp('/h/h-dom/transactions')
    await user.click(await screen.findByRole('button', { name: 'Dodaj transakcję' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa transakcja' }))
    await within(dialog.getByRole('combobox', { name: 'Koperta' })).findByRole('option', { name: 'Codzienne · Jedzenie' })
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '42')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Koperta' }), 'e-jedzenie')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(created).toHaveLength(1))
    expect(created[0]).toMatchObject({ amount: { amount: '-42.00' }, envelope_id: 'e-jedzenie' })
    expect(created[0]).not.toHaveProperty('splits')

    await user.click(await screen.findByRole('button', { name: 'Dodaj transakcję' }))
    const next = within(screen.getByRole('dialog', { name: 'Nowa transakcja' }))
    await user.type(next.getByRole('textbox', { name: 'Kwota' }), '100')
    await user.click(next.getByRole('button', { name: 'Podziel na koperty' }))
    await user.clear(next.getByRole('textbox', { name: 'Kwota, linia 1' }))
    await user.type(next.getByRole('textbox', { name: 'Kwota, linia 1' }), '70')
    await user.selectOptions(next.getByRole('combobox', { name: 'Koperta, linia 1' }), 'e-jedzenie')
    await user.type(next.getByRole('textbox', { name: 'Kwota, linia 2' }), '30')
    await user.selectOptions(next.getByRole('combobox', { name: 'Koperta, linia 2' }), 'e-wakacje')
    await user.click(next.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(created).toHaveLength(2))
    expect(created[1]?.splits).toEqual([
      { amount: { amount: '-70.00', currency: 'PLN' }, envelope_id: 'e-jedzenie' },
      { amount: { amount: '-30.00', currency: 'PLN' }, envelope_id: 'e-wakacje' },
    ])
  })

  it('offers an envelope only for a transfer between on- and off-budget wallets', async () => {
    let sent: TransferCreate | undefined
    server.use(
      http.post(apiUrl('/households/:hid/transfers'), async ({ request }) => {
        sent = (await request.json()) as TransferCreate
        const from = f.transaction({ id: 't-a', kind: 'transfer', payee_id: null, payee_name: null, amount: f.money('-200.00'), transfer_pair_id: 'p', transfer_wallet_id: 'w-lokata', envelope_id: 'e-wakacje' })
        const to = f.transaction({ id: 't-b', wallet_id: 'w-lokata', kind: 'transfer', payee_id: null, payee_name: null, amount: f.money('200.00'), transfer_pair_id: 'p', transfer_wallet_id: 'w-konto' })
        return HttpResponse.json({ from, to }, { status: 201 })
      }),
    )
    const { user } = renderApp('/h/h-dom/wallets/w-konto')
    await user.click(await screen.findByRole('button', { name: 'Przelew' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przelew między portfelami' }))
    const envelope = await dialog.findByRole('combobox', { name: 'Koperta (wypływ z budżetu)' })
    await within(envelope).findByRole('option', { name: 'Cele · Wakacje' })
    await user.selectOptions(envelope, 'e-wakacje')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '200')
    await user.click(dialog.getByRole('button', { name: 'Przelej' }))
    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toMatchObject({ from_wallet_id: 'w-konto', to_wallet_id: 'w-lokata', envelope_id: 'e-wakacje' })
  })

  it('sets and clears a payee default envelope', async () => {
    let createdPayee: PayeeCreate | undefined
    let updatedPayee: PayeeUpdate | undefined
    server.use(
      http.get(apiUrl('/households/:hid/payees'), () =>
        HttpResponse.json({ items: [f.payee({ default_envelope_id: 'e-jedzenie' })] }),
      ),
      http.post(apiUrl('/households/:hid/payees'), async ({ request }) => {
        createdPayee = (await request.json()) as PayeeCreate
        return HttpResponse.json(f.payee({ id: 'p-kino', name: 'Kino' }), { status: 201 })
      }),
      http.patch(apiUrl('/households/:hid/payees/:pid'), async ({ request }) => {
        updatedPayee = (await request.json()) as PayeeUpdate
        return HttpResponse.json(f.payee())
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/payees')
    const row = await screen.findByRole('article', { name: 'Kontrahent Sklep' })
    expect(await within(row).findByText('Domyślna koperta: Jedzenie')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Nowy kontrahent' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowy kontrahent' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Kino')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Domyślna koperta' }), 'e-wakacje')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(createdPayee).toEqual({ name: 'Kino', patterns: [], default_envelope_id: 'e-wakacje' }))

    await user.click(await screen.findByRole('button', { name: 'Edytuj Sklep' }))
    const edit = within(screen.getByRole('dialog', { name: 'Edycja: Sklep' }))
    expect(edit.getByRole('combobox', { name: 'Domyślna koperta' })).toHaveValue('e-jedzenie')
    await user.selectOptions(edit.getByRole('combobox', { name: 'Domyślna koperta' }), '')
    await user.click(edit.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(updatedPayee).toEqual({ name: 'Sklep', patterns: [], default_envelope_id: null }))
  })

  it('filters the register by envelope and shows the envelope on the row', async () => {
    const queries: URLSearchParams[] = []
    server.use(
      http.get(apiUrl('/households/:hid/transactions'), ({ request }) => {
        queries.push(new URL(request.url).searchParams)
        return HttpResponse.json({ items: [f.transaction({ envelope_id: 'e-jedzenie' })], next_cursor: null })
      }),
    )
    const { user } = renderApp('/h/h-dom/transactions')
    const row = await screen.findByRole('article', { name: /Sklep/ })
    expect(await within(row).findByTitle('Koperta')).toHaveTextContent('Jedzenie')
    const filter = screen.getByRole('combobox', { name: 'Koperta' })
    await within(filter).findByRole('option', { name: 'Cele · Wakacje' })
    await user.selectOptions(filter, 'e-wakacje')
    await waitFor(() => expect(queries.at(-1)?.get('envelope_id')).toBe('e-wakacje'))
  })

  it('shows Ready to Assign and overspent envelopes on the dashboard', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/budget/months/:month'), ({ params }) =>
        HttpResponse.json(
          f.budgetMonth({
            month: String(params.month),
            ready_to_assign: f.money('-25.00'),
            groups: [
              f.budgetGroup({
                envelopes: [
                  f.budgetEnvelope({ activity: f.money('-150.00') }),
                  f.budgetEnvelope({ id: 'e-paliwo', name: 'Paliwo', activity: f.money('-300.00') }),
                  f.budgetEnvelope({ id: 'e-ok', name: 'OK', assigned: f.money('10.00') }),
                ],
              }),
            ],
          }),
        ),
      ),
    )
    renderApp('/h/h-dom')
    const tile = await screen.findByRole('region', { name: /^Budżet — / })
    const rta = within(tile).getByRole('group', { name: 'Do przydzielenia' })
    expect(plain(rta.textContent)).toContain('−25,00 zł')
    expect(rta.textContent).toContain('Przydzielono więcej, niż masz.')
    const list = within(tile).getByRole('list', { name: 'Przekroczone koperty' })
    expect(plain(list.textContent)).toBe('Paliwo−300,00 złJedzenie−150,00 zł')
    expect(within(tile).getByRole('link', { name: 'Przejdź do budżetu' })).toHaveAttribute('href', '/h/h-dom/budget')
  })

  it('lists the transactions of an envelope, counting only its split line', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/budget/months/:month'), () =>
        HttpResponse.json(f.budgetMonth({ groups: [f.budgetGroup({ envelopes: [f.budgetEnvelope({ activity: f.money('-30.00') })] })] })),
      ),
      http.get(apiUrl('/households/:hid/budget/months/:month/envelopes/:eid/transactions'), () =>
        HttpResponse.json({
          items: [
            f.transaction({
              amount: f.money('-100.00'),
              splits: [
                { envelope_id: 'e-jedzenie', amount: f.money('-30.00'), base_amount: f.money('-30.00'), note: '' },
                { envelope_id: 'e-wakacje', amount: f.money('-70.00'), base_amount: f.money('-70.00'), note: '' },
              ],
            }),
          ],
          next_cursor: null,
        }),
      ),
    )
    const { user } = renderApp('/h/h-dom/budget/2026-10')
    await user.click(await screen.findByRole('button', { name: 'Jedzenie' }))
    const list = within(await screen.findByRole('list', { name: 'Transakcje koperty' }))
    const item = list.getByRole('article', { name: /Sklep/ })
    expect(plain(item.textContent)).toContain('część transakcji na 100,00 zł')
    expect(plain(item.textContent)).toContain('−30,00 zł')
  })
})
