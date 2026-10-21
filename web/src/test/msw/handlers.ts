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
  http.get(apiUrl('/households/:hid/budget/envelope-groups'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/budget/envelopes'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/budget/months/:month'), ({ params }) =>
    HttpResponse.json(f.budgetMonth({ month: String(params.month) })),
  ),
  http.get(apiUrl('/households/:hid/budget/months/:month/envelopes/:eid/transactions'), () =>
    HttpResponse.json({ items: [], next_cursor: null }),
  ),
  http.get(apiUrl('/households/:hid/fx-rates'), ({ request }) => {
    const currency = new URL(request.url).searchParams.get('currency') ?? 'USD'
    return HttpResponse.json(f.fxQuote({ currency, rate: '4.00000000' }))
  }),
  http.get(apiUrl('/vendors'), () => HttpResponse.json({ items: f.vendors })),
  http.get(apiUrl('/households/:hid/subscriptions'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/subscriptions/summary'), () => HttpResponse.json(f.subscriptionSummary())),
  http.get(apiUrl('/households/:hid/subscriptions/upcoming'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/households/:hid/charges'), () => HttpResponse.json({ items: [], next_cursor: null })),
  http.post(apiUrl('/auth/logout'), () => new HttpResponse(null, { status: 204 })),
  http.get(apiUrl('/me/notifications/unread-count'), () => HttpResponse.json({ count: 0 })),
  http.get(apiUrl('/me/notifications'), () => HttpResponse.json({ items: [], next_cursor: null })),
  http.get(apiUrl('/me/channel-types'), () => HttpResponse.json({ items: f.channelTypes() })),
  http.get(apiUrl('/me/channels'), () => HttpResponse.json({ items: [] })),
  http.get(apiUrl('/me/households/:hid/notification-preferences'), () => HttpResponse.json(f.preferences())),
]
