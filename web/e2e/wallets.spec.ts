import { expect, test } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi } from './support/helpers.ts'

// Część B na prawdziwym serwerze: portfele PLN i USD → wydatek → przelew
// wielowalutowy → salda (kurs USD z atrapy NBP: 4,0 zł) → rejestr i pulpit.

test('wallets → expense → multi-currency transfer → balances', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom portfeli' })
  const home = `/h/${household.id}`

  // Pierwszy portfel z pustego stanu, z saldem otwarcia.
  await page.goto(`${home}/wallets`)
  await expect(page.getByRole('heading', { level: 1, name: 'Portfele' })).toBeVisible()
  await expectAccessible(page, 'portfele — pusty stan')
  await page.getByRole('button', { name: 'Dodaj pierwszy portfel' }).click()
  const create = page.getByRole('dialog', { name: 'Nowy portfel' })
  await create.getByRole('textbox', { name: 'Nazwa' }).fill('Konto')
  await create.getByRole('textbox', { name: 'Saldo otwarcia' }).fill('1000')
  await expectAccessible(page, 'nowy portfel')
  await create.getByRole('button', { name: 'Dodaj portfel' }).click()
  await expect(page.getByRole('article', { name: 'Portfel Konto' })).toContainText('1000,00 zł')

  // Drugi portfel w dolarach, bez salda.
  await page.getByRole('button', { name: 'Nowy portfel' }).click()
  const usd = page.getByRole('dialog', { name: 'Nowy portfel' })
  await usd.getByRole('textbox', { name: 'Nazwa' }).fill('Dolary')
  await usd.getByRole('combobox', { name: 'Waluta' }).selectOption('USD')
  await usd.getByRole('button', { name: 'Dodaj portfel' }).click()
  await expect(page.getByRole('article', { name: 'Portfel Dolary' })).toContainText('0,00 USD')
  await expectAccessible(page, 'portfele')

  // Wydatek z klawiatury na stronie portfela, z nowym kontrahentem.
  await page.getByRole('link', { name: 'Konto' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Konto' })).toBeVisible()
  await page.getByRole('list', { name: 'Transakcje' }).waitFor()
  await page.keyboard.press('n')
  const expense = page.getByRole('dialog', { name: 'Nowa transakcja' })
  await expect(expense.getByRole('textbox', { name: 'Kwota' })).toBeFocused()
  await page.keyboard.type('50')
  await expense.getByRole('combobox', { name: 'Kontrahent' }).fill('Piekarnia Nowa')
  await expectAccessible(page, 'nowa transakcja')
  await page.keyboard.press('Enter')
  await expect(expense).toBeHidden()
  const balances = page.getByRole('definition').first()
  await expect(balances).toHaveText('950,00 zł')
  await expect(page.getByRole('article', { name: /Piekarnia Nowa/ })).toContainText('−50,00 zł')
  await expectAccessible(page, 'portfel z rejestrem')

  // Przelew PLN → USD z kwotą docelową.
  await page.getByRole('button', { name: 'Przelew' }).click()
  const transfer = page.getByRole('dialog', { name: 'Przelew między portfelami' })
  await transfer.getByRole('combobox', { name: 'Do portfela' }).selectOption({ label: 'Dolary (USD)' })
  await transfer.getByRole('textbox', { name: 'Kwota wysłana (PLN)' }).fill('400')
  await transfer.getByRole('textbox', { name: 'Kwota otrzymana (USD)' }).fill('100')
  await expectAccessible(page, 'przelew')
  await transfer.getByRole('button', { name: 'Przelej' }).click()
  await expect(transfer).toBeHidden()
  await expect(balances).toHaveText('550,00 zł')
  await expect(page.getByRole('article', { name: /Przelew do: Dolary/ })).toContainText('−400,00 zł')

  // Salda: USD po kursie z atrapy NBP (100 × 4,0 zł), suma budżetowa w złotych.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Portfele' }).click()
  const dollars = page.getByRole('article', { name: 'Portfel Dolary' })
  await expect(dollars).toContainText('100,00 USD')
  await expect(dollars).toContainText('≈ 400,00 zł')
  await expect(page.getByRole('region', { name: 'Sumy portfeli' })).toContainText('Na portfelach budżetowych950,00 zł')

  // Rejestr globalny: filtr rodzaju w adresie, szybka zmiana statusu.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Transakcje' }).click()
  const register = page.getByRole('list', { name: 'Transakcje' })
  await expect(register.getByRole('article')).toHaveCount(4)
  await expect(register.getByRole('article', { name: /Przelew z: Konto/ })).toContainText('+100,00 USD')
  await expect(register.getByRole('article', { name: /Przelew z: Konto/ })).toContainText('+400,00 zł')
  await page.getByRole('combobox', { name: 'Rodzaj' }).selectOption('transfer')
  await expect(page).toHaveURL(/kind=transfer/)
  await expect(register.getByRole('article')).toHaveCount(2)
  await page.getByRole('button', { name: 'Wyczyść filtry' }).click()
  const bakery = register.getByRole('article', { name: /Piekarnia Nowa/ })
  await bakery.getByRole('button', { name: 'Rozliczona — oznacz jako oczekująca' }).click()
  await expect(bakery.getByRole('button', { name: 'Oczekująca — oznacz jako rozliczona' })).toBeVisible()
  await expectAccessible(page, 'rejestr transakcji')

  // Kontrahent powstał z nazwy.
  await page.goto(`${home}/settings/payees`)
  await expect(page.getByRole('article', { name: 'Kontrahent Piekarnia Nowa' })).toBeVisible()

  // Pulpit: suma i ostatnie transakcje.
  await page.goto(home)
  await expect(page.getByRole('region', { name: 'Sumy portfeli' })).toContainText('950,00 zł')
  await expect(page.getByRole('list', { name: 'Ostatnie transakcje' })).toContainText('Piekarnia Nowa')
  await expectAccessible(page, 'pulpit z portfelami')
})

test('an owner checks and sets a manual rate', async ({ page }) => {
  await loginViaApi(page.request)
  // Waluta bazowa EUR: kurs CHF liczony krzyżowo przez PLN z atrapy (4,0 / 4,0 = 1).
  const household = await createHousehold(page.request, { name: 'Dom kursów', base_currency: 'EUR' })
  const api = `/api/households/${household.id}`
  const wallet = await (
    await page.request.post(`${api}/wallets`, { data: { name: 'Franki', type: 'cash', currency: 'CHF' } })
  ).json()
  const created = await page.request.post(`${api}/transactions`, {
    data: { wallet_id: wallet.id, amount: { amount: '-10.00', currency: 'CHF' }, payee_name: 'Kiosk', date: '2026-10-20' },
  })
  expect(created.status()).toBe(201)
  const transaction = await created.json()
  expect(transaction.base_amount).toMatchObject({ amount: '-10.00', currency: 'EUR' })

  await page.goto(`/h/${household.id}/settings/rates?currency=CHF&date=2026-10-19`)
  await expect(page.getByRole('heading', { level: 2, name: 'Kursy walut' })).toBeVisible()
  const form = page.getByRole('form', { name: 'Ustaw kurs ręcznie' })
  await expect(form.getByRole('combobox', { name: 'Waluta' })).toHaveValue('CHF')
  await form.getByRole('textbox', { name: /Kurs/ }).fill('1,05')
  await expectAccessible(page, 'kursy walut')
  await form.getByRole('button', { name: 'Zapisz kurs' }).click()
  await expect(page.getByRole('article', { name: /Kurs CHF z 19 paź 2026/ })).toContainText('1 CHF = 1,05 EUR')
})
