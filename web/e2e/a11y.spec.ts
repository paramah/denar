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
  let walletPath = ''

  test.beforeEach(async ({ page }) => {
    await loginViaApi(page.request)
    const household = await createHousehold(page.request)
    householdPath = `/h/${household.id}`
    // Coś do pokazania na listach.
    const api = `/api/households/${household.id}`
    await page.request.post(`${api}/bots`, { data: { name: 'Raporty', scopes: ['reports:read'] } })
    const wallet = await (
      await page.request.post(`${api}/wallets`, {
        data: { name: 'Konto', type: 'checking', currency: 'PLN', opening_balance: { amount: '1200.00', currency: 'PLN' } },
      })
    ).json()
    await page.request.post(`${api}/wallets`, { data: { name: 'Lokata', type: 'savings', currency: 'USD', on_budget: false } })
    await page.request.post(`${api}/transactions`, {
      data: { wallet_id: wallet.id, amount: { amount: '-35.90', currency: 'PLN' }, payee_name: 'Apteka', tags: ['zdrowie'], status: 'pending' },
    })
    walletPath = `/wallets/${wallet.id}`
    // Budżet: grupa, koperty z celem i przekroczeniem.
    const group = await (await page.request.post(`${api}/budget/envelope-groups`, { data: { name: 'Codzienne' } })).json()
    const food = await (
      await page.request.post(`${api}/budget/envelopes`, {
        data: { group_id: group.id, name: 'Jedzenie', goal: { type: 'monthly', amount: { amount: '500.00', currency: 'PLN' } } },
      })
    ).json()
    await page.request.post(`${api}/budget/envelopes`, { data: { group_id: group.id, name: 'Paliwo', position: 1 } })
    await page.request.post(`${api}/transactions`, {
      data: { wallet_id: wallet.id, amount: { amount: '-120.00', currency: 'PLN' }, payee_name: 'Targ', envelope_id: food.id },
    })
  })

  const screens: [string, string, RegExp | string][] = [
    ['dashboard', '', /Dzień dobry|Dobry wieczór/],
    ['wallets', '/wallets', 'Portfele'],
    ['transactions', '/transactions', 'Transakcje'],
    ['budget', '/budget', 'Budżet'],
    ['subscriptions', '/subscriptions', 'Subskrypcje'],
    ['notifications', '/notifications', 'Powiadomienia'],
    ['settings general', '/settings/general', 'Ustawienia'],
    ['settings members', '/settings/members', 'Ustawienia'],
    ['settings invitations', '/settings/invitations', 'Ustawienia'],
    ['settings bots', '/settings/bots', 'Ustawienia'],
    ['settings payees', '/settings/payees', 'Ustawienia'],
    ['settings rates', '/settings/rates', 'Ustawienia'],
    ['settings audit', '/settings/audit', 'Ustawienia'],
  ]

  for (const [label, path, heading] of screens) {
    test(label, async ({ page }) => {
      await page.goto(householdPath + path)
      await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible()
      await expectAccessible(page, label)
    })
  }

  test('wallet page with register', async ({ page }) => {
    await page.goto(householdPath + walletPath)
    await expect(page.getByRole('heading', { level: 1, name: 'Konto' })).toBeVisible()
    await expect(page.getByRole('article', { name: /Apteka/ })).toBeVisible()
    await expectAccessible(page, 'portfel')
  })

  test('transaction, transfer and wallet dialogs', async ({ page }) => {
    await page.goto(householdPath + walletPath)
    await page.getByRole('button', { name: 'Dodaj transakcję' }).click()
    await expect(page.getByRole('dialog', { name: 'Nowa transakcja' })).toBeVisible()
    await expectAccessible(page, 'dialog transakcji')
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Przelew' }).click()
    await expect(page.getByRole('dialog', { name: 'Przelew między portfelami' })).toBeVisible()
    await expectAccessible(page, 'dialog przelewu')
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Edytuj', exact: true }).click()
    await expect(page.getByRole('dialog', { name: 'Edycja portfela Konto' })).toBeVisible()
    await expectAccessible(page, 'dialog portfela')
  })

  test('budget dialogs and envelope details', async ({ page }) => {
    await page.goto(`${householdPath}/budget`)
    await expect(page.getByRole('article', { name: 'Koperta Jedzenie' })).toBeVisible()
    await expectAccessible(page, 'budżet z kopertami')
    await page.getByRole('article', { name: 'Koperta Jedzenie' }).getByRole('button', { name: 'Jedzenie' }).click()
    await expect(page.getByRole('dialog', { name: 'Jedzenie' }).getByRole('list', { name: 'Transakcje koperty' })).toBeVisible()
    await expectAccessible(page, 'szczegóły koperty')
    await page.getByRole('dialog', { name: 'Jedzenie' }).getByRole('button', { name: 'Edytuj' }).click()
    await expect(page.getByRole('dialog', { name: 'Edycja koperty Jedzenie' })).toBeVisible()
    await expectAccessible(page, 'edycja koperty')
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Przenieś środki' }).click()
    await expect(page.getByRole('dialog', { name: 'Przenieś środki' })).toBeVisible()
    await expectAccessible(page, 'przeniesienie środków')
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Układ' }).click()
    await expectAccessible(page, 'tryb układu')
  })

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
