import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { HouseholdCreate, HouseholdUpdate, Me } from '../types.ts'

export function useCurrencies() {
  return useQuery({
    queryKey: keys.currencies(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/currencies', { signal })).items,
    staleTime: staleTime.currencies,
  })
}

export function useCreateHousehold() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: HouseholdCreate) => unwrap(await api.POST('/households', { body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me() })
    },
  })
}

export function useUpdateHousehold(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: HouseholdUpdate) =>
      unwrap(await api.PATCH('/households/{householdId}', { params: { path: { householdId: hid } }, body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me() })
      await client.invalidateQueries({ queryKey: keys.audit(hid) })
    },
  })
}

export function useDeleteHousehold(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.DELETE('/households/{householdId}', { params: { path: { householdId: hid } } })
    },
    onSuccess: () => {
      forgetHousehold(client, hid)
    },
  })
}

/**
 * Gospodarstwo, z którego się wyszło albo które usunięto: znika z /me od
 * razu (przekierowanie na start nie trafi już do niego), potem /me się odświeża.
 */
export function forgetHousehold(client: QueryClient, hid: string) {
  client.setQueryData<Me>(keys.me(), (me) => (me ? { ...me, households: me.households.filter((h) => h.id !== hid) } : me))
  client.removeQueries({ queryKey: keys.household(hid) })
  void client.invalidateQueries({ queryKey: keys.me() })
}
