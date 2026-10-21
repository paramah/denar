import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import { scopeGroups, scopeLabel } from '../../lib/scopes.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { botStatus, botWalletsLabel } from './BotsTab.tsx'
import { examplePrompts } from './AssistantConnect.tsx'

describe('settings in English', () => {
  it('shows the tabs and the household form', async () => {
    setLocale('en')
    renderApp('/h/h-dom/settings/general')
    const nav = await screen.findByRole('navigation', { name: 'Settings' })
    expect(within(nav).getAllByRole('link').map((link) => link.textContent)).toEqual([
      'Household',
      'Members',
      'Invitations',
      'Bots',
      'Payees',
      'Exchange rates',
      'Notifications',
      'Connections',
      'Change log',
    ])
    expect(screen.getByRole('heading', { name: 'Household details' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Name' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Base currency' })).toHaveAttribute('readonly')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(screen.getByRole('heading', { name: 'Delete household' })).toBeInTheDocument()
    expect(document.title).toBe('Household settings · Denar')
  })

  it('shows a bot row and the creation form with scope labels', async () => {
    setLocale('en')
    server.use(http.get(apiUrl('/households/:hid/bots'), () => HttpResponse.json({ items: [f.bot()] })))
    const { user } = renderApp('/h/h-dom/settings/bots')
    const row = await screen.findByRole('article', { name: 'Bot Claude' })
    expect(row).toHaveTextContent('Token dnr_ab12…, created')
    expect(row).toHaveTextContent('not used yet')
    expect(row).toHaveTextContent('Wallets: all visible to the creator')
    expect(within(row).getByText('Active')).toBeInTheDocument()
    expect(within(row).getByRole('button', { name: 'Disable Claude' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'New bot' }))
    const form = within(screen.getByRole('dialog', { name: 'New bot' }))
    expect(form.getByRole('checkbox', { name: 'Wallets: read' })).toBeChecked()
    expect(form.getByRole('checkbox', { name: 'Transactions: write' })).not.toBeChecked()
    expect(form.getByRole('checkbox', { name: 'Token expires' })).toBeChecked()
    expect(form.getByText('Recommended. By default after 90 days.')).toBeInTheDocument()
    expect(form.getByRole('button', { name: 'Create bot' })).toBeInTheDocument()
  })

  it('translates helpers outside components at call time', () => {
    setLocale('en')
    expect(scopeLabel('budget:write')).toBe('Budget: write')
    expect(scopeGroups[0]?.description).toBe('Accounts, cash, cards and balances')
    expect(botStatus(f.bot({ disabled: true })).label).toBe('Disabled')
    expect(botWalletsLabel(f.bot({ wallet_ids: ['w-x'] }), [])).toBe('hidden from you: 1')
    expect(examplePrompts(['wallets:read'])).toEqual(['What are my wallet balances?'])
  })

  it('shows the change log filters', async () => {
    setLocale('en')
    renderApp('/h/h-dom/settings/audit')
    expect(await screen.findByRole('heading', { name: 'Change log' })).toBeInTheDocument()
    const who = screen.getByRole('combobox', { name: 'Who' })
    expect(within(who).getByRole('option', { name: 'Everyone' })).toBeInTheDocument()
  })
})
