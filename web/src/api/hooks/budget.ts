import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type {
  BudgetMonth,
  BudgetMove,
  EnvelopeCreate,
  EnvelopeGroupCreate,
  EnvelopeGroupUpdate,
  EnvelopeUpdate,
  MoneyInput,
} from '../types.ts'

export const envelopeActivityPageSize = 50

/** Widok miesiąca budżetu: grupy, koperty, Do przydzielenia. */
export function useBudgetMonth(hid: string, month: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.budgetMonth(hid, month),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/budget/months/{month}', {
          params: { path: { householdId: hid, month } },
          signal,
        }),
      ),
    staleTime: staleTime.budget,
    enabled: options.enabled ?? true,
  })
}

export function useEnvelopeGroups(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.envelopeGroups(hid),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/budget/envelope-groups', {
          params: { path: { householdId: hid } },
          signal,
        }),
      ).items,
    staleTime: staleTime.budget,
    enabled: options.enabled ?? true,
  })
}

/** Wszystkie koperty, także ukryte (formularze muszą pokazać kopertę starej transakcji). */
export function useEnvelopes(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.envelopes(hid),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/budget/envelopes', {
          params: { path: { householdId: hid }, query: { include_hidden: true } },
          signal,
        }),
      ).items,
    staleTime: staleTime.budget,
    enabled: options.enabled ?? true,
  })
}

/** Transakcje koperty w miesiącu (z widocznych portfeli budżetowych). */
export function useEnvelopeActivity(hid: string, month: string, envelopeId: string) {
  return useInfiniteQuery({
    queryKey: keys.envelopeActivity(hid, month, envelopeId),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/budget/months/{month}/envelopes/{envelopeId}/transactions', {
          params: {
            path: { householdId: hid, month, envelopeId },
            query: { limit: envelopeActivityPageSize, cursor: pageParam },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.budget,
  })
}

/**
 * Operacje na przydziałach zwracają cały miesiąc: wpisujemy go do cache,
 * a pozostałe miesiące (przeniesienia, RTA) odświeżą się przy następnym użyciu.
 */
async function monthChanged(client: QueryClient, hid: string, month: BudgetMonth) {
  client.setQueryData(keys.budgetMonth(hid, month.month), month)
  await Promise.all([
    client.invalidateQueries({
      queryKey: keys.budgetMonth(hid),
      predicate: (query) => query.queryKey[4] !== month.month,
    }),
    client.invalidateQueries({ queryKey: keys.audit(hid) }),
  ])
}

export function useSetAssignment(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async ({ month, envelopeId, amount }: { month: string; envelopeId: string; amount: MoneyInput }) =>
      unwrap(
        await api.PUT('/households/{householdId}/budget/months/{month}/assignments/{envelopeId}', {
          params: { path: { householdId: hid, month, envelopeId } },
          body: { amount },
        }),
      ),
    onSuccess: (month) => monthChanged(client, hid, month),
  })
}

export function useMoveBetweenEnvelopes(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async ({ month, body }: { month: string; body: BudgetMove }) =>
      unwrap(
        await api.POST('/households/{householdId}/budget/months/{month}/moves', {
          params: { path: { householdId: hid, month } },
          body,
        }),
      ),
    onSuccess: (month) => monthChanged(client, hid, month),
  })
}

export function useAutoAssign(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (month: string) =>
      unwrap(
        await api.POST('/households/{householdId}/budget/months/{month}/auto-assign', {
          params: { path: { householdId: hid, month } },
        }),
      ),
    onSuccess: (result) => monthChanged(client, hid, result.month),
  })
}

/** Po zmianie grup lub kopert: cały budżet (nazwy, kolejność, sumy) i dziennik. */
function useBudgetStructureChanged(hid: string) {
  const client = useQueryClient()
  return async (options: { payees?: boolean } = {}) => {
    await Promise.all([
      client.invalidateQueries({ queryKey: keys.budget(hid) }),
      client.invalidateQueries({ queryKey: keys.audit(hid) }),
      // Usunięta koperta znika z domyślnej koperty kontrahentów.
      options.payees ? client.invalidateQueries({ queryKey: keys.payees(hid) }) : undefined,
    ])
  }
}

export function useCreateEnvelopeGroup(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async (body: EnvelopeGroupCreate) =>
      unwrap(
        await api.POST('/households/{householdId}/budget/envelope-groups', {
          params: { path: { householdId: hid } },
          body,
        }),
      ),
    onSuccess: () => changed(),
  })
}

export function useUpdateEnvelopeGroup(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async ({ groupId, body }: { groupId: string; body: EnvelopeGroupUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/budget/envelope-groups/{groupId}', {
          params: { path: { householdId: hid, groupId } },
          body,
        }),
      ),
    onSuccess: () => changed(),
  })
}

export function useDeleteEnvelopeGroup(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async (groupId: string) => {
      await api.DELETE('/households/{householdId}/budget/envelope-groups/{groupId}', {
        params: { path: { householdId: hid, groupId } },
      })
    },
    onSuccess: () => changed(),
  })
}

export function useCreateEnvelope(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async (body: EnvelopeCreate) =>
      unwrap(
        await api.POST('/households/{householdId}/budget/envelopes', {
          params: { path: { householdId: hid } },
          body,
        }),
      ),
    onSuccess: () => changed(),
  })
}

export function useUpdateEnvelope(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async ({ envelopeId, body }: { envelopeId: string; body: EnvelopeUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/budget/envelopes/{envelopeId}', {
          params: { path: { householdId: hid, envelopeId } },
          body,
        }),
      ),
    onSuccess: () => changed(),
  })
}

export function useDeleteEnvelope(hid: string) {
  const changed = useBudgetStructureChanged(hid)
  return useMutation({
    mutationFn: async (envelopeId: string) => {
      await api.DELETE('/households/{householdId}/budget/envelopes/{envelopeId}', {
        params: { path: { householdId: hid, envelopeId } },
      })
    },
    onSuccess: () => changed({ payees: true }),
  })
}

/**
 * Nowa kolejność elementów: pozycja = indeks. Wysyła tylko te, których
 * pozycja się zmienia (pozycje z API mogą się powtarzać, np. same zera).
 */
export function reorderPatches<T extends { id: string; position: number }>(items: T[], from: number, to: number) {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) {
    return []
  }
  const next = [...items]
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved!)
  return next.flatMap((item, index) => (item.position === index ? [] : [{ id: item.id, position: index }]))
}
