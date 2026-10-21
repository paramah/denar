import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { PriceChange, SubscriptionCreate, SubscriptionUpdate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => (value ?? '').replace(/[\u00a0\u202f]/g, ' ')

const sub = f.subscription()
const konto = f.wallet()

beforeEach(() => {
  // Tylko Date: timery zostają prawdziwe (MSW, user-event).
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-21T10:00:00Z'))
  server.use(
    http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto] })),
    http.get(apiUrl('/households/:hid/subscriptions/s-openai'), () => HttpResponse.json(sub)),
    http.get(apiUrl('/households/:hid/subscriptions/s-openai/prices'), () =>
      HttpResponse.json({ items: [f.pricePoint()] }),
    ),
  )
})

afterEach(() => {
  vi.useRealTimers()
})

describe('subscriptions overview', () => {
  it('shows base-currency costs, the incomplete note and the 30-day renewals', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/subscriptions/summary'), () =>
        HttpResponse.json(
          f.subscriptionSummary({
            monthly: f.money('112.00'),
            yearly: f.money('1344.00'),
            incomplete: true,
            missing_currencies: ['GBP'],
            counts: { active: 2, trial: 1, paused: 0, cancelled: 1 },
            by_vendor: [
              { vendor: 'openai', vendor_name: 'OpenAI', count: 1, monthly: f.money('80.00'), yearly: f.money('960.00') },
              { vendor: 'netflix', vendor_name: 'Netflix', count: 1, monthly: f.money('32.00'), yearly: f.money('384.00') },
            ],
            by_envelope: [{ envelope_id: null, count: 2, monthly: f.money('112.00'), yearly: f.money('1344.00') }],
            upcoming: [{ charge: f.charge(), base_amount: f.money('80.00') }],
            trials_ending: [
              { subscription_id: 's-claude', name: 'Claude Pro', vendor: 'anthropic', trial_ends_at: '2026-10-25', price: f.money('20.00', 'USD') },
            ],
            price_changes: [
              { subscription_id: 's-nf', name: 'Netflix', effective_from: '2026-10-01', old_price: f.money('29.00'), new_price: f.money('32.00') },
            ],
          }),
        ),
      ),
    )
    renderApp('/h/h-dom/subscriptions')
    const cost = await screen.findByRole('region', { name: 'Subskrypcje kosztują miesięcznie' })
    expect(plain(cost.textContent)).toContain('112,00 zł')
    expect(plain(cost.textContent)).toContain('Rocznie 1344,00 zł')
    expect(plain(cost.textContent)).toContain('Brak dzisiejszego kursu GBP')
    expect(within(cost).getByRole('link', { name: 'Ustaw kurs ręcznie' })).toHaveAttribute('href', '/h/h-dom/settings/rates?currency=GBP')

    const renewals = screen.getByRole('list', { name: 'Odnowienia w 30 dni' })
    const item = within(renewals).getByRole('listitem')
    expect(plain(item.textContent)).toContain('31 paź 2026')
    expect(plain(item.textContent)).toContain('za 10 dni')
    expect(plain(item.textContent)).toContain('20,00 USD')
    expect(plain(item.textContent)).toContain('≈ 80,00 zł')
    expect(within(item).getByRole('link', { name: /ChatGPT Plus/ })).toHaveAttribute('href', '/h/h-dom/subscriptions/s-openai')

    const vendors = screen.getByRole('region', { name: 'Miesięcznie wg dostawcy' })
    expect(plain(vendors.textContent)).toContain('OpenAI1 subskrypcja80,00 zł')
    expect(plain(screen.getByRole('region', { name: 'Miesięcznie wg koperty' }).textContent)).toContain('Bez koperty')
    expect(plain(screen.getByRole('region', { name: 'Kończące się okresy próbne' }).textContent)).toContain('za 4 dni')
    expect(plain(screen.getByRole('region', { name: 'Ostatnie zmiany cen' }).textContent)).toMatch(/29,00 zł.*zmiana na.*32,00 zł/)
  })

  it('invites to add the first subscription', async () => {
    const { user } = renderApp('/h/h-dom/subscriptions')
    await user.click(await screen.findByRole('button', { name: 'Dodaj pierwszą subskrypcję' }))
    expect(screen.getByRole('dialog', { name: 'Nowa subskrypcja' })).toBeInTheDocument()
  })
})

describe('subscription form', () => {
  it('creates a USD subscription with cycle, reminders and a match rule', async () => {
    let sent: SubscriptionCreate | undefined
    server.use(
      http.post(apiUrl('/households/:hid/subscriptions'), async ({ request }) => {
        sent = (await request.json()) as SubscriptionCreate
        return HttpResponse.json(sub, { status: 201 })
      }),
    )
    const { user, router } = renderApp('/h/h-dom/subscriptions/list')
    await user.click(await screen.findByRole('button', { name: 'Nowa subskrypcja' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa subskrypcja' }))
    await waitFor(() => expect(dialog.getByRole('option', { name: 'Konto (PLN)' })).toBeInTheDocument())
    await user.type(dialog.getByRole('combobox', { name: 'Dostawca' }), 'OpenAI')
    expect(dialog.getByRole('textbox', { name: 'Nazwa' })).toHaveValue('OpenAI')
    await user.clear(dialog.getByRole('textbox', { name: 'Nazwa' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'ChatGPT Plus')
    await user.type(dialog.getByRole('textbox', { name: 'Cena' }), '20')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Waluta' }), 'USD')
    expect(dialog.getByText('Koszty przeliczymy na PLN.')).toBeInTheDocument()
    const anchor = dialog.getByLabelText('Dzień płatności')
    await user.clear(anchor)
    await user.type(anchor, '2026-01-31')
    expect(dialog.getByText('co miesiąc, 31.')).toBeInTheDocument()
    expect(dialog.getByText('W krótszych miesiącach — ostatniego dnia miesiąca.')).toBeInTheDocument()
    expect(dialog.getByText('Najbliższe płatności: 31 paź 2026, 30 lis 2026, 31 gru 2026')).toBeInTheDocument()

    await user.selectOptions(dialog.getByRole('combobox', { name: 'Portfel płatności' }), 'w-konto')
    await user.click(dialog.getByRole('checkbox', { name: 'tydzień przed' }))
    await user.click(dialog.getByText('Zaawansowane: dopasowanie płatności'))
    await user.type(dialog.getByRole('textbox', { name: 'Wzorzec kontrahenta' }), 'OPENAI')
    await user.clear(dialog.getByRole('textbox', { name: 'Tolerancja kwoty (%)' }))
    await user.type(dialog.getByRole('textbox', { name: 'Tolerancja kwoty (%)' }), '15')
    await user.type(dialog.getByRole('textbox', { name: 'Adres zarządzania' }), 'https://chatgpt.com/account')
    await user.click(dialog.getByRole('button', { name: 'Dodaj subskrypcję' }))

    await waitFor(() => expect(sent).toBeDefined())
    expect(sent).toEqual({
      name: 'ChatGPT Plus',
      vendor: 'openai',
      price: { amount: '20.00', currency: 'USD' },
      cycle: { unit: 'month', every: 1, anchor: '2026-01-31' },
      reminder_offsets: [7, 3],
      match_rule: { payee_pattern: 'OPENAI', amount_tolerance_pct: 15, date_window_days: 3 },
      wallet_id: 'w-konto',
      manage_url: 'https://chatgpt.com/account',
    })
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/subscriptions/s-openai'))
  })

  it('describes other cycles in Polish', async () => {
    const { user } = renderApp('/h/h-dom/subscriptions/list')
    await user.click(await screen.findByRole('button', { name: 'Nowa subskrypcja' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa subskrypcja' }))
    await user.clear(dialog.getByRole('textbox', { name: 'Co ile' }))
    await user.type(dialog.getByRole('textbox', { name: 'Co ile' }), '2')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Jednostka' }), 'week')
    expect(dialog.getByRole('option', { name: 'tygodnie' })).toBeInTheDocument()
    expect(dialog.getByText('co 2 tygodnie, w środę')).toBeInTheDocument()
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Jednostka' }), 'year')
    expect(dialog.getByText('co 2 lata, 21 października')).toBeInTheDocument()
  })

  it('validates the form without calling the API', async () => {
    const created = vi.fn()
    server.use(
      http.post(apiUrl('/households/:hid/subscriptions'), () => {
        created()
        return HttpResponse.json(sub, { status: 201 })
      }),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/list')
    await user.click(await screen.findByRole('button', { name: 'Nowa subskrypcja' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa subskrypcja' }))
    await user.type(dialog.getByRole('combobox', { name: 'Dostawca' }), 'Nieznany')
    await user.type(dialog.getByRole('textbox', { name: 'Cena' }), '12,345')
    await user.clear(dialog.getByRole('textbox', { name: 'Co ile' }))
    await user.type(dialog.getByRole('textbox', { name: 'Co ile' }), '0')
    await user.click(dialog.getByRole('checkbox', { name: 'Okres próbny' }))
    await user.type(dialog.getByRole('textbox', { name: 'Adres zarządzania' }), 'ftp://example.test')
    await user.click(dialog.getByRole('button', { name: 'Dodaj subskrypcję' }))

    expect(dialog.getByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription(/Podaj nazwę\./)
    expect(dialog.getByRole('combobox', { name: 'Dostawca' })).toHaveAccessibleDescription(/Wybierz dostawcę z listy/)
    expect(dialog.getByRole('textbox', { name: 'Cena' })).toHaveAccessibleDescription(/Najwyżej 2 miejsca po przecinku\./)
    expect(dialog.getByRole('textbox', { name: 'Co ile' })).toHaveAccessibleDescription('Od 1 do 365.')
    expect(dialog.getByLabelText('Koniec okresu próbnego')).toHaveAccessibleDescription('Podaj koniec okresu próbnego.')
    expect(dialog.getByRole('textbox', { name: 'Adres zarządzania' })).toHaveAccessibleDescription(/http:\/\/ lub https:\/\//)
    expect(dialog.getByText('Uzupełnij cykl, żeby zobaczyć najbliższe płatności.')).toBeInTheDocument()
    expect(created).not.toHaveBeenCalled()
  })

  it('limits reminders to five and shows a duplicate name at the field', async () => {
    server.use(http.post(apiUrl('/households/:hid/subscriptions'), () => problemResponse(409, 'conflict', { reason: 'duplicate-name' })))
    const { user } = renderApp('/h/h-dom/subscriptions/list')
    await user.click(await screen.findByRole('button', { name: 'Nowa subskrypcja' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa subskrypcja' }))
    for (const name of ['w dniu płatności', '1 dzień przed', 'tydzień przed', '2 tygodnie przed']) {
      await user.click(dialog.getByRole('checkbox', { name }))
    }
    expect(dialog.getByRole('checkbox', { name: 'miesiąc przed' })).toBeDisabled()
    await user.type(dialog.getByRole('combobox', { name: 'Dostawca' }), 'Netflix')
    await user.type(dialog.getByRole('textbox', { name: 'Cena' }), '43')
    await user.click(dialog.getByRole('button', { name: 'Dodaj subskrypcję' }))
    expect(await dialog.findByText('Masz już subskrypcję o tej nazwie.')).toBeInTheDocument()
  })

  it('sends only changed fields when editing', async () => {
    let sent: SubscriptionUpdate | undefined
    server.use(
      http.patch(apiUrl('/households/:hid/subscriptions/:sid'), async ({ request }) => {
        sent = (await request.json()) as SubscriptionUpdate
        return HttpResponse.json(sub)
      }),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/s-openai')
    await user.click(await screen.findByRole('button', { name: 'Edytuj' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja: ChatGPT Plus' }))
    expect(dialog.queryByRole('textbox', { name: 'Cena' })).not.toBeInTheDocument()
    await user.clear(dialog.getByRole('textbox', { name: 'Nazwa' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'ChatGPT Team')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Portfel płatności' }), '')
    await user.click(dialog.getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(sent).toEqual({ name: 'ChatGPT Team', wallet_id: null }))
  })
})

describe('subscription page', () => {
  it('changes the price from a date and shows the history', async () => {
    let sent: PriceChange | undefined
    server.use(
      http.get(apiUrl('/households/:hid/subscriptions/s-openai/prices'), () =>
        HttpResponse.json({
          items: [f.pricePoint(), f.pricePoint({ id: 'pp-2', effective_from: '2026-11-15', price: f.money('25.00', 'USD') })],
        }),
      ),
      http.post(apiUrl('/households/:hid/subscriptions/:sid/prices'), async ({ request }) => {
        sent = (await request.json()) as PriceChange
        return HttpResponse.json(sub, { status: 201 })
      }),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/s-openai')
    const history = await screen.findByRole('list', { name: 'Ceny od najnowszej' })
    const [future, current] = within(history).getAllByRole('listitem')
    expect(plain(future!.textContent)).toContain('(zaplanowana)')
    expect(plain(future!.textContent)).toContain('+5,00 USD')
    expect(plain(current!.textContent)).toContain('(obowiązuje)')
    expect(plain(screen.getByRole('region', { name: 'Cena i cykl' }).textContent)).toContain('co miesiąc, 31.')

    await user.click(screen.getByRole('button', { name: 'Zmień cenę' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Zmiana ceny: ChatGPT Plus' }))
    expect(dialog.getByLabelText('Obowiązuje od')).toHaveValue('2026-10-21')
    await user.clear(dialog.getByLabelText('Obowiązuje od'))
    await user.type(dialog.getByLabelText('Obowiązuje od'), '2026-12-01')
    await user.clear(dialog.getByRole('textbox', { name: 'Nowa cena' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nowa cena' }), '22,50')
    await user.click(dialog.getByRole('button', { name: 'Zapisz cenę' }))
    await waitFor(() => expect(sent).toEqual({ effective_from: '2026-12-01', price: { amount: '22.50', currency: 'USD' } }))
  })

  it('pauses, and deletes after confirmation', async () => {
    const patches: SubscriptionUpdate[] = []
    let deleted = false
    server.use(
      http.patch(apiUrl('/households/:hid/subscriptions/:sid'), async ({ request }) => {
        patches.push((await request.json()) as SubscriptionUpdate)
        return HttpResponse.json(f.subscription({ status: 'paused' }))
      }),
      http.delete(apiUrl('/households/:hid/subscriptions/:sid'), () => {
        deleted = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { user, router } = renderApp('/h/h-dom/subscriptions/s-openai')
    await user.click(await screen.findByRole('button', { name: 'Wstrzymaj' }))
    await waitFor(() => expect(patches).toEqual([{ status: 'paused' }]))
    await user.click(screen.getByRole('button', { name: 'Usuń' }))
    const confirm = within(screen.getByRole('alertdialog', { name: 'Usunąć ChatGPT Plus?' }))
    await user.click(confirm.getByRole('button', { name: 'Usuń subskrypcję' }))
    await waitFor(() => expect(deleted).toBe(true))
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/subscriptions/list'))
  })

  it('matches, posts, skips and unmatches charges from the schedule', async () => {
    const calls: { action: string; body: unknown }[] = []
    const record = (action: string) =>
      http.post(apiUrl(`/households/:hid/charges/:cid/${action}`), async ({ request, params }) => {
        const text = await request.text()
        calls.push({ action: `${action}:${String(params.cid)}`, body: text ? JSON.parse(text) : null })
        if (action === 'post') {
          return HttpResponse.json(
            { charge: f.charge({ status: 'posted', transaction_id: 't-new' }), transaction: f.transaction({ id: 't-new', amount: f.money('-81.20'), source: 'subscription' }) },
            { status: 201 },
          )
        }
        return HttpResponse.json(f.charge())
      })
    server.use(
      http.get(apiUrl('/households/:hid/charges'), ({ request }) => {
        expect(new URL(request.url).searchParams.get('subscription_id')).toBe('s-openai')
        return HttpResponse.json({
          items: [
            f.charge({ id: 'c-aug', due_date: '2026-08-31', status: 'matched', transaction_id: 't-aug' }),
            f.charge({ id: 'c-sep', due_date: '2026-09-30', status: 'missed' }),
            f.charge({ id: 'c-oct', due_date: '2026-10-31' }),
          ],
          next_cursor: null,
        })
      }),
      http.get(apiUrl('/households/:hid/charges/:cid/suggestions'), () =>
        HttpResponse.json({ items: [{ score: 90, transaction: f.transaction({ id: 't-sep', date: '2026-09-30', payee_name: 'OpenAI', amount: f.money('-80.50') }) }] }),
      ),
      record('match'),
      record('post'),
      record('skip'),
      record('unmatch'),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/s-openai')
    const missed = await screen.findByRole('article', { name: 'płatność 30 wrz 2026' })
    expect(within(missed).getByText('Przegapiona')).toBeInTheDocument()

    await user.click(within(missed).getByRole('button', { name: 'Dopasuj' }))
    const match = within(screen.getByRole('dialog', { name: 'Dopasuj płatność' }))
    expect(await match.findByText('Zgodność 90%')).toBeInTheDocument()
    await user.click(match.getByRole('button', { name: 'Potwierdź: OpenAI, 30 wrz 2026' }))
    await waitFor(() => expect(calls).toContainEqual({ action: 'match:c-sep', body: { transaction_id: 't-sep' } }))

    // Portfel PLN, płatność USD: kwota z wyciągu jest wymagana.
    await user.click(within(screen.getByRole('article', { name: 'płatność 31 paź 2026' })).getByRole('button', { name: 'Zaksięguj' }))
    const post = within(screen.getByRole('dialog', { name: 'Zaksięguj płatność' }))
    const amount = post.getByRole('textbox', { name: 'Zapłacona kwota (PLN)' })
    expect(amount).toHaveValue('')
    expect(amount).toHaveAccessibleDescription(/Portfel jest w PLN, a płatność w USD/)
    await user.click(post.getByRole('button', { name: 'Zaksięguj wydatek' }))
    expect(amount).toHaveAccessibleDescription(/Podaj kwotę\./)
    await user.type(amount, '81,20')
    await user.click(post.getByRole('button', { name: 'Zaksięguj wydatek' }))
    await waitFor(() =>
      expect(calls).toContainEqual({
        action: 'post:c-oct',
        body: { wallet_id: 'w-konto', amount: { amount: '81.20', currency: 'PLN' }, date: '2026-10-31' },
      }),
    )
    expect(await screen.findByText('Zaksięgowano wydatek −81,20 zł.')).toBeInTheDocument()

    await user.click(within(screen.getByRole('article', { name: 'płatność 31 paź 2026' })).getByRole('button', { name: 'Pomiń' }))
    await user.click(within(screen.getByRole('alertdialog', { name: 'Pominąć płatność?' })).getByRole('button', { name: 'Pomiń płatność' }))
    await waitFor(() => expect(calls.map((c) => c.action)).toContain('skip:c-oct'))

    await user.click(within(screen.getByRole('article', { name: 'płatność 31 sie 2026' })).getByRole('button', { name: 'Cofnij dopasowanie' }))
    await waitFor(() => expect(calls.map((c) => c.action)).toContain('unmatch:c-aug'))
  })

  it('shows a not-found state', async () => {
    server.use(http.get(apiUrl('/households/:hid/subscriptions/s-gone'), () => problemResponse(404, 'not_found')))
    renderApp('/h/h-dom/subscriptions/s-gone')
    expect(await screen.findByRole('heading', { level: 1, name: 'Nie znaleziono subskrypcji' })).toBeInTheDocument()
  })
})

describe('subscription list', () => {
  it('filters by status, vendor and wallet and shows readable cards', async () => {
    const queries: URLSearchParams[] = []
    server.use(
      http.get(apiUrl('/households/:hid/subscriptions'), ({ request }) => {
        queries.push(new URL(request.url).searchParams)
        return HttpResponse.json({
          items: [sub, f.subscription({ id: 's-nf', name: 'Netflix', vendor: 'netflix', vendor_name: 'Netflix', price: f.money('43.00'), cycle: { unit: 'week', every: 2, anchor: '2026-10-07' }, status: 'paused', next_due: null })],
        })
      }),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/list')
    const card = await screen.findByRole('article', { name: 'Subskrypcja ChatGPT Plus' })
    expect(plain(card.textContent)).toContain('OpenAI, co miesiąc, 31., z portfela Konto')
    expect(plain(card.textContent)).toContain('20,00 USD')
    expect(plain(card.textContent)).toContain('Aktywna')
    expect(plain(card.textContent)).toContain('Następna: 31 paź 2026 (za 10 dni)')
    await waitFor(() => expect(plain(card.textContent)).toContain('≈ 80,00 zł'))
    const paused = screen.getByRole('article', { name: 'Subskrypcja Netflix' })
    expect(plain(paused.textContent)).toContain('co 2 tygodnie, w środę')
    expect(plain(paused.textContent)).toContain('Wstrzymana')

    await user.selectOptions(screen.getByRole('combobox', { name: 'Status' }), 'paused')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Dostawca' }), 'netflix')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Portfel' }), 'w-konto')
    await waitFor(() => {
      const last = queries[queries.length - 1]!
      expect([last.get('status'), last.get('vendor'), last.get('wallet_id')]).toEqual(['paused', 'netflix', 'w-konto'])
    })
  })
})

describe('matching queue', () => {
  it('lists open past charges with suggestions and confirms one quickly', async () => {
    let matched: unknown
    server.use(
      http.get(apiUrl('/households/:hid/charges'), ({ request }) => {
        const params = new URL(request.url).searchParams
        if (params.getAll('status').join(',') === 'expected,missed') {
          expect(params.get('to')).toBe('2026-10-21')
          return HttpResponse.json({
            items: [f.charge({ id: 'c-sep', due_date: '2026-09-30', status: 'missed' }), f.charge({ id: 'c-oct20', due_date: '2026-10-20', subscription_name: 'Netflix', vendor: 'netflix', amount: f.money('43.00') })],
            next_cursor: null,
          })
        }
        return HttpResponse.json({ items: [], next_cursor: null })
      }),
      http.get(apiUrl('/households/:hid/charges/:cid/suggestions'), ({ params }) =>
        HttpResponse.json({
          items:
            params.cid === 'c-oct20'
              ? [{ score: 100, transaction: f.transaction({ id: 't-nf', date: '2026-10-20', payee_name: 'Netflix', amount: f.money('-43.00') }) }]
              : [],
        }),
      ),
      http.post(apiUrl('/households/:hid/charges/:cid/match'), async ({ request, params }) => {
        matched = { charge: params.cid, ...((await request.json()) as object) }
        return HttpResponse.json(f.charge({ id: 'c-oct20', status: 'matched', transaction_id: 't-nf' }))
      }),
    )
    const { user } = renderApp('/h/h-dom/subscriptions/charges')
    const tabs = await screen.findByRole('navigation', { name: 'Subskrypcje' })
    await waitFor(() => expect(plain(within(tabs).getByRole('link', { name: /Do dopasowania/ }).textContent)).toBe('Do dopasowania: 2'))
    const queue = await screen.findByRole('list', { name: 'Płatności do dopasowania' })
    const [first, second] = within(queue).getAllByRole('article')
    expect(first).toHaveAccessibleName('ChatGPT Plus, płatność 30 wrz 2026')
    expect(await within(first!).findByText('Brak pasujących transakcji.')).toBeInTheDocument()
    await user.click(await within(second!).findByRole('button', { name: 'Potwierdź: Netflix, 20 paź 2026' }))
    await waitFor(() => expect(matched).toEqual({ charge: 'c-oct20', transaction_id: 't-nf' }))
  })

  it('celebrates an empty queue', async () => {
    renderApp('/h/h-dom/subscriptions/charges')
    expect(await screen.findByRole('heading', { name: 'Wszystko dopasowane' })).toBeInTheDocument()
  })
})

describe('integration with the register and the dashboard', () => {
  it('links a subscription badge and binds an expense to a charge', async () => {
    let matched: unknown
    server.use(
      http.get(apiUrl('/households/:hid/transactions'), () =>
        HttpResponse.json({
          items: [
            f.transaction({ id: 't-linked', payee_name: 'OPENAI *CHATGPT', amount: f.money('-80.40'), date: '2026-09-30' }),
            f.transaction({ id: 't-free', payee_name: 'OpenAI', amount: f.money('-80.00'), date: '2026-10-20' }),
            f.transaction({ id: 't-in', kind: 'income', payee_name: 'Pracodawca', amount: f.money('3000.00') }),
          ],
          next_cursor: null,
        }),
      ),
      http.get(apiUrl('/households/:hid/charges'), ({ request }) => {
        const params = new URL(request.url).searchParams
        expect(params.getAll('status')).toEqual(['matched', 'posted'])
        expect([params.get('from'), params.get('to')]).toEqual(['2026-08-30', '2026-11-20'])
        return HttpResponse.json({ items: [f.charge({ id: 'c-sep', status: 'matched', transaction_id: 't-linked' })], next_cursor: null })
      }),
      http.get(apiUrl('/households/:hid/transactions/:tid/charge-suggestions'), () =>
        HttpResponse.json({ items: [{ score: 80, charge: f.charge() }] }),
      ),
      http.post(apiUrl('/households/:hid/charges/:cid/match'), async ({ request, params }) => {
        matched = { charge: params.cid, ...((await request.json()) as object) }
        return HttpResponse.json(f.charge({ status: 'matched', transaction_id: 't-free' }))
      }),
    )
    const { user } = renderApp('/h/h-dom/transactions')
    const linked = await screen.findByRole('article', { name: /OPENAI \*CHATGPT/ })
    expect(await within(linked).findByRole('link', { name: 'Subskrypcja: ChatGPT Plus' })).toHaveAttribute('href', '/h/h-dom/subscriptions/s-openai')
    expect(within(linked).queryByRole('button', { name: /Powiąż z subskrypcją/ })).not.toBeInTheDocument()
    expect(within(screen.getByRole('article', { name: /Pracodawca/ })).queryByRole('button', { name: /Powiąż/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Powiąż z subskrypcją: OpenAI' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Powiąż z subskrypcją' }))
    await user.click(await dialog.findByRole('button', { name: 'Powiąż: ChatGPT Plus, 31 paź 2026' }))
    await waitFor(() => expect(matched).toEqual({ charge: 'c-oct', transaction_id: 't-free' }))
  })

  it('shows the monthly cost and next renewals on the dashboard', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/subscriptions/summary'), () =>
        HttpResponse.json(
          f.subscriptionSummary({
            monthly: f.money('80.00'),
            yearly: f.money('960.00'),
            counts: { active: 1, trial: 0, paused: 0, cancelled: 0 },
            upcoming: [{ charge: f.charge(), base_amount: f.money('80.00') }],
          }),
        ),
      ),
    )
    renderApp('/h/h-dom')
    const tile = await screen.findByRole('region', { name: 'Subskrypcje' })
    expect(plain(tile.textContent)).toContain('Subskrypcje — koszt/mies.80,00 zł')
    expect(within(tile).getByRole('list', { name: 'Odnowienia w 30 dni' })).toHaveTextContent('ChatGPT Plus')
  })
})
