import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import type { BudgetMonth, BudgetMove, EnvelopeCreate, EnvelopeUpdate, Goal, MoneyInput } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => value?.replace(/[\u00a0\u202f]/g, ' ')

const monthlyFood: Goal = { type: 'monthly', amount: f.money('800.00'), target_date: null }
const trip: Goal = { type: 'target_by_date', amount: f.money('1200.00'), target_date: '2027-02-28' }

type State = { assigned: Record<string, string> }

/** Miesiąc z dwiema grupami; przydziały ze stanu testu. */
function view(state: State, overrides: Partial<BudgetMonth> = {}): BudgetMonth {
  const food = f.budgetEnvelope({
    id: 'e-jedzenie',
    name: 'Jedzenie',
    assigned: f.money(state.assigned['e-jedzenie'] ?? '800.00'),
    activity: f.money('-950.00'),
    goal: monthlyFood,
    goal_status: { needed: f.money('800.00'), underfunded: f.money('0.00'), funded: true, progress: 100 },
  })
  const fun = f.budgetEnvelope({
    id: 'e-rozrywka',
    name: 'Rozrywka',
    position: 1,
    assigned: f.money(state.assigned['e-rozrywka'] ?? '200.00'),
    activity: f.money('-80.00'),
  })
  const old = f.budgetEnvelope({ id: 'e-stare', name: 'Stare', position: 2, hidden: true })
  const holiday = f.budgetEnvelope({
    id: 'e-wakacje',
    group_id: 'g-cele',
    name: 'Wakacje',
    assigned: f.money(state.assigned['e-wakacje'] ?? '0.00'),
    goal: trip,
    goal_status: { needed: f.money('240.00'), underfunded: f.money('240.00'), funded: false, progress: 0 },
  })
  return f.budgetMonth({
    ready_to_assign: f.money('3850.00'),
    overspent_last_month: f.money('-150.00'),
    assigned_in_future: f.money('100.00'),
    incomplete: true,
    pending_transactions: 2,
    income: f.money('5000.00'),
    uncategorized: f.money('-30.00'),
    transfers: f.money('-200.00'),
    assigned: f.money('1000.00'),
    activity: f.money('-1030.00'),
    available: f.money('-30.00'),
    underfunded: f.money('240.00'),
    groups: [
      f.budgetGroup({
        assigned: f.money('1000.00'),
        activity: f.money('-1030.00'),
        available: f.money('-30.00'),
        envelopes: [food, fun, old],
      }),
      f.budgetGroup({ id: 'g-cele', name: 'Cele', position: 1, envelopes: [holiday] }),
    ],
    ...overrides,
  })
}

describe('BudgetPage', () => {
  let state: State
  let puts: { envelopeId: string; month: string; amount: MoneyInput }[]
  let moves: BudgetMove[]
  let patches: { id: string; body: EnvelopeUpdate }[]
  let created: EnvelopeCreate[]

  beforeEach(() => {
    state = { assigned: {} }
    puts = []
    moves = []
    patches = []
    created = []
    server.use(
      http.get(apiUrl('/households/:hid/budget/months/:month'), () => HttpResponse.json(view(state))),
      http.put(apiUrl('/households/:hid/budget/months/:month/assignments/:eid'), async ({ request, params }) => {
        const { amount } = (await request.json()) as { amount: MoneyInput }
        puts.push({ envelopeId: String(params.eid), month: String(params.month), amount })
        if (amount.amount === '999999.00') {
          return problemResponse(422, 'validation', { errors: [{ field: 'amount.amount', message: 'za dużo' }] })
        }
        state.assigned[String(params.eid)] = amount.amount
        return HttpResponse.json(view(state))
      }),
      http.post(apiUrl('/households/:hid/budget/months/:month/moves'), async ({ request }) => {
        moves.push((await request.json()) as BudgetMove)
        return HttpResponse.json(view(state))
      }),
      http.post(apiUrl('/households/:hid/budget/months/:month/auto-assign'), () => {
        state.assigned['e-wakacje'] = '240.00'
        return HttpResponse.json({
          assigned: [{ envelope_id: 'e-wakacje', amount: f.money('240.00') }],
          month: view(state, { ready_to_assign: f.money('3610.00'), underfunded: f.money('0.00') }),
        })
      }),
      http.patch(apiUrl('/households/:hid/budget/envelopes/:eid'), async ({ request, params }) => {
        const body = (await request.json()) as EnvelopeUpdate
        patches.push({ id: String(params.eid), body })
        return HttpResponse.json(f.envelope({ id: String(params.eid), ...(body.hidden !== undefined ? { hidden: body.hidden } : {}) }))
      }),
      http.post(apiUrl('/households/:hid/budget/envelopes'), async ({ request }) => {
        const body = (await request.json()) as EnvelopeCreate
        created.push(body)
        if (body.name === 'Jedzenie') {
          return problemResponse(409, 'conflict', { reason: 'duplicate-name' })
        }
        return HttpResponse.json(f.envelope({ id: 'e-nowa', name: body.name, group_id: body.group_id }), { status: 201 })
      }),
      http.delete(apiUrl('/households/:hid/budget/envelopes/:eid'), () =>
        problemResponse(409, 'conflict', { reason: 'envelope-in-use' }),
      ),
      http.delete(apiUrl('/households/:hid/budget/envelope-groups/:gid'), () =>
        problemResponse(409, 'conflict', { reason: 'group-not-empty' }),
      ),
    )
  })

  async function open(path = '/h/h-dom/budget/2026-10') {
    const view = renderApp(path)
    await screen.findByRole('article', { name: 'Koperta Jedzenie' })
    return view
  }

  it('shows Ready to Assign, the month notes, groups with totals and envelope states', async () => {
    await open()
    expect(screen.getByRole('heading', { level: 1, name: 'Budżet' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'październik 2026' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Poprzedni miesiąc: wrzesień 2026' })).toHaveAttribute('href', '/h/h-dom/budget/2026-09')
    expect(screen.getByRole('link', { name: 'Następny miesiąc: listopad 2026' })).toHaveAttribute('href', '/h/h-dom/budget/2026-11')

    const rta = screen.getByRole('group', { name: 'Do przydzielenia' })
    expect(plain(rta.textContent)).toContain('3850,00 zł')
    const notes = within(screen.getByRole('list', { name: 'Uwagi do miesiąca' }))
    expect(plain(notes.getByText(/Przekroczenia z miesiąca wrzesień 2026/).textContent)).toContain('−150,00 zł')
    expect(plain(notes.getByText(/Przydzielono na kolejne miesiące/).textContent)).toContain('100,00 zł')
    expect(notes.getByText(/Transakcje czekające na kurs waluty: 2/)).toBeInTheDocument()

    const food = within(screen.getByRole('article', { name: 'Koperta Jedzenie' }))
    expect(food.getByRole('textbox', { name: 'Przydzielono: Jedzenie' })).toHaveValue('800,00')
    expect(plain(food.getByTitle(/Przekroczono/).textContent)).toBe('−150,00 zł (przekroczono)')
    expect(food.getByRole('progressbar', { name: 'Postęp celu Jedzenie' })).toHaveAttribute('aria-valuenow', '100')
    expect(food.getByText('Cel pokryty')).toBeInTheDocument()
    const holiday = within(screen.getByRole('article', { name: 'Koperta Wakacje' }))
    expect(plain(holiday.getByText(/^Brakuje/).textContent)).toContain('Brakuje 240,00 zł')

    // Ukryta koperta poza siatką, dopóki nie zaznaczysz „Pokaż ukryte”.
    expect(screen.queryByRole('article', { name: 'Koperta Stare' })).not.toBeInTheDocument()
    const flows = screen.getByRole('region', { name: 'Poza kopertami' })
    expect(plain(flows.textContent)).toContain('Bez koperty−30,00 zł')
  })

  it('edits an assignment inline: Enter saves, Escape cancels, Tab saves and moves on', async () => {
    const { user } = await open()
    const food = screen.getByRole('textbox', { name: 'Przydzielono: Jedzenie' })
    await user.click(food)
    await user.keyboard('{Control>}a{/Control}950,5{Enter}')
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0]).toEqual({ envelopeId: 'e-jedzenie', month: '2026-10', amount: { amount: '950.50', currency: 'PLN' } })
    await waitFor(() => expect(food).toHaveValue('950,50'))
    expect(food).toHaveFocus()

    await user.keyboard('{Control>}a{/Control}1{Escape}')
    expect(food).toHaveValue('950,50')

    await user.keyboard('{Control>}a{/Control}-20{Tab}')
    await waitFor(() => expect(puts).toHaveLength(2))
    expect(puts[1]?.amount.amount).toBe('-20.00')
    expect(screen.getByRole('textbox', { name: 'Przydzielono: Rozrywka' })).toHaveFocus()

    // Tab bez zmian nie wysyła nic i przechodzi do następnej grupy.
    await user.keyboard('{Tab}')
    expect(screen.getByRole('textbox', { name: 'Przydzielono: Wakacje' })).toHaveFocus()
    expect(puts).toHaveLength(2)
  })

  it('rejects an invalid amount inline and reports a server error', async () => {
    const { user } = await open()
    const fun = screen.getByRole('textbox', { name: 'Przydzielono: Rozrywka' })
    await user.click(fun)
    await user.keyboard('{Control>}a{/Control}12,345{Enter}')
    expect(fun).toHaveAccessibleDescription('Najwyżej 2 miejsca po przecinku.')
    expect(puts).toEqual([])
    await user.keyboard('{Escape}')
    expect(fun).toHaveValue('200,00')
    expect(fun).not.toHaveAttribute('aria-invalid')

    await user.keyboard('{Control>}a{/Control}999999{Enter}')
    expect(await screen.findByText(/Nie zapisano przydziału „Rozrywka”/)).toBeInTheDocument()
    await waitFor(() => expect(fun).toHaveValue('200,00'))
  })

  it('moves money between envelopes and from Ready to Assign', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Przenieś środki' }))
    let dialog = within(screen.getByRole('dialog', { name: 'Przenieś środki' }))
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Z' }), 'e-rozrywka')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do' }), 'e-jedzenie')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '20')
    await user.click(dialog.getByRole('button', { name: 'Przenieś' }))
    await waitFor(() => expect(moves).toHaveLength(1))
    expect(moves[0]).toEqual({
      from_envelope_id: 'e-rozrywka',
      to_envelope_id: 'e-jedzenie',
      amount: { amount: '20.00', currency: 'PLN' },
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    // Z kwoty do przydzielenia: przydział koperty rośnie o kwotę (na bigint).
    await user.click(screen.getByRole('button', { name: 'Przenieś środki' }))
    dialog = within(screen.getByRole('dialog', { name: 'Przenieś środki' }))
    expect(dialog.getByRole('combobox', { name: 'Z' })).toHaveValue('rta')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do' }), 'e-rozrywka')
    await user.type(dialog.getByRole('textbox', { name: 'Kwota' }), '0,10')
    await user.click(dialog.getByRole('button', { name: 'Przenieś' }))
    await waitFor(() => expect(puts).toHaveLength(1))
    expect(puts[0]).toMatchObject({ envelopeId: 'e-rozrywka', amount: { amount: '200.10', currency: 'PLN' } })
  })

  it('validates the move before calling the API', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Przenieś środki' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przenieś środki' }))
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Z' }), 'e-jedzenie')
    await user.selectOptions(dialog.getByRole('combobox', { name: 'Do' }), 'e-jedzenie')
    await user.click(dialog.getByRole('button', { name: 'Przenieś' }))
    expect(dialog.getByRole('combobox', { name: 'Do' })).toHaveAccessibleDescription('Wybierz inne miejsce docelowe niż źródło.')
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveAccessibleDescription('Podaj kwotę.')
    expect(moves).toEqual([])
  })

  it('covers an overspent envelope from its details', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Jedzenie' }))
    const details = within(screen.getByRole('dialog', { name: 'Jedzenie' }))
    expect(await details.findByText('Brak transakcji w tym miesiącu')).toBeInTheDocument()
    expect(details.getByRole('link', { name: 'Pokaż w rejestrze' })).toHaveAttribute(
      'href',
      '/h/h-dom/transactions?envelope=e-jedzenie&from=2026-10-01&to=2026-10-31',
    )
    await user.click(details.getByRole('button', { name: 'Pokryj przekroczenie' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Przenieś środki' }))
    expect(dialog.getByRole('combobox', { name: 'Do' })).toHaveValue('e-jedzenie')
    expect(dialog.getByRole('textbox', { name: 'Kwota' })).toHaveValue('150,00')
  })

  it('auto-assigns goals and summarises the result', async () => {
    const { user } = await open()
    expect(plain(screen.getByText(/Cele potrzebują jeszcze/).textContent)).toContain('240,00 zł')
    await user.click(screen.getByRole('button', { name: 'Uzupełnij cele' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Uzupełnianie celów' }))
    expect(plain(dialog.getByRole('list', { name: 'Uzupełnione koperty' }).textContent)).toBe('Wakacje+240,00 zł')
    expect(plain(dialog.getByText(/Zostało do przydzielenia/).textContent)).toContain('3610,00 zł')
    await user.click(dialog.getByRole('button', { name: 'Gotowe' }))
    expect(screen.getByRole('textbox', { name: 'Przydzielono: Wakacje' })).toHaveValue('240,00')
  })

  it('validates goals when creating an envelope and maps a duplicate name', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Dodaj kopertę do grupy Cele' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowa koperta' }))
    expect(dialog.getByRole('combobox', { name: 'Grupa' })).toHaveValue('g-cele')
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa koperty' }), 'Auto')
    await user.click(dialog.getByRole('radio', { name: 'Kwota do daty' }))
    await user.click(dialog.getByRole('button', { name: 'Dodaj kopertę' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota celu' })).toHaveAccessibleDescription('Podaj kwotę.')
    expect(dialog.getByLabelText('Do dnia')).toHaveAccessibleDescription('Podaj datę, do której chcesz zebrać kwotę.')
    expect(created).toEqual([])

    await user.type(dialog.getByRole('textbox', { name: 'Kwota celu' }), '6000')
    await user.type(dialog.getByLabelText('Do dnia'), '2099-06-30')
    await user.click(dialog.getByRole('button', { name: 'Dodaj kopertę' }))
    await waitFor(() => expect(created).toHaveLength(1))
    expect(created[0]).toEqual({
      group_id: 'g-cele',
      name: 'Auto',
      position: 1,
      goal: { type: 'target_by_date', amount: { amount: '6000.00', currency: 'PLN' }, target_date: '2099-06-30' },
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Dodaj kopertę do grupy Codzienne' }))
    const again = within(screen.getByRole('dialog', { name: 'Nowa koperta' }))
    await user.type(again.getByRole('textbox', { name: 'Nazwa koperty' }), 'Jedzenie')
    await user.click(again.getByRole('radio', { name: 'Co miesiąc' }))
    await user.type(again.getByRole('textbox', { name: 'Kwota miesięcznie' }), '0')
    await user.click(again.getByRole('button', { name: 'Dodaj kopertę' }))
    expect(again.getByRole('textbox', { name: 'Kwota miesięcznie' })).toHaveAccessibleDescription('Kwota musi być większa od zera.')
    await user.clear(again.getByRole('textbox', { name: 'Kwota miesięcznie' }))
    await user.type(again.getByRole('textbox', { name: 'Kwota miesięcznie' }), '300')
    await user.click(again.getByRole('button', { name: 'Dodaj kopertę' }))
    expect(await again.findByText('Koperta o tej nazwie już istnieje.')).toBeInTheDocument()
    expect(created[1]?.goal).toEqual({ type: 'monthly', amount: { amount: '300.00', currency: 'PLN' } })
  })

  it('removes a goal when editing an envelope', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Jedzenie' }))
    await user.click(within(screen.getByRole('dialog', { name: 'Jedzenie' })).getByRole('button', { name: 'Edytuj' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edycja koperty Jedzenie' }))
    expect(dialog.getByRole('textbox', { name: 'Kwota miesięcznie' })).toHaveValue('800,00')
    await user.click(dialog.getByRole('radio', { name: 'Bez celu' }))
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(patches).toHaveLength(1))
    expect(patches[0]).toEqual({ id: 'e-jedzenie', body: { name: 'Jedzenie', hidden: false, goal: null } })
  })

  it('explains a 409 envelope-in-use and offers to hide the envelope', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Rozrywka' }))
    await user.click(within(screen.getByRole('dialog', { name: 'Rozrywka' })).getByRole('button', { name: 'Usuń' }))
    const confirm = within(screen.getByRole('alertdialog', { name: 'Usunąć kopertę Rozrywka?' }))
    await user.click(confirm.getByRole('button', { name: 'Usuń kopertę' }))
    expect(await confirm.findByText('Tej koperty nie da się usunąć')).toBeInTheDocument()
    await user.click(confirm.getByRole('button', { name: 'Ukryj kopertę' }))
    await waitFor(() => expect(patches).toEqual([{ id: 'e-rozrywka', body: { hidden: true } }]))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it('explains a 409 group-not-empty and reorders envelopes with buttons', async () => {
    const { user } = await open()
    await user.click(screen.getByRole('button', { name: 'Układ' }))
    expect(screen.getByRole('button', { name: 'Zakończ układanie' })).toHaveAttribute('aria-pressed', 'true')
    // W trybie układu widać też ukryte koperty.
    expect(screen.getByRole('article', { name: 'Koperta Stare' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Przesuń kopertę Jedzenie w dół' }))
    await waitFor(() => expect(patches).toHaveLength(2))
    expect(patches).toEqual([
      { id: 'e-rozrywka', body: { position: 0 } },
      { id: 'e-jedzenie', body: { position: 1 } },
    ])
    expect(screen.getByRole('button', { name: 'Przesuń kopertę Jedzenie w górę' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: 'Usuń grupę Codzienne' }))
    const confirm = within(screen.getByRole('alertdialog', { name: 'Usunąć grupę Codzienne?' }))
    await user.click(confirm.getByRole('button', { name: 'Usuń grupę' }))
    expect(await confirm.findByText('W grupie są koperty')).toBeInTheDocument()
    expect(confirm.getByRole('button', { name: 'Ukryj koperty grupy (2)' })).toBeInTheDocument()
  })

  it('is read-only for a viewer', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    await open()
    expect(screen.queryByRole('textbox', { name: /Przydzielono/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Przenieś środki' })).not.toBeInTheDocument()
    expect(plain(screen.getByRole('article', { name: 'Koperta Jedzenie' }).textContent)).toContain('800,00 zł')
  })

  it('opens the current budget month by default and rejects a bad month', async () => {
    renderApp('/h/h-dom/budget/2026-13')
    expect(await screen.findByText('Nie ma takiego miesiąca')).toBeInTheDocument()
  })

  it('offers to create the first group when the budget is empty', async () => {
    server.use(http.get(apiUrl('/households/:hid/budget/months/:month'), () => HttpResponse.json(f.budgetMonth())))
    renderApp('/h/h-dom/budget/2026-10')
    expect(await screen.findByText('Załóż pierwsze koperty')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Nowa grupa kopert' })).toBeInTheDocument()
  })

  it('speaks English after switching the language', async () => {
    setLocale('en')
    const { user } = renderApp('/h/h-dom/budget/2026-10')
    const food = within(await screen.findByRole('article', { name: 'Envelope Jedzenie' }))
    expect(screen.getByRole('heading', { level: 1, name: 'Budget' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /^Previous month: / })).toHaveAttribute('href', '/h/h-dom/budget/2026-09')
    expect(plain(screen.getByRole('group', { name: 'Ready to Assign' }).textContent)).toContain('3,850.00')
    const notes = within(screen.getByRole('list', { name: 'Notes for the month' }))
    expect(notes.getByText(/2 transactions are waiting for an exchange rate/)).toBeInTheDocument()
    expect(plain(notes.getByText(/Assigned to future months/).textContent)).toContain('100.00')
    // Pole przydziału pokazuje kwotę z kropką dziesiętną.
    expect(food.getByRole('textbox', { name: 'Assigned: Jedzenie' })).toHaveValue('800.00')
    expect(food.getByTitle(/Overspent/)).toHaveTextContent('(overspent)')
    expect(screen.getByRole('toolbar', { name: 'Budget actions' })).toBeInTheDocument()
    expect(screen.getByText('Outside envelopes')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Add an envelope to group Cele' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'New envelope' }))
    await user.click(dialog.getByRole('radio', { name: 'Amount by date' }))
    await user.click(dialog.getByRole('button', { name: 'Add envelope' }))
    expect(dialog.getByRole('textbox', { name: 'Envelope name' })).toHaveAccessibleDescription('Enter an envelope name.')
    expect(dialog.getByLabelText('By date')).toHaveAccessibleDescription('Enter the date by which you want to save the amount.')
  })
})
