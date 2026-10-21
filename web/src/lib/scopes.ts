import type { Role, Scope } from '../api/types.ts'

export type ScopeGroup = {
  id: string
  label: string
  description: string
  read: Scope
  write?: Scope
}

/** Scope'y botów pogrupowane według obszaru: wiersz to obszar, kolumny odczyt/zapis. */
export const scopeGroups: ScopeGroup[] = [
  { id: 'wallets', label: 'Portfele', description: 'Konta, gotówka, karty i salda', read: 'wallets:read', write: 'wallets:write' },
  {
    id: 'transactions',
    label: 'Transakcje',
    description: 'Wydatki, wpływy i przelewy',
    read: 'transactions:read',
    write: 'transactions:write',
  },
  { id: 'budget', label: 'Budżet', description: 'Koperty i przydziały', read: 'budget:read', write: 'budget:write' },
  {
    id: 'subscriptions',
    label: 'Subskrypcje',
    description: 'Koszty cykliczne i odnowienia',
    read: 'subscriptions:read',
    write: 'subscriptions:write',
  },
  { id: 'reports', label: 'Raporty', description: 'Podsumowania i analizy wydatków', read: 'reports:read' },
  { id: 'notifications', label: 'Powiadomienia', description: 'Przypomnienia w aplikacji', read: 'notifications:read' },
]

export const allScopes: Scope[] = scopeGroups.flatMap((group) => (group.write ? [group.read, group.write] : [group.read]))
export const readScopes: Scope[] = scopeGroups.map((group) => group.read)

/**
 * Zestaw „Asystent AI”: odczyt wszystkiego i wprowadzanie wydatków
 * (narzędzia MCP record_expense/record_income/transfer). Budżetu ani
 * subskrypcji asystent nie zmienia, dopóki ktoś nie doda zapisu ręcznie.
 */
export const assistantScopes: Scope[] = allScopes.filter((scope) => scope.endsWith(':read') || scope === 'transactions:write')

const scopeLabels: Record<Scope, string> = Object.fromEntries(
  scopeGroups.flatMap((group) => [
    [group.read, `${group.label}: odczyt`],
    ...(group.write ? [[group.write, `${group.label}: zapis`]] : []),
  ]),
) as Record<Scope, string>

export function scopeLabel(scope: Scope): string {
  return scopeLabels[scope] ?? scope
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
