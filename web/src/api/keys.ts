import type { operations } from './schema.d.ts'

export type AuditFilters = Pick<
  NonNullable<operations['listAudit']['parameters']['query']>,
  'actor_kind' | 'actor_id' | 'resource_type'
>

export type TransactionFilters = Pick<
  NonNullable<operations['searchTransactions']['parameters']['query']>,
  'wallet_id' | 'from' | 'to' | 'payee_id' | 'envelope_id' | 'q' | 'kind' | 'status' | 'tag'
>

// Pomija puste filtry, żeby jeden stan filtrów dawał jeden klucz.
function normalize<T extends object>(filters: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(filters).filter(([, value]) => value !== undefined && value !== null && value !== ''),
  ) as Partial<T>
}

/**
 * Klucze zapytań całej aplikacji. Wszystko, co dotyczy gospodarstwa, leży pod
 * `['household', id]`, więc unieważnienie tego prefiksu odświeża całe
 * gospodarstwo. Części B/C dopisują tu swoje klucze (np. wallets, transactions).
 */
export const keys = {
  authConfig: () => ['auth-config'] as const,
  me: () => ['me'] as const,
  currencies: () => ['currencies'] as const,
  invitationPreview: (token: string) => ['invitation-preview', token] as const,

  household: (hid: string) => ['household', hid] as const,
  members: (hid: string) => ['household', hid, 'members'] as const,
  invitations: (hid: string) => ['household', hid, 'invitations'] as const,
  bots: (hid: string) => ['household', hid, 'bots'] as const,
  audit: (hid: string, filters?: AuditFilters) =>
    filters ? (['household', hid, 'audit', normalize(filters)] as const) : (['household', hid, 'audit'] as const),

  /** Bez opcji — prefiks wszystkich list portfeli. */
  wallets: (hid: string, options?: { archived: boolean }) =>
    options ? (['household', hid, 'wallets', options] as const) : (['household', hid, 'wallets'] as const),
  wallet: (hid: string, id?: string) =>
    id ? (['household', hid, 'wallet', id] as const) : (['household', hid, 'wallet'] as const),
  /** Bez filtrów — prefiks wszystkich rejestrów (każdego filtra i ostatnich transakcji). */
  transactions: (hid: string, filters?: TransactionFilters & { limit?: number }) =>
    filters
      ? (['household', hid, 'transactions', normalize(filters)] as const)
      : (['household', hid, 'transactions'] as const),
  payees: (hid: string) => ['household', hid, 'payees'] as const,
  fxQuote: (hid: string, currency?: string, date?: string) =>
    currency ? (['household', hid, 'fx-quote', currency, date ?? ''] as const) : (['household', hid, 'fx-quote'] as const),
  fxOverrides: (hid: string) => ['household', hid, 'fx-overrides'] as const,

  /** Prefiks wszystkiego, co dotyczy budżetu (miesiące, grupy, koperty, historia kopert). */
  budget: (hid: string) => ['household', hid, 'budget'] as const,
  /** Bez miesiąca — prefiks wszystkich miesięcy. */
  budgetMonth: (hid: string, month?: string) =>
    month ? (['household', hid, 'budget', 'month', month] as const) : (['household', hid, 'budget', 'month'] as const),
  envelopeGroups: (hid: string) => ['household', hid, 'budget', 'groups'] as const,
  envelopes: (hid: string) => ['household', hid, 'budget', 'envelopes'] as const,
  envelopeActivity: (hid: string, month: string, envelopeId: string) =>
    ['household', hid, 'budget', 'activity', month, envelopeId] as const,
}

export { normalize as normalizeFilters }
