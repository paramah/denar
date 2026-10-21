import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { channelSummary, describeField, validateForm, emptyForm } from './channelForm.ts'
import { unreadLabel } from './NotificationBell.tsx'
import { dayLabel } from './model.ts'

describe('notifications in English', () => {
  it('shows the empty inbox and the bell in English', async () => {
    setLocale('en')
    server.use(http.get(apiUrl('/me/notifications'), () => HttpResponse.json({ items: [], next_cursor: null })))
    renderApp('/h/h-dom/notifications')
    expect(await screen.findByRole('heading', { level: 1, name: 'Notifications' })).toBeInTheDocument()
    expect(await screen.findByText('No notifications yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Mark all as read' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Unread only' })).toBeInTheDocument()
    expect(screen.getAllByText('Notifications, none unread').length).toBeGreaterThan(0)
  })

  it('shows channels and the channel dialog in English', async () => {
    setLocale('en')
    server.use(
      http.get(apiUrl('/me/channels'), () =>
        HttpResponse.json({ items: [f.channel({ last_error: 'boom', last_error_at: null })] }),
      ),
    )
    const { user } = renderApp('/h/h-dom/settings/notifications')
    expect(await screen.findByRole('heading', { name: 'Channels' })).toBeInTheDocument()
    expect(await screen.findByText('Unverified')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send test: Mój webhook' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'What to notify about and where' })).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: 'Add channel' })[0]!)
    const dialog = await screen.findByRole('dialog', { name: 'New notification channel' })
    expect(within(dialog).getByRole('group', { name: 'Channel type' })).toBeInTheDocument()
    expect(within(dialog).getByText('How to set up: Webhook')).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: /^URL/ })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
  })

  it('translates the model helpers', () => {
    setLocale('en')
    const now = new Date(2026, 9, 21, 12)
    expect(dayLabel('2026-10-21', now)).toBe('Today')
    expect(dayLabel('2026-10-20', now)).toBe('Yesterday')
    expect(unreadLabel(0)).toBe('none unread')
    expect(unreadLabel(5)).toBe('5 unread')
    expect(describeField('ntfy', 'topic')).toMatchObject({ label: 'Topic', placeholder: 'denar-home-7f3k' })
    expect(channelSummary(f.channel({ type: 'matrix', config: { homeserver: 'https://m.org', room_id: '!a:m.org' } }))).toBe(
      '!a:m.org on https://m.org',
    )
    const webhook = f.channelTypes().find((t) => t.type === 'webhook')!
    expect(validateForm(emptyForm('webhook'), webhook, null)).toEqual({
      name: 'Enter a channel name.',
      'config.url': 'This field is required.',
    })
  })
})
