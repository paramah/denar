import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { FxOverride, FxOverrideSet, Payee, PayeeCreate, PayeeUpdate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { parsePatterns } from './PayeesTab.tsx'

describe('PayeesTab', () => {
  let payees: Payee[]
  let created: PayeeCreate | undefined
  let updated: PayeeUpdate | undefined
  let deleted: string | undefined

  beforeEach(() => {
    payees = [f.payee({ patterns: ['SKLEP 24'] }), f.payee({ id: 'p-orlen', name: 'Orlen', patterns: [] })]
    created = updated = deleted = undefined
    server.use(
      http.get(apiUrl('/households/:hid/payees'), () => HttpResponse.json({ items: payees })),
      http.post(apiUrl('/households/:hid/payees'), async ({ request }) => {
        created = (await request.json()) as PayeeCreate
        if (created.name === 'Orlen') {
          return problemResponse(409, 'conflict', { reason: 'duplicate-name' })
        }
        const payee = f.payee({ id: 'p-new', name: created.name, patterns: created.patterns ?? [] })
        payees = [...payees, payee]
        return HttpResponse.json(payee, { status: 201 })
      }),
      http.patch(apiUrl('/households/:hid/payees/:pid'), async ({ request, params }) => {
        updated = (await request.json()) as PayeeUpdate
        const payee = { ...payees.find((p) => p.id === params.pid)!, ...updated } as Payee
        payees = payees.map((p) => (p.id === payee.id ? payee : p))
        return HttpResponse.json(payee)
      }),
      http.delete(apiUrl('/households/:hid/payees/:pid'), ({ params }) => {
        deleted = params.pid as string
        payees = payees.filter((p) => p.id !== params.pid)
        return new HttpResponse(null, { status: 204 })
      }),
    )
  })

  it('parses one pattern per line', () => {
    expect(parsePatterns(' ORLEN \n\nstacja orlen\nORLEN')).toEqual(['ORLEN', 'stacja orlen'])
  })

  it('lists payees with patterns and edits names and patterns', async () => {
    const { user } = renderApp('/h/h-dom/settings/payees')
    const sklep = await screen.findByRole('article', { name: 'Kontrahent Sklep' })
    expect(within(sklep).getByText('SKLEP 24')).toBeInTheDocument()
    expect(within(screen.getByRole('article', { name: 'Kontrahent Orlen' })).getByText(/Bez wzorców/)).toBeInTheDocument()

    await user.click(within(sklep).getByRole('button', { name: 'Edytuj Sklep' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja: Sklep' }))
    await user.clear(dialog.getByRole('textbox', { name: 'Nazwa' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Żabka')
    await user.type(dialog.getByRole('textbox', { name: 'Wzorce' }), '{Enter}ZABKA')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(updated).toEqual({ name: 'Żabka', patterns: ['SKLEP 24', 'ZABKA'] }))
    expect(await screen.findByRole('article', { name: 'Kontrahent Żabka' })).toBeInTheDocument()
  })

  it('creates a payee and shows a duplicate at the name', async () => {
    const { user } = renderApp('/h/h-dom/settings/payees')
    await user.click(await screen.findByRole('button', { name: 'Nowy kontrahent' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowy kontrahent' }))
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(dialog.getByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription('Podaj nazwę.')
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Orlen')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(await dialog.findByText('Kontrahent o tej nazwie już istnieje.')).toBeInTheDocument()
    await user.clear(dialog.getByRole('textbox', { name: 'Nazwa' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Lidl')
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    expect(await screen.findByRole('article', { name: 'Kontrahent Lidl' })).toBeInTheDocument()
    expect(created).toEqual({ name: 'Lidl', patterns: [] })
  })

  it('deletes a payee after a warning', async () => {
    const { user } = renderApp('/h/h-dom/settings/payees')
    await user.click(await screen.findByRole('button', { name: 'Usuń Orlen' }))
    const confirm = within(screen.getByRole('alertdialog', { name: 'Usunąć kontrahenta Orlen?' }))
    expect(confirm.getByText('Transakcje zostaną, ale bez kontrahenta.')).toBeInTheDocument()
    await user.click(confirm.getByRole('button', { name: 'Usuń kontrahenta' }))
    await waitFor(() => expect(screen.queryByRole('article', { name: 'Kontrahent Orlen' })).not.toBeInTheDocument())
    expect(deleted).toBe('p-orlen')
  })

  it('is read-only for a viewer', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    renderApp('/h/h-dom/settings/payees')
    await screen.findByRole('article', { name: 'Kontrahent Sklep' })
    expect(screen.queryByRole('button', { name: /Nowy kontrahent|Edytuj|Usuń/ })).not.toBeInTheDocument()
  })
})

describe('RatesTab', () => {
  let overrides: FxOverride[]
  let set: FxOverrideSet | undefined
  let quoted: URLSearchParams | undefined

  beforeEach(() => {
    overrides = [f.fxOverride()]
    set = undefined
    quoted = undefined
    server.use(
      http.get(apiUrl('/households/:hid/fx-rates'), ({ request }) => {
        quoted = new URL(request.url).searchParams
        if (quoted.get('currency') === 'JPY') {
          return HttpResponse.json(f.fxQuote({ currency: 'JPY', rate: null, rate_date: null, source: 'unavailable', date: quoted.get('date')! }))
        }
        return HttpResponse.json(
          f.fxQuote({ currency: quoted.get('currency')!, date: quoted.get('date')!, rate_date: '2026-10-16' }),
        )
      }),
      http.get(apiUrl('/households/:hid/fx-rates/overrides'), () => HttpResponse.json({ items: overrides })),
      http.put(apiUrl('/households/:hid/fx-rates/overrides'), async ({ request }) => {
        set = (await request.json()) as FxOverrideSet
        const saved = f.fxOverride({ currency: set.currency, date: set.date, rate: set.rate })
        overrides = [saved, ...overrides]
        return HttpResponse.json(saved)
      }),
    )
  })

  it('quotes a rate for a currency and day, and says when there is none', async () => {
    const { user } = renderApp('/h/h-dom/settings/rates')
    expect(await screen.findByText('1 EUR = 4,0123 PLN')).toBeInTheDocument()
    expect(screen.getByText('NBP')).toBeInTheDocument()
    expect(screen.getByText(/notowanie z 16 paź 2026/)).toBeInTheDocument()
    const checker = screen.getAllByRole('combobox', { name: 'Waluta' })[0]!
    await user.selectOptions(checker, 'JPY')
    expect(await screen.findByText('Brak kursu')).toBeInTheDocument()
    expect(quoted?.get('currency')).toBe('JPY')
    expect(screen.getByRole('article', { name: 'Kurs USD z 19 paź 2026' })).toHaveTextContent('1 USD = 4,1 PLN')
  })

  it('lets an owner set a manual rate, prefilled from a pending transaction link', async () => {
    const { user } = renderApp('/h/h-dom/settings/rates?currency=USD&date=2026-10-20')
    const form = within(await screen.findByRole('form', { name: 'Ustaw kurs ręcznie' }))
    await within(form.getByRole('combobox', { name: 'Waluta' })).findByRole('option', { name: 'USD' })
    expect(form.getByRole('combobox', { name: 'Waluta' })).toHaveValue('USD')
    expect(form.getByLabelText('Na dzień')).toHaveValue('2026-10-20')
    await user.type(form.getByRole('textbox', { name: 'Kurs (PLN za 1 USD)' }), '0')
    await user.click(form.getByRole('button', { name: 'Zapisz kurs' }))
    expect(form.getByRole('textbox', { name: 'Kurs (PLN za 1 USD)' })).toHaveAccessibleDescription('Kurs musi być większy od zera.')
    await user.clear(form.getByRole('textbox', { name: 'Kurs (PLN za 1 USD)' }))
    await user.type(form.getByRole('textbox', { name: 'Kurs (PLN za 1 USD)' }), '4,2512')
    await user.click(form.getByRole('button', { name: 'Zapisz kurs' }))
    await waitFor(() => expect(set).toEqual({ currency: 'USD', date: '2026-10-20', rate: '4.2512' }))
    expect(await screen.findByRole('article', { name: 'Kurs USD z 20 paź 2026' })).toBeInTheDocument()
  })

  it('hides the manual rate form from a member', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'member' })]))))
    renderApp('/h/h-dom/settings/rates')
    expect(await screen.findByRole('heading', { name: 'Kursy ustawione ręcznie' })).toBeInTheDocument()
    expect(screen.queryByRole('form', { name: 'Ustaw kurs ręcznie' })).not.toBeInTheDocument()
  })
})
