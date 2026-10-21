import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { BotCreate, BotUpdate } from '../types.ts'

export function useBots(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.bots(hid),
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/households/{householdId}/bots', { params: { path: { householdId: hid } }, signal })).items,
    staleTime: staleTime.bots,
    enabled: options.enabled ?? true,
  })
}

function useBotsChanged(hid: string) {
  const client = useQueryClient()
  return async () => {
    await client.invalidateQueries({ queryKey: keys.bots(hid) })
    await client.invalidateQueries({ queryKey: keys.audit(hid) })
  }
}

/** Token wraca tylko w odpowiedzi; nie trafia do cache. */
export function useCreateBot(hid: string) {
  const changed = useBotsChanged(hid)
  return useMutation({
    mutationFn: async (body: BotCreate) =>
      unwrap(await api.POST('/households/{householdId}/bots', { params: { path: { householdId: hid } }, body })),
    onSuccess: changed,
  })
}

export function useUpdateBot(hid: string) {
  const changed = useBotsChanged(hid)
  return useMutation({
    mutationFn: async ({ botId, body }: { botId: string; body: BotUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/bots/{botId}', {
          params: { path: { householdId: hid, botId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}

export function useRotateBotToken(hid: string) {
  const changed = useBotsChanged(hid)
  return useMutation({
    mutationFn: async (botId: string) =>
      unwrap(
        await api.POST('/households/{householdId}/bots/{botId}/token', {
          params: { path: { householdId: hid, botId } },
        }),
      ),
    onSuccess: changed,
  })
}

export function useDeleteBot(hid: string) {
  const changed = useBotsChanged(hid)
  return useMutation({
    mutationFn: async (botId: string) => {
      await api.DELETE('/households/{householdId}/bots/{botId}', { params: { path: { householdId: hid, botId } } })
    },
    onSuccess: changed,
  })
}
