import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'

export function useAuthConfig() {
  return useQuery({
    queryKey: keys.authConfig(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/auth/config', { signal })),
    staleTime: staleTime.authConfig,
  })
}

/** Zalogowany człowiek z gospodarstwami i rolami. Źródło prawdy o sesji. */
export function useMe() {
  return useQuery({
    queryKey: keys.me(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/me', { signal })),
    staleTime: staleTime.me,
    retry: false,
  })
}

/** useMe() tam, gdzie RequireSession już zagwarantował dane. */
export function useSessionUser() {
  const me = useMe()
  if (!me.data?.user) {
    throw new Error('useSessionUser outside RequireSession')
  }
  return { user: me.data.user, households: me.data.households }
}

export function useLogin() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: { email: string; password: string }) =>
      unwrap(await api.POST('/auth/login', { body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me() })
    },
  })
}

export function useLogout() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.POST('/auth/logout')
    },
    onSettled: () => {
      client.clear()
    },
  })
}
