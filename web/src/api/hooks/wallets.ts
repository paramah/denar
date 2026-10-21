import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { WalletCreate, WalletUpdate } from '../types.ts'

/** Portfele widoczne dla zalogowanego, z saldami (według nazwy). */
export function useWallets(hid: string, options: { includeArchived?: boolean; enabled?: boolean } = {}) {
  const archived = options.includeArchived ?? false
  return useQuery({
    queryKey: keys.wallets(hid, { archived }),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/wallets', {
          params: { path: { householdId: hid }, query: archived ? { include_archived: true } : {} },
          signal,
        }),
      ).items,
    staleTime: staleTime.wallets,
    enabled: options.enabled ?? true,
  })
}

export function useWallet(hid: string, walletId: string) {
  return useQuery({
    queryKey: keys.wallet(hid, walletId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/wallets/{walletId}', {
          params: { path: { householdId: hid, walletId } },
          signal,
        }),
      ),
    staleTime: staleTime.wallets,
  })
}

/** Po zmianie portfela: listy, portfel, rejestry (nazwy portfeli) i dziennik. */
export function useWalletsChanged(hid: string) {
  const client = useQueryClient()
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: keys.wallets(hid) }),
      client.invalidateQueries({ queryKey: keys.wallet(hid) }),
      client.invalidateQueries({ queryKey: keys.transactions(hid) }),
      // Portfel w budżecie / saldo otwarcia zmieniają sumy budżetu.
      client.invalidateQueries({ queryKey: keys.budget(hid) }),
      client.invalidateQueries({ queryKey: keys.audit(hid) }),
    ])
  }
}

export function useCreateWallet(hid: string) {
  const changed = useWalletsChanged(hid)
  return useMutation({
    mutationFn: async (body: WalletCreate) =>
      unwrap(await api.POST('/households/{householdId}/wallets', { params: { path: { householdId: hid } }, body })),
    onSuccess: changed,
  })
}

export function useUpdateWallet(hid: string) {
  const changed = useWalletsChanged(hid)
  return useMutation({
    mutationFn: async ({ walletId, body }: { walletId: string; body: WalletUpdate }) =>
      unwrap(
        await api.PATCH('/households/{householdId}/wallets/{walletId}', {
          params: { path: { householdId: hid, walletId } },
          body,
        }),
      ),
    onSuccess: changed,
  })
}
