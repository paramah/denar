import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { Role } from '../types.ts'

export function useMembers(hid: string) {
  return useQuery({
    queryKey: keys.members(hid),
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/households/{householdId}/members', { params: { path: { householdId: hid } }, signal }))
        .items,
    staleTime: staleTime.members,
  })
}

export function useUpdateMemberRole(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId, role }: { userId: string; role: Role }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/members/{userId}', {
          params: { path: { householdId: hid, userId } },
          body: { role },
        }),
      ),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.household(hid) })
      await client.invalidateQueries({ queryKey: keys.me() })
    },
  })
}

export function useRemoveMember(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (userId: string) => {
      await api.DELETE('/households/{householdId}/members/{userId}', {
        params: { path: { householdId: hid, userId } },
      })
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.household(hid) })
    },
  })
}
