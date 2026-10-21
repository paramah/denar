import { http, HttpResponse, type RequestHandler } from 'msw'
import * as f from './fixtures.ts'
import { apiUrl } from './url.ts'

/** Odpowiedź problem+json zgodna z kontraktem. */
export function problemResponse(status: number, code: string, extra: Record<string, unknown> = {}, headers?: HeadersInit) {
  return HttpResponse.json(f.problem(status, code, extra), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })
}

/**
 * Domyślne API: zalogowana Ada, właścicielka jednego gospodarstwa „Dom”.
 * Testy nadpisują to, czego potrzebują, przez server.use(...).
 */
export const handlers: RequestHandler[] = [
  http.get(apiUrl('/auth/config'), () => HttpResponse.json(f.authConfig)),
  http.get(apiUrl('/me'), () => HttpResponse.json(f.me())),
  http.get(apiUrl('/currencies'), () =>
    HttpResponse.json({
      items: [
        { code: 'EUR', decimals: 2 },
        { code: 'JPY', decimals: 0 },
        { code: 'PLN', decimals: 2 },
        { code: 'USD', decimals: 2 },
      ],
    }),
  ),
  http.get(apiUrl('/households/:hid/members'), () => HttpResponse.json({ items: [f.member()] })),
  http.get(apiUrl('/households/:hid/bots'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/invitations'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/audit'), () => HttpResponse.json({ items: [], next_cursor: null })),
  http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/transactions'), () => HttpResponse.json({ items: [], next_cursor: null })),
  http.get(apiUrl('/households/:hid/payees'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/fx-rates/overrides'), () => HttpResponse.json({ items: [] })),
  http.post(apiUrl('/auth/logout'), () => new HttpResponse(null, { status: 204 })),
]
