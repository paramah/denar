import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { Role } from '../types.ts'

export function useInvitations(hid: string) {
  return useQuery({
    queryKey: keys.invitations(hid),
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/households/{householdId}/invitations', { params: { path: { householdId: hid } }, signal }))
        .items,
    staleTime: staleTime.invitations,
  })
}

/** Link z tokenem wraca tylko tutaj; trzymaj go w stanie komponentu, nie w cache. */
export function useCreateInvitation(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: { email: string; role: Role }) =>
      unwrap(await api.POST('/households/{householdId}/invitations', { params: { path: { householdId: hid } }, body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.invitations(hid) })
      await client.invalidateQueries({ queryKey: keys.audit(hid) })
    },
  })
}

export function useRevokeInvitation(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (invitationId: string) => {
      await api.DELETE('/households/{householdId}/invitations/{invitationId}', {
        params: { path: { householdId: hid, invitationId } },
      })
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.invitations(hid) })
      await client.invalidateQueries({ queryKey: keys.audit(hid) })
    },
  })
}

export function useInvitationPreview(token: string | null) {
  return useQuery({
    queryKey: keys.invitationPreview(token ?? ''),
    queryFn: async ({ signal }) => unwrap(await api.POST('/invitations/preview', { body: { token: token! }, signal })),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
  })
}

export function useAcceptInvitation() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (token: string) => unwrap(await api.POST('/invitations/accept', { body: { token } })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me() })
    },
  })
}

export function useRegisterWithInvitation() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: { token: string; name: string; password: string }) =>
      unwrap(await api.POST('/invitations/register', { body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me() })
    },
  })
}
