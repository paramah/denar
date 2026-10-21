import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import type { CsvImportOptions } from '../../api/types.ts'
import * as f from '../../test/msw/fixtures.ts'
import { problemResponse } from '../../test/msw/handlers.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

const plain = (value: string | null | undefined) => (value ?? '').replace(/[\u00a0\u202f]/g, ' ')

const konto = f.wallet()
const usd = f.wallet({ id: 'w-usd', name: 'Dolary', currency: 'USD' })

type Upload = { fileName: string; text: string; options: CsvImportOptions }

/** Zapisuje części multipart każdego żądania pod ścieżką. */
function captureUploads(path: string, respond: (upload: Upload, n: number) => Response) {
  const uploads: Upload[] = []
  server.use(
    http.post(apiUrl(path), async ({ request }) => {
      expect(request.headers.get('content-type')).toMatch(/^multipart\/form-data; boundary=/)
      const data = await request.formData()
      const file = data.get('file') as File
      const upload = { fileName: file.name, text: await file.text(), options: JSON.parse(String(data.get('options'))) }
      uploads.push(upload)
      return respond(upload, uploads.length)
    }),
  )
  return uploads
}

const csv = (name = 'wyciag.csv', body = 'Data;Kwota;Kontrahent\n2026-10-05;-49,00;NETFLIX.COM\n') =>
  new File([body], name, { type: 'text/csv' })

beforeEach(() => {
  server.use(http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [konto, usd] })))
})

async function chooseFile(user: ReturnType<typeof renderApp>['user'], file = csv()) {
  await user.upload(await screen.findByLabelText('Plik CSV'), file)
}

describe('import wizard', () => {
  it('previews a file with a bank preset, imports it and links to the register and the charge queue', async () => {
    const previews = captureUploads('/households/:hid/imports/csv/preview', () =>
      HttpResponse.json(f.importPreview({ preset: 'mbank' })),
    )
    const imports = captureUploads('/households/:hid/imports/csv', () =>
      HttpResponse.json(f.importBatch({ preset: 'mbank' }), { status: 201 }),
    )
    const { user, router } = renderApp('/h/h-dom/import')

    expect(await screen.findByRole('heading', { level: 1, name: 'Import transakcji' })).toBeInTheDocument()
    const steps = await screen.findByRole('list', { name: 'Kroki importu' })
    expect(within(steps).getByText('Plik').closest('li')).toHaveAttribute('aria-current', 'step')
    expect(screen.getByText(/do 5 MiB/)).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Portfel docelowy' })).toHaveValue('w-konto')
    expect(screen.getByRole('radio', { name: /Dowolny CSV/ })).toBeChecked()

    await chooseFile(user)
    expect(screen.getByText('wyciag.csv')).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: /mBank/ }))
    await user.click(screen.getByRole('button', { name: 'Dalej: podgląd' }))

    expect(await screen.findByRole('heading', { name: '2. Sprawdź podgląd' })).toHaveFocus()
    expect(previews).toHaveLength(1)
    // Nazwę pliku jsdom gubi między swoim FormData a Request z Node ("blob"); sprawdza ją e2e.
    expect(previews[0]!.text).toContain('NETFLIX.COM')
    expect(previews[0]!.options).toEqual({ wallet_id: 'w-konto', preset: 'mbank', preview_limit: 50 })

    const detected = screen.getByLabelText('Wykryto w pliku')
    expect(plain(detected.textContent)).toContain('Separatorśrednik (;)')
    expect(plain(detected.textContent)).toContain('Nagłóweklinia 1')
    expect(plain(detected.textContent)).toContain('KolumnyDataKwotaKontrahentTytuł')
    expect(plain(screen.getByLabelText('Liczniki podglądu').textContent)).toBe('Wiersze3Nowe2Duplikaty0Błędy1')
    const rows = screen.getByRole('region', { name: 'Wiersze podglądu' })
    const netflix = within(rows).getByText('NETFLIX.COM').closest('tr')!
    expect(within(netflix).getByText('Nowa')).toBeInTheDocument()
    expect(within(netflix).getByText('nie rozpoznano')).toBeInTheDocument()
    expect(plain(netflix.textContent)).toContain('−49,00 zł')
    expect(within(rows).getByText('Pracodawca')).toBeInTheDocument()
    const errors = screen.getByRole('region', { name: 'Błędy wierszy' })
    expect(plain(errors.textContent)).toContain('Linia 4')
    expect(plain(errors.textContent)).toContain('kwota: „abc” to nie liczba.')

    await user.click(screen.getByRole('button', { name: 'Importuj 2 transakcje' }))
    expect(await screen.findByRole('heading', { name: '3. Gotowe' })).toBeInTheDocument()
    expect(imports[0]!.options).toEqual({ wallet_id: 'w-konto', preset: 'mbank' })
    expect(screen.getByText('Dodano 2 transakcje do portfela Konto.')).toBeInTheDocument()
    expect(screen.getByText('Subskrypcje odhaczone same')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Zobacz w rejestrze' })).toHaveAttribute('href', '/h/h-dom/wallets/w-konto')
    expect(screen.getByRole('link', { name: 'Kolejka dopasowań subskrypcji' })).toHaveAttribute(
      'href',
      '/h/h-dom/subscriptions/charges',
    )
    await user.click(screen.getByRole('link', { name: 'Szczegóły importu' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/import/imp-1'))
    // Partia z odpowiedzi importu jest już w cache — bez GET.
    expect(await screen.findByRole('heading', { level: 1, name: 'wyciag.csv' })).toBeInTheDocument()
  })

  it('takes the wallet from the address (the wallet page link)', async () => {
    renderApp('/h/h-dom/import?wallet=w-usd')
    expect(await screen.findByRole('combobox', { name: 'Portfel docelowy' })).toHaveValue('w-usd')
  })

  it('adjusts the parser options and previews again with only the overrides', async () => {
    const previews = captureUploads('/households/:hid/imports/csv/preview', () => HttpResponse.json(f.importPreview()))
    const { user } = renderApp('/h/h-dom/import')
    await chooseFile(user)
    await user.click(await screen.findByRole('button', { name: 'Dalej: podgląd' }))
    await screen.findByRole('heading', { name: '2. Sprawdź podgląd' })

    const toggle = screen.getByRole('button', { name: 'Dostosuj' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await user.click(toggle)
    const panel = screen.getByRole('form', { name: 'Dostosuj odczyt pliku' })
    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Separator pól' }), ',')
    await user.type(within(panel).getByRole('textbox', { name: 'Format daty' }), 'DD.MM.YYYY')
    await user.click(within(panel).getByRole('checkbox', { name: /Odwróć znak kwot/ }))
    // Kolumny z wykrytych nagłówków; alternatywy jako lista.
    const payee = within(panel).getByRole('combobox', { name: 'Kontrahent' })
    expect(within(payee).getByRole('option', { name: 'Jak w presecie: Kontrahent' })).toBeInTheDocument()
    await user.selectOptions(payee, 'Tytuł')
    await user.click(within(panel).getByRole('button', { name: 'Dodaj alternatywę: Kontrahent' }))
    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Kontrahent — alternatywa 2' }), 'Kontrahent')
    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Identyfikator operacji' }), 'Nie używaj')
    await user.click(within(panel).getByRole('button', { name: 'Zastosuj i odśwież podgląd' }))

    await waitFor(() => expect(previews).toHaveLength(2))
    expect(previews[1]!.options).toEqual({
      wallet_id: 'w-konto',
      preset: 'generic',
      preview_limit: 50,
      delimiter: ',',
      date_format: 'DD.MM.YYYY',
      invert_amount: true,
      columns: { payee: ['Tytuł', 'Kontrahent'], external_id: [] },
    })

    await user.click(within(panel).getByRole('button', { name: 'Przywróć ustawienia presetu' }))
    expect(within(panel).getByRole('combobox', { name: 'Separator pól' })).toHaveValue('')
    expect(within(panel).getByRole('combobox', { name: 'Kontrahent' })).toHaveValue('__preset__')
    expect(within(panel).queryByRole('button', { name: 'Przywróć ustawienia presetu' })).not.toBeInTheDocument()
  })

  it('opens the adjust panel with a Polish message when the date column is not found', async () => {
    const previews = captureUploads('/households/:hid/imports/csv/preview', (_, n) =>
      n === 1
        ? problemResponse(422, 'validation', {
            errors: [{ field: 'options.columns.date', message: 'no header row has the date column "date / data"' }],
          })
        : HttpResponse.json(f.importPreview({ detected: { delimiter: ',', encoding: 'utf-8', header_line: null, headers: [] } })),
    )
    const { user } = renderApp('/h/h-dom/import')
    await chooseFile(user)
    await user.click(await screen.findByRole('button', { name: 'Dalej: podgląd' }))

    expect(await screen.findByText('Nie udało się odczytać pliku z tymi ustawieniami')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Importuj/ })).not.toBeInTheDocument()
    const panel = screen.getByRole('form', { name: 'Dostosuj odczyt pliku' })
    // Bez wykrytych nagłówków kolumny wpisuje się ręcznie (nazwy lub numery).
    const date = within(panel).getByRole('textbox', { name: 'Data' })
    expect(date).toHaveAccessibleDescription(
      /Nie znaleziono wiersza nagłówka z kolumną daty \(date \/ data\)\. Wskaż kolumnę daty/,
    )
    await user.click(within(panel).getByRole('checkbox', { name: /Plik ma wiersz nagłówka/ }))
    await user.type(date, '1')
    await user.type(within(panel).getByRole('textbox', { name: 'Kwota' }), '2, 3')
    await user.click(within(panel).getByRole('button', { name: 'Zastosuj i odśwież podgląd' }))

    await waitFor(() => expect(previews).toHaveLength(2))
    expect(previews[1]!.options).toMatchObject({ has_header: false, columns: { date: ['1'], amount: ['2', '3'] } })
    expect(await screen.findByRole('button', { name: 'Importuj 2 transakcje' })).toBeInTheDocument()
    expect(screen.queryByText('Nie udało się odczytać pliku z tymi ustawieniami')).not.toBeInTheDocument()
  })

  it('refuses a file over 5 MiB before sending it and shows file errors of the server', async () => {
    captureUploads('/households/:hid/imports/csv/preview', () =>
      problemResponse(422, 'validation', { errors: [{ field: 'file', message: 'has no transactions' }] }),
    )
    const { user } = renderApp('/h/h-dom/import')
    const big = new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'duzy.csv', { type: 'text/csv' })
    await chooseFile(user, big)
    const input = screen.getByLabelText('Plik CSV')
    expect(input).toHaveAccessibleDescription(/większy niż 5 MiB/)
    await user.click(screen.getByRole('button', { name: 'Dalej: podgląd' }))
    expect(input).toHaveAccessibleDescription(/Wybierz plik CSV z wyciągiem/)

    await chooseFile(user)
    await user.click(screen.getByRole('button', { name: 'Dalej: podgląd' }))
    await waitFor(() => expect(input).toHaveAccessibleDescription(/W pliku nie ma żadnych operacji\./))
    expect(screen.getByRole('heading', { name: '1. Wybierz portfel i plik' })).toBeInTheDocument()
  })

  it('offers no import when every row is a duplicate', async () => {
    captureUploads('/households/:hid/imports/csv/preview', () =>
      HttpResponse.json(
        f.importPreview({
          new_count: 0,
          duplicate_count: 2,
          error_count: 0,
          errors: [],
          rows: [f.importPreviewRow({ status: 'duplicate' })],
        }),
      ),
    )
    const { user } = renderApp('/h/h-dom/import')
    await chooseFile(user)
    await user.click(await screen.findByRole('button', { name: 'Dalej: podgląd' }))
    expect(await screen.findByText(/Nie ma nic nowego do zaimportowania — wszystkie operacje już są w portfelu/)).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Wiersze podglądu' })).getByText('Duplikat')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Importuj \d/ })).not.toBeInTheDocument()
    // Wstecz zachowuje plik.
    await user.click(screen.getByRole('button', { name: 'Wstecz' }))
    expect(screen.getByText('wyciag.csv')).toBeInTheDocument()
  })

  it('explains to a viewer that importing needs write access', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'viewer' })]))))
    renderApp('/h/h-dom/import')
    expect(await screen.findByRole('heading', { name: 'Import wymaga prawa zapisu' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Plik CSV')).not.toBeInTheDocument()
  })

  it('is linked from the wallet page and the register', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/wallets/w-konto'), () => HttpResponse.json(konto)),
      http.get(apiUrl('/households/:hid/wallets/w-konto'), () => HttpResponse.json(konto)),
    )
    const { router } = renderApp('/h/h-dom/wallets/w-konto')
    expect(await screen.findByRole('link', { name: 'Importuj CSV' })).toHaveAttribute('href', '/h/h-dom/import?wallet=w-konto')
    await router.navigate('/h/h-dom/transactions')
    await waitFor(() =>
      expect(screen.getByRole('link', { name: 'Importuj CSV' })).toHaveAttribute('href', '/h/h-dom/import'),
    )
  })
})

describe('import history', () => {
  it('lists batches with counters, filters by wallet and opens the details', async () => {
    const queries: (string | null)[] = []
    server.use(
      http.get(apiUrl('/households/:hid/imports'), ({ request }) => {
        const wallet = new URL(request.url).searchParams.get('wallet_id')
        queries.push(wallet)
        return HttpResponse.json({
          items: wallet
            ? [f.importBatch({ id: 'imp-2', wallet_id: 'w-usd', source_ref: 'revolut.csv', preset: 'revolut' })]
            : [f.importBatch(), f.importBatch({ id: 'imp-0', status: 'reverted', reverted_count: 2, source_ref: 'stary.csv' })],
          next_cursor: null,
        })
      }),
    )
    const { user, router } = renderApp('/h/h-dom/import/history')
    const list = await screen.findByRole('list', { name: 'Partie importu' })
    const first = within(list).getByRole('article', { name: 'wyciag.csv' })
    expect(plain(first.textContent)).toContain('Konto · generic · Ada Kowalska')
    expect(plain(first.textContent)).toContain('2 nowe · 0 dupl. · 1 błąd · 1 subskr.')
    expect(within(first).getByText('Zaimportowano')).toBeInTheDocument()
    expect(within(within(list).getByRole('article', { name: 'stary.csv' })).getByText('Cofnięto')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Portfel' }), 'w-usd')
    expect(await within(screen.getByRole('list', { name: 'Partie importu' })).findByText('revolut.csv')).toBeInTheDocument()
    expect(queries).toEqual([null, 'w-usd'])
    expect(router.state.location.search).toBe('?wallet=w-usd')
  })

  it('invites to the first import when there is none', async () => {
    renderApp('/h/h-dom/import/history')
    expect(await screen.findByRole('heading', { name: 'Nie ma jeszcze importów' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Importuj CSV' })).toHaveAttribute('href', '/h/h-dom/import')
  })
})

describe('import batch', () => {
  it('shows the row errors and reverts the import after confirmation', async () => {
    let reverted = false
    server.use(
      http.get(apiUrl('/households/:hid/imports/imp-1'), () =>
        HttpResponse.json(reverted ? f.importBatch({ status: 'reverted', reverted_count: 2 }) : f.importBatch()),
      ),
      http.delete(apiUrl('/households/:hid/imports/imp-1'), () => {
        reverted = true
        return HttpResponse.json(
          f.importBatch({
            status: 'reverted',
            reverted_count: 2,
            reverted_by: { kind: 'user', id: 'u-ada' },
            reverted_at: '2026-10-21T09:00:00Z',
          }),
        )
      }),
    )
    const { user } = renderApp('/h/h-dom/import/imp-1')
    expect(await screen.findByRole('heading', { level: 1, name: 'wyciag.csv' })).toBeInTheDocument()
    const skipped = screen.getByRole('region', { name: 'Pominięte wiersze' })
    expect(plain(skipped.textContent)).toContain('Linia 4kwota: „abc” to nie liczba.')

    await user.click(screen.getByRole('button', { name: 'Cofnij import' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Cofnąć ten import?' })
    expect(plain(dialog.textContent)).toContain('Usuniemy 2 transakcje z tego importu')
    await user.click(within(dialog).getByRole('button', { name: 'Cofnij import' }))

    expect(await screen.findByText('Cofnięto import: usunięto 2 transakcje.')).toBeInTheDocument()
    expect(await screen.findByText('Import cofnięty')).toBeInTheDocument()
    expect(screen.getByText('Cofnięto')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Cofnij import' })).not.toBeInTheDocument()
  })

  it('says in Polish that the import was reverted already (409)', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/imports/imp-1'), () => HttpResponse.json(f.importBatch())),
      http.delete(apiUrl('/households/:hid/imports/imp-1'), () =>
        problemResponse(409, 'conflict', { reason: 'import-reverted' }),
      ),
    )
    const { user } = renderApp('/h/h-dom/import/imp-1')
    await user.click(await screen.findByRole('button', { name: 'Cofnij import' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Cofnąć ten import?' })
    await user.click(within(dialog).getByRole('button', { name: 'Cofnij import' }))
    expect(await within(dialog).findByText('Ten import został już cofnięty.')).toBeInTheDocument()
  })

  it('shows a missing batch as not found', async () => {
    server.use(http.get(apiUrl('/households/:hid/imports/nope'), () => problemResponse(404, 'not_found')))
    renderApp('/h/h-dom/import/nope')
    expect(await screen.findByRole('heading', { level: 1, name: 'Nie znaleziono importu' })).toBeInTheDocument()
  })
})

describe('connections tab', () => {
  it('explains the empty state and leads to the CSV import', async () => {
    renderApp('/h/h-dom/settings/connections')
    expect(await screen.findByRole('heading', { name: 'Brak połączeń' })).toBeInTheDocument()
    expect(
      screen.getByText(/Integracje z bankami i dostawcami \(PSD2, OpenAI, Apple\) pojawią się w przyszłych wersjach; teraz użyj importu CSV/),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Importuj CSV' })).toHaveAttribute('href', '/h/h-dom/import')
    expect(await screen.findByText(/Żaden dostawca nie jest jeszcze dostępny/)).toBeInTheDocument()
  })

  it('lists connections with status, last sync, error and mapped accounts', async () => {
    server.use(
      http.get(apiUrl('/households/:hid/connections'), () =>
        HttpResponse.json({
          items: [
            f.connection(),
            f.connection({
              id: 'c-2',
              display_name: 'OpenAI',
              provider_name: 'OpenAI',
              kind: 'billing',
              status: 'error',
              last_sync_at: null,
              last_error: 'invalid api key',
              accounts: [{ external_id: 'org', name: '', currency: null, wallet_id: null }],
            }),
          ],
        }),
      ),
      http.get(apiUrl('/connection-providers'), () =>
        HttpResponse.json({ items: [{ key: 'fake', name: 'Bank demo', kind: 'bank' }] }),
      ),
    )
    renderApp('/h/h-dom/settings/connections')
    const bank = await screen.findByRole('article', { name: 'Konto w banku demo' })
    expect(within(bank).getByText('Aktywne')).toBeInTheDocument()
    expect(plain(bank.textContent)).toContain('Bank demo · Bank · ostatnia synchronizacja')
    expect(plain(within(bank).getByRole('list', { name: 'Konta: Konto w banku demo' }).textContent)).toContain('eKonto (PLN)→ Konto')
    const openai = screen.getByRole('article', { name: 'OpenAI' })
    expect(within(openai).getByText('Błąd')).toBeInTheDocument()
    expect(plain(openai.textContent)).toContain('Rachunki dostawcy · jeszcze nie synchronizowano')
    expect(plain(openai.textContent)).toContain('Ostatni błąd: invalid api key')
    expect(plain(openai.textContent)).toContain('nie przypisano do portfela')
    expect(within(screen.getByRole('list', { name: 'Dostawcy połączeń' })).getByText('Bank demo')).toBeInTheDocument()
  })

  it('is hidden from members', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.json(f.me([f.household({ role: 'member' })]))))
    const { router } = renderApp('/h/h-dom/settings/connections')
    await waitFor(() => expect(router.state.location.pathname).toBe('/h/h-dom/settings/general'))
    expect(screen.queryByRole('link', { name: 'Połączenia' })).not.toBeInTheDocument()
  })
})
