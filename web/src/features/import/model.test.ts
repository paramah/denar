import { describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import {
  checkFile,
  countLabel,
  delimiterLabel,
  emptyParserForm,
  importOptions,
  isCustomized,
  maxFileBytes,
  parserOverrides,
  presetColumnsInFile,
  rowFieldLabel,
} from './model.ts'

const [generic, mbank] = f.importPresets()

describe('parserOverrides', () => {
  it('sends nothing for an untouched form', () => {
    expect(parserOverrides(emptyParserForm(mbank), mbank)).toEqual({})
    expect(isCustomized(emptyParserForm(generic), generic)).toBe(false)
  })

  it('sends only what differs from the preset', () => {
    const form = {
      ...emptyParserForm(mbank),
      delimiter: 'tab',
      encoding: 'utf-8',
      dateFormat: ' DD.MM.YYYY ',
      decimal: '.',
      skipRows: '3',
      hasHeader: false,
      invertAmount: true,
      acceptStates: 'COMPLETED, PENDING',
      columns: { payee: ['Opis', ''], external_id: [] },
      columnText: { date: '1, 2' },
    }
    expect(parserOverrides(form, mbank)).toEqual({
      delimiter: 'tab',
      encoding: 'utf-8',
      date_format: 'DD.MM.YYYY',
      decimal_separator: '.',
      skip_rows: 3,
      has_header: false,
      invert_amount: true,
      accept_states: ['COMPLETED', 'PENDING'],
      columns: { date: ['1', '2'], payee: ['Opis'], external_id: [] },
    })
    expect(isCustomized(form, mbank)).toBe(true)
  })

  it('builds the request options with the wallet, the preset and the preview limit', () => {
    expect(importOptions('w-1', 'mbank', emptyParserForm(mbank), mbank, 20)).toEqual({
      wallet_id: 'w-1',
      preset: 'mbank',
      preview_limit: 20,
    })
  })
})

describe('helpers', () => {
  it('finds the preset columns in the file like the parser (case, leading #, numbers)', () => {
    expect(presetColumnsInFile(mbank, 'date', ['#Data operacji', '#Opis operacji', '#Kwota'])).toEqual(['#Data operacji'])
    expect(presetColumnsInFile(mbank, 'memo', ['#Data operacji', '#Opis operacji'])).toEqual(['#Opis operacji'])
    expect(presetColumnsInFile(generic, 'amount', ['DATA', 'KWOTA'])).toEqual(['KWOTA'])
    expect(presetColumnsInFile(undefined, 'date', ['Data'])).toEqual([])
  })

  it('checks the file size before sending', () => {
    expect(checkFile(new File([], 'pusty.csv'))).toBe('empty')
    expect(checkFile(new File([new Uint8Array(maxFileBytes)], 'ok.csv'))).toBeNull()
    expect(checkFile(new File([new Uint8Array(maxFileBytes + 1)], 'duzy.csv'))).toBe('tooLarge')
  })

  it.each([
    [';', 'średnik (;)'],
    ['tab', 'tabulator'],
    ['\t', 'tabulator'],
    ['|', 'kreska pionowa (|)'],
  ])('names the delimiter %j', (value, label) => {
    expect(delimiterLabel(value)).toBe(label)
  })

  it('declines numbers and names row fields in Polish', () => {
    expect(countLabel(1, 'transakcja', 'transakcje', 'transakcji')).toBe('1 transakcja')
    expect(countLabel(3, 'transakcja', 'transakcje', 'transakcji')).toBe('3 transakcje')
    expect(countLabel(12, 'transakcja', 'transakcje', 'transakcji')).toBe('12 transakcji')
    expect(countLabel(22, 'transakcja', 'transakcje', 'transakcji')).toBe('22 transakcje')
    expect(rowFieldLabel('amount')).toBe('kwota')
    expect(rowFieldLabel('row')).toBe('wiersz')
  })
})
