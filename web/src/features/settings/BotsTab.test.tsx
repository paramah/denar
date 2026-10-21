import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { Bot, BotCreate } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { botStatus } from './BotsTab.tsx'

const noStore = { 'Cache-Control': 'no-store' }

describe('BotsTab', () => {
  let bots: Bot[]
  let created: BotCreate | undefined

  beforeEach(() => {
    bots = []
    created = undefined
    server.use(
      http.get(apiUrl('/households/:hid/bots'), () => HttpResponse.json({ items: bots })),
      http.post(apiUrl('/households/:hid/bots'), async ({ request }) => {
        created = (await request.json()) as BotCreate
        if (created.name === 'Zajęta') {
          return problemResponse(409, 'conflict', { reason: 'duplicate-name' })
        }
        const bot = f.bot({
          name: created.name,
          scopes: created.scopes as Bot['scopes'],
          effective_scopes: created.scopes as Bot['scopes'],
          expires_at: created.expires_at ?? null,
        })
        bots = [bot]
        return HttpResponse.json({ bot, token: 'dnr_sekret_raz' }, { status: 201, headers: noStore })
      }),
    )
  })

  it('creates a bot with grouped scopes and shows its token once', async () => {
    const { user } = renderApp('/h/h-dom/settings/bots')
    await user.click(await screen.findByRole('button', { name: 'Utwórz pierwszego bota' }))
    const dialog = screen.getByRole('dialog', { name: 'Nowy bot' })
    const form = within(dialog)

    // Read-only by default; writing implies reading.
    expect(form.getByRole('checkbox', { name: 'Portfele: odczyt' })).toBeChecked()
    expect(form.getByRole('checkbox', { name: 'Transakcje: zapis' })).not.toBeChecked()
    await user.click(form.getByRole('checkbox', { name: 'Budżet: odczyt' }))
    await user.click(form.getByRole('checkbox', { name: 'Budżet: zapis' }))
    expect(form.getByRole('checkbox', { name: 'Budżet: odczyt' })).toBeChecked()
    expect(form.getByRole('checkbox', { name: 'Token wygasa' })).toBeChecked()

    await user.type(form.getByRole('textbox', { name: 'Nazwa' }), 'Claude')
    await user.click(form.getByRole('button', { name: 'Utwórz bota' }))

    const tokenDialog = await screen.findByRole('dialog', { name: 'Token bota Claude' })
    expect(within(tokenDialog).getByRole('textbox', { name: 'Token' })).toHaveValue('dnr_sekret_raz')
    expect(created?.scopes).toEqual([
      'wallets:read',
      'transactions:read',
      'budget:read',
      'budget:write',
      'subscriptions:read',
      'reports:read',
      'notifications:read',
    ])
    // About 90 days ahead.
    const days = (new Date(created!.expires_at!).getTime() - Date.now()) / 86_400_000
    expect(days).toBeGreaterThan(89)
    expect(days).toBeLessThan(91)

    await user.click(within(tokenDialog).getByRole('button', { name: 'Zapisałem token' }))
    expect(screen.queryByDisplayValue('dnr_sekret_raz')).not.toBeInTheDocument()
    expect(await screen.findByRole('article', { name: 'Bot Claude' })).toBeInTheDocument()
  })

  it('creates a bot without expiry and validates the form', async () => {
    const { user } = renderApp('/h/h-dom/settings/bots')
    await user.click(await screen.findByRole('button', { name: 'Utwórz pierwszego bota' }))
    const form = within(screen.getByRole('dialog', { name: 'Nowy bot' }))
    for (const box of form.getAllByRole('checkbox', { checked: true })) {
      if (box.getAttribute('aria-label')?.endsWith('odczyt')) {
        await user.click(box)
      }
    }
    await user.click(form.getByRole('button', { name: 'Utwórz bota' }))
    expect(form.getByRole('textbox', { name: 'Nazwa' })).toHaveAccessibleDescription(/Podaj nazwę bota/)
    expect(form.getByRole('alert')).toHaveTextContent('Wybierz co najmniej jedno uprawnienie.')
    expect(created).toBeUndefined()

    await user.type(form.getByRole('textbox', { name: 'Nazwa' }), 'Zajęta')
    await user.click(form.getByRole('button', { name: 'Tylko odczyt' }))
    await user.click(form.getByRole('checkbox', { name: 'Token wygasa' }))
    await user.click(form.getByRole('button', { name: 'Utwórz bota' }))
    expect(await form.findByText('Bot o tej nazwie już istnieje. Wybierz inną.')).toBeInTheDocument()
    expect(created?.expires_at).toBeNull()
  })

  it('disables, rotates and deletes a bot', async () => {
    bots = [f.bot()]
    let rotated = false
    server.use(
      http.patch(apiUrl('/households/:hid/bots/:bid'), async ({ request }) => {
        const body = (await request.json()) as { disabled: boolean }
        bots = [f.bot({ disabled: body.disabled })]
        return HttpResponse.json(bots[0])
      }),
      http.post(apiUrl('/households/:hid/bots/:bid/token'), () => {
        rotated = true
        return HttpResponse.json({ bot: f.bot({ token_prefix: 'dnr_cd34' }), token: 'dnr_nowy' }, { headers: noStore })
      }),
      http.delete(apiUrl('/households/:hid/bots/:bid'), () => {
        bots = []
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/bots')
    const row = await screen.findByRole('article', { name: 'Bot Claude' })
    expect(within(row).getByText('Aktywny')).toBeInTheDocument()

    await user.click(within(row).getByRole('button', { name: 'Wyłącz Claude' }))
    expect(await within(row).findByText('Wyłączony')).toBeInTheDocument()

    await user.click(within(row).getByRole('button', { name: 'Nowy token dla Claude' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Wygeneruj nowy token' }))
    const tokenDialog = await screen.findByRole('dialog', { name: 'Token bota Claude' })
    expect(within(tokenDialog).getByRole('textbox', { name: 'Token' })).toHaveValue('dnr_nowy')
    expect(rotated).toBe(true)
    await user.click(within(tokenDialog).getByRole('button', { name: 'Zapisałem token' }))

    await user.click(within(row).getByRole('button', { name: 'Usuń Claude' }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Usuń bota' }))
    expect(await screen.findByRole('heading', { name: 'Nie ma jeszcze botów' })).toBeInTheDocument()
  })

  it('marks scopes the creator no longer has', async () => {
    bots = [f.bot({ scopes: ['reports:read', 'transactions:write'], effective_scopes: ['reports:read'] })]
    renderApp('/h/h-dom/settings/bots')
    const row = await screen.findByRole('article', { name: 'Bot Claude' })
    expect(within(row).getByText(/Transakcje: zapis/)).toHaveTextContent('(nieaktywne)')
  })
})

describe('botStatus', () => {
  const now = new Date('2026-10-21T00:00:00Z')
  it.each([
    [f.bot({ disabled: true }), 'Wyłączony'],
    [f.bot({ expires_at: '2026-10-20T00:00:00Z' }), 'Wygasł'],
    [f.bot({ effective_scopes: [] }), 'Bez uprawnień'],
    [f.bot(), 'Aktywny'],
  ])('%#', (bot, label) => {
    expect(botStatus(bot, now).label).toBe(label)
  })
})

describe('bots for a member', () => {
  it('still offers creating own bots', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'member' })]))))
    renderApp('/h/h-dom/settings/bots')
    await waitFor(() => expect(screen.getAllByRole('button', { name: /Nowy bot|Utwórz pierwszego bota/ }).length).toBe(2))
  })
})

describe('BotsTab — wallets', () => {
  let created: BotCreate | undefined
  let patched: Record<string, unknown> | undefined
  let current: Bot

  beforeEach(() => {
    created = undefined
    patched = undefined
    current = f.bot({ wallet_ids: ['w-konto', 'w-ukryty'] })
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () =>
        HttpResponse.json({ items: [f.wallet(), f.wallet({ id: 'w-usd', name: 'Dolary', currency: 'USD', balance: f.money('0.00', 'USD'), cleared_balance: f.money('0.00', 'USD') })] }),
      ),
      http.get(apiUrl('/households/:hid/bots'), () => HttpResponse.json({ items: [current] })),
      http.post(apiUrl('/households/:hid/bots'), async ({ request }) => {
        created = (await request.json()) as BotCreate
        const bot = f.bot({ id: 'b-new', name: created.name, wallet_ids: created.wallet_ids ?? [] })
        return HttpResponse.json({ bot, token: 'dnr_x' }, { status: 201, headers: noStore })
      }),
      http.patch(apiUrl('/households/:hid/bots/:bid'), async ({ request }) => {
        patched = (await request.json()) as Record<string, unknown>
        current = { ...current, wallet_ids: (patched.wallet_ids as string[]) ?? current.wallet_ids }
        return HttpResponse.json(current)
      }),
    )
  })

  it('shows the wallet restriction and changes it', async () => {
    const { user } = renderApp('/h/h-dom/settings/bots')
    const row = await screen.findByRole('article', { name: 'Bot Claude' })
    await waitFor(() => expect(row).toHaveTextContent('Portfele: Konto, niewidoczne dla Ciebie: 1'))
    await user.click(within(row).getByRole('button', { name: 'Portfele bota Claude' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Portfele bota Claude' }))
    expect(dialog.getByRole('checkbox', { name: 'Konto (PLN)' })).toBeChecked()
    expect(dialog.getByText(/niewidoczne dla Ciebie: 1/)).toBeInTheDocument()
    await user.click(dialog.getByRole('checkbox', { name: 'Dolary (USD)' }))
    await user.click(dialog.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(patched).toEqual({ wallet_ids: ['w-konto', 'w-ukryty', 'w-usd'] }))

    await user.click(within(row).getByRole('button', { name: 'Portfele bota Claude' }))
    const again = within(screen.getByRole('dialog', { name: 'Portfele bota Claude' }))
    await user.click(again.getByRole('checkbox', { name: 'Tylko wybrane portfele' }))
    await user.click(again.getByRole('button', { name: 'Zapisz' }))
    await waitFor(() => expect(patched).toEqual({ wallet_ids: [] }))
    await waitFor(() => expect(row).toHaveTextContent('Portfele: wszystkie widoczne dla twórcy'))
  })

  it('limits a new bot to chosen wallets', async () => {
    const { user } = renderApp('/h/h-dom/settings/bots')
    await user.click(await screen.findByRole('button', { name: 'Nowy bot' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Nowy bot' }))
    await user.type(dialog.getByRole('textbox', { name: 'Nazwa' }), 'Import')
    await user.click(dialog.getByRole('checkbox', { name: 'Tylko wybrane portfele' }))
    await user.click(dialog.getByRole('button', { name: 'Utwórz bota' }))
    expect(await dialog.findByText('Wybierz co najmniej jeden portfel albo pozwól na wszystkie.')).toBeInTheDocument()
    expect(created).toBeUndefined()
    await user.click(dialog.getByRole('checkbox', { name: 'Dolary (USD)' }))
    await user.click(dialog.getByRole('button', { name: 'Utwórz bota' }))
    await waitFor(() => expect(created?.wallet_ids).toEqual(['w-usd']))
  })
})
