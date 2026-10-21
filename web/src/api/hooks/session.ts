import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useContext } from 'react'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { Household, Me, User, UserUpdate } from '../types.ts'

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

export type SessionUser = { user: User; households: Household[] }

/**
 * Zalogowany człowiek, podawany przez RequireSession. Kontekst, nie
 * bezpośrednio useMe(): po wylogowaniu cache znika, a komponenty, które
 * jeszcze się renderują, dostają ostatnią znaną wartość zamiast błędu.
 */
export const SessionContext = createContext<SessionUser | null>(null)

/** Użytkownik i jego gospodarstwa pod RequireSession. */
export function useSessionUser(): SessionUser {
  const session = useContext(SessionContext)
  if (!session) {
    throw new Error('useSessionUser outside RequireSession')
  }
  return session
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

/** Zmienia preferencje człowieka (język); odświeża /me. */
export function useUpdateMe() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: UserUpdate) => unwrap(await api.PATCH('/me', { body })),
    onSuccess: async (user) => {
      // Od razu nowy użytkownik w /me (np. wybór języka po ponownym zamontowaniu), potem odświeżenie.
      client.setQueryData<Me>(keys.me(), (me) => (me ? { ...me, user } : me))
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
