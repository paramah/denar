import { useInfiniteQuery } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys, type AuditFilters } from '../keys.ts'
import { staleTime } from '../queryClient.ts'

export const auditPageSize = 50

/** Dziennik od najnowszych, stronicowany kursorem. */
export function useAudit(hid: string, filters: AuditFilters) {
  return useInfiniteQuery({
    queryKey: keys.audit(hid, filters),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/audit', {
          params: {
            path: { householdId: hid },
            query: {
              limit: auditPageSize,
              cursor: pageParam,
              actor_kind: filters.actor_kind || undefined,
              actor_id: filters.actor_id || undefined,
              resource_type: filters.resource_type || undefined,
            },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.audit,
  })
}
