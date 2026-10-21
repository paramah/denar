import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import { cycleLabel, cycleNote, reminderLabel, relativeDay } from '../../lib/subscriptions.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

// Subskrypcje po angielsku (013): ekran i funkcje cyklu w drugim języku.
// Setup testów przywraca polski po każdym teście.

const plain = (value: string | null | undefined) => (value ?? '').replace(/[\u00a0\u202f]/g, ' ')

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-21T10:00:00Z'))
  setLocale('en')
  server.use(
    http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [f.wallet()] })),
    http.get(apiUrl('/households/:hid/subscriptions/s-openai'), () => HttpResponse.json(f.subscription())),
    http.get(apiUrl('/households/:hid/subscriptions/s-openai/prices'), () => HttpResponse.json({ items: [f.pricePoint()] })),
  )
})

afterEach(() => {
  vi.useRealTimers()
})

describe('subscriptions in English', () => {
  it('names cycles, reminders and relative days', () => {
    expect(cycleLabel({ unit: 'month', every: 1, anchor: '2026-01-31' })).toBe('monthly, day 31')
    expect(cycleLabel({ unit: 'week', every: 2, anchor: '2026-10-21' })).toBe('every 2 weeks, on Wednesday')
    expect(cycleLabel({ unit: 'year', every: 1, anchor: '2026-02-14' })).toBe('yearly, 14 February')
    expect(cycleLabel({ unit: 'day', every: 12, anchor: '2026-10-01' })).toBe('every 12 days')
    expect(cycleNote({ unit: 'month', anchor: '2026-01-31' })).toBe('In shorter months — the last day of the month.')
    expect(reminderLabel(1)).toBe('1 day before')
    expect(reminderLabel(3)).toBe('3 days before')
    expect(reminderLabel(7)).toBe('a week before')
    expect(relativeDay('2026-10-26', '2026-10-21')).toBe('in 5 days')
    expect(relativeDay('2026-10-20', '2026-10-21')).toBe('yesterday')
  })

  it('shows the empty overview', async () => {
    renderApp('/h/h-dom/subscriptions')
    expect(await screen.findByRole('heading', { name: "You aren't tracking any subscriptions yet" })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add your first subscription' })).toBeInTheDocument()
    const tabs = screen.getByRole('navigation', { name: 'Subscriptions' })
    expect(within(tabs).getByRole('link', { name: 'Overview' })).toBeInTheDocument()
    expect(within(tabs).getByRole('link', { name: 'To match' })).toBeInTheDocument()
  })

  it('invites from the dashboard tile with a named link', async () => {
    renderApp('/h/h-dom')
    const link = await screen.findByRole('link', { name: 'Add subscriptions' })
    expect(link).toHaveAttribute('href', '/h/h-dom/subscriptions')
    expect(plain(link.parentElement?.textContent)).toBe(
      "Paying for Netflix or ChatGPT? Add subscriptions and we'll show their cost and upcoming renewals.",
    )
  })

  it('shows the subscription page and the form', async () => {
    const { user } = renderApp('/h/h-dom/subscriptions/s-openai')
    expect(await screen.findByRole('heading', { level: 1, name: 'ChatGPT Plus' })).toBeInTheDocument()
    const hero = screen.getByRole('region', { name: 'Price and cycle' })
    expect(plain(hero.textContent)).toContain('monthly, day 31')
    expect(plain(hero.textContent)).toContain('Next payment')
    expect(plain(hero.textContent)).toContain('in 10 days')
    const details = screen.getByRole('region', { name: 'Details' })
    expect(plain(details.textContent)).toContain('3 days before')
    expect(plain(details.textContent)).toContain('vendor and subscription name, amount ±10%, date ±3 days')
    expect(screen.getByRole('heading', { name: 'Payment schedule' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Edit: ChatGPT Plus' }))
    expect(dialog.getByRole('combobox', { name: 'Unit' })).toHaveTextContent('month')
    expect(dialog.getByText('Billing cycle')).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Save changes' })).toBeInTheDocument()
  })
})
