import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { PayeeCreate, PayeeUpdate } from '../types.ts'

export function usePayees(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.payees(hid),
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/households/{householdId}/payees', { params: { path: { householdId: hid } }, signal }))
        .items,
    staleTime: staleTime.payees,
    enabled: options.enabled ?? true,
  })
}

function usePayeesChanged(hid: string) {
  const client = useQueryClient()
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: keys.payees(hid) }),
      client.invalidateQueries({ queryKey: keys.transactions(hid) }),
      client.invalidateQueries({ queryKey: keys.audit(hid) }),
    ])
  }
}

export function useCreatePayee(hid: string) {
  const changed = usePayeesChanged(hid)
  return useMutation({
    mutationFn: async (body: PayeeCreate) =>
      unwrap(await api.POST('/households/{householdId}/payees', { params: { path: { householdId: hid } }, body })),
    onSuccess: changed,
  })
}

export function useUpdatePayee(hid: string) {
  const changed = usePayeesChanged(hid)
  return useMutation({
    mutationFn: async ({ payeeId, body }: { payeeId: string; body: PayeeUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/payees/{payeeId}', {
          params: { path: { householdId: hid, payeeId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

export function useDeletePayee(hid: string) {
  const changed = usePayeesChanged(hid)
  return useMutation({
    mutationFn: async (payeeId: string) => {
      await api.DELETE('/households/{householdId}/payees/{payeeId}', {
        params: { path: { householdId: hid, payeeId } },
      })
    },
    onSuccess: changed,
  })
}
