import type { operations } from './schema.d.ts'

export type AuditFilters = Pick<
  NonNullable<operations['listAudit']['parameters']['query']>,
  'actor_kind' | 'actor_id' | 'resource_type'
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
}
