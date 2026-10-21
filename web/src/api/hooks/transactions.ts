import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys, normalizeFilters, type TransactionFilters } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { TransactionCreate, TransactionUpdate, TransferCreate } from '../types.ts'

export const transactionPageSize = 50

function query(filters: TransactionFilters) {
  return normalizeFilters(filters) as TransactionFilters
}

/** Rejestr od najnowszych, stronicowany kursorem. */
export function useTransactions(hid: string, filters: TransactionFilters) {
  return useInfiniteQuery({
    queryKey: keys.transactions(hid, filters),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/transactions', {
          params: {
            path: { householdId: hid },
            query: { ...query(filters), limit: transactionPageSize, cursor: pageParam },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.transactions,
  })
}

/** Kilka najnowszych transakcji (pulpit). */
export function useRecentTransactions(hid: string, limit: number, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.transactions(hid, { limit }),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/transactions', {
          params: { path: { householdId: hid }, query: { limit } },
          signal,
        }),
      ).items,
    staleTime: staleTime.transactions,
    enabled: options.enabled ?? true,
  })
}

/**
 * Po zmianie transakcji: rejestry, salda portfeli, kontrahenci (mogli
 * powstać z nazwy) i dziennik.
 */
export function useTransactionsChanged(hid: string) {
  const client = useQueryClient()
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: keys.transactions(hid) }),
      client.invalidateQueries({ queryKey: keys.wallets(hid) }),
      client.invalidateQueries({ queryKey: keys.wallet(hid) }),
      client.invalidateQueries({ queryKey: keys.payees(hid) }),
      client.invalidateQueries({ queryKey: keys.audit(hid) }),
    ])
  }
}

export function useCreateTransaction(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (body: TransactionCreate) =>
      unwrap(
        await api.POST('/households/{householdId}/transactions', { params: { path: { householdId: hid } }, body }),
      ),
    onSuccess: changed,
  })
}

export function useUpdateTransaction(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async ({ transactionId, body }: { transactionId: string; body: TransactionUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/transactions/{transactionId}', {
          params: { path: { householdId: hid, transactionId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

export function useDeleteTransaction(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (transactionId: string) => {
      await api.DELETE('/households/{householdId}/transactions/{transactionId}', {
        params: { path: { householdId: hid, transactionId } },
      })
    },
    onSuccess: changed,
  })
}

export function useCreateTransfer(hid: string) {
  const changed = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (body: TransferCreate) =>
      unwrap(await api.POST('/households/{householdId}/transfers', { params: { path: { householdId: hid } }, body })),
    onSuccess: changed,
  })
}
