import { expect, test } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi } from './support/helpers.ts'

// Język interfejsu (013): przełącznik na logowaniu (zapis w przeglądarce) i
// profil (PATCH /me). Wszystkie specyfikacje dzielą bazę, a konto startowe ma
// zostać z językiem „automatycznie” (null) — inaczej kolejne testy szukające
// polskich tekstów dostałyby angielski interfejs.

test('login page switches language in the browser', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Zaloguj się' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'pl')

  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await expect(page.getByRole('button', { name: 'English' })).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByRole('button', { name: 'English' })).toBeFocused()
  await expectAccessible(page, 'login in English')

  // Wybór przetrwa przeładowanie.
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  await page.getByRole('button', { name: 'Polski' }).click()
  await expect(page.getByRole('heading', { name: 'Zaloguj się' })).toBeVisible()
})

test.describe('profile language', () => {
  test.afterEach(async ({ page }) => {
    // Bez względu na wynik testu konto wraca do „automatycznie”.
    await page.request.patch('/api/me', { data: { locale: null } })
  })

  test('switches the whole interface to English and back', async ({ page }) => {
    await loginViaApi(page.request)
    const household = await createHousehold(page.request)
    await page.goto(`/h/${household.id}`)

    const rail = page.getByRole('complementary', { name: 'Menu aplikacji' })
    await rail.getByRole('button', { name: /menu konta/ }).click()
    await rail.getByRole('link', { name: 'Profil' }).click()
    await expect(page).toHaveURL(/\/profile$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Profil' })).toBeVisible()

    const select = page.getByRole('combobox', { name: 'Język interfejsu i powiadomień' })
    await expect(select).toHaveValue('auto')
    await select.selectOption('en')

    await expect(page.getByRole('heading', { level: 1, name: 'Profile' })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await expect(page.getByRole('combobox', { name: 'Language of the interface and notifications' })).toHaveValue('en')
    await expect(page.getByText('Language saved.')).toBeVisible()
    await expect(page).toHaveTitle('Profile · Denar')
    await expectAccessible(page, 'profile in English')

    // Zapisane na serwerze: po przeładowaniu i w powłoce gospodarstwa nadal po angielsku.
    const me = await (await page.request.get('/api/me')).json()
    expect(me.user.locale).toBe('en')
    await page.goto(`/h/${household.id}`)
    const englishRail = page.getByRole('complementary', { name: 'App menu' })
    await expect(englishRail.getByRole('navigation', { name: 'Main' })).toBeVisible()
    await expect(englishRail.getByRole('link', { name: 'Wallets' })).toBeVisible()
    await expect(englishRail.getByRole('link', { name: 'Settings' })).toBeVisible()
    await englishRail.getByRole('button', { name: /account menu/ }).click()
    await expect(englishRail.getByRole('button', { name: 'Sign out' })).toBeVisible()
    await expectAccessible(page, 'app shell in English')
    await englishRail.getByRole('link', { name: 'Profile' }).click()

    await page.getByRole('combobox', { name: 'Language of the interface and notifications' }).selectOption('pl')
    await expect(page.getByRole('heading', { level: 1, name: 'Profil' })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('lang', 'pl')

    await page.getByRole('combobox', { name: 'Język interfejsu i powiadomień' }).selectOption('auto')
    await expect.poll(async () => (await (await page.request.get('/api/me')).json()).user.locale).toBeNull()
    await expect(page.getByRole('heading', { level: 1, name: 'Profil' })).toBeVisible()
  })
})
