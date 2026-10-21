import { QueryClient } from '@tanstack/react-query'
import { ApiError, NetworkError } from './errors.ts'

const second = 1000
const minute = 60 * second

/** Jak długo dane danego rodzaju są świeże. */
export const staleTime = {
  authConfig: 5 * minute,
  /** Do 401 lub wylogowania; unieważniane ręcznie. */
  me: Infinity,
  currencies: Infinity,
  members: minute,
  bots: 30 * second,
  invitations: 30 * second,
  audit: 15 * second,
  wallets: 15 * second,
  transactions: 15 * second,
  payees: minute,
  fx: 5 * minute,
  budget: 15 * second,
  vendors: Infinity,
  subscriptions: 15 * second,
} as const

const maxRetries = 2

// Ponawiamy tylko to, co może minąć samo: 5xx i sieć.
function shouldRetry(failureCount: number, error: unknown): boolean {
  const transient = error instanceof NetworkError || (error instanceof ApiError && error.status >= 500)
  return transient && failureCount < maxRetries
}

export function retryDelay(attempt: number): number {
  return Math.min(second * 2 ** attempt, 8 * second)
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30 * second,
        gcTime: 5 * minute,
        refetchOnWindowFocus: true,
        retry: shouldRetry,
        retryDelay,
      },
      mutations: { retry: false },
    },
  })
}

/** Klient działającej aplikacji. Testy tworzą własny. */
export const queryClient = createQueryClient()
