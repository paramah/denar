import { setupServer } from 'msw/node'
import { validateResponse } from '../openapiValidator.ts'
import { handlers } from './handlers.ts'

export { apiUrl } from './url.ts'

/**
 * The mock API of every test. It is strict: each response it returns is
 * checked against api/openapi.yaml, and the test fails if one does not match
 * (see setup.ts).
 */
export const server = setupServer(...handlers)

const offContractHeader = 'X-Test-Off-Contract'

/**
 * Marks a response the contract cannot describe, e.g. the error page of a
 * proxy. The reason documents the exception at the place it is made.
 */
export function offContract<R extends Response>(response: R, reason: string): R {
  response.headers.set(offContractHeader, reason)
  return response
}

async function violationsOf(request: Request, response: Response): Promise<string[]> {
  if (response.headers.has(offContractHeader)) {
    return []
  }
  const { pathname } = new URL(request.url)
  if (!pathname.startsWith('/api/')) {
    return [`${request.method} ${pathname} is outside /api`]
  }
  const text = await response.text()
  let body: unknown = text === '' ? undefined : text
  try {
    body = text === '' ? undefined : JSON.parse(text)
  } catch {
    // Not JSON: validated as text, which no schema of the contract accepts.
  }
  return validateResponse({
    method: request.method,
    path: pathname.slice('/api'.length),
    status: response.status,
    headers: Object.fromEntries(response.headers),
    body,
  })
}

const checks: Promise<string[]>[] = []

server.events.on('response:mocked', ({ request, response }) => {
  checks.push(violationsOf(request, response.clone()))
})

/** Returns and forgets what the mocked responses so far broke in the contract. */
export async function takeContractViolations(): Promise<string[]> {
  // The server announces a response one tick after it has delivered it.
  await new Promise((resolve) => setTimeout(resolve, 0))
  return (await Promise.all(checks.splice(0))).flat()
}
