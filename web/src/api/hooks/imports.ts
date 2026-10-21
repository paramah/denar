import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../client.ts'
import { keys } from '../keys.ts'
import { staleTime } from '../queryClient.ts'
import type { CsvImportOptions } from '../types.ts'
import { useTransactionsChanged } from './transactions.ts'

export const importPageSize = 30

/** Plik i opcje parsera jednego żądania multipart (008 AD-801). */
export type CsvUpload = { file: File; options: CsvImportOptions }

// Kontrakt opisuje części jako stringi (`file` binarny, `options` JSON);
// openapi-fetch wysyła FormData bez nagłówka Content-Type — granicę ustawia przeglądarka.
function uploadBody(upload: CsvUpload) {
  return { file: upload.file as unknown as string, options: JSON.stringify(upload.options) }
}

function toFormData(body: { file: string; options: string }): FormData {
  const data = new FormData()
  const file = body.file as unknown as File
  data.append('file', file, file.name)
  data.append('options', body.options)
  return data
}

/** Presety banków z domyślnymi opcjami parsera (generic pierwszy). */
export function useImportPresets() {
  return useQuery({
    queryKey: keys.importPresets(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/import-presets', { signal })).items,
    staleTime: Infinity,
  })
}

/** Podgląd bez zapisu: liczniki, pierwsze wiersze, błędy i to, co parser wykrył. */
export function usePreviewCsvImport(hid: string) {
  return useMutation({
    mutationFn: async (upload: CsvUpload) =>
      unwrap(
        await api.POST('/households/{householdId}/imports/csv/preview', {
          params: { path: { householdId: hid } },
          body: uploadBody(upload),
          bodySerializer: toFormData,
        }),
      ),
  })
}

/** Import pliku: nowe transakcje, partia w historii, auto-dopasowanie subskrypcji. */
export function useImportCsv(hid: string) {
  const client = useQueryClient()
  const transactionsChanged = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (upload: CsvUpload) =>
      unwrap(
        await api.POST('/households/{householdId}/imports/csv', {
          params: { path: { householdId: hid } },
          body: uploadBody({ file: upload.file, options: { ...upload.options, preview_limit: undefined } }),
          bodySerializer: toFormData,
        }),
      ),
    onSuccess: async (batch) => {
      client.setQueryData(keys.importBatch(hid, batch.id), batch)
      // Listy historii; partię z odpowiedzi już mamy.
      await Promise.all([client.invalidateQueries({ queryKey: [...keys.importsAll(hid), 'list'] }), transactionsChanged()])
    },
  })
}

/** Historia importów od najnowszych, stronicowana kursorem. */
export function useImports(hid: string, filters: { walletId?: string } = {}) {
  return useInfiniteQuery({
    queryKey: [...keys.imports(hid, { wallet_id: filters.walletId }), 'pages'],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/imports', {
          params: {
            path: { householdId: hid },
            query: { limit: importPageSize, cursor: pageParam, wallet_id: filters.walletId || undefined },
          },
          signal,
        }),
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    staleTime: staleTime.transactions,
  })
}

export function useImportBatch(hid: string, importId: string) {
  return useQuery({
    queryKey: keys.importBatch(hid, importId),
    queryFn: async ({ signal }) =>
      unwrap(
        await api.GET('/households/{householdId}/imports/{importId}', {
          params: { path: { householdId: hid, importId } },
          signal,
        }),
      ),
    staleTime: staleTime.transactions,
  })
}

/** Cofnięcie partii: usuwa jej transakcje i odpina ich obciążenia subskrypcji. */
export function useRevertImport(hid: string) {
  const client = useQueryClient()
  const transactionsChanged = useTransactionsChanged(hid)
  return useMutation({
    mutationFn: async (importId: string) =>
      unwrap(
        await api.DELETE('/households/{householdId}/imports/{importId}', {
          params: { path: { householdId: hid, importId } },
        }),
      ),
    onSuccess: async (batch) => {
      client.setQueryData(keys.importBatch(hid, batch.id), batch)
      // Listy historii; partię z odpowiedzi już mamy.
      await Promise.all([client.invalidateQueries({ queryKey: [...keys.importsAll(hid), 'list'] }), transactionsChanged()])
    },
  })
}

/** Dostawcy połączeń zarejestrowani na tym serwerze (zwykle żaden). */
export function useConnectionProviders(options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.connectionProviders(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/connection-providers', { signal })).items,
    enabled: options.enabled ?? true,
    staleTime: 5 * 60_000,
  })
}

/** Połączenia gospodarstwa z kontami (owner/admin), bez poświadczeń. */
export function useConnections(hid: string, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: keys.connections(hid),
    queryFn: async ({ signal }) =>
      unwrap(await api.GET('/households/{householdId}/connections', { params: { path: { householdId: hid } }, signal }))
        .items,
    enabled: options.enabled ?? true,
    staleTime: staleTime.channels,
  })
}
