import { expect, test } from '@playwright/test'
import { expectAccessible, loginViaUi, unique } from './support/helpers.ts'

// Główny przepływ części A na prawdziwym serwerze: logowanie → gospodarstwo →
// zaproszenie i rejestracja z linku → bot i jego token → dziennik zmian.

test('the binary serves the SPA and the API on one origin', async ({ request }) => {
  const page = await request.get('/')
  expect(page.status()).toBe(200)
  expect(await page.text()).toContain('<div id="root">')
  const config = await request.get('/api/auth/config')
  expect(await config.json()).toMatchObject({ instance_name: 'Denar E2E', local_login: true, oidc_enabled: false })
})

test('login → household → invitation → bot token → audit', async ({ page, browser, baseURL }) => {
  const householdName = `Dom E2E ${unique()}`
  const invitee = `jan-${unique()}@example.test`

  // Logowanie i nowe gospodarstwo.
  await loginViaUi(page, undefined, undefined, '/households/new')
  await expect(page.getByRole('heading', { level: 1, name: 'Nowe gospodarstwo' })).toBeVisible()
  await expectAccessible(page, 'nowe gospodarstwo')
  await page.getByRole('textbox', { name: 'Nazwa' }).fill(householdName)
  await page.getByRole('combobox', { name: 'Waluta bazowa' }).selectOption('EUR')
  await page.getByRole('combobox', { name: 'Miesiąc budżetowy zaczyna się' }).selectOption('10')
  await page.getByRole('button', { name: 'Załóż gospodarstwo' }).click()
  await expect(page).toHaveURL(/\/h\/[0-9A-Z]+$/)
  const householdUrl = new URL(page.url()).pathname
  await expect(page.getByRole('heading', { level: 1, name: /Ada/ })).toBeVisible()
  await expect(page.getByRole('region', { name: 'Sumy portfeli' })).toContainText('0,00 €')
  await expectAccessible(page, 'pulpit')

  // Zaproszenie: link pokazany raz.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Ustawienia' }).click()
  await page.getByRole('navigation', { name: 'Ustawienia' }).getByRole('link', { name: 'Zaproszenia' }).click()
  await page.getByRole('textbox', { name: 'E-mail' }).fill(invitee)
  await page.getByRole('combobox', { name: 'Rola' }).selectOption('member')
  await page.getByRole('button', { name: 'Utwórz link' }).click()
  const linkDialog = page.getByRole('dialog', { name: 'Link z zaproszeniem' })
  const link = await linkDialog.getByRole('textbox', { name: 'Link z zaproszeniem' }).inputValue()
  expect(link).toMatch(new RegExp(`^${baseURL}/invite#.+`))
  await expectAccessible(page, 'link z zaproszeniem')
  await linkDialog.getByRole('button', { name: 'Gotowe' }).click()
  await expect(page.getByText(invitee)).toBeVisible()

  // Zaproszona osoba w osobnej przeglądarce zakłada konto z linku.
  const guest = await browser.newContext({ locale: 'pl-PL', colorScheme: 'dark', reducedMotion: 'reduce' })
  const guestPage = await guest.newPage()
  await guestPage.goto(link)
  await expect(guestPage.getByRole('heading', { name: `Dołącz do gospodarstwa ${householdName}` })).toBeVisible()
  await expect(guestPage).toHaveURL(/\/invite$/)
  await expectAccessible(guestPage, 'zaproszenie')
  await guestPage.getByLabel('Imię lub nazwa').fill('Jan Nowak')
  await guestPage.getByLabel('Hasło', { exact: true }).fill('haslo-jana-123456')
  await guestPage.getByLabel('Powtórz hasło').fill('haslo-jana-123456')
  await guestPage.getByRole('button', { name: 'Załóż konto i dołącz' }).click()
  await expect(guestPage).toHaveURL(new RegExp(`${householdUrl}$`))
  await expect(guestPage.getByRole('heading', { level: 1, name: /Jan/ })).toBeVisible()
  await expect(guestPage.getByText('Domownik', { exact: true })).toBeVisible()
  await guest.close()

  // Bot: tworzenie, jednorazowy token, wywołanie API Bearerem.
  await page.getByRole('navigation', { name: 'Ustawienia' }).getByRole('link', { name: 'Boty' }).click()
  await page.getByRole('button', { name: 'Utwórz pierwszego bota' }).click()
  const botDialog = page.getByRole('dialog', { name: 'Nowy bot' })
  await botDialog.getByRole('textbox', { name: 'Nazwa' }).fill('Claude E2E')
  await botDialog.getByRole('checkbox', { name: 'Transakcje: zapis' }).check()
  await expectAccessible(page, 'nowy bot')
  await botDialog.getByRole('button', { name: 'Utwórz bota' }).click()
  const tokenDialog = page.getByRole('dialog', { name: 'Token bota Claude E2E' })
  const token = await tokenDialog.getByRole('textbox', { name: 'Token' }).inputValue()
  expect(token).toMatch(/^dnr_/)
  await tokenDialog.getByRole('button', { name: 'Zapisałem token' }).click()
  await expect(page.getByRole('article', { name: 'Bot Claude E2E' })).toContainText('Aktywny')

  const anonymous = await page.context().browser()!.newContext()
  const asBot = await anonymous.request.get(`${baseURL}/api/me`, { headers: { Authorization: `Bearer ${token}` } })
  expect(asBot.status()).toBe(200)
  expect(await asBot.json()).toMatchObject({ kind: 'bot', bot: { name: 'Claude E2E' } })
  await anonymous.close()

  // Dziennik zmian pokazuje, co się stało, z nazwami aktorów.
  await page.getByRole('navigation', { name: 'Ustawienia' }).getByRole('link', { name: 'Dziennik zmian' }).click()
  const entries = page.getByRole('list', { name: 'Wpisy dziennika' })
  for (const action of ['Utworzenie bota', 'Dołączenie do gospodarstwa', 'Wysłanie zaproszenia', 'Założenie gospodarstwa']) {
    await expect(entries.getByText(action)).toBeVisible()
  }
  await expect(entries.getByText('Jan Nowak')).toBeVisible()
  await page.getByRole('combobox', { name: 'Czego dotyczy' }).selectOption('bot')
  await expect(entries.getByText('Założenie gospodarstwa')).toHaveCount(0)
  await expect(entries.getByText('Utworzenie bota')).toBeVisible()
  await expectAccessible(page, 'dziennik zmian')
})

test('a lost session sends the user to the login page and back', async ({ page }) => {
  await loginViaUi(page)
  await expect(page).not.toHaveURL(/\/login/)
  await page.context().clearCookies()
  await page.goto('/households/new')
  await expect(page).toHaveURL(/\/login\?return_to=%2Fhouseholds%2Fnew$/)
  await loginViaUi(page, undefined, undefined, '/households/new')
  await expect(page.getByRole('heading', { level: 1, name: 'Nowe gospodarstwo' })).toBeVisible()
})
