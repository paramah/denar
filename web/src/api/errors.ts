import type { components } from './schema.d.ts'

type Problem = components['schemas']['Problem']

export type ProblemCode = Problem['code']
export type FieldError = NonNullable<Problem['errors']>[number]

/**
 * Odpowiedź API, która nie jest sukcesem (application/problem+json). Dla 5xx
 * zostają tylko status i kod: opis awarii serwera nie jest dla interfejsu.
 */
export class ApiError extends Error {
  override readonly name = 'ApiError'
  readonly status: number
  readonly code: ProblemCode
  /** Ostatni segment `type`, np. `last-owner`, `duplicate-email`. */
  readonly reason: string
  readonly title: string
  readonly detail?: string
  readonly errors: FieldError[]
  /** Sekundy z Retry-After (429). */
  readonly retryAfter?: number

  constructor(init: {
    status: number
    code: ProblemCode
    type?: string
    title: string
    detail?: string
    errors?: FieldError[]
    retryAfter?: number
  }) {
    super(`API responded with ${init.status} (${init.code})`)
    const server = init.status >= 500
    this.status = init.status
    this.code = init.code
    this.reason = init.type?.split(':').pop() ?? ''
    this.title = server ? '' : init.title
    this.detail = server ? undefined : init.detail
    this.errors = server ? [] : (init.errors ?? [])
    this.retryAfter = init.retryAfter
  }
}

/** Żądanie nie dostało odpowiedzi: brak sieci, DNS, zerwane połączenie. */
export class NetworkError extends Error {
  override readonly name = 'NetworkError'

  constructor(cause?: unknown) {
    super('The API could not be reached', { cause })
  }
}

/** Żądanie anulowane przez AbortSignal; to nie jest błąd do zgłaszania. */
export function isAbortError(err: unknown): boolean {
  return err instanceof Error && (err.name === 'AbortError' || err.name === 'TimeoutError')
}

export type ErrorKind =
  | 'unauthenticated'
  | 'forbidden'
  | 'notFound'
  | 'conflict'
  | 'validation'
  | 'rateLimited'
  | 'server'
  | 'network'

const kindByStatus: Record<number, ErrorKind> = {
  401: 'unauthenticated',
  403: 'forbidden',
  404: 'notFound',
  409: 'conflict',
  422: 'validation',
  429: 'rateLimited',
}

/** Co interfejs ma zrobić z błędem. Nieznane traktujemy jak błąd serwera. */
export function classify(err: unknown): ErrorKind {
  if (err instanceof NetworkError) {
    return 'network'
  }
  if (err instanceof ApiError) {
    return kindByStatus[err.status] ?? 'server'
  }
  return 'server'
}

/** Błędy pól z problem+json (422) jako mapa pole → komunikat (pierwszy wygrywa). */
export function fieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) {
    return {}
  }
  const out: Record<string, string> = {}
  for (const { field, message } of err.errors) {
    out[field] ??= message
  }
  return out
}

/** Komunikat dla człowieka; `fallback` dla błędów bez lepszego opisu. */
export function errorMessage(err: unknown, fallback = 'Coś poszło nie tak. Spróbuj ponownie.'): string {
  switch (classify(err)) {
    case 'network':
      return 'Brak połączenia z serwerem. Sprawdź sieć i spróbuj ponownie.'
    case 'server':
      return 'Serwer nie mógł wykonać operacji. Spróbuj ponownie za chwilę.'
    case 'rateLimited': {
      const seconds = (err as ApiError).retryAfter
      return seconds ? `Za dużo prób. Spróbuj ponownie za ${seconds} s.` : 'Za dużo prób. Spróbuj ponownie za chwilę.'
    }
    case 'forbidden':
      return 'Nie masz uprawnień do tej operacji.'
    case 'notFound':
      return 'Nie znaleziono — mogło zostać usunięte.'
    case 'unauthenticated':
      return 'Sesja wygasła. Zaloguj się ponownie.'
    default: {
      const api = err as ApiError
      return api.detail || api.title || fallback
    }
  }
}
