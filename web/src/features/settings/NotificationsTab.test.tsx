import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Channel, NotificationPreference } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const path = '/h/h-dom/settings/notifications'

function channelsApi(initial: Channel[]) {
  let items = [...initial]
  const sent: { method: string; path: string; body?: unknown }[] = []
  server.use(
    http.get(apiUrl('/me/channels'), () => HttpResponse.json({ items })),
    http.post(apiUrl('/me/channels'), async ({ request }) => {
      const body = (await request.json()) as Partial<Channel> & { secrets?: Record<string, string> }
      sent.push({ method: 'POST', path: '/me/channels', body })
      const created = f.channel({
        id: 'c-new',
        type: body.type!,
        name: body.name!,
        config: body.config ?? {},
        has_secret: !!body.secrets,
        enabled: body.enabled ?? true,
      })
      items = [...items, created]
      return HttpResponse.json(created, { status: 201 })
    }),
    http.patch(apiUrl('/me/channels/:id'), async ({ params, request }) => {
      const body = (await request.json()) as Partial<Channel> & { secrets?: Record<string, string> | null }
      sent.push({ method: 'PATCH', path: `/me/channels/${String(params.id)}`, body })
      items = items.map((c) =>
        c.id === params.id
          ? {
              ...c,
              ...(body.name !== undefined ? { name: body.name } : {}),
              ...(body.enabled !== undefined ? { enabled: body.enabled } : {}),
              ...(body.config ? { config: body.config } : {}),
              ...(body.secrets !== undefined ? { has_secret: body.secrets !== null } : {}),
            }
          : c,
      )
      return HttpResponse.json(items.find((c) => c.id === params.id)!)
    }),
    http.delete(apiUrl('/me/channels/:id'), ({ params }) => {
      sent.push({ method: 'DELETE', path: `/me/channels/${String(params.id)}` })
      items = items.filter((c) => c.id !== params.id)
      return new HttpResponse(null, { status: 204 })
    }),
  )
  return sent
}

describe('NotificationsTab — channels', () => {
  it('builds the form from the channel types; unavailable ones are disabled with the reason', async () => {
    const sent = channelsApi([])
    const { user } = renderApp(path)
    expect(await screen.findByRole('heading', { name: 'Nie masz jeszcze kanałów' })).toBeInTheDocument()
    expect(screen.getByRole('list', { name: 'Niedostępne typy kanałów' })).toHaveTextContent(
      'E-mail niedostępny — Serwer nie ma skonfigurowanej poczty (SMTP).',
    )
    await user.click(screen.getAllByRole('button', { name: 'Dodaj kanał' })[0]!)
    const dialog = await screen.findByRole('dialog', { name: 'Nowy kanał powiadomień' })
    const email = within(dialog).getByRole('radio', { name: /E-mail/ })
    expect(email).toBeDisabled()
    expect(email).toHaveAccessibleName(/Niedostępny: Serwer nie ma skonfigurowanej poczty/)
    // Pierwszy dostępny typ — webhook.
    expect(within(dialog).getByRole('radio', { name: /Webhook/ })).toBeChecked()
    expect(within(dialog).getByText(/X-Denar-Signature: sha256=/)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('radio', { name: /Telegram/ }))
    expect(within(dialog).queryByRole('textbox', { name: 'Adres URL' })).not.toBeInTheDocument()
    expect(within(dialog).getByText(/getUpdates/)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Dodaj kanał' }))
    expect(within(dialog).getByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription(/Podaj nazwę kanału/)
    expect(within(dialog).getByRole('textbox', { name: 'chat_id' })).toHaveAccessibleDescription(/To pole jest wymagane/)
    const token = within(dialog).getByLabelText('Token bota')
    expect(token).toHaveAttribute('type', 'password')
    expect(token).toHaveAccessibleDescription(/To pole jest wymagane/)

    await user.type(within(dialog).getByRole('textbox', { name: 'Nazwa' }), 'Telegram Ady')
    await user.type(within(dialog).getByRole('textbox', { name: 'chat_id' }), '123456789')
    await user.type(token, '123:ABC')
    await user.click(within(dialog).getByRole('button', { name: 'Dodaj kanał' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(sent[0]).toEqual({
      method: 'POST',
      path: '/me/channels',
      body: { type: 'telegram', name: 'Telegram Ady', config: { chat_id: '123456789' }, secrets: { bot_token: '123:ABC' }, enabled: true },
    })
    const row = await screen.findByRole('article', { name: 'Telegram Ady' })
    expect(row).toHaveTextContent('Niezweryfikowany')
    expect(row).toHaveTextContent('z sekretem')
  })

  it('keeps, replaces or removes the stored secret when editing', async () => {
    const sent = channelsApi([f.channel({ has_secret: true })])
    const { user } = renderApp(path)
    await user.click(await screen.findByRole('button', { name: 'Edytuj: Mój webhook' }))
    let dialog = await screen.findByRole('dialog', { name: 'Edycja kanału Mój webhook' })
    expect(within(dialog).getByRole('radio', { name: /Zostaw bez zmian/ })).toBeChecked()
    expect(within(dialog).queryByLabelText(/Sekret podpisu/)).not.toBeInTheDocument()
    // Bez zmian — nic nie idzie do serwera.
    await user.click(within(dialog).getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(sent).toEqual([])

    await user.click(screen.getByRole('button', { name: 'Edytuj: Mój webhook' }))
    dialog = await screen.findByRole('dialog', { name: 'Edycja kanału Mój webhook' })
    await user.click(within(dialog).getByRole('radio', { name: /Zmień/ }))
    await user.type(within(dialog).getByLabelText(/Sekret podpisu/), 'nowy-sekret')
    await user.click(within(dialog).getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(sent[0]!.body).toEqual({ secrets: { secret: 'nowy-sekret' } })

    await user.click(await screen.findByRole('button', { name: 'Edytuj: Mój webhook' }))
    dialog = await screen.findByRole('dialog', { name: 'Edycja kanału Mój webhook' })
    await user.click(within(dialog).getByRole('radio', { name: /Usuń/ }))
    const url = within(dialog).getByRole('textbox', { name: 'Adres URL' })
    await user.clear(url)
    await user.type(url, 'https://inny.test/hook')
    await user.click(within(dialog).getByRole('button', { name: 'Zapisz zmiany' }))
    await waitFor(() => expect(sent).toHaveLength(2))
    expect(sent[1]!.body).toEqual({ config: { url: 'https://inny.test/hook' }, secrets: null })
    const row = await screen.findByRole('article', { name: 'Mój webhook' })
    await waitFor(() => expect(row).not.toHaveTextContent('z sekretem'))
  })

  it('does not offer removing a required secret', async () => {
    channelsApi([f.channel({ id: 'c-tg', type: 'telegram', name: 'TG', config: { chat_id: '1' }, has_secret: true })])
    const { user } = renderApp(path)
    await user.click(await screen.findByRole('button', { name: 'Edytuj: TG' }))
    const dialog = await screen.findByRole('dialog', { name: 'Edycja kanału TG' })
    expect(within(dialog).getByRole('radio', { name: /Zmień/ })).toBeInTheDocument()
    expect(within(dialog).queryByRole('radio', { name: /Usuń/ })).not.toBeInTheDocument()
  })

  it('sends a test and shows the result, toggles and deletes', async () => {
    const sent = channelsApi([f.channel({ last_error: 'HTTP 500', last_error_at: '2026-10-21T09:00:00Z' })])
    let ok = false
    server.use(
      http.post(apiUrl('/me/channels/:id/test'), () =>
        HttpResponse.json({ ok, error: ok ? null : 'HTTP 404 Not Found', tested_at: '2026-10-21T10:00:00Z' }),
      ),
    )
    const { user } = renderApp(path)
    const row = await screen.findByRole('article', { name: 'Mój webhook' })
    expect(row).toHaveTextContent('Ostatni błąd')
    expect(row).toHaveTextContent('HTTP 500')
    await user.click(within(row).getByRole('button', { name: 'Wyślij test: Mój webhook' }))
    expect(await within(row).findByRole('status')).toHaveTextContent('Test nieudany: HTTP 404 Not Found')
    ok = true
    await user.click(within(row).getByRole('button', { name: 'Wyślij test: Mój webhook' }))
    await waitFor(() => expect(within(row).getByRole('status')).toHaveTextContent('Test wysłany'))

    await user.click(within(row).getByRole('button', { name: 'Wyłącz: Mój webhook' }))
    expect(await screen.findByText('Wyłączono kanał Mój webhook.')).toBeInTheDocument()
    expect(sent.at(-1)).toEqual({ method: 'PATCH', path: '/me/channels/c-hook', body: { enabled: false } })
    expect(await within(row).findByRole('button', { name: 'Włącz: Mój webhook' })).toBeInTheDocument()

    await user.click(within(row).getByRole('button', { name: 'Usuń: Mój webhook' }))
    const confirm = await screen.findByRole('alertdialog', { name: 'Usunąć kanał?' })
    await user.click(within(confirm).getByRole('button', { name: 'Usuń kanał' }))
    expect(await screen.findByRole('heading', { name: 'Nie masz jeszcze kanałów' })).toBeInTheDocument()
    expect(sent.at(-1)).toEqual({ method: 'DELETE', path: '/me/channels/c-hook' })
  })

  it('explains a server without a master key', async () => {
    channelsApi([])
    server.use(
      http.get(apiUrl('/me/channel-types'), () =>
        HttpResponse.json({
          items: f.channelTypes({
            telegram: { available: false, unavailable_reason: 'secrets_unavailable' },
            matrix: { available: false, unavailable_reason: 'secrets_unavailable' },
          }),
        }),
      ),
      http.post(apiUrl('/me/channels'), () => problemResponse(409, 'conflict', { reason: 'secrets-unavailable' })),
    )
    const { user } = renderApp(path)
    await user.click((await screen.findAllByRole('button', { name: 'Dodaj kanał' }))[0]!)
    const dialog = await screen.findByRole('dialog', { name: 'Nowy kanał powiadomień' })
    expect(within(dialog).getByRole('radio', { name: /Matrix/ })).toHaveAccessibleName(/DENAR_MASTER_KEY/)
    expect(within(dialog).getByLabelText(/Sekret podpisu/)).toBeDisabled()
    await user.type(within(dialog).getByRole('textbox', { name: 'Nazwa' }), 'Hook')
    await user.type(within(dialog).getByRole('textbox', { name: 'Adres URL' }), 'https://x.test')
    await user.click(within(dialog).getByRole('button', { name: 'Dodaj kanał' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(/nie ma klucza DENAR_MASTER_KEY/)
  })
})

describe('NotificationsTab — preferences', () => {
  it('saves the matrix of event types and channels', async () => {
    channelsApi([f.channel(), f.channel({ id: 'c-ntfy', type: 'ntfy', name: 'Telefon', enabled: false, config: { topic: 'dom' } })])
    let saved: unknown
    server.use(
      http.get(apiUrl('/me/households/:hid/notification-preferences'), () =>
        HttpResponse.json(f.preferences({ 'subscription.charge_missed': { channel_ids: ['c-ntfy'] } })),
      ),
      http.put(apiUrl('/me/households/:hid/notification-preferences'), async ({ request }) => {
        saved = await request.json()
        return HttpResponse.json(saved as { items: NotificationPreference[] })
      }),
    )
    const { user } = renderApp(path)
    const matrix = await screen.findByRole('table')
    expect(within(matrix).getAllByRole('columnheader').map((c) => c.textContent)).toEqual([
      'Zdarzenie',
      'W aplikacji',
      'Mój webhook',
      'Telefon (wyłączony)',
    ])
    expect(within(matrix).getByRole('checkbox', { name: 'Przegapiona płatność: Telefon' })).toBeChecked()
    const save = screen.getByRole('button', { name: 'Zapisz preferencje' })
    expect(save).toBeDisabled()
    await user.click(within(matrix).getByRole('checkbox', { name: 'Przekroczona koperta: Mój webhook' }))
    await user.click(within(matrix).getByRole('checkbox', { name: 'Zmiana ceny subskrypcji: w aplikacji' }))
    await user.click(save)
    expect(await screen.findByText('Zapisano preferencje powiadomień.')).toBeInTheDocument()
    expect(saved).toEqual(
      f.preferences({
        'subscription.charge_missed': { channel_ids: ['c-ntfy'] },
        'envelope.overspent': { channel_ids: ['c-hook'] },
        'subscription.price_changed': { in_app: false },
      }),
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Zapisz preferencje' })).toBeDisabled())
  })

  it('is there for a viewer too', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    renderApp(path)
    expect(await screen.findByRole('heading', { name: 'O czym i gdzie powiadamiać' })).toBeInTheDocument()
    expect(await screen.findByRole('checkbox', { name: 'Zbliżające się odnowienie: w aplikacji' })).toBeChecked()
  })
})

describe('InvitationsTab — e-mail', () => {
  it('sends the link by e-mail when the server has SMTP and reports the result', async () => {
    let body: unknown
    server.use(
      http.get(apiUrl('/me/channel-types'), () =>
        HttpResponse.json({ items: f.channelTypes({ email: { available: true, unavailable_reason: null } }) }),
      ),
      http.post(apiUrl('/households/:hid/invitations'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(
          {
            invitation: f.invitation({ email: 'jan@example.test' }),
            token: 'tok',
            url: 'http://denar.test/invite#tok',
            email_sent: false,
            email_error: 'dial tcp: connection refused',
          },
          { status: 201, headers: { 'Cache-Control': 'no-store' } },
        )
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/invitations')
    await user.type(await screen.findByRole('textbox', { name: 'E-mail' }), 'jan@example.test')
    expect(await screen.findByRole('checkbox', { name: 'Wyślij link e-mailem' })).toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Utwórz i wyślij' }))
    const dialog = await screen.findByRole('dialog', { name: 'Link z zaproszeniem' })
    expect(body).toEqual({ email: 'jan@example.test', role: 'member', send_email: true })
    expect(dialog).toHaveTextContent('Nie udało się wysłać e-maila')
    expect(dialog).toHaveTextContent('connection refused')
  })

  it('hides the option without SMTP', async () => {
    renderApp('/h/h-dom/settings/invitations')
    expect(await screen.findByRole('button', { name: 'Utwórz link' })).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: 'Wyślij link e-mailem' })).not.toBeInTheDocument()
  })
})
