import { describe, expect, it } from 'vitest'
import { loginUrl, safeReturnTo } from './returnTo.ts'

describe('safeReturnTo', () => {
  it.each([
    ['/h/1/settings', '/h/1/settings'],
    ['/invite', '/invite'],
    [null, '/'],
    ['https://evil.example', '/'],
    ['//evil.example', '/'],
    ['/\\evil', '/'],
    ['/api/me', '/'],
    ['/login?return_to=/x', '/'],
  ])('%s → %s', (value, expected) => {
    expect(safeReturnTo(value)).toBe(expected)
  })
})

describe('loginUrl', () => {
  it('remembers where the user was', () => {
    expect(loginUrl({ pathname: '/h/1/settings/bots', search: '?x=1' })).toBe(
      '/login?return_to=%2Fh%2F1%2Fsettings%2Fbots%3Fx%3D1',
    )
    expect(loginUrl({ pathname: '/', search: '' })).toBe('/login')
  })
})
