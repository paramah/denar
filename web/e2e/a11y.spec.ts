import { expect, test } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi } from './support/helpers.ts'

// Dostępność każdego ekranu części A w motywie jasnym, ciemnym i na telefonie.

test('login page', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Zaloguj się' })).toBeVisible()
  await expectAccessible(page, 'logowanie')
})

test('invitation without a token', async ({ page }) => {
  await page.goto('/invite')
  await expect(page.getByRole('heading', { name: 'Brak zaproszenia w linku' })).toBeVisible()
  await expectAccessible(page, 'zaproszenie bez tokenu')
})

test.describe('signed in', () => {
  let householdPath = ''

  test.beforeEach(async ({ page }) => {
    await loginViaApi(page.request)
    const household = await createHousehold(page.request)
    householdPath = `/h/${household.id}`
    // Coś do pokazania na listach.
    await page.request.post(`/api${householdPath.replace('/h/', '/households/')}/bots`, {
      data: { name: 'Raporty', scopes: ['reports:read'] },
    })
  })

  const screens: [string, string, RegExp | string][] = [
    ['dashboard', '', /Dzień dobry|Dobry wieczór/],
    ['wallets', '/wallets', 'Portfele'],
    ['budget', '/budget', 'Budżet'],
    ['subscriptions', '/subscriptions', 'Subskrypcje'],
    ['notifications', '/notifications', 'Powiadomienia'],
    ['settings general', '/settings/general', 'Ustawienia'],
    ['settings members', '/settings/members', 'Ustawienia'],
    ['settings invitations', '/settings/invitations', 'Ustawienia'],
    ['settings bots', '/settings/bots', 'Ustawienia'],
    ['settings audit', '/settings/audit', 'Ustawienia'],
  ]

  for (const [label, path, heading] of screens) {
    test(label, async ({ page }) => {
      await page.goto(householdPath + path)
      await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible()
      await expectAccessible(page, label)
    })
  }

  test('user menu and household switcher', async ({ page, isMobile }) => {
    await page.goto(householdPath)
    if ((page.viewportSize()?.width ?? 1280) < 768 || isMobile) {
      await page.getByRole('button', { name: 'Otwórz menu' }).click()
    }
    const menu = page.getByRole('button', { name: /menu konta/ }).filter({ visible: true })
    await menu.click()
    await expect(page.getByRole('button', { name: 'Wyloguj się' })).toBeVisible()
    await expectAccessible(page, 'menu użytkownika')
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'Wyloguj się' })).toBeHidden()
  })
})
