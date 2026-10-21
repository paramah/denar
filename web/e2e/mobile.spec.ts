import { expect, test } from '@playwright/test'
import { createHousehold, loginViaApi } from './support/helpers.ts'

// Nawigacja na wąskim ekranie: menu w wysuwanym panelu, bez poziomego przewijania.

test('navigation works from the menu button on a narrow screen', async ({ page }) => {
  test.skip((page.viewportSize()?.width ?? 1280) >= 768, 'tylko dla telefonu')
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Telefon' })
  await page.goto(`/h/${household.id}`)

  await page.getByRole('button', { name: 'Otwórz menu' }).click()
  const drawer = page.getByRole('dialog', { name: 'Menu aplikacji' })
  await expect(drawer).toBeVisible()
  await drawer.getByRole('link', { name: 'Ustawienia' }).click()
  await expect(drawer).toBeHidden()
  await expect(page).toHaveURL(new RegExp(`/h/${household.id}/settings/general$`))
  await expect(page.getByRole('heading', { level: 1, name: 'Ustawienia' })).toBeVisible()

  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBeLessThanOrEqual(0)
})

test('the register fits a narrow screen', async ({ page }) => {
  test.skip((page.viewportSize()?.width ?? 1280) >= 768, 'tylko dla telefonu')
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Telefon z portfelem' })
  const api = `/api/households/${household.id}`
  const wallet = await (
    await page.request.post(`${api}/wallets`, {
      data: { name: 'Bardzo długa nazwa portfela na telefon', type: 'checking', currency: 'PLN', opening_balance: { amount: '123456.78', currency: 'PLN' } },
    })
  ).json()
  await page.request.post(`${api}/transactions`, {
    data: { wallet_id: wallet.id, amount: { amount: '-1234.56', currency: 'PLN' }, payee_name: 'Sklep z bardzo długą nazwą kontrahenta', tags: ['dom', 'remont'] },
  })
  for (const path of [`/wallets/${wallet.id}`, '/transactions', '/wallets', '']) {
    await page.goto(`/h/${household.id}${path}`)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await page.waitForLoadState('networkidle')
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow, path).toBeLessThanOrEqual(0)
  }
})
