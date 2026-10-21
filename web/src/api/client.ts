import createClient, { type Middleware } from 'openapi-fetch'
import { ApiError, isAbortError, NetworkError } from './errors.ts'
import type { components, paths } from './schema.d.ts'

type Problem = components['schemas']['Problem']

let unauthenticatedHandler: (() => void) | null = null

/**
 * Rejestruje reakcję na utratę sesji (401). Jedna dla całej aplikacji;
 * null ją usuwa.
 */
export function setUnauthenticatedHandler(handler: (() => void) | null): void {
  unauthenticatedHandler = handler
}

/** Strony, na których brak sesji jest oczekiwany. */
export const publicPaths = ['/login', '/invite']

// Złe hasło i sprawdzenie sesji na stronach publicznych kończą się 401,
// choć żadna sesja nie wygasła.
function expectsUnauthenticated(schemaPath: string): boolean {
  return (
    schemaPath === '/auth/login' ||
    (schemaPath === '/me' && publicPaths.includes(window.location.pathname))
  )
}

async function readProblem(response: Response): Promise<Partial<Problem>> {
  if (!response.headers.get('Content-Type')?.startsWith('application/problem+json')) {
    return {}
  }
  try {
    const body: unknown = await response.json()
    return typeof body === 'object' && body !== null ? body : {}
  } catch {
    return {}
  }
}

function retryAfterSeconds(response: Response): number | undefined {
  const value = response.headers.get('Retry-After')?.trim() ?? ''
  const seconds = /^\d+$/.test(value) ? Number(value) : 0
  return seconds > 0 ? seconds : undefined
}

const errors: Middleware = {
  async onResponse({ response, schemaPath }) {
    if (response.ok) {
      return undefined
    }
    const problem = await readProblem(response)
    if (response.status === 401 && !expectsUnauthenticated(schemaPath)) {
      unauthenticatedHandler?.()
    }
    throw new ApiError({
      status: response.status,
      code: problem.code ?? 'internal',
      type: problem.type,
      title: problem.title ?? '',
      detail: problem.detail,
      errors: problem.errors,
      retryAfter: retryAfterSeconds(response),
    })
  },
  onError({ error }) {
    return isAbortError(error) ? undefined : new NetworkError(error)
  },
}

/**
 * Klient REST tej instancji. Wywołania zwracają `data` albo rzucają
 * ApiError/NetworkError; `error` nigdy nie jest ustawiony.
 */
export function createApiClient(options: { fetch?: (request: Request) => Promise<Response> } = {}) {
  const client = createClient<paths>({
    baseUrl: `${window.location.origin}/api`,
    // Sprawdzane przy każdym wywołaniu, żeby testy mogły podmienić fetch (MSW).
    fetch: options.fetch ?? ((request) => globalThis.fetch(request)),
  })
  client.use(errors)
  return client
}

export const api = createApiClient()

/** Treść udanej odpowiedzi; brak treści to błąd wywołującego. */
export function unwrap<T>(result: { data?: T }): T {
  if (result.data === undefined) {
    throw new Error('The API answered without a body')
  }
  return result.data
}
