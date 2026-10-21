import { expect, test, type Locator } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi } from './support/helpers.ts'

// Część C1 na prawdziwym serwerze: wpływ → koperty → przydziały z klawiatury →
// wydatek z kopertą → przekroczenie → następny miesiąc → kwota do przydzielenia.

const plain = (text: string | null) => (text ?? '').replace(/[\u00a0\u202f]/g, ' ')

/** Tekst elementu (spacje twarde → zwykłe) z ponawianiem, aż zawiera oczekiwany. */
async function expectText(locator: Locator, text: string) {
  await expect.poll(async () => plain(await locator.textContent())).toContain(text)
}

test('income → envelopes → assignments → overspending → next month', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom budżetu' })
  const home = `/h/${household.id}`
  const api = `/api/households/${household.id}`
  const wallet = await (
    await page.request.post(`${api}/wallets`, { data: { name: 'Konto', type: 'checking', currency: 'PLN' } })
  ).json()

  // Wpływ z szybkiego dodawania na stronie portfela.
  await page.goto(`${home}/wallets/${wallet.id}`)
  await page.getByRole('list', { name: 'Transakcje' }).or(page.getByText('Nie ma jeszcze transakcji')).waitFor()
  await page.getByRole('button', { name: 'Dodaj transakcję' }).first().click()
  const income = page.getByRole('dialog', { name: 'Nowa transakcja' })
  await income.getByText('Wpływ', { exact: true }).click()
  await income.getByRole('textbox', { name: 'Kwota' }).fill('3000')
  await income.getByRole('combobox', { name: 'Kontrahent' }).fill('Pracodawca')
  await income.getByRole('button', { name: 'Zapisz' }).click()
  await expect(income).toBeHidden()

  // Pusty budżet: cały wpływ czeka na przydział.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Budżet' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Budżet' })).toBeVisible()
  const rta = page.getByRole('group', { name: 'Do przydzielenia' })
  await expectText(rta, '3000,00 zł')
  await expectAccessible(page, 'budżet — pusty')

  // Grupa i koperty z celem.
  await page.getByRole('button', { name: 'Nowa grupa kopert' }).click()
  const group = page.getByRole('dialog', { name: 'Nowa grupa kopert' })
  await group.getByRole('textbox', { name: 'Nazwa grupy' }).fill('Codzienne')
  await expectAccessible(page, 'nowa grupa')
  await group.getByRole('button', { name: 'Dodaj grupę' }).click()
  await expect(group).toBeHidden()

  await page.getByRole('button', { name: 'Dodaj kopertę do grupy Codzienne' }).click()
  let envelope = page.getByRole('dialog', { name: 'Nowa koperta' })
  await envelope.getByRole('textbox', { name: 'Nazwa koperty' }).fill('Jedzenie')
  await envelope.getByText('Co miesiąc', { exact: true }).click()
  await envelope.getByRole('textbox', { name: 'Kwota miesięcznie' }).fill('800')
  await expectAccessible(page, 'nowa koperta z celem')
  await envelope.getByRole('button', { name: 'Dodaj kopertę' }).click()
  await expect(envelope).toBeHidden()
  await page.getByRole('button', { name: 'Dodaj kopertę do grupy Codzienne' }).click()
  envelope = page.getByRole('dialog', { name: 'Nowa koperta' })
  await envelope.getByRole('textbox', { name: 'Nazwa koperty' }).fill('Rozrywka')
  await envelope.getByRole('button', { name: 'Dodaj kopertę' }).click()
  await expect(envelope).toBeHidden()

  // Przydziały z klawiatury: Enter zapisuje, Tab zapisuje i przechodzi dalej.
  const food = page.getByRole('textbox', { name: 'Przydzielono: Jedzenie' })
  const fun = page.getByRole('textbox', { name: 'Przydzielono: Rozrywka' })
  await food.focus()
  await page.keyboard.type('500')
  await page.keyboard.press('Tab')
  await expect(fun).toBeFocused()
  await page.keyboard.type('200')
  await page.keyboard.press('Enter')
  await expectText(rta, '2300,00 zł')
  await expect(food).toHaveValue('500,00')
  await expectText(page.getByRole('article', { name: 'Koperta Jedzenie' }), 'Brakuje 300,00 zł')
  await expectAccessible(page, 'budżet z kopertami')

  // Wydatek z kopertą z rejestru globalnego (skrót n).
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Transakcje' }).click()
  await page.getByRole('list', { name: 'Transakcje' }).waitFor()
  await page.keyboard.press('n')
  const expense = page.getByRole('dialog', { name: 'Nowa transakcja' })
  await expect(expense.getByRole('textbox', { name: 'Kwota' })).toBeFocused()
  await page.keyboard.type('650')
  await expense.getByRole('combobox', { name: 'Kontrahent' }).fill('Targ')
  await expense.getByRole('combobox', { name: 'Koperta' }).selectOption({ label: 'Codzienne · Jedzenie' })
  await expense.getByRole('button', { name: 'Zapisz' }).click()
  await expect(expense).toBeHidden()
  await expect(page.getByRole('article', { name: /Targ/ })).toContainText('Jedzenie')

  // Przekroczenie: koperta na minusie w siatce i na pulpicie.
  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Budżet' }).click()
  const foodRow = page.getByRole('article', { name: 'Koperta Jedzenie' })
  await expectText(foodRow, '−150,00 zł')
  await expect(foodRow.getByTitle(/Przekroczono/)).toBeVisible()
  await expectText(rta, '2300,00 zł')

  // Szczegóły koperty: transakcja miesiąca.
  await foodRow.getByRole('button', { name: 'Jedzenie' }).click()
  const details = page.getByRole('dialog', { name: 'Jedzenie' })
  await expect(details.getByRole('list', { name: 'Transakcje koperty' })).toContainText('Targ')
  await expectAccessible(page, 'szczegóły koperty')
  await page.keyboard.press('Escape')

  await page.getByRole('navigation', { name: 'Główna' }).getByRole('link', { name: 'Pulpit' }).click()
  const tile = page.getByRole('region', { name: /^Budżet — / })
  await expectText(tile.getByRole('list', { name: 'Przekroczone koperty' }), 'Jedzenie−150,00 zł')
  await expectAccessible(page, 'pulpit z budżetem')

  // Następny miesiąc: przekroczenie zeruje kopertę i pomniejsza kwotę do przydzielenia.
  await tile.getByRole('link', { name: 'Przejdź do budżetu' }).click()
  await page.getByRole('link', { name: /^Następny miesiąc/ }).click()
  await expect(page).toHaveURL(/\/budget\/\d{4}-\d{2}$/)
  await expectText(rta, '2150,00 zł')
  await expectText(page.getByRole('list', { name: 'Uwagi do miesiąca' }), '−150,00 zł')
  await expectText(page.getByRole('article', { name: 'Koperta Jedzenie' }), '0,00 zł')
  await expectText(page.getByRole('article', { name: 'Koperta Rozrywka' }), '200,00 zł')

  // Z kwoty do przydzielenia do koperty, potem uzupełnienie celów.
  await page.getByRole('button', { name: 'Przenieś środki' }).click()
  const move = page.getByRole('dialog', { name: 'Przenieś środki' })
  const to = move.getByRole('combobox', { name: 'Do' })
  await to.selectOption((await to.locator('option', { hasText: 'Rozrywka' }).getAttribute('value')) ?? '')
  await move.getByRole('textbox', { name: 'Kwota' }).fill('50')
  await expectAccessible(page, 'przeniesienie środków')
  await move.getByRole('button', { name: 'Przenieś' }).click()
  await expect(move).toBeHidden()
  await expectText(rta, '2100,00 zł')
  await expectText(page.getByRole('article', { name: 'Koperta Rozrywka' }), '250,00 zł')

  await page.getByRole('button', { name: 'Uzupełnij cele' }).click()
  const auto = page.getByRole('dialog', { name: 'Uzupełnianie celów' })
  await expectText(auto.getByRole('list', { name: 'Uzupełnione koperty' }), 'Jedzenie+800,00 zł')
  await expectAccessible(page, 'wynik auto-przydziału')
  await auto.getByRole('button', { name: 'Gotowe' }).click()
  await expectText(rta, '1300,00 zł')
  await expect(page.getByRole('textbox', { name: 'Przydzielono: Jedzenie' })).toHaveValue('800,00')

  // Bieżący miesiąc widzi przydziały przyszłe.
  await page.getByRole('link', { name: 'Bieżący miesiąc' }).click()
  await expectText(page.getByRole('list', { name: 'Uwagi do miesiąca' }), 'Przydzielono na kolejne miesiące: 850,00 zł')
  await expectText(rta, '1450,00 zł')
})

test('envelope in use cannot be deleted, only hidden; order changes with buttons', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom kopert' })
  const api = `/api/households/${household.id}`
  const wallet = await (
    await page.request.post(`${api}/wallets`, {
      data: { name: 'Konto', type: 'checking', currency: 'PLN', opening_balance: { amount: '100.00', currency: 'PLN' } },
    })
  ).json()
  const daily = await (await page.request.post(`${api}/budget/envelope-groups`, { data: { name: 'Codzienne' } })).json()
  const food = await (await page.request.post(`${api}/budget/envelopes`, { data: { group_id: daily.id, name: 'Jedzenie' } })).json()
  await page.request.post(`${api}/budget/envelopes`, { data: { group_id: daily.id, name: 'Paliwo', position: 1 } })
  await page.request.post(`${api}/transactions`, {
    data: { wallet_id: wallet.id, amount: { amount: '-10.00', currency: 'PLN' }, envelope_id: food.id },
  })

  await page.goto(`/h/${household.id}/budget`)
  await page.getByRole('button', { name: 'Układ' }).click()
  await page.getByRole('button', { name: 'Przesuń kopertę Jedzenie w dół' }).click()
  const rows = page.getByRole('list', { name: 'Koperty: Codzienne' }).getByRole('article')
  await expect(rows.first()).toHaveAccessibleName('Koperta Paliwo')
  await expect(page.getByRole('button', { name: 'Przesuń kopertę Jedzenie w górę' })).toBeFocused()
  await expectAccessible(page, 'tryb układu')

  await page.getByRole('button', { name: 'Usuń grupę Codzienne' }).click()
  let confirm = page.getByRole('alertdialog', { name: 'Usunąć grupę Codzienne?' })
  await confirm.getByRole('button', { name: 'Usuń grupę' }).click()
  await expect(confirm.getByText('W grupie są koperty')).toBeVisible()
  await expectAccessible(page, 'grupa niepusta')
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: 'Zakończ układanie' }).click()
  await page.getByRole('article', { name: 'Koperta Jedzenie' }).getByRole('button', { name: 'Jedzenie' }).click()
  await page.getByRole('dialog', { name: 'Jedzenie' }).getByRole('button', { name: 'Usuń' }).click()
  confirm = page.getByRole('alertdialog', { name: 'Usunąć kopertę Jedzenie?' })
  await confirm.getByRole('button', { name: 'Usuń kopertę' }).click()
  await expect(confirm.getByText('Tej koperty nie da się usunąć')).toBeVisible()
  await confirm.getByRole('button', { name: 'Ukryj kopertę' }).click()
  await expect(confirm).toBeHidden()
  await expect(page.getByRole('article', { name: 'Koperta Jedzenie' })).toBeHidden()
  await page.getByRole('checkbox', { name: 'Pokaż ukryte (1)' }).check()
  await expect(page.getByRole('article', { name: 'Koperta Jedzenie' })).toContainText('Ukryta')
})
