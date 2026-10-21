import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys, type NotificationFilters } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { Channel, ChannelCreate, ChannelUpdate, Notification, NotificationPreference } from '../types.ts'

/** Co ile dzwonek pyta o licznik (serwer nie wysyła powiadomień na żywo). */
export const unreadPollInterval = 60_000
export const notificationPageSize = 30

/** Licznik nieprzeczytanych ze wszystkich gospodarstw: odpytywany co minutę i przy powrocie do okna. */
export function useUnreadCount() {
  return useQuery({
    queryKey: keys.unreadCount(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/me/notifications/unread-count', { signal })).count,
    refetchInterval: unreadPollInterval,
    refetchOnWindowFocus: 'always',
    staleTime: staleTime.notifications,
  })
}

/** Najnowsze powiadomienia (panel dzwonka, pulpit). */
export function useRecentNotifications(limit: number, options: { householdId?: string; enabled?: boolean } = {}) {
  const filters: NotificationFilters = { household_id: options.householdId, limit }
  return useQuery({
    queryKey: keys.notificationList(filters),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/me/notifications', {
          params: { query: { limit, household_id: options.householdId || undefined } },
          signal,
        }),
      ).items,
    enabled: options.enabled ?? true,
    staleTime: staleTime.notifications,
  })
}

/** Skrzynka powiadomień od najnowszych, stronicowana kursorem. */
export function useNotifications(filters: { householdId?: string; unread: boolean }) {
  const key: NotificationFilters = { household_id: filters.householdId, unread: filters.unread || undefined }
  return useInfiniteQuery({
    queryKey: [...keys.notificationList(key), 'pages'],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/me/notifications', {
          params: {
            query: {
              limit: notificationPageSize,
              cursor: pageParam,
              household_id: filters.householdId || undefined,
              unread: filters.unread || undefined,
            },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.notifications,
  })
}

// Oznaczenie przeczytanego zmienia licznik i każdą listę (także filtr „nieprzeczytane”).
async function notificationsChanged(client: QueryClient) {
  await client.invalidateQueries({ queryKey: keys.notifications() })
}

export function useMarkNotificationRead() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (notification: Notification) =>
      unwrap(
        await api.POST('/me/notifications/{notificationId}/read', {
          params: { path: { notificationId: notification.id } },
        }),
      ),
    onSuccess: () => notificationsChanged(client),
  })
}

export function useMarkAllNotificationsRead() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (householdId?: string) =>
      unwrap(await api.POST('/me/notifications/read-all', { body: householdId ? { household_id: householdId } : {} }))
        .updated,
    onSuccess: () => notificationsChanged(client),
  })
}

/** Typy kanałów z polami i dostępnością na tym serwerze (SMTP, klucz główny). */
export function useChannelTypes(options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.channelTypes(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/me/channel-types', { signal })).items,
    enabled: options.enabled ?? true,
    staleTime: 5 * 60_000,
  })
}

export function useChannels() {
  return useQuery({
    queryKey: keys.channels(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/me/channels', { signal })).items,
    staleTime: staleTime.channels,
  })
}

function replaceChannel(client: QueryClient, channel: Channel) {
  client.setQueryData<Channel[]>(keys.channels(), (old) => old?.map((c) => (c.id === channel.id ? channel : c)))
}

export function useCreateChannel() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (body: ChannelCreate) => unwrap(await api.POST('/me/channels', { body })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.channels() })
    },
  })
}

export function useUpdateChannel() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: ChannelUpdate }) =>
      unwrap(await api.PATCH('/me/channels/{channelId}', { params: { path: { channelId: id } }, body })),
    onSuccess: async (channel) => {
      replaceChannel(client, channel)
      await client.invalidateQueries({ queryKey: keys.channels() })
    },
  })
}

export function useDeleteChannel() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE('/me/channels/{channelId}', { params: { path: { channelId: id } } })
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.channels() })
      // Preferencje z usuniętym kanałem serwer pomija — odśwież macierze.
      await client.invalidateQueries({
        predicate: (query) => query.queryKey[0] === 'household' && query.queryKey[2] === 'notification-preferences',
      })
    },
  })
}

/** Wiadomość testowa teraz; wynik zapisuje też stan kanału (zweryfikowany / ostatni błąd). */
export function useTestChannel() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.POST('/me/channels/{channelId}/test', { params: { path: { channelId: id } } })),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.channels() })
    },
  })
}

export function useNotificationPreferences(hid: string) {
  return useQuery({
    queryKey: keys.notificationPreferences(hid),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/me/households/{householdId}/notification-preferences', {
          params: { path: { householdId: hid } },
          signal,
        }),
      ).items,
    staleTime: staleTime.channels,
  })
}

export function useSetNotificationPreferences(hid: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (items: NotificationPreference[]) =>
      unwrap(
        await api.PUT('/me/households/{householdId}/notification-preferences', {
          params: { path: { householdId: hid } },
          body: { items },
        }),
      ).items,
    onSuccess: (items) => {
      client.setQueryData(keys.notificationPreferences(hid), items)
    },
  })
}
