import type {
  AuditEntry,
  AuthConfig,
  Bot,
  Household,
  Invitation,
  Me,
  Member,
  Role,
  User,
} from '../../api/types.ts'

// Dane testowe zgodne z api/openapi.yaml (sprawdza to strict MSW w setup.ts).

export const user: User = {
  id: 'u-ada',
  email: 'ada@example.test',
  name: 'Ada Kowalska',
  instance_admin: true,
  created_at: '2026-10-01T10:00:00Z',
}

export const authConfig: AuthConfig = {
  oidc_enabled: false,
  local_login: true,
  instance_name: 'Denar testowy',
  version: 'test',
}

export function household(overrides: Partial<Household> = {}): Household {
  return {
    id: 'h-dom',
    name: 'Dom',
    base_currency: 'PLN',
    timezone: 'Europe/Warsaw',
    month_start_day: 1,
    created_at: '2026-10-01T10:00:00Z',
    role: 'owner',
    ...overrides,
  }
}

export function me(households: Household[] = [household()]): Me {
  return { kind: 'user', user, households }
}

export function member(overrides: Partial<Member> = {}): Member {
  return {
    user_id: 'u-ada',
    email: 'ada@example.test',
    name: 'Ada Kowalska',
    role: 'owner',
    joined_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function bot(overrides: Partial<Bot> = {}): Bot {
  return {
    id: 'b-claude',
    household_id: 'h-dom',
    name: 'Claude',
    scopes: ['reports:read', 'wallets:read'],
    effective_scopes: ['reports:read', 'wallets:read'],
    wallet_ids: [],
    created_by: 'u-ada',
    creator_role: 'owner' as Role,
    created_at: '2026-10-02T10:00:00Z',
    expires_at: '2027-01-01T00:00:00Z',
    disabled: false,
    token_prefix: 'dnr_ab12',
    last_used_at: null,
    ...overrides,
  }
}

export function invitation(overrides: Partial<Invitation> = {}): Invitation {
  return {
    id: 'i-jan',
    email: 'jan@example.test',
    role: 'member',
    state: 'pending',
    created_by: 'u-ada',
    created_at: '2026-10-03T10:00:00Z',
    expires_at: '2026-10-10T10:00:00Z',
    accepted_at: null,
    revoked_at: null,
    ...overrides,
  }
}

export function auditEntry(n: number, overrides: Partial<AuditEntry> = {}): AuditEntry {
  return {
    id: `a-${String(n).padStart(3, '0')}`,
    at: '2026-10-05T10:00:00Z',
    actor: { kind: 'user', id: 'u-ada' },
    action: 'household.updated',
    resource_type: 'household',
    resource_id: 'h-dom',
    changes: { name: { from: 'Stary dom', to: 'Dom' } },
    ...overrides,
  }
}

export function problem(status: number, code: string, extra: Record<string, unknown> = {}) {
  return {
    type: `urn:denar:problem:${extra.reason ?? code}`,
    code,
    title: 'Problem',
    status,
    ...Object.fromEntries(Object.entries(extra).filter(([key]) => key !== 'reason')),
  }
}
