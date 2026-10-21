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

  vendors: () => ['vendors'] as const,
  /** Prefiks wszystkiego o subskrypcjach i obciążeniach (zmiana jednego unieważnia całość). */
  subscriptionsAll: (hid: string) => ['household', hid, 'subscriptions'] as const,
  subscriptions: (hid: string, filters: SubscriptionFilters = {}) =>
    ['household', hid, 'subscriptions', 'list', normalize(filters)] as const,
  subscription: (hid: string, id: string) => ['household', hid, 'subscriptions', 'one', id] as const,
  subscriptionPrices: (hid: string, id: string) => ['household', hid, 'subscriptions', 'prices', id] as const,
  subscriptionSummary: (hid: string) => ['household', hid, 'subscriptions', 'summary'] as const,
  upcomingRenewals: (hid: string, days: number) => ['household', hid, 'subscriptions', 'upcoming', days] as const,
  charges: (hid: string, filters: ChargeFilters & { limit?: number }) =>
    ['household', hid, 'subscriptions', 'charges', normalize(filters)] as const,
  chargeSuggestions: (hid: string, chargeId: string, limit: number) =>
    ['household', hid, 'subscriptions', 'charge-suggestions', chargeId, limit] as const,
  transactionChargeSuggestions: (hid: string, transactionId: string) =>
    ['household', hid, 'subscriptions', 'transaction-suggestions', transactionId] as const,

  /** Powiadomienia są osobiste (wszystkie gospodarstwa) — prefiks listy i licznika. */
  notifications: () => ['notifications'] as const,
  notificationList: (filters: NotificationFilters = {}) => ['notifications', 'list', normalize(filters)] as const,
  unreadCount: () => ['notifications', 'unread-count'] as const,
  channelTypes: () => ['channel-types'] as const,
  channels: () => ['channels'] as const,
  notificationPreferences: (hid: string) => ['household', hid, 'notification-preferences'] as const,

  importPresets: () => ['import-presets'] as const,
  /** Prefiks historii importów i szczegółów partii. */
  importsAll: (hid: string) => ['household', hid, 'imports'] as const,
  imports: (hid: string, filters: { wallet_id?: string } = {}) =>
    ['household', hid, 'imports', 'list', normalize(filters)] as const,
  importBatch: (hid: string, id: string) => ['household', hid, 'imports', 'one', id] as const,
  connectionProviders: () => ['connection-providers'] as const,
  connections: (hid: string) => ['household', hid, 'connections'] as const,
}

export type NotificationFilters = { household_id?: string; unread?: boolean; limit?: number }

export type SubscriptionFilters = Pick<
  NonNullable<operations['listSubscriptions']['parameters']['query']>,
  'status' | 'vendor' | 'wallet_id'
>

export type ChargeFilters = {
  subscription_id?: string
  status?: string[]
  from?: string
  to?: string
}

export { normalize as normalizeFilters }
