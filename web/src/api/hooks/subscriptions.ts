import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys, normalizeFilters, type ChargeFilters, type SubscriptionFilters } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { ChargePost, PriceChange, SubscriptionCreate, SubscriptionUpdate } from '../types.ts'
import { useTransactionsChanged } from './transactions.ts'

export const chargePageSize = 50

/** Słownik dostawców (stały, poza gospodarstwem). */
export function useVendors() {
  return useQuery({
    queryKey: keys.vendors(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/vendors', { signal })).items,
    staleTime: staleTime.vendors,
  })
}

export function useSubscriptions(hid: string, filters: SubscriptionFilters = {}) {
  return useQuery({
    queryKey: keys.subscriptions(hid, filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/subscriptions', {
          params: { path: { householdId: hid }, query: normalizeFilters(filters) },
          signal,
        }),
      ).items,
    staleTime: staleTime.subscriptions,
  })
}

export function useSubscription(hid: string, subscriptionId: string) {
  return useQuery({
    queryKey: keys.subscription(hid, subscriptionId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/subscriptions/{subscriptionId}', {
          params: { path: { householdId: hid, subscriptionId } },
          signal,
        }),
      ),
    staleTime: staleTime.subscriptions,
  })
}

/** Historia cen od najstarszej. */
export function useSubscriptionPrices(hid: string, subscriptionId: string) {
  return useQuery({
    queryKey: keys.subscriptionPrices(hid, subscriptionId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/subscriptions/{subscriptionId}/prices', {
          params: { path: { householdId: hid, subscriptionId } },
          signal,
        }),
      ).items,
    staleTime: staleTime.subscriptions,
  })
}

export function useSubscriptionSummary(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.subscriptionSummary(hid),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/subscriptions/summary', {
          params: { path: { householdId: hid } },
          signal,
        }),
      ),
    staleTime: staleTime.subscriptions,
    enabled: options.enabled ?? true,
  })
}

/** Obciążenia (termin rosnąco), stronicowane kursorem. */
export function useCharges(hid: string, filters: ChargeFilters, options: { enabled?: boolean } = {}) {
  return useInfiniteQuery({
    queryKey: keys.charges(hid, filters),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/charges', {
          params: {
            path: { householdId: hid },
            query: { ...normalizeFilters(filters), limit: chargePageSize, cursor: pageParam },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.subscriptions,
    enabled: options.enabled ?? true,
  })
}

/**
 * Obciążenia powiązane z transakcjami z zakresu dat (rejestr: plakietka
 * z linkiem do subskrypcji). API nie zwraca obciążenia przy transakcji,
 * więc rejestr pyta o dopasowane i zaksięgowane z okna dat widocznych wierszy.
 */
export function useLinkedCharges(hid: string, from: string, to: string, options: { enabled?: boolean } = {}) {
  const filters: ChargeFilters & { limit: number } = { status: ['matched', 'posted'], from, to, limit: 200 }
  return useQuery({
    queryKey: keys.charges(hid, filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/charges', {
          params: { path: { householdId: hid }, query: filters },
          signal,
        }),
      ).items,
    staleTime: staleTime.subscriptions,
    enabled: (options.enabled ?? true) && from !== '' && to !== '',
  })
}

/** Transakcje, które mogą opłacać obciążenie, od najlepszej. */
export function useChargeSuggestions(hid: string, chargeId: string, limit = 5, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.chargeSuggestions(hid, chargeId, limit),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/charges/{chargeId}/suggestions', {
          params: { path: { householdId: hid, chargeId }, query: { limit } },
          signal,
        }),
      ).items,
    staleTime: staleTime.subscriptions,
    enabled: options.enabled ?? true,
  })
}

/** Otwarte obciążenia, które transakcja może opłacać. */
export function useTransactionChargeSuggestions(hid: string, transactionId: string) {
  return useQuery({
    queryKey: keys.transactionChargeSuggestions(hid, transactionId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/transactions/{transactionId}/charge-suggestions', {
          params: { path: { householdId: hid, transactionId } },
          signal,
        }),
      ).items,
    staleTime: staleTime.subscriptions,
  })
}

/** Po zmianie subskrypcji: wszystko o subskrypcjach, cel kopert i dziennik. */
function useSubscriptionsChanged(hid: string) {
  const client = useQueryClient()
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: keys.subscriptionsAll(hid) }),
      client.invalidateQueries({ queryKey: keys.audit(hid) }),
    ])
  }
}

export function useCreateSubscription(hid: string) {
  const changed = useSubscriptionsChanged(hid)
  return useMutation({
    mutationFn: async (body: SubscriptionCreate) =>
      unwrap(
        await api.POST('/households/{householdId}/subscriptions', { params: { path: { householdId: hid } }, body }),
      ),
    onSuccess: changed,
  })
}

export function useUpdateSubscription(hid: string) {
  const changed = useSubscriptionsChanged(hid)
  return useMutation({
    mutationFn: async ({ subscriptionId, body }: { subscriptionId: string; body: SubscriptionUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/subscriptions/{subscriptionId}', {
          params: { path: { householdId: hid, subscriptionId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

export function useDeleteSubscription(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (subscriptionId: string) => {
      await api.DELETE('/households/{householdId}/subscriptions/{subscriptionId}', {
        params: { path: { householdId: hid, subscriptionId } },
      })
    },
    // Bez unieważniania strony usuniętej subskrypcji (dałaby 404 przed przejściem).
    onSuccess: async (_, subscriptionId) => {
      client.removeQueries({ queryKey: keys.subscription(hid, subscriptionId) })
      await Promise.all([
        client.invalidateQueries({
          queryKey: keys.subscriptionsAll(hid),
          predicate: (query) => !query.queryKey.includes(subscriptionId),
        }),
        client.invalidateQueries({ queryKey: keys.audit(hid) }),
      ])
    },
  })
}

export function useChangeSubscriptionPrice(hid: string) {
  const changed = useSubscriptionsChanged(hid)
  return useMutation({
    mutationFn: async ({ subscriptionId, body }: { subscriptionId: string; body: PriceChange }) =>
      unwrap(
        await api.POST('/households/{householdId}/subscriptions/{subscriptionId}/prices', {
          params: { path: { householdId: hid, subscriptionId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

/**
 * Akcje obciążeń zmieniają powiązania transakcji (rejestr, sugestie), a
 * zaksięgowanie tworzy wydatek — odświeżamy to samo, co po zmianie transakcji.
 */
export function useMatchCharge(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async ({ chargeId, transactionId }: { chargeId: string; transactionId: string }) =>
      unwrap(
        await api.POST('/households/{householdId}/charges/{chargeId}/match', {
          params: { path: { householdId: hid, chargeId } },
          body: { transaction_id: transactionId },
        }),
      ),
    onSuccess: changed,
  })
}

export function useUnmatchCharge(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (chargeId: string) =>
      unwrap(
        await api.POST('/households/{householdId}/charges/{chargeId}/unmatch', {
          params: { path: { householdId: hid, chargeId } },
        }),
      ),
    onSuccess: changed,
  })
}

export function usePostCharge(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async ({ chargeId, body }: { chargeId: string; body: ChargePost }) =>
      unwrap(
        await api.POST('/households/{householdId}/charges/{chargeId}/post', {
          params: { path: { householdId: hid, chargeId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

export function useSkipCharge(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (chargeId: string) =>
      unwrap(
        await api.POST('/households/{householdId}/charges/{chargeId}/skip', {
          params: { path: { householdId: hid, chargeId } },
        }),
      ),
    onSuccess: changed,
  })
}
