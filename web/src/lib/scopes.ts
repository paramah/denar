import type { Role, Scope } from '../api/types.ts'
import { tk } from '../i18n/translate.ts'

export type ScopeGroup = {
  id: string
  label: string
  description: string
  read: Scope
  write?: Scope
}

type ScopeArea = 'wallets' | 'transactions' | 'budget' | 'subscriptions' | 'reports' | 'notifications'

/** Grupa z nazwą i opisem czytanymi z settings.json w chwili odczytu (gettery). */
function group(id: ScopeArea, read: Scope, write?: Scope): ScopeGroup {
  return {
    id,
    get label() {
      return tk(`settings:scopes.${id}.label`)
    },
    get description() {
      return tk(`settings:scopes.${id}.description`)
    },
    read,
    ...(write ? { write } : {}),
  }
}

/** Scope'y botów pogrupowane według obszaru: wiersz to obszar, kolumny odczyt/zapis. */
export const scopeGroups: ScopeGroup[] = [
  group('wallets', 'wallets:read', 'wallets:write'),
  group('transactions', 'transactions:read', 'transactions:write'),
  group('budget', 'budget:read', 'budget:write'),
  group('subscriptions', 'subscriptions:read', 'subscriptions:write'),
  group('reports', 'reports:read'),
  group('notifications', 'notifications:read'),
]

export const allScopes: Scope[] = scopeGroups.flatMap((group) => (group.write ? [group.read, group.write] : [group.read]))
export const readScopes: Scope[] = scopeGroups.map((group) => group.read)

/**
 * Zestaw „Asystent AI”: odczyt wszystkiego i wprowadzanie wydatków
 * (narzędzia MCP record_expense/record_income/transfer). Budżetu ani
 * subskrypcji asystent nie zmienia, dopóki ktoś nie doda zapisu ręcznie.
 */
export const assistantScopes: Scope[] = allScopes.filter((scope) => scope.endsWith(':read') || scope === 'transactions:write')

export function scopeLabel(scope: Scope): string {
  const found = scopeGroups.find((g) => g.read === scope || g.write === scope)
  if (!found) {
    return scope
  }
  return tk(scope === found.read ? 'settings:scopes.read' : 'settings:scopes.write', { area: found.label })
}

/** Zaznaczenie zapisu pociąga odczyt; odznaczenie odczytu zabiera zapis. */
export function toggleScope(selected: Scope[], scope: Scope, on: boolean): Scope[] {
  const set = new Set(selected)
  const group = scopeGroups.find((g) => g.read === scope || g.write === scope)
  if (on) {
    set.add(scope)
    if (group && scope === group.write) {
      set.add(group.read)
    }
  } else {
    set.delete(scope)
    if (group?.write && scope === group.read) {
      set.delete(group.write)
    }
  }
  return allScopes.filter((s) => set.has(s))
}

/** Viewer daje botom tylko odczyt (jak RolePermitsScope na serwerze). */
export function roleAllowsScope(role: Role | null | undefined, scope: Scope): boolean {
  if (!role) {
    return false
  }
  return role !== 'viewer' || scope.endsWith(':read')
}
