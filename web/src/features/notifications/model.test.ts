import { describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import {
  canRemoveSecrets,
  channelSummary,
  createBody,
  emptyForm,
  formFromChannel,
  secretsAvailable,
  updateBody,
  validateForm,
} from './channelForm.ts'
import { dayLabel, groupByDay, notificationIcon, notificationLink } from './model.ts'
import { completePreferences, toggleChannel, toggleInApp } from './PreferencesSection.tsx'
import { unreadLabel } from './NotificationBell.tsx'

const types = f.channelTypes()
const info = (type: string) => types.find((t) => t.type === type)!

describe('notification model', () => {
  it('groups by local day, newest first, with Polish day labels', () => {
    const now = new Date(2026, 9, 21, 12, 0)
    const at = (d: number, h: number) => new Date(2026, 9, d, h, 0).toISOString()
    const groups = groupByDay(
      [
        f.notification({ id: 'a', created_at: at(21, 9) }),
        f.notification({ id: 'b', created_at: at(21, 1) }),
        f.notification({ id: 'c', created_at: at(20, 23) }),
        f.notification({ id: 'd', created_at: at(14, 10) }),
      ],
      now,
    )
    expect(groups.map((g) => [g.label, g.items.map((n) => n.id)])).toEqual([
      ['Dziś', ['a', 'b']],
      ['Wczoraj', ['c']],
      ['Środa, 14 października', ['d']],
    ])
    expect(dayLabel('2025-12-31', now)).toBe('31 grudnia 2025')
  })

  it('keeps links inside the app', () => {
    expect(notificationLink(f.notification({ link: '/h/h-dom/budget/2026-10' }))).toBe('/h/h-dom/budget/2026-10')
    expect(notificationLink(f.notification({ link: 'https://evil.test/' }))).toBe('/h/h-dom')
    expect(notificationLink(f.notification({ link: '//evil.test' }))).toBe('/h/h-dom')
    expect(notificationLink(f.notification({ link: '/api/me' }))).toBe('/h/h-dom')
  })

  it('has an icon per event type and a fallback', () => {
    expect(notificationIcon('envelope.goal_due')).toBe('target')
    expect(notificationIcon('something.new')).toBe('bell')
  })

  it.each([
    [0, 'brak nieprzeczytanych'],
    [1, '1 nieprzeczytane'],
    [3, '3 nieprzeczytane'],
    [5, '5 nieprzeczytanych'],
    [12, '12 nieprzeczytanych'],
    [22, '22 nieprzeczytane'],
  ])('unread label %i', (n, want) => {
    expect(unreadLabel(n)).toBe(want)
  })
})

describe('channel form', () => {
  it('builds a create body without empty fields', () => {
    const form = { ...emptyForm('ntfy'), name: ' Telefon ', config: { server: ' ', topic: 'denar-dom' }, secrets: { token: '' } }
    expect(createBody(form, info('ntfy'))).toEqual({ type: 'ntfy', name: 'Telefon', config: { topic: 'denar-dom' }, enabled: true })
    const hook = { ...emptyForm('webhook'), name: 'Hook', config: { url: 'https://x.test' }, secrets: { secret: 's3cret' } }
    expect(createBody(hook, info('webhook'))).toEqual({
      type: 'webhook',
      name: 'Hook',
      config: { url: 'https://x.test' },
      secrets: { secret: 's3cret' },
      enabled: true,
    })
  })

  it('validates required config and secrets', () => {
    const form = { ...emptyForm('telegram'), name: '' }
    expect(validateForm(form, info('telegram'), null)).toEqual({
      name: 'Podaj nazwę kanału.',
      'config.chat_id': 'To pole jest wymagane.',
      'secrets.bot_token': 'To pole jest wymagane.',
    })
    // Edycja z zapisanym tokenem: „zostaw bez zmian” go nie wymaga.
    const channel = f.channel({ type: 'telegram', name: 'TG', config: { chat_id: '1' }, has_secret: true })
    expect(validateForm(formFromChannel(channel), info('telegram'), channel)).toEqual({})
    expect(validateForm({ ...formFromChannel(channel), secretMode: 'change' }, info('telegram'), channel)).toEqual({
      'secrets.bot_token': 'To pole jest wymagane.',
    })
  })

  it('sends only changes, with secrets kept, replaced or removed', () => {
    const channel = f.channel({ has_secret: true })
    const form = formFromChannel(channel)
    expect(form.secretMode).toBe('keep')
    expect(updateBody(form, info('webhook'), channel)).toEqual({})
    expect(updateBody({ ...form, name: 'Nowy', enabled: false }, info('webhook'), channel)).toEqual({ name: 'Nowy', enabled: false })
    expect(updateBody({ ...form, config: { url: 'https://new.test' } }, info('webhook'), channel)).toEqual({
      config: { url: 'https://new.test' },
    })
    expect(updateBody({ ...form, secretMode: 'change', secrets: { secret: 'nowy' } }, info('webhook'), channel)).toEqual({
      secrets: { secret: 'nowy' },
    })
    expect(updateBody({ ...form, secretMode: 'remove' }, info('webhook'), channel)).toEqual({ secrets: null })
    // „Zmień” bez wpisanej wartości niczego nie zmienia.
    expect(updateBody({ ...form, secretMode: 'change' }, info('webhook'), channel)).toEqual({})
  })

  it('allows removing secrets only when none is required', () => {
    expect(canRemoveSecrets(info('webhook'))).toBe(true)
    expect(canRemoveSecrets(info('telegram'))).toBe(false)
    expect(canRemoveSecrets(info('email'))).toBe(false)
  })

  it('knows when the server has no master key', () => {
    expect(secretsAvailable(types)).toBe(true)
    const noKey = f.channelTypes({
      telegram: { available: false, unavailable_reason: 'secrets_unavailable' },
      matrix: { available: false, unavailable_reason: 'secrets_unavailable' },
    })
    expect(secretsAvailable(noKey)).toBe(false)
  })

  it('summarises the target of a channel', () => {
    expect(channelSummary(f.channel())).toBe('https://example.test/hook')
    expect(channelSummary(f.channel({ type: 'ntfy', config: { server: 'https://ntfy.sh', topic: 'dom' } }))).toBe('https://ntfy.sh/dom')
    expect(channelSummary(f.channel({ type: 'matrix', config: { homeserver: 'https://m.org', room_id: '!a:m.org' } }))).toBe(
      '!a:m.org na https://m.org',
    )
  })
})

describe('preferences', () => {
  it('fills missing types with defaults in the server order', () => {
    const prefs = completePreferences([{ type: 'envelope.goal_due', in_app: false, channel_ids: ['c-1'] }])
    expect(prefs).toHaveLength(6)
    expect(prefs[0]).toEqual({ type: 'subscription.renewal_upcoming', in_app: true, channel_ids: [] })
    expect(prefs[5]).toEqual({ type: 'envelope.goal_due', in_app: false, channel_ids: ['c-1'] })
  })

  it('toggles in-app and channels per type', () => {
    let prefs = completePreferences([])
    prefs = toggleChannel(prefs, 'envelope.overspent', 'c-1', true)
    prefs = toggleChannel(prefs, 'envelope.overspent', 'c-1', true)
    prefs = toggleInApp(prefs, 'subscription.trial_ending', false)
    expect(prefs.find((p) => p.type === 'envelope.overspent')?.channel_ids).toEqual(['c-1'])
    expect(prefs.find((p) => p.type === 'subscription.trial_ending')?.in_app).toBe(false)
    prefs = toggleChannel(prefs, 'envelope.overspent', 'c-1', false)
    expect(prefs.find((p) => p.type === 'envelope.overspent')?.channel_ids).toEqual([])
  })
})
