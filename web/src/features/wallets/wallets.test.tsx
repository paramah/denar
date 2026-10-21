import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { Wallet, WalletCreate, WalletUpdate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { walletTotals } from './totals.ts'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const konto = f.wallet()
const dolary = f.wallet({
  id: 'w-usd',
  name: 'Dolary',
  currency: 'USD',
  balance: f.money('100.00', 'USD'),
  cleared_balance: f.money('90.00', 'USD'),
  base_balance: f.money('400.00'),
})
const lokata = f.wallet({
  id: 'w-lokata',
  name: 'Lokata',
  type: 'savings',
  on_budget: false,
  visibility: 'private',
  member_ids: ['u-ada'],
  balance: f.money('5000.00'),
  cleared_balance: f.money('5000.00'),
  base_balance: f.money('5000.00'),
})
const stary = f.wallet({ id: 'w-stary', name: 'Stary', archived_at: '2026-10-10T10:00:00Z', base_balance: f.money('77.00') })

describe('walletTotals', () => {
  it('sums active wallets in the base currency on bigint', () => {
    const totals = walletTotals([konto, dolary, lokata, stary, f.wallet({ id: 'x', rate_pending: true, base_balance: f.money('0.01') })], 2)
    expect(totals).toEqual({ onBudget: '1400.01', offBudget: '5000.00', all: '6400.01', partial: true, count: 4 })
  })
})

describe('WalletsPage', () => {
  let created: WalletCreate | undefined
  let archivedAsked = false

  beforeEach(() => {
    created = undefined
    archivedAsked = false
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), ({ request }) => {
        const archived = new URL(request.url).searchParams.get('include_archived') === 'true'
        archivedAsked ||= archived
        return HttpResponse.json({ items: archived ? [dolary, konto, lokata, stary] : [dolary, konto, lokata] })
      }),
      http.get(apiUrl('/households/:hid/members'), () =>
        HttpResponse.json({ items: [f.member(), f.member({ user_id: 'u-jan', name: 'Jan Nowak', email: 'jan@example.test', role: 'member' })] }),
      ),
      http.post(apiUrl('/households/:hid/wallets'), async ({ request }) => {
        created = (await request.json()) as WalletCreate
        if (created.name === 'Konto') {
          return problemResponse(409, 'conflict', { reason: 'duplicate-name' })
        }
        return HttpResponse.json(f.wallet({ id: 'w-new', name: created.name, currency: created.currency }), { status: 201 })
      }),
    )
  })

  it('shows balances, badges and totals in the base currency', async () => {
    renderApp('/h/h-dom/wallets')
    const list = await screen.findByRole('list', { name: 'Portfele' })
    expect(within(list).getAllByRole('article').map((a) => a.getAttribute('aria-label'))).toEqual([
      'Portfel Dolary',
      'Portfel Konto',
      'Portfel Lokata',
    ])
    const usd = within(list).getByRole('article', { name: 'Portfel Dolary' })
    expect(plain(usd.textContent)).toContain('100,00 USD')
    expect(plain(usd.textContent)).toContain('≈ 400,00 zł')
    expect(plain(usd.textContent)).toContain('90,00 USD')
    const savings = within(list).getByRole('article', { name: 'Portfel Lokata' })
    expect(within(savings).getByText('Poza budżetem')).toBeInTheDocument()
    expect(within(savings).getByText('Prywatny')).toBeInTheDocument()

    const totals = screen.getByRole('region', { name: 'Sumy portfeli' })
    expect(plain(totals.textContent)).toContain('Na portfelach budżetowych1400,00 zł')
    expect(plain(totals.textContent)).toContain('Poza budżetem5000,00 zł')
    expect(plain(totals.textContent)).toContain('Razem6400,00 zł')
  })

  it('shows archived wallets on request, outside the totals', async () => {
    const { user } = renderApp('/h/h-dom/wallets')
    await screen.findByRole('list', { name: 'Portfele' })
    await user.click(screen.getByRole('checkbox', { name: 'Pokaż zarchiwizowane' }))
    const archived = await screen.findByRole('article', { name: 'Portfel Stary' })
    expect(within(archived).getByText('Zarchiwizowany')).toBeInTheDocument()
    expect(archivedAsked).toBe(true)
    expect(plain(screen.getByRole('region', { name: 'Sumy portfeli' }).textContent)).toContain('Razem6400,00 zł')
  })

  it('creates a private wallet with an opening balance', async () => {
    const { user } = renderApp('/h/h-dom/wallets')
    await user.click(await screen.findByRole('button', { name: 'Nowy portfel' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowy portfel' }))
    expect(dialog.getByRole('textbox', { name: 'Nazwa' })).toHaveFocus()
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Karta USD')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Typ' }), 'card')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Waluta' }), 'USD')
    await user.type(dialog.getByRole('textbox', { name: 'Saldo otwarcia' }), '1 000,5')
    await user.click(dialog.getByRole('checkbox', { name: 'Portfel w budżecie' }))
    expect(dialog.getByRole('group', { name: 'Kto widzi portfel' })).toBeInTheDocument()
    await user.click(dialog.getByRole('radio', { name: /Tylko wybrane osoby/ }))
    expect(dialog.getByRole('checkbox', { name: 'Ada Kowalska (twórca — zawsze)' })).toBeDisabled()
    await user.click(dialog.getByRole('checkbox', { name: 'Jan Nowak' }))
    await user.click(dialog.getByRole('button', { name: 'Dodaj portfel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(created).toMatchObject({
      name: 'Karta USD',
      type: 'card',
      currency: 'USD',
      on_budget: false,
      visibility: 'private',
      member_ids: ['u-jan'],
      opening_balance: { amount: '1000.50', currency: 'USD' },
    })
    expect(created?.opened_on).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(await screen.findByText('Dodano portfel Karta USD.')).toBeInTheDocument()
  })

  it('validates the form and shows a duplicate name at the field', async () => {
    const { user } = renderApp('/h/h-dom/wallets')
    await user.click(await screen.findByRole('button', { name: 'Nowy portfel' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowy portfel' }))
    await user.type(dialog.getByRole('textbox', { name: 'Saldo otwarcia' }), '12,345')
    await user.click(dialog.getByRole('button', { name: 'Dodaj portfel' }))
    expect(dialog.getByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription('Podaj nazwę portfela.')
    expect(dialog.getByRole('textbox', { name: 'Saldo otwarcia' })).toHaveAccessibleDescription(/Najwyżej 2 miejsca/)
    expect(created).toBeUndefined()

    await user.clear(dialog.getByRole('textbox', { name: 'Saldo otwarcia' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Konto')
    await user.click(dialog.getByRole('button', { name: 'Dodaj portfel' }))
    expect(await dialog.findByText('Portfel o tej nazwie już istnieje.')).toBeInTheDocument()
    expect(created).not.toHaveProperty('opening_balance')
  })

  it('has no write actions for a viewer', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    renderApp('/h/h-dom/wallets')
    await screen.findByRole('list', { name: 'Portfele' })
    expect(screen.queryByRole('button', { name: 'Nowy portfel' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Przelew' })).not.toBeInTheDocument()
  })
})

describe('WalletPage', () => {
  let patched: WalletUpdate | undefined

  beforeEach(() => {
    patched = undefined
    let current: Wallet = dolary
    server.use(
      http.get(apiUrl('/households/:hid/wallets/:wid'), ({ params }) =>
        params.wid === 'w-usd' ? HttpResponse.json(current) : problemResponse(404, 'not_found'),
      ),
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [current, konto] })),
      http.patch(apiUrl('/households/:hid/wallets/:wid'), async ({ request }) => {
        patched = (await request.json()) as WalletUpdate
        current = { ...current, archived_at: patched.archived ? '2026-10-21T10:00:00Z' : null }
        return HttpResponse.json(current)
      }),
    )
  })

  it('shows the balances and archives the wallet', async () => {
    const { user } = renderApp('/h/h-dom/wallets/w-usd')
    expect(await screen.findByRole('heading', { level: 1, name: 'Dolary' })).toBeInTheDocument()
    expect(plain(screen.getByLabelText('Salda portfela').textContent)).toBe(
      'Saldo100,00 USDRozliczone90,00 USDW walucie PLN400,00 zł',
    )
    await user.click(screen.getByRole('button', { name: 'Archiwizuj' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Archiwizuj' }))
    expect(await screen.findByText(/Portfel jest zarchiwizowany/)).toBeInTheDocument()
    expect(patched).toEqual({ archived: true })
    expect(screen.queryByRole('button', { name: 'Dodaj transakcję' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Przywróć' }))
    await waitFor(() => expect(patched).toEqual({ archived: false }))
  })

  it('says when the wallet is hidden or gone', async () => {
    renderApp('/h/h-dom/wallets/w-obcy')
    expect(await screen.findByRole('heading', { level: 1, name: 'Nie znaleziono portfela' })).toBeInTheDocument()
  })

  it('lets only the creator or an admin of a shared wallet change who sees it', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/wallets/:wid'), () =>
        HttpResponse.json({ ...dolary, visibility: 'private', member_ids: ['u-jan', 'u-ada'], created_by: 'u-jan' }),
      ),
    )
    const { user } = renderApp('/h/h-dom/wallets/w-usd')
    await user.click(await screen.findByRole('button', { name: 'Edytuj' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja portfela Dolary' }))
    expect(dialog.getByRole('textbox', { name: 'Waluta' })).toHaveAttribute('readonly')
    expect(dialog.queryByRole('group', { name: 'Kto widzi portfel' })).not.toBeInTheDocument()
  })
})
