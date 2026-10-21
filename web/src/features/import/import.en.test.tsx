import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '../../i18n/locale.ts'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'
import { batchTitle, delimiterLabel, fileProblemMessage, rowFieldLabel } from './model.ts'

beforeEach(() => {
  server.use(http.get(apiUrl('/households/:hid/wallets'), () => HttpResponse.json({ items: [f.wallet()] })))
})

describe('import in English', () => {
  it('shows the wizard, the preview and the parser panel in English', async () => {
    setLocale('en')
    server.use(http.post(apiUrl('/households/:hid/imports/csv/preview'), () => HttpResponse.json(f.importPreview())))
    const { user } = renderApp('/h/h-dom/import')
    expect(await screen.findByRole('heading', { level: 1, name: 'Import transactions' })).toBeInTheDocument()
    expect(await screen.findByRole('list', { name: 'Import steps' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Target wallet' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'History' })).toBeInTheDocument()

    await user.upload(screen.getByLabelText('CSV file'), new File(['Data;Kwota\n2026-10-05;-1,00\n'], 'a.csv'))
    await user.click(screen.getByRole('button', { name: 'Next: preview' }))
    expect(await screen.findByRole('heading', { name: '2. Check the preview' })).toBeInTheDocument()
    expect(screen.getByLabelText('Preview counters').textContent).toContain('Duplicates')
    expect(screen.getByRole('button', { name: /^Import \d+ transactions?$/ })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Adjust' }))
    const panel = screen.getByRole('form', { name: 'Adjust how the file is read' })
    expect(within(panel).getByRole('combobox', { name: 'Field separator' })).toBeInTheDocument()
    expect(within(panel).getByRole('combobox', { name: 'Payee' })).toBeInTheDocument()
    expect(within(panel).getByRole('button', { name: 'Apply and refresh preview' })).toBeInTheDocument()
  })

  it('shows the empty history in English', async () => {
    setLocale('en')
    renderApp('/h/h-dom/import/history')
    expect(await screen.findByRole('heading', { name: 'Import history' })).toBeInTheDocument()
    expect(await screen.findByText('No imports yet')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Wallet' })).toBeInTheDocument()
  })

  it('translates the model helpers', () => {
    setLocale('en')
    expect(delimiterLabel(';')).toBe('semicolon (;)')
    expect(rowFieldLabel('row')).toBe('row')
    expect(rowFieldLabel('amount')).toBe('amount')
    expect(fileProblemMessage('empty')).toBe('The file is empty — choose a statement with transactions.')
    expect(batchTitle(f.importBatch({ source: 'csv', source_ref: '' }))).toBe('CSV file')
  })
})
