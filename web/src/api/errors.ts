import { tk } from '../i18n/translate.ts'
import { reasonMessage, translateFieldMessage } from './messages.ts'
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

/**
 * Błędy pól z problem+json (422) jako mapa pole → komunikat po polsku
 * (pierwszy wygrywa). Serwer pisze komunikaty po angielsku — tłumaczy je
 * `translateFieldMessage`.
 */
export function fieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) {
    return {}
  }
  const out: Record<string, string> = {}
  for (const { field, message } of err.errors) {
    out[field] ??= translateFieldMessage(message)
  }
  return out
}

/** Komunikat dla człowieka; `fallback` dla błędów bez lepszego opisu. */
export function errorMessage(err: unknown, fallback?: string): string {
  switch (classify(err)) {
    case 'network':
      return tk('errors:generic.network')
    case 'server':
      return tk('errors:generic.server')
    case 'rateLimited': {
      const seconds = (err as ApiError).retryAfter
      return seconds ? tk('errors:generic.rateLimitedFor', { seconds }) : tk('errors:generic.rateLimited')
    }
    case 'forbidden':
      return tk('errors:generic.forbidden')
    case 'notFound':
      return tk('errors:generic.notFound')
    case 'unauthenticated':
      return tk('errors:generic.unauthenticated')
    default: {
      // Opis z serwera jest po angielsku; w języku interfejsu znamy przyczyny (`type`).
      const api = err as ApiError
      const first = api.status === 422 ? api.errors[0] : undefined
      if (first) {
        return translateFieldMessage(first.message)
      }
      return reasonMessage(api.reason) ?? fallback ?? tk('errors:generic.fallback')
    }
  }
}
