import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { FxOverrideSet } from '../types.ts'

/** Kurs waluty do bazowej na dzień — taki, jaki dostałaby transakcja. */
export function useFxQuote(hid: string, currency: string, date: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.fxQuote(hid, currency, date),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/fx-rates', {
          params: { path: { householdId: hid }, query: { currency, ...(date ? { date } : {}) } },
          signal,
        }),
      ),
    staleTime: staleTime.fx,
    enabled: (options.enabled ?? true) && currency !== '',
  })
}

export function useFxOverrides(hid: string) {
  return useQuery({
    queryKey: keys.fxOverrides(hid),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/fx-rates/overrides', {
          params: { path: { householdId: hid } },
          signal,
        }),
      ).items,
    staleTime: staleTime.fx,
  })
}

/** Ręczny kurs uzupełnia oczekujące kwoty bazowe, więc odświeża też salda i rejestry. */
export function useSetFxOverride(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: FxOverrideSet) =>
      unwrap(
        await api.PUT('/households/{householdId}/fx-rates/overrides', {
          params: { path: { householdId: hid } },
          body,
        }),
      ),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: keys.fxOverrides(hid) }),
        client.invalidateQueries({ queryKey: keys.fxQuote(hid) }),
        client.invalidateQueries({ queryKey: keys.wallets(hid) }),
        client.invalidateQueries({ queryKey: keys.wallet(hid) }),
        client.invalidateQueries({ queryKey: keys.transactions(hid) }),
        client.invalidateQueries({ queryKey: keys.audit(hid) }),
      ])
    },
  })
}
