import { expect, test } from '@playwright/test'
import {
  createHousehold,
  deleteAllChannels,
  expectAccessible,
  loginViaApi,
  subscriptionWithReminder,
  unique,
  webhookURL,
} from './support/helpers.ts'

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
    ['settings notifications', '/settings/notifications', 'Ustawienia'],
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

  test('subscription screens and dialogs', async ({ page }) => {
    const api = `/api${householdPath.replace('/h/', '/households/')}`
    const yesterday = new Date(Date.now() - 86_400_000).toLocaleDateString('en-CA', { timeZone: 'Europe/Warsaw' })
    const sub = await (
      await page.request.post(`${api}/subscriptions`, {
        data: {
          name: 'ChatGPT Plus',
          vendor: 'openai',
          price: { amount: '20.00', currency: 'USD' },
          cycle: { unit: 'month', anchor: yesterday },
          match_rule: { date_window_days: 1 },
        },
      })
    ).json()
    await page.request.post(`${api}/subscriptions`, {
      data: { name: 'Netflix', vendor: 'netflix', price: { amount: '43.00', currency: 'PLN' }, cycle: { unit: 'month', anchor: yesterday }, trial_ends_at: yesterday },
    })
    await page.goto(`${householdPath}/subscriptions`)
    await expect(page.getByRole('region', { name: 'Subskrypcje kosztują miesięcznie' })).toBeVisible()
    await expectAccessible(page, 'przegląd subskrypcji')
    await page.goto(`${householdPath}/subscriptions/list`)
    await expect(page.getByRole('article', { name: 'Subskrypcja Netflix' })).toBeVisible()
    await expectAccessible(page, 'lista subskrypcji')
    await page.goto(`${householdPath}/subscriptions/charges`)
    await expect(page.getByRole('list', { name: 'Płatności do dopasowania' })).toBeVisible()
    await expectAccessible(page, 'kolejka dopasowań')
    await page.goto(`${householdPath}/subscriptions/${sub.id}`)
    await expect(page.getByRole('list', { name: 'Płatności' })).toBeVisible()
    await expectAccessible(page, 'strona subskrypcji')
    await page.getByRole('button', { name: 'Edytuj' }).click()
    await expect(page.getByRole('dialog', { name: 'Edycja: ChatGPT Plus' })).toBeVisible()
    await expectAccessible(page, 'edycja subskrypcji')
    await page.keyboard.press('Escape')
    await page.getByRole('list', { name: 'Płatności' }).getByRole('button', { name: 'Dopasuj' }).first().click()
    await expect(page.getByRole('dialog', { name: 'Dopasuj płatność' })).toBeVisible()
    await expectAccessible(page, 'dopasowanie płatności')
  })

  test('notifications: inbox, bell panel and channel dialogs', async ({ page }) => {
    const hid = householdPath.replace('/h/', '')
    await deleteAllChannels(page.request)
    await page.request.post('/api/me/channels', {
      data: { type: 'webhook', name: 'Hook a11y', config: { url: webhookURL('a11y') }, secrets: { secret: 's' } },
    })
    const sub = await subscriptionWithReminder(page.request, hid, `Kino a11y ${unique()}`)
    await page.goto(`${householdPath}/notifications?household=${hid}`)
    await expect(async () => {
      await page.reload()
      await expect(page.getByRole('article', { name: `Odnowienie: ${sub.name} za 3 dni` })).toBeVisible({ timeout: 1_000 })
    }).toPass({ timeout: 30_000 })
    await expectAccessible(page, 'skrzynka powiadomień')
    let overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(0)

    // Dzwonek: w pasku na komputerze, w górnej belce na telefonie (widoczny jest jeden).
    await page.getByRole('button', { name: /^Powiadomienia, / }).click()
    const panel = page.getByRole('region', { name: 'Powiadomienia' })
    await expect(panel.getByRole('link', { name: `Nieprzeczytane: Odnowienie: ${sub.name} za 3 dni` })).toBeInViewport()
    await expectAccessible(page, 'panel powiadomień')
    await page.keyboard.press('Escape')
    await expect(panel).toBeHidden()

    await page.goto(`${householdPath}/settings/notifications`)
    await expect(page.getByRole('table')).toBeVisible()
    await expectAccessible(page, 'kanały i preferencje')
    overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(0)
    await page.getByRole('button', { name: 'Dodaj kanał' }).first().click()
    await page.getByRole('dialog', { name: 'Nowy kanał powiadomień' }).getByRole('radio', { name: /Matrix/ }).check()
    await expectAccessible(page, 'nowy kanał (Matrix)')
    await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Edytuj: Hook a11y' }).click()
    const edit = page.getByRole('dialog', { name: 'Edycja kanału Hook a11y' })
    await edit.getByRole('radio', { name: /Zmień/ }).check()
    await expect(edit.getByLabel('Sekret podpisu (opcjonalnie)')).toBeVisible()
    await expectAccessible(page, 'edycja kanału z sekretem')
  })

  test('unknown address inside the household', async ({ page }) => {
    await page.goto(`${householdPath}/nie-ma-takiej-strony`)
    await expect(page.getByRole('heading', { level: 1, name: 'Nie ma takiej strony' })).toBeVisible()
    await expectAccessible(page, 'strona 404 w gospodarstwie')
    await page.getByRole('link', { name: 'Wróć na pulpit' }).click()
    await expect(page).toHaveURL(new RegExp(`${householdPath}$`))
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(0)
  })

  test('bot token with the assistant connection', async ({ page }) => {
    await page.goto(`${householdPath}/settings/bots`)
    await page.getByRole('button', { name: 'Nowy token dla Raporty' }).click()
    await page.getByRole('button', { name: 'Wygeneruj nowy token' }).click()
    const dialog = page.getByRole('dialog', { name: 'Token bota Raporty' })
    await expect(dialog.getByRole('region', { name: 'Podłącz do asystenta AI' })).toBeVisible()
    await dialog.getByText('Inne klienty: konfiguracja JSON').click()
    await expectAccessible(page, 'token i podłączenie asystenta')
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(0)
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
