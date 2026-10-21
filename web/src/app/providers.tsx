import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { queryClient as appQueryClient } from '../api/queryClient.ts'
import { ToastProvider } from '../components/Toast/Toast.tsx'
import { ThemeProvider } from './ThemeProvider.tsx'

type Props = {
  children: ReactNode
  /** Testy podają własny. */
  queryClient?: QueryClient
}

/** Wszystko, na co strona może liczyć nad sobą, poza routerem. */
export function Providers({ children, queryClient = appQueryClient }: Props) {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <ToastProvider>{children}</ToastProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
