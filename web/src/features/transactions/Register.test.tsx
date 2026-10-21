import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { Transaction, TransactionUpdate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const konto = f.wallet()
const dolary = f.wallet({ id: 'w-usd', name: 'Dolary', currency: 'USD', balance: f.money('0.00', 'USD'), cleared_balance: f.money('0.00', 'USD'), base_balance: f.money('0.00') })

function page(n: number, count: number): Transaction[] {
  return Array.from({ length: count }, (_, i) =>
    f.transaction({ id: `t-${n}-${i}`, payee_name: `Sklep ${n}-${i}`, date: '2026-10-1' + String(9 - n) }),
  )
}

describe('Register', () => {
  let queries: URLSearchParams[]
  let deleted: string[]
  let patched: TransactionUpdate | undefined
  let items: Transaction[]

  beforeEach(() => {
    queries = []
    deleted = []
    patched = undefined
    items = [
      f.transaction({ id: 't-pend', payee_name: 'Apteka', status: 'pending', tags: ['zdrowie'] }),
      f.transaction({
        id: 't-usd',
        wallet_id: 'w-usd',
        payee_name: 'Amazon',
        amount: f.money('-20.00', 'USD'),
        source: 'bot',
      }),
      f.transaction({ id: 't-rec', payee_name: 'Czynsz', status: 'reconciled' }),
      f.transaction({
        id: 't-tr',
        kind: 'transfer',
        payee_id: null,
        payee_name: null,
        amount: f.money('-400.00'),
        transfer_pair_id: 'pair',
        transfer_wallet_id: 'w-usd',
      }),
    ]
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [dolary, konto] })),
      http.get(apiUrl('/households/:hid/payees'), () => HttpResponse.json({ items: [f.payee(), f.payee({ id: 'p-apt', name: 'Apteka' })] })),
      http.get(apiUrl('/households/:hid/transactions'), ({ request }) => {
        const query = new URL(request.url).searchParams
        queries.push(query)
        if (query.get('q') === 'stron') {
          const cursor = query.get('cursor')
          return HttpResponse.json(cursor ? { items: page(2, 2), next_cursor: null } : { items: page(1, 3), next_cursor: 'c2' })
        }
        return HttpResponse.json({ items: items.filter((t) => !deleted.includes(t.id)), next_cursor: null })
      }),
      http.patch(apiUrl('/households/:hid/transactions/:tid'), async ({ request, params }) => {
        patched = (await request.json()) as TransactionUpdate
        const t = items.find((x) => x.id === params.tid)!
        Object.assign(t, patched)
        return HttpResponse.json(t)
      }),
      http.delete(apiUrl('/households/:hid/transactions/:tid'), ({ params }) => {
        const t = items.find((x) => x.id === params.tid)!
        deleted.push(...items.filter((x) => x.id === t.id || (t.transfer_pair_id && x.transfer_pair_id === t.transfer_pair_id)).map((x) => x.id))
        return new HttpResponse(null, { status: 204 })
      }),
    )
  })

  it('lists transactions with wallet, tags, source, base amount and rate_pending', async () => {
    renderApp('/h/h-dom/transactions')
    const list = await screen.findByRole('list', { name: 'Transakcje' })
    const rows = within(list).getAllByRole('article')
    expect(rows.map((row) => row.getAttribute('aria-label'))).toEqual([
      'Apteka, 20 paź 2026',
      'Amazon, 20 paź 2026',
      'Czynsz, 20 paź 2026',
      'Przelew do: Dolary, 20 paź 2026',
    ])
    const usd = rows[1]!
    expect(within(usd).getByRole('link', { name: 'Dolary' })).toHaveAttribute('href', '/h/h-dom/wallets/w-usd')
    expect(within(usd).getByText('Bot')).toBeInTheDocument()
    expect(within(usd).getByText('Czeka na kurs')).toBeInTheDocument()
    expect(within(usd).getByRole('link', { name: 'Ustaw kurs ręcznie' })).toHaveAttribute(
      'href',
      '/h/h-dom/settings/rates?currency=USD&date=2026-10-20',
    )
    expect(plain(usd.textContent)).toContain('−20,00 USD')
    expect(within(rows[0]!).getByText('#zdrowie')).toBeInTheDocument()
    expect(within(rows[2]!).getByText('Uzgodniona')).toBeInTheDocument()
  })

  it('hides the manual rate link from a member', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'member' })]))))
    renderApp('/h/h-dom/transactions')
    const row = await screen.findByRole('article', { name: /Amazon/ })
    expect(within(row).getByText('Czeka na kurs')).toBeInTheDocument()
    expect(within(row).queryByRole('link', { name: 'Ustaw kurs ręcznie' })).not.toBeInTheDocument()
  })

  it('keeps filters in the address and sends them to the API', async () => {
    const { user, router } = renderApp('/h/h-dom/transactions?status=pending&wallet=w-konto')
    const search = await screen.findByRole('search', { name: 'Filtry transakcji' })
    expect(within(search).getByRole('combobox', { name: 'Status' })).toHaveValue('pending')
    await waitFor(() => expect(queries.at(-1)?.get('status')).toBe('pending'))
    expect(queries.at(-1)?.get('wallet_id')).toBe('w-konto')

    await user.selectOptions(within(search).getByRole('combobox', { name: 'Kontrahent' }), 'p-apt')
    await user.selectOptions(within(search).getByRole('combobox', { name: 'Rodzaj' }), 'expense')
    await user.type(within(search).getByRole('searchbox', { name: 'Szukaj' }), 'leki')
    await waitFor(() => expect(queries.at(-1)?.get('q')).toBe('leki'))
    expect(queries.at(-1)?.get('payee_id')).toBe('p-apt')
    expect(queries.at(-1)?.get('kind')).toBe('expense')
    expect(router.state.location.search).toBe('?status=pending&wallet=w-konto&payee=p-apt&kind=expense&q=leki')

    await user.click(within(search).getByRole('button', { name: 'Wyczyść filtry' }))
    await waitFor(() => expect(router.state.location.search).toBe(''))
    await waitFor(() => expect([...(queries.at(-1)?.keys() ?? [])]).toEqual(['limit']))
  })

  it('loads more with the cursor', async () => {
    const { user } = renderApp('/h/h-dom/transactions?q=stron')
    const list = await screen.findByRole('list', { name: 'Transakcje' })
    expect(within(list).getAllByRole('article')).toHaveLength(3)
    await user.click(screen.getByRole('button', { name: 'Pokaż więcej' }))
    await waitFor(() => expect(within(list).getAllByRole('article')).toHaveLength(5))
    expect(queries.at(-1)?.get('cursor')).toBe('c2')
    expect(screen.queryByRole('button', { name: 'Pokaż więcej' })).not.toBeInTheDocument()
  })

  it('toggles pending and cleared in one click', async () => {
    const { user } = renderApp('/h/h-dom/transactions')
    const row = await screen.findByRole('article', { name: /Apteka/ })
    await user.click(within(row).getByRole('button', { name: 'Oczekująca — oznacz jako rozliczona' }))
    await waitFor(() => expect(patched).toEqual({ status: 'cleared' }))
    expect(await within(row).findByRole('button', { name: 'Rozliczona — oznacz jako oczekująca' })).toBeInTheDocument()
  })

  it('deletes a whole transfer after a warning', async () => {
    const { user } = renderApp('/h/h-dom/transactions')
    const row = await screen.findByRole('article', { name: /Przelew do: Dolary/ })
    await user.click(within(row).getByRole('button', { name: 'Usuń: Przelew do: Dolary' }))
    const confirm = within(screen.getByRole('alertdialog', { name: 'Usunąć przelew?' }))
    expect(confirm.getByText('Zniknie cały przelew — z obu portfeli.')).toBeInTheDocument()
    await user.click(confirm.getByRole('button', { name: 'Usuń przelew' }))
    await waitFor(() => expect(screen.queryByRole('article', { name: /Przelew/ })).not.toBeInTheDocument())
    expect(await screen.findByText('Usunięto przelew.')).toBeInTheDocument()
  })

  it('shows a register to a viewer without write actions', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    renderApp('/h/h-dom/transactions')
    const row = await screen.findByRole('article', { name: /Apteka/ })
    expect(within(row).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Dodaj transakcję' })).not.toBeInTheDocument()
  })

  it('opens the quick add with the n key', async () => {
    const { user } = renderApp('/h/h-dom/transactions')
    await screen.findByRole('list', { name: 'Transakcje' })
    await user.keyboard('n')
    expect(await screen.findByRole('dialog', { name: 'Nowa transakcja' })).toBeInTheDocument()
  })

  it('shows an empty state with a call to action', async () => {
    items = []
    renderApp('/h/h-dom/transactions')
    expect(await screen.findByRole('heading', { name: 'Nie ma jeszcze transakcji' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dodaj pierwszą transakcję' })).toBeInTheDocument()
  })
})
