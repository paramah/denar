import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Role } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { assignableRoles } from './MembersTab.tsx'

function asRole(role: Role) {
  server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role })]))))
}

describe('settings tabs', () => {
  it.each<[Role, string[]]>([
    ['owner', ['Gospodarstwo', 'Członkowie', 'Zaproszenia', 'Boty', 'Kontrahenci', 'Kursy walut', 'Powiadomienia', 'Połączenia', 'Dziennik zmian']],
    ['admin', ['Gospodarstwo', 'Członkowie', 'Zaproszenia', 'Boty', 'Kontrahenci', 'Kursy walut', 'Powiadomienia', 'Połączenia', 'Dziennik zmian']],
    ['member', ['Gospodarstwo', 'Członkowie', 'Boty', 'Kontrahenci', 'Kursy walut', 'Powiadomienia']],
    ['viewer', ['Gospodarstwo', 'Członkowie', 'Kontrahenci', 'Kursy walut', 'Powiadomienia']],
  ])('%s sees %j', async (role, tabs) => {
    asRole(role)
    renderApp('/h/h-dom/settings')
    const nav = await screen.findByRole('navigation', { name: 'Ustawienia' })
    expect(within(nav).getAllByRole('link').map((link) => link.textContent)).toEqual(tabs)
  })

  it('sends a member away from the audit log', async () => {
    asRole('member')
    const { router } = renderApp('/h/h-dom/settings/audit')
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/settings/general'))
  })
})

describe('GeneralTab', () => {
  it('saves the changed settings', async () => {
    let sent: unknown
    server.use(
      http.patch(apiUrl('/households/:hid'), async ({ request }) => {
        sent = await request.json()
        return HttpResponse.json(f.household({ name: 'Nowy dom' }))
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/general')
    const name = await screen.findByRole('textbox', { name: 'Nazwa' })
    expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeDisabled()
    await user.clear(name)
    await user.type(name, 'Nowy dom')
    await user.click(screen.getByRole('button', { name: 'Zapisz zmiany' }))
    expect(await screen.findByText('Zapisano ustawienia gospodarstwa.')).toBeInTheDocument()
    expect(sent).toEqual({ name: 'Nowy dom', timezone: 'Europe/Warsaw', month_start_day: 1 })
  })

  it('is read-only for a viewer, with the currency locked for everyone', async () => {
    asRole('viewer')
    renderApp('/h/h-dom/settings/general')
    expect(await screen.findByRole('textbox', { name: 'Nazwa' })).toHaveAttribute('readonly')
    expect(screen.getByRole('textbox', { name: 'Waluta bazowa' })).toHaveAttribute('readonly')
    expect(screen.queryByRole('button', { name: 'Zapisz zmiany' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Usuń gospodarstwo' })).not.toBeInTheDocument()
  })

  it('deletes the household only after its name is typed', async () => {
    let deleted = false
    server.use(
      http.delete(apiUrl('/households/:hid'), () => {
        deleted = true
        return new HttpResponse(null, { status: 204 })
      }),
      http.get(apiUrl('/me'), () => HttpResponse.json(f.me(deleted ? [] : [f.household()]))),
    )
    const { user, router } = renderApp('/h/h-dom/settings/general')
    await user.click(await screen.findByRole('button', { name: 'Usuń gospodarstwo' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Usunąć gospodarstwo Dom?' })
    const confirm = within(dialog).getByRole('button', { name: 'Usuń na zawsze' })
    expect(confirm).toBeDisabled()
    await user.type(within(dialog).getByRole('textbox'), 'Dom')
    await user.click(confirm)
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(deleted).toBe(true)
    expect(await screen.findByRole('heading', { name: 'Witaj, Ada Kowalska' })).toBeInTheDocument()
  })
})

describe('MembersTab', () => {
  it('knows which roles an actor may give', () => {
    expect(assignableRoles('owner', f.member({ role: 'owner' }))).toEqual(['owner', 'admin', 'member', 'viewer'])
    expect(assignableRoles('admin', f.member({ role: 'member' }))).toEqual(['admin', 'member', 'viewer'])
    expect(assignableRoles('admin', f.member({ role: 'owner' }))).toEqual([])
    expect(assignableRoles('member', f.member({ role: 'viewer' }))).toEqual([])
  })

  it('changes a role and explains the last-owner rule', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/members'), () =>
        HttpResponse.json({
          items: [f.member(), f.member({ user_id: 'u-jan', name: 'Jan Nowak', email: 'jan@example.test', role: 'member' })],
        }),
      ),
      http.patch(apiUrl('/households/:hid/members/:uid'), async ({ params, request }) => {
        if (params.uid === 'u-ada') {
          return problemResponse(409, 'conflict', { reason: 'last-owner' })
        }
        const { role } = (await request.json()) as { role: Role }
        return HttpResponse.json(f.member({ user_id: 'u-jan', name: 'Jan Nowak', email: 'jan@example.test', role }))
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/members')
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Rola: Jan Nowak' }), 'admin')
    expect(await screen.findByText('Jan Nowak: rola administrator.')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Rola: Ada Kowalska' }), 'member')
    expect(await screen.findByText(/musi mieć co najmniej jednego właściciela/)).toBeInTheDocument()
  })

  it('lets a viewer only leave', async () => {
    asRole('viewer')
    server.use(
      http.get(apiUrl('/households/:hid/members'), () =>
        HttpResponse.json({
          items: [f.member({ role: 'viewer' }), f.member({ user_id: 'u-jan', name: 'Jan Nowak', role: 'owner' })],
        }),
      ),
    )
    renderApp('/h/h-dom/settings/members')
    expect(await screen.findByRole('button', { name: 'Opuść gospodarstwo' })).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Usuń/ })).not.toBeInTheDocument()
  })
})

describe('InvitationsTab', () => {
  it('creates an invitation and shows its link once', async () => {
    let list = [] as ReturnType<typeof f.invitation>[]
    server.use(
      http.get(apiUrl('/households/:hid/invitations'), () => HttpResponse.json({ items: list })),
      http.post(apiUrl('/households/:hid/invitations'), async ({ request }) => {
        const body = (await request.json()) as { email: string; role: Role }
        const created = f.invitation({ email: body.email, role: body.role })
        list = [created]
        return HttpResponse.json(
          { invitation: created, token: 'tok', url: 'http://denar.test/invite#tok' },
          { status: 201, headers: { 'Cache-Control': 'no-store' } },
        )
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/invitations')
    expect(await screen.findByRole('heading', { name: 'Nie ma jeszcze zaproszeń' })).toBeInTheDocument()
    await user.type(screen.getByRole('textbox', { name: 'E-mail' }), 'jan@example.test')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Rola' }), 'viewer')
    await user.click(screen.getByRole('button', { name: 'Utwórz link' }))

    const dialog = await screen.findByRole('dialog', { name: 'Link z zaproszeniem' })
    expect(within(dialog).getByRole('textbox', { name: 'Link z zaproszeniem' })).toHaveValue('http://denar.test/invite#tok')
    await user.click(within(dialog).getByRole('button', { name: 'Gotowe' }))
    expect(screen.queryByDisplayValue('http://denar.test/invite#tok')).not.toBeInTheDocument()
    expect(await screen.findByText('Oczekuje')).toBeInTheDocument()
  })
})
