import type {
  AuditEntry,
  Charge,
  PricePoint,
  Subscription,
  SubscriptionSummary,
  Vendor,
  BudgetEnvelope,
  BudgetGroup,
  BudgetMonth,
  Envelope,
  EnvelopeGroup,
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

export function envelopeGroup(overrides: Partial<EnvelopeGroup> = {}): EnvelopeGroup {
  return {
    id: 'g-codzienne',
    name: 'Codzienne',
    position: 0,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function envelope(overrides: Partial<Envelope> = {}): Envelope {
  return {
    id: 'e-jedzenie',
    group_id: 'g-codzienne',
    name: 'Jedzenie',
    position: 0,
    hidden: false,
    goal: null,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

const zero = (currency = 'PLN') => money('0.00', currency)

/** Koperta w widoku miesiąca; available = carried + assigned + activity liczone na bigint. */
export function budgetEnvelope(overrides: Partial<BudgetEnvelope> & { carried?: string } = {}): BudgetEnvelope {
  const { carried, ...rest } = overrides
  const assigned = rest.assigned ?? zero()
  const activity = rest.activity ?? zero()
  const carriedOver = rest.carried_over ?? money(carried ?? '0.00')
  const minor = BigInt(assigned.minor) + BigInt(activity.minor) + BigInt(carriedOver.minor)
  const negative = minor < 0n
  const digits = (negative ? -minor : minor).toString().padStart(3, '0')
  const available = money(`${negative ? '-' : ''}${digits.slice(0, -2)}.${digits.slice(-2)}`)
  return {
    id: 'e-jedzenie',
    group_id: 'g-codzienne',
    name: 'Jedzenie',
    position: 0,
    hidden: false,
    goal: null,
    goal_status: null,
    assigned,
    activity,
    carried_over: carriedOver,
    available,
    ...rest,
  }
}

export function budgetGroup(overrides: Partial<BudgetGroup> = {}): BudgetGroup {
  return {
    id: 'g-codzienne',
    name: 'Codzienne',
    position: 0,
    assigned: zero(),
    activity: zero(),
    available: zero(),
    envelopes: [],
    ...overrides,
  }
}

export function budgetMonth(overrides: Partial<BudgetMonth> = {}): BudgetMonth {
  return {
    month: '2026-10',
    start: '2026-10-01',
    end: '2026-10-31',
    currency: 'PLN',
    ready_to_assign: zero(),
    assigned_in_future: zero(),
    overspent_last_month: zero(),
    income: zero(),
    uncategorized: zero(),
    transfers: zero(),
    assigned: zero(),
    activity: zero(),
    available: zero(),
    underfunded: zero(),
    incomplete: false,
    pending_transactions: 0,
    groups: [],
    ...overrides,
  }
}

export const vendors: Vendor[] = [
  { key: 'openai', name: 'OpenAI' },
  { key: 'anthropic', name: 'Anthropic' },
  { key: 'netflix', name: 'Netflix' },
  { key: 'spotify', name: 'Spotify' },
  { key: 'other', name: 'Inne' },
]

export function subscription(overrides: Partial<Subscription> = {}): Subscription {
  const price = overrides.price ?? money('20.00', 'USD')
  return {
    id: 's-openai',
    name: 'ChatGPT Plus',
    vendor: 'openai',
    vendor_name: 'OpenAI',
    external_ref: null,
    price,
    price_effective_from: '2026-01-31',
    cycle: { unit: 'month', every: 1, anchor: '2026-01-31' },
    trial_ends_at: null,
    status: 'active',
    cancelled_at: null,
    ends_at: null,
    wallet_id: 'w-konto',
    envelope_id: null,
    reminder_offsets: [3],
    match_rule: { payee_pattern: '', amount_tolerance_pct: 10, date_window_days: 3 },
    note: '',
    manage_url: null,
    next_due: '2026-10-31',
    monthly: price,
    yearly: money('240.00', price.currency),
    created_by: { kind: 'user', id: 'u-ada' },
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function pricePoint(overrides: Partial<PricePoint> = {}): PricePoint {
  return {
    id: 'pp-1',
    effective_from: '2026-01-31',
    price: money('20.00', 'USD'),
    created_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function charge(overrides: Partial<Charge> = {}): Charge {
  return {
    id: 'c-oct',
    subscription_id: 's-openai',
    subscription_name: 'ChatGPT Plus',
    vendor: 'openai',
    wallet_id: 'w-konto',
    due_date: '2026-10-31',
    amount: money('20.00', 'USD'),
    status: 'expected',
    transaction_id: null,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...overrides,
  }
}

export function subscriptionSummary(overrides: Partial<SubscriptionSummary> = {}): SubscriptionSummary {
  return {
    currency: 'PLN',
    monthly: money('0.00'),
    yearly: money('0.00'),
    incomplete: false,
    missing_currencies: [],
    counts: { trial: 0, active: 0, paused: 0, cancelled: 0 },
    by_vendor: [],
    by_envelope: [],
    upcoming: [],
    trials_ending: [],
    price_changes: [],
    ...overrides,
  }
}
