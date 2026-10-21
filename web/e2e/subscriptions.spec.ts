import { expect, test, type Locator } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi } from './support/helpers.ts'

// Część C2 na prawdziwym serwerze (atrapa NBP: 1 USD = 4,00 zł):
// subskrypcja USD (OpenAI, dzień 31.) → harmonogram płatności → wydatek
// w rejestrze → sugestia i potwierdzenie → zmiana ceny → koszt w złotych.

const plain = (text: string | null) => (text ?? '').replace(/[\u00a0\u202f]/g, ' ')

async function expectText(locator: Locator, text: string | RegExp) {
  const poll = expect.poll(async () => plain(await locator.textContent()))
  if (typeof text === 'string') {
    await poll.toContain(text)
  } else {
    await poll.toMatch(text)
  }
}

test('USD subscription → charges → expense → suggestion → price change → costs in PLN', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom subskrypcji' })
  const home = `/h/${household.id}`
  const api = `/api/households/${household.id}`
  await page.request.post(`${api}/wallets`, {
    data: { name: 'Karta', type: 'card', currency: 'PLN', opening_balance: { amount: '2000.00', currency: 'PLN' } },
  })

  // Pusty przegląd zaprasza do dodania.
  await page.goto(`${home}/subscriptions`)
  await expect(page.getByRole('heading', { name: 'Nie śledzisz jeszcze subskrypcji' })).toBeVisible()
  await expectAccessible(page, 'subskrypcje — pusto')

  // Formularz: dostawca ze słownika, cena w USD, cykl od 31. dnia.
  await page.getByRole('button', { name: 'Nowa subskrypcja' }).click()
  const form = page.getByRole('dialog', { name: 'Nowa subskrypcja' })
  await form.getByRole('combobox', { name: 'Dostawca' }).fill('OpenAI')
  await expect(form.getByRole('textbox', { name: 'Nazwa' })).toHaveValue('OpenAI')
  await form.getByRole('textbox', { name: 'Nazwa' }).fill('ChatGPT Plus')
  await form.getByRole('textbox', { name: 'Cena' }).fill('20')
  await form.getByRole('combobox', { name: 'Waluta' }).selectOption('USD')
  await form.getByLabel('Dzień płatności').fill('2025-01-31')
  await expect(form.getByText('co miesiąc, 31.')).toBeVisible()
  await expect(form.getByText('W krótszych miesiącach — ostatniego dnia miesiąca.')).toBeVisible()
  await form.getByRole('combobox', { name: 'Portfel płatności' }).selectOption({ label: 'Karta (PLN)' })
  await form.getByRole('checkbox', { name: 'tydzień przed' }).check()
  await form.getByText('Zaawansowane: dopasowanie płatności').click()
  await expectAccessible(page, 'formularz subskrypcji')
  await form.getByRole('button', { name: 'Dodaj subskrypcję' }).click()
  await expect(form).toBeHidden()

  // Strona subskrypcji: obciążenia powstały od razu.
  await expect(page.getByRole('heading', { level: 1, name: 'ChatGPT Plus' })).toBeVisible()
  const subscriptionUrl = page.url()
  await expectText(page.getByRole('region', { name: 'Cena i cykl' }), '20,00 USD')
  await expectText(page.getByRole('region', { name: 'Cena i cykl' }), '≈ 80,00 zł')
  const schedule = page.getByRole('list', { name: 'Płatności' })
  await expect(schedule.getByRole('article').first()).toBeVisible()
  const firstDue = (await schedule.getByRole('article').first().locator('time').getAttribute('datetime')) ?? ''
  expect(firstDue).toMatch(/^\d{4}-\d{2}-(28|29|30|31)$/)
  await expectText(schedule.getByRole('article').first(), 'Oczekiwana')
  await expectAccessible(page, 'strona subskrypcji')

  // Wydatek z karty w złotych w dniu płatności (kurs banku nieco inny).
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Transakcje' }).click()
  await page.getByRole('list', { name: 'Transakcje' }).or(page.getByText('Nie ma jeszcze transakcji')).first().waitFor()
  await page.getByRole('button', { name: 'Dodaj transakcję' }).first().click()
  const expense = page.getByRole('dialog', { name: 'Nowa transakcja' })
  await expense.getByRole('textbox', { name: 'Kwota' }).fill('80,40')
  await expense.getByRole('combobox', { name: 'Kontrahent' }).fill('OPENAI *CHATGPT')
  await expense.getByLabel('Data').fill(firstDue)
  await expense.getByRole('button', { name: 'Zapisz' }).click()
  await expect(expense).toBeHidden()
  const row = page.getByRole('article', { name: /OPENAI \*CHATGPT/ })
  await expect(row.getByRole('button', { name: 'Powiąż z subskrypcją: OPENAI *CHATGPT' })).toBeVisible()

  // Sugestia na stronie subskrypcji i potwierdzenie.
  await page.goto(subscriptionUrl)
  const charge = page.getByRole('list', { name: 'Płatności' }).getByRole('article').first()
  await charge.getByRole('button', { name: 'Dopasuj' }).click()
  const match = page.getByRole('dialog', { name: 'Dopasuj płatność' })
  await expect(match.getByText(/Zgodność \d+%/)).toBeVisible()
  await expectAccessible(page, 'dopasowanie płatności')
  await match.getByRole('button', { name: /^Potwierdź: OPENAI \*CHATGPT/ }).click()
  await expect(match).toBeHidden()
  await expectText(charge, 'Dopasowana')
  await expect(charge.getByRole('button', { name: 'Cofnij dopasowanie' })).toBeVisible()

  // Rejestr: plakietka prowadzi do subskrypcji.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Transakcje' }).click()
  const badge = page.getByRole('article', { name: /OPENAI \*CHATGPT/ }).getByRole('link', { name: 'Subskrypcja: ChatGPT Plus' })
  await expect(badge).toBeVisible()
  await badge.click()
  await expect(page.getByRole('heading', { level: 1, name: 'ChatGPT Plus' })).toBeVisible()

  // Podwyżka od dziś: 25 USD.
  await page.getByRole('button', { name: 'Zmień cenę' }).click()
  const price = page.getByRole('dialog', { name: 'Zmiana ceny: ChatGPT Plus' })
  await price.getByRole('textbox', { name: 'Nowa cena' }).fill('25')
  await expectAccessible(page, 'zmiana ceny')
  await price.getByRole('button', { name: 'Zapisz cenę' }).click()
  await expect(price).toBeHidden()
  const history = page.getByRole('list', { name: 'Ceny od najnowszej' })
  await expectText(history.getByRole('listitem').first(), '25,00 USD')
  await expectText(history.getByRole('listitem').first(), '+5,00 USD')

  // Przegląd: koszt w złotych po kursie atrapy, zmiana ceny na liście.
  await page.getByRole('link', { name: 'Wszystkie subskrypcje' }).click()
  await expectText(page.getByRole('article', { name: 'Subskrypcja ChatGPT Plus' }), '≈ 100,00 zł')
  await expectAccessible(page, 'lista subskrypcji')
  await page.getByRole('navigation', { name: 'Subskrypcje' }).getByRole('link', { name: 'Przegląd' }).click()
  const cost = page.getByRole('region', { name: 'Subskrypcje kosztują miesięcznie' })
  await expectText(cost, '100,00 zł')
  await expectText(cost, 'Rocznie 1200,00 zł')
  await expectText(page.getByRole('region', { name: 'Ostatnie zmiany cen' }), /20,00 USD.*25,00 USD/)
  await expectText(page.getByRole('region', { name: 'Miesięcznie wg dostawcy' }), 'OpenAI')
  await expectAccessible(page, 'przegląd subskrypcji')

  // Pulpit: koszt miesięczny i odnowienia.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Pulpit' }).click()
  await expectText(page.getByRole('region', { name: 'Subskrypcje' }), 'Subskrypcje — koszt/mies.100,00 zł')
})

test('matching queue: a past charge is posted with an explicit PLN amount', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom kolejki' })
  const api = `/api/households/${household.id}`
  const wallet = await (await page.request.post(`${api}/wallets`, { data: { name: 'Konto', type: 'checking', currency: 'PLN' } })).json()
  // Termin wczoraj (okno 1 dzień — tyle wstecz powstają obciążenia nowej subskrypcji).
  const yesterday = new Date(Date.now() - 86_400_000).toLocaleDateString('en-CA', { timeZone: 'Europe/Warsaw' })
  const created = await page.request.post(`${api}/subscriptions`, {
    data: {
      name: 'Spotify Duo',
      vendor: 'spotify',
      price: { amount: '9.99', currency: 'EUR' },
      cycle: { unit: 'month', anchor: yesterday },
      wallet_id: wallet.id,
      match_rule: { date_window_days: 1 },
    },
  })
  expect(created.status()).toBe(201)

  await page.goto(`/h/${household.id}/subscriptions/charges`)
  const item = page.getByRole('list', { name: 'Płatności do dopasowania' }).getByRole('article').first()
  await expect(item).toHaveAccessibleName(/^Spotify Duo, płatność/)
  await expectText(item, 'Brak pasujących transakcji.')
  await expectAccessible(page, 'kolejka dopasowań')
  await item.getByRole('button', { name: 'Zaksięguj' }).click()
  const post = page.getByRole('dialog', { name: 'Zaksięguj płatność' })
  await post.getByRole('textbox', { name: 'Zapłacona kwota (PLN)' }).fill('42,17')
  await expectAccessible(page, 'zaksięgowanie płatności')
  await post.getByRole('button', { name: 'Zaksięguj wydatek' }).click()
  await expect(post).toBeHidden()
  await expect(page.getByRole('heading', { name: 'Wszystko dopasowane' })).toBeVisible()

  await page.goto(`/h/${household.id}/transactions`)
  const row = page.getByRole('article', { name: /Spotify/ })
  await expectText(row, '−42,17 zł')
  await expect(row.getByRole('link', { name: 'Subskrypcja: Spotify Duo' })).toBeVisible()
})
