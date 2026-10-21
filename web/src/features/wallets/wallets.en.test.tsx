import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const konto = f.wallet()
const lokata = f.wallet({ id: 'w-lokata', name: 'Lokata', type: 'savings', on_budget: false, visibility: 'private' })
const stary = f.wallet({ id: 'w-stary', name: 'Stary', archived_at: '2026-10-10T10:00:00Z' })

describe('wallets in English', () => {
  beforeEach(() => {
    server.use(
      http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto, lokata] })),
      http.get(apiUrl('/households/:hid/wallets/:wid'), ({ params }) =>
        HttpResponse.json(params.wid === 'w-stary' ? stary : konto),
      ),
      http.get(apiUrl('/households/:hid/transactions'), () => HttpResponse.json({ items: [], next_cursor: null })),
    )
  })

  it('shows the wallet list, totals and the new wallet form', async () => {
    setLocale('en')
    const { user } = renderApp('/h/h-dom/wallets')
    expect(await screen.findByRole('heading', { level: 1, name: 'Wallets' })).toBeInTheDocument()
    expect(document.title).toBe('Wallets · Denar')
    const totals = within(await screen.findByRole('region', { name: 'Wallet totals' }))
    expect(totals.getByText('In budget wallets')).toBeInTheDocument()
    expect(totals.getByText('Active wallets: 2')).toBeInTheDocument()
    const card = within(screen.getByRole('article', { name: 'Wallet Lokata' }))
    expect(card.getByText('Off budget')).toBeInTheDocument()
    expect(card.getByText('Private')).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Show archived' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'New wallet' }))
    const dialog = within(screen.getByRole('dialog', { name: 'New wallet' }))
    expect(dialog.getByRole('textbox', { name: 'Name' })).toBeInTheDocument()
    expect(dialog.getByText('Who can see the wallet')).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: 'Add wallet' }))
    expect(await dialog.findByText('Enter a wallet name.')).toBeInTheDocument()
  })

  it('shows an archived wallet page', async () => {
    setLocale('en')
    renderApp('/h/h-dom/wallets/w-stary')
    expect(await screen.findByRole('heading', { level: 1, name: 'Stary' })).toBeInTheDocument()
    expect(screen.getByText('The wallet is archived: it does not accept new transactions or transfers.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Restore' })).toBeInTheDocument()
    expect(screen.getByText('Balance')).toBeInTheDocument()
    expect(screen.getByText('Archived')).toBeInTheDocument()
  })
})
