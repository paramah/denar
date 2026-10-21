import { createHmac } from 'node:crypto'
import { expect, test, type Page } from '@playwright/test'
import {
  createHousehold,
  deleteAllChannels,
  expectAccessible,
  loginViaApi,
  receivedWebhooks,
  subscriptionWithReminder,
  unique,
  webhookURL,
} from './support/helpers.ts'

// Powiadomienia (005) na binarium: kanał webhook do atrapy z podpisem, test
// kanału, preferencja, przypomnienie subskrypcji → powiadomienie w dzwonku
// i dostawa na webhook. Joby subskrypcji i rozsyłki chodzą co 2 s (playwright.config.ts).

const bell = (page: Page) => page.getByRole('complementary', { name: 'Menu aplikacji' }).getByRole('button', { name: /^Powiadomienia, / })

test('webhook channel → test → preference → reminder in the bell and on the webhook', async ({ page }) => {
  await loginViaApi(page.request)
  await deleteAllChannels(page.request)
  const household = await createHousehold(page.request, { name: 'Dom powiadomień' })
  const home = `/h/${household.id}`
  const hook = `e2e-${unique()}`
  const secret = `sekret-${unique()}`

  // Ustawienia → Powiadomienia: e-mail niedostępny (brak SMTP), dodanie webhooka.
  await page.goto(`${home}/settings/notifications`)
  await expect(page.getByRole('heading', { name: 'Nie masz jeszcze kanałów' })).toBeVisible()
  await expect(page.getByRole('list', { name: 'Niedostępne typy kanałów' })).toContainText('E-mail niedostępny')
  await page.getByRole('button', { name: 'Dodaj kanał' }).first().click()
  const dialog = page.getByRole('dialog', { name: 'Nowy kanał powiadomień' })
  await expect(dialog.getByRole('radio', { name: /E-mail/ })).toBeDisabled()
  await expect(dialog.getByRole('radio', { name: /Webhook/ })).toBeChecked()
  await dialog.getByRole('textbox', { name: 'Nazwa' }).fill('Atrapa')
  await dialog.getByRole('textbox', { name: 'Adres URL' }).fill(webhookURL(hook))
  await dialog.getByLabel('Sekret podpisu (opcjonalnie)').fill(secret)
  await expectAccessible(page, 'formularz kanału')
  await dialog.getByRole('button', { name: 'Dodaj kanał' }).click()
  await expect(dialog).toBeHidden()

  const row = page.getByRole('article', { name: 'Atrapa' })
  await expect(row).toContainText('Niezweryfikowany')
  await expect(row).toContainText('z sekretem')
  await row.getByRole('button', { name: 'Wyślij test: Atrapa' }).click()
  await expect(row.getByRole('status')).toContainText('Test wysłany')
  await expect(row).toContainText('Zweryfikowany')

  // Atrapa dostała wiadomość testową z poprawnym podpisem HMAC.
  const [testMessage] = await receivedWebhooks(page.request, hook)
  expect(testMessage).toBeDefined()
  const signature = `sha256=${createHmac('sha256', secret).update(testMessage!.body).digest('hex')}`
  expect(testMessage!.headers['x-denar-signature']).toBe(signature)

  // Preferencja: odnowienia także na webhook.
  await page.getByRole('checkbox', { name: 'Zbliżające się odnowienie: Atrapa' }).check()
  await page.getByRole('button', { name: 'Zapisz preferencje' }).click()
  await expect(page.getByText('Zapisano preferencje powiadomień.')).toBeVisible()
  await expectAccessible(page, 'ustawienia powiadomień')

  // Subskrypcja z przypomnieniem → job → rozsyłka → dzwonek.
  const sub = await subscriptionWithReminder(page.request, household.id, `Kino ${unique()}`)
  const title = `Odnowienie: ${sub.name} za 3 dni`
  await page.goto(`${home}/notifications?household=${household.id}`)
  await expect(async () => {
    await page.reload()
    await expect(page.getByRole('article', { name: title })).toBeVisible({ timeout: 1_000 })
  }).toPass({ timeout: 30_000 })
  await expectAccessible(page, 'skrzynka powiadomień')

  // Dzwonek pokazuje nieprzeczytane; panel prowadzi do subskrypcji i oznacza przeczytane.
  await expect(bell(page)).toHaveAccessibleName(/Powiadomienia, \d+ nieprzeczytan/)
  await bell(page).click()
  const panel = page.getByRole('region', { name: 'Powiadomienia' })
  await expect(panel.getByRole('link', { name: `Nieprzeczytane: ${title}` })).toBeVisible()
  await expectAccessible(page, 'panel powiadomień')
  await panel.getByRole('link', { name: `Nieprzeczytane: ${title}` }).click()
  await expect(page).toHaveURL(new RegExp(`${home}/subscriptions/${sub.id}$`))
  await expect(page.getByRole('heading', { level: 1, name: sub.name })).toBeVisible()
  await bell(page).click()
  await expect(panel.getByRole('link', { name: title, exact: true })).toBeVisible()

  // Ta sama wiadomość poszła na webhook (preferencja), podpisana.
  await expect(async () => {
    const delivery = (await receivedWebhooks(page.request, hook)).find(
      (r) => r.headers['x-denar-event'] === 'subscription.renewal_upcoming',
    )
    expect(delivery).toBeDefined()
    expect(delivery!.headers['x-denar-signature']).toBe(`sha256=${createHmac('sha256', secret).update(delivery!.body).digest('hex')}`)
    const payload = JSON.parse(delivery!.body) as { title: string; url: string; household: { name: string } }
    expect(payload.title).toBe(title)
    expect(payload.household.name).toBe('Dom powiadomień')
    expect(payload.url).toContain(`/h/${household.id}/subscriptions/${sub.id}`)
  }).toPass({ timeout: 30_000 })
})

test('a failing channel shows the reason; editing keeps or removes the secret', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request)
  const hook = `fail-${unique()}`
  const created = await page.request.post('/api/me/channels', {
    data: {
      type: 'webhook',
      name: `Zepsuty ${hook}`,
      config: { url: webhookURL(hook).replace('/webhook/', '/webhook-fail/') },
      secrets: { secret: 'x' },
    },
  })
  expect(created.status()).toBe(201)
  const name = `Zepsuty ${hook}`

  await page.goto(`/h/${household.id}/settings/notifications`)
  const row = page.getByRole('article', { name })
  await row.getByRole('button', { name: `Wyślij test: ${name}` }).click()
  await expect(row.getByRole('status')).toContainText('Test nieudany')
  await expect(row).toContainText('Ostatni błąd')

  // Edycja: sekret zostaje, potem — usunięty.
  await row.getByRole('button', { name: `Edytuj: ${name}` }).click()
  const dialog = page.getByRole('dialog', { name: `Edycja kanału ${name}` })
  await expect(dialog.getByRole('radio', { name: /Zostaw bez zmian/ })).toBeChecked()
  await expectAccessible(page, 'edycja kanału')
  await dialog.getByRole('radio', { name: /Usuń/ }).check()
  await dialog.getByRole('button', { name: 'Zapisz zmiany' }).click()
  await expect(dialog).toBeHidden()
  await expect(row).not.toContainText('z sekretem')

  await row.getByRole('button', { name: `Usuń: ${name}` }).click()
  await page.getByRole('alertdialog', { name: 'Usunąć kanał?' }).getByRole('button', { name: 'Usuń kanał' }).click()
  await expect(row).toBeHidden()
})
