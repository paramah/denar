import { describe, expect, it } from 'vitest'
import { allScopes, roleAllowsScope, scopeLabel, toggleScope } from './scopes.ts'

describe('scopes', () => {
  it('covers every scope of the contract once', () => {
    expect(allScopes).toHaveLength(10)
    expect(new Set(allScopes).size).toBe(10)
  })

  it('write implies read, and losing read loses write', () => {
    expect(toggleScope([], 'wallets:write', true)).toEqual(['wallets:read', 'wallets:write'])
    expect(toggleScope(['wallets:read', 'wallets:write'], 'wallets:read', false)).toEqual([])
    expect(toggleScope(['wallets:read', 'wallets:write'], 'wallets:write', false)).toEqual(['wallets:read'])
  })

  it('labels scopes in Polish', () => {
    expect(scopeLabel('transactions:write')).toBe('Transakcje: zapis')
  })

  it('a viewer creator allows only reads', () => {
    expect(roleAllowsScope('viewer', 'budget:write')).toBe(false)
    expect(roleAllowsScope('viewer', 'budget:read')).toBe(true)
    expect(roleAllowsScope('member', 'budget:write')).toBe(true)
    expect(roleAllowsScope(null, 'budget:read')).toBe(false)
  })
})
