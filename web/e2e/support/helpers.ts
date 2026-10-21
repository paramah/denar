import AxeBuilder from '@axe-core/playwright'
import { expect, type APIRequestContext, type Page } from '@playwright/test'
import { adminEmail, adminPassword, nbpURL } from './state.ts'

export const unique = () => Math.random().toString(36).slice(2, 8)

/** Logowanie przez formularz. */
export async function loginViaUi(page: Page, email = adminEmail, password = adminPassword(), returnTo?: string) {
  await page.goto(returnTo ? `/login?return_to=${encodeURIComponent(returnTo)}` : '/login')
  await page.getByRole('textbox', { name: 'E-mail' }).fill(email)
  await page.getByLabel('Hasło').fill(password)
  await page.getByRole('button', { name: 'Zaloguj się' }).click()
}

/** Logowanie przez API; cookie sesji trafia do kontekstu strony. */
export async function loginViaApi(request: APIRequestContext, email = adminEmail, password = adminPassword()) {
  const response = await request.post('/api/auth/login', { data: { email, password } })
  expect(response.ok()).toBe(true)
}

export async function createHousehold(
  request: APIRequestContext,
  body: Partial<{ name: string; base_currency: string; timezone: string; month_start_day: number }> = {},
): Promise<{ id: string; name: string }> {
  const response = await request.post('/api/households', {
    data: { name: `Dom ${unique()}`, base_currency: 'PLN', timezone: 'Europe/Warsaw', month_start_day: 1, ...body },
  })
  expect(response.status()).toBe(201)
  return (await response.json()) as { id: string; name: string }
}

/** axe: bez naruszeń poważnych i krytycznych (WCAG 2.x A/AA). */
export async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(
    serious.map((v) => `${v.id}: ${v.help} — ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
    `axe on ${label}`,
  ).toEqual([])
}

/** Data za `days` dni w strefie gospodarstw testowych (RRRR-MM-DD). */
export function dayFromToday(days: number): string {
  return new Date(Date.now() + days * 86_400_000).toLocaleDateString('en-CA', { timeZone: 'Europe/Warsaw' })
}

/**
 * Subskrypcja z płatnością za 3 dni i przypomnieniem 3 dni wcześniej: job
 * subskrypcji (co 2 s w e2e) dodaje zdarzenie, rozsyłka robi z niego powiadomienie.
 */
export async function subscriptionWithReminder(request: APIRequestContext, householdId: string, name: string) {
  const response = await request.post(`/api/households/${householdId}/subscriptions`, {
    data: {
      name,
      vendor: 'netflix',
      price: { amount: '43.00', currency: 'PLN' },
      cycle: { unit: 'month', anchor: dayFromToday(3) },
      reminder_offsets: [3],
    },
  })
  expect(response.status()).toBe(201)
  return (await response.json()) as { id: string; name: string }
}

/** Adres odbiorcy webhooków w atrapie (e2e/support/nbp-stub.mjs). */
export const webhookURL = (name: string) => `${nbpURL}/webhook/${name}`

export type ReceivedWebhook = { headers: Record<string, string>; body: string }

/** Żądania, które dostała atrapa pod /webhook/<nazwa>. */
export async function receivedWebhooks(request: APIRequestContext, name: string): Promise<ReceivedWebhook[]> {
  const response = await request.get(webhookURL(name))
  expect(response.ok()).toBe(true)
  return (await response.json()) as ReceivedWebhook[]
}

/** Kanały są osobiste (konto startowe dzieli je między testami): czysty start, limit 20. */
export async function deleteAllChannels(request: APIRequestContext) {
  const list = (await (await request.get('/api/me/channels')).json()) as { items: { id: string }[] }
  for (const channel of list.items) {
    expect((await request.delete(`/api/me/channels/${channel.id}`)).status()).toBe(204)
  }
}
