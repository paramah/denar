import type {
  AuditEntry,
  AuthConfig,
  Bot,
  FxOverride,
  FxQuote,
  Money,
  Payee,
  Transaction,
  Wallet,
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

const decimalsByCurrency: Record<string, number> = { JPY: 0 }

/** Kwota odpowiedzi API z poprawnym `minor` (liczone na bigint, jak w aplikacji). */
export function money(amount: string, currency = 'PLN'): Money {
  const decimals = decimalsByCurrency[currency] ?? 2
  const negative = amount.startsWith('-')
  const [whole = '0', fraction = ''] = amount.replace('-', '').split('.')
  const minor = Number(BigInt(whole + fraction.padEnd(decimals, '0')) * (negative ? -1n : 1n))
  return { amount, currency, minor }
}

export function wallet(overrides: Partial<Wallet> = {}): Wallet {
  const currency = overrides.currency ?? 'PLN'
  return {
    id: 'w-konto',
    name: 'Konto',
    type: 'checking',
    currency,
    on_budget: true,
    visibility: 'shared',
    member_ids: [],
    created_by: 'u-ada',
    created_at: '2026-10-01T10:00:00Z',
    archived_at: null,
    balance: money('1000.00', currency),
    cleared_balance: money('1000.00', currency),
    base_balance: money('1000.00', 'PLN'),
    rate_pending: false,
    ...overrides,
  }
}

export function transaction(overrides: Partial<Transaction> = {}): Transaction {
  const amount = overrides.amount ?? money('-50.00')
  return {
    id: 't-1',
    wallet_id: 'w-konto',
    date: '2026-10-20',
    amount,
    base_amount: amount.currency === 'PLN' ? amount : null,
    fx_rate: amount.currency === 'PLN' ? '1' : null,
    rate_pending: amount.currency !== 'PLN',
    payee_id: 'p-sklep',
    payee_name: 'Sklep',
    envelope_id: null,
    kind: 'expense',
    status: 'cleared',
    source: 'manual',
    tags: [],
    note: '',
    external_id: null,
    transfer_pair_id: null,
    transfer_wallet_id: null,
    splits: [],
    created_by: { kind: 'user', id: 'u-ada' },
    created_at: '2026-10-20T10:00:00Z',
    updated_at: '2026-10-20T10:00:00Z',
    ...overrides,
  }
}

export function payee(overrides: Partial<Payee> = {}): Payee {
  return {
    id: 'p-sklep',
    name: 'Sklep',
    patterns: [],
    default_envelope_id: null,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function fxQuote(overrides: Partial<FxQuote> = {}): FxQuote {
  return {
    currency: 'USD',
    base_currency: 'PLN',
    date: '2026-10-20',
    rate: '4.01230000',
    rate_date: '2026-10-20',
    source: 'nbp',
    ...overrides,
  }
}

export function fxOverride(overrides: Partial<FxOverride> = {}): FxOverride {
  return {
    currency: 'USD',
    date: '2026-10-19',
    rate: '4.1',
    created_by: { kind: 'user', id: 'u-ada' },
    created_at: '2026-10-19T10:00:00Z',
    ...overrides,
  }
}
