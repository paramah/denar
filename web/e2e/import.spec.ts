import { expect, test, type Locator } from '@playwright/test'
import { createHousehold, expectAccessible, loginViaApi, previewMbank } from './support/helpers.ts'

// Część C4 na prawdziwym serwerze: wyciąg mBanku (Windows-1250, preambuła,
// stopka, jeden błędny wiersz) → podgląd → import → transakcje w rejestrze
// → ponowny podgląd = same duplikaty → cofnięcie partii.

const plain = (text: string | null) => (text ?? '').replace(/[\u00a0\u202f]/g, ' ')

async function expectText(locator: Locator, text: string | RegExp) {
  const poll = expect.poll(async () => plain(await locator.textContent()))
  if (typeof text === 'string') {
    await poll.toContain(text)
  } else {
    await poll.toMatch(text)
  }
}

test('mBank CSV → preview → import → register → duplicates on a second preview → revert', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom z importem' })
  const home = `/h/${household.id}`
  const api = `/api/households/${household.id}`
  const wallet = (await (
    await page.request.post(`${api}/wallets`, { data: { name: 'eKonto', type: 'checking', currency: 'PLN' } })
  ).json()) as { id: string }
  // Reguła kontrahenta: „biedronka” w opisie banku → Biedronka.
  expect((await page.request.post(`${api}/payees`, { data: { name: 'Biedronka', patterns: ['biedronka'] } })).status()).toBe(201)

  // Ze strony portfela — portfel już wybrany.
  await page.goto(`${home}/wallets/${wallet.id}`)
  await page.getByRole('link', { name: 'Importuj CSV' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Import transakcji' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Portfel docelowy' })).toHaveValue(wallet.id)
  await expect(page.getByText(/do 5 MiB/)).toBeVisible()
  await previewMbank(page)

  // Podgląd: wykryte kodowanie i nagłówki, liczniki, kontrahent z reguły, błąd wiersza.
  const detected = page.getByLabel('Wykryto w pliku')
  await expectText(detected, 'średnik (;)')
  await expectText(detected, 'Windows-1250')
  await expectText(detected, 'Data operacji')
  await expectText(page.getByLabel('Liczniki podglądu'), 'Wiersze4Nowe3Duplikaty0Błędy1')
  const rows = page.getByRole('region', { name: 'Wiersze podglądu' })
  const biedronka = rows.getByRole('row', { name: /BIEDRONKA 1234/ })
  await expect(biedronka.getByRole('cell', { name: 'Biedronka', exact: true })).toBeVisible()
  await expectText(biedronka, '−123,45 zł')
  await expectText(rows.getByRole('row', { name: /WYNAGRODZENIE ACME/ }), '+5000,00 zł')
  await expect(rows.getByText('Nowa')).toHaveCount(3)
  await expectText(page.getByRole('region', { name: 'Błędy wierszy' }), /Linia \d+data: „2026-10-32” to nie data/)
  await expectAccessible(page, 'import — podgląd')

  await page.getByRole('button', { name: 'Importuj 3 transakcje' }).click()
  await expect(page.getByRole('heading', { name: '3. Gotowe' })).toBeVisible()
  await expect(page.getByText('Dodano 3 transakcje do portfela eKonto.')).toBeVisible()
  await expectText(page.getByLabel('Liczniki importu'), 'Nowe transakcje3')
  await expectAccessible(page, 'import — podsumowanie')

  // Rejestr portfela: trzy transakcje z importu.
  await page.getByRole('link', { name: 'Zobacz w rejestrze' }).click()
  const register = page.getByRole('list', { name: 'Transakcje' })
  await expect(register.getByRole('article', { name: /^Biedronka,/ })).toBeVisible()
  await expect(register.getByRole('article', { name: /^NETFLIX\.COM/ })).toBeVisible()
  await expect(register.getByRole('article', { name: /^WYNAGRODZENIE ACME/ })).toBeVisible()
  await expect(register.getByRole('article')).toHaveCount(3)

  // Ten sam plik drugi raz: same duplikaty, nic do zaimportowania.
  await page.getByRole('link', { name: 'Importuj CSV' }).click()
  await previewMbank(page)
  await expectText(page.getByLabel('Liczniki podglądu'), 'Wiersze4Nowe0Duplikaty3Błędy1')
  await expect(page.getByRole('region', { name: 'Wiersze podglądu' }).getByText('Duplikat')).toHaveCount(3)
  await expect(page.getByText(/Nie ma nic nowego do zaimportowania/)).toBeVisible()
  await expect(page.getByRole('button', { name: /^Importuj \d/ })).toHaveCount(0)

  // Historia → partia → cofnięcie.
  await page.getByRole('navigation', { name: 'Import' }).getByRole('link', { name: 'Historia' }).click()
  const batch = page.getByRole('list', { name: 'Partie importu' }).getByRole('article', { name: 'mbank-pazdziernik.csv' })
  await expectText(batch, '3 nowe · 0 dupl. · 1 błąd')
  await expect(batch.getByText('Zaimportowano')).toBeVisible()
  await expectAccessible(page, 'historia importów')
  await batch.getByRole('link', { name: 'mbank-pazdziernik.csv' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'mbank-pazdziernik.csv' })).toBeVisible()
  await expectText(page.getByRole('region', { name: 'Pominięte wiersze' }), '„2026-10-32” to nie data')
  await page.getByRole('button', { name: 'Cofnij import' }).click()
  const confirm = page.getByRole('alertdialog', { name: 'Cofnąć ten import?' })
  await expectText(confirm, 'Usuniemy 3 transakcje')
  await expectAccessible(page, 'cofnięcie importu')
  await confirm.getByRole('button', { name: 'Cofnij import' }).click()
  await expect(page.getByText('Cofnięto import: usunięto 3 transakcje.')).toBeVisible()
  await expect(page.getByText('Import cofnięty')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Cofnij import' })).toHaveCount(0)

  // Rejestr znów pusty; w dzienniku — import i jego cofnięcie po polsku.
  await page.getByRole('link', { name: 'Rejestr portfela' }).click()
  await expect(page.getByText('Nie ma jeszcze transakcji')).toBeVisible()
  await page.goto(`${home}/settings/audit`)
  await expect(page.getByText('Cofnięcie importu').first()).toBeVisible()
  await expect(page.getByText('Import transakcji').first()).toBeVisible()
})

test('connections: empty state leads to the CSV import', async ({ page }) => {
  await loginViaApi(page.request)
  const household = await createHousehold(page.request, { name: 'Dom bez połączeń' })
  await page.goto(`/h/${household.id}/settings/connections`)
  await expect(page.getByRole('heading', { name: 'Brak połączeń' })).toBeVisible()
  await expect(page.getByText(/PSD2, OpenAI, Apple/)).toBeVisible()
  await page.getByRole('main').getByRole('link', { name: 'Importuj CSV' }).click()
  await expect(page.getByRole('heading', { name: '1. Wybierz portfel i plik' }).or(page.getByRole('heading', { name: 'Najpierw dodaj portfel' }))).toBeVisible()
})
