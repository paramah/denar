import type { CsvColumns, CsvImportOptions, CsvParseOptions, ImportBatch, ImportPreset } from '../../api/types.ts'

/** Limit pliku po stronie serwera (008 FR-812). */
export const maxFileBytes = 5 * 1024 * 1024
export const previewLimit = 50

export type ColumnField = keyof CsvColumns

/** Kolumny w kolejności formularza, z etykietą i podpowiedzią. */
export const columnFields: { field: ColumnField; label: string; hint: string }[] = [
  { field: 'date', label: 'Data', hint: 'Pierwsza istniejąca z listy.' },
  { field: 'amount', label: 'Kwota', hint: 'Kwota ze znakiem. Bez niej — obciążenie i uznanie.' },
  { field: 'debit', label: 'Obciążenie', hint: 'Wydatki w osobnej kolumnie (znak pomijany).' },
  { field: 'credit', label: 'Uznanie', hint: 'Wpływy w osobnej kolumnie.' },
  { field: 'payee', label: 'Kontrahent', hint: 'Pierwsza niepusta wartość w wierszu.' },
  { field: 'memo', label: 'Opis', hint: 'Wszystkie kolumny z listy, złączone.' },
  { field: 'external_id', label: 'Identyfikator operacji', hint: 'Bez niego duplikaty rozpoznaje skrót daty, kwoty i opisu.' },
  { field: 'currency', label: 'Waluta', hint: 'Inna niż waluta portfela — błąd wiersza.' },
  { field: 'fee', label: 'Opłata', hint: 'Odejmowana od kwoty.' },
  { field: 'state', label: 'Stan', hint: 'Z listą „Importowane stany” pomija np. operacje oczekujące.' },
]

export const delimiterOptions = [
  { value: ';', label: 'Średnik (;)' },
  { value: ',', label: 'Przecinek (,)' },
  { value: 'tab', label: 'Tabulator' },
  { value: '|', label: 'Kreska pionowa (|)' },
]

export const encodingOptions = [
  { value: 'utf-8', label: 'UTF-8' },
  { value: 'windows-1250', label: 'Windows-1250 (polskie banki)' },
]

export const decimalOptions = [
  { value: ',', label: 'Przecinek (12,34)' },
  { value: '.', label: 'Kropka (12.34)' },
]

/** Wykryty separator czytelnie („średnik”). */
export function delimiterLabel(value: string): string {
  switch (value) {
    case ';':
      return 'średnik (;)'
    case ',':
      return 'przecinek (,)'
    case '\t':
    case 'tab':
      return 'tabulator'
    case '|':
      return 'kreska pionowa (|)'
    default:
      return value || '—'
  }
}

export function encodingLabel(value: string): string {
  return encodingOptions.find((o) => o.value === value.toLowerCase())?.label ?? (value || '—')
}

/**
 * Ustawienia parsera z panelu „Dostosuj”. Puste pole (i brak kolumny
 * w `columns`) znaczy „jak w presecie” — w presecie `generic` to wykrywanie.
 */
export type ParserForm = {
  delimiter: string
  encoding: string
  dateFormat: string
  decimal: string
  skipRows: string
  hasHeader: boolean
  invertAmount: boolean
  acceptStates: string
  columns: Partial<Record<ColumnField, string[]>>
  /** Kolumny wpisane ręcznie (plik bez wykrytych nagłówków): nazwy lub numery po przecinku. */
  columnText: Partial<Record<ColumnField, string>>
}

export function emptyParserForm(preset?: ImportPreset): ParserForm {
  const o = preset?.options ?? {}
  return {
    delimiter: '',
    encoding: '',
    dateFormat: '',
    decimal: '',
    skipRows: String(o.skip_rows ?? 0),
    hasHeader: o.has_header ?? true,
    invertAmount: o.invert_amount ?? false,
    acceptStates: (o.accept_states ?? []).join(', '),
    columns: {},
    columnText: {},
  }
}

export function splitList(text: string): string[] {
  return text
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
}

/** Czy formularz zmienia coś względem presetu (pokazuje „Przywróć preset”). */
export function isCustomized(form: ParserForm, preset?: ImportPreset): boolean {
  return Object.keys(parserOverrides(form, preset)).length > 0
}

const sameList = (a: string[] = [], b: string[] = []) => a.length === b.length && a.every((v, i) => v === b[i])

/** Tylko to, czym formularz różni się od presetu (pole w żądaniu zastępuje preset w całości). */
export function parserOverrides(form: ParserForm, preset?: ImportPreset): CsvParseOptions {
  const o = preset?.options ?? {}
  const out: CsvParseOptions = {}
  if (form.delimiter) out.delimiter = form.delimiter
  if (form.encoding) out.encoding = form.encoding
  if (form.dateFormat.trim()) out.date_format = form.dateFormat.trim()
  if (form.decimal) out.decimal_separator = form.decimal
  const skip = Number.parseInt(form.skipRows, 10)
  if (Number.isFinite(skip) && skip !== (o.skip_rows ?? 0)) out.skip_rows = skip
  if (form.hasHeader !== (o.has_header ?? true)) out.has_header = form.hasHeader
  if (form.invertAmount !== (o.invert_amount ?? false)) out.invert_amount = form.invertAmount
  const states = splitList(form.acceptStates)
  if (!sameList(states, o.accept_states ?? [])) out.accept_states = states
  const columns: CsvColumns = {}
  for (const { field } of columnFields) {
    const text = form.columnText[field]?.trim()
    const value = text ? splitList(text) : form.columns[field]
    if (value !== undefined) columns[field] = value.filter(Boolean)
  }
  if (Object.keys(columns).length > 0) out.columns = columns
  return out
}

/** Opcje żądania podglądu/importu. */
export function importOptions(
  walletId: string,
  presetKey: string,
  form: ParserForm,
  preset?: ImportPreset,
  limit = previewLimit,
): CsvImportOptions {
  return { wallet_id: walletId, preset: presetKey, ...parserOverrides(form, preset), preview_limit: limit }
}

/** Kolumny presetu, które istnieją w pliku (porównanie jak w parserze: bez wielkości liter i wiodącego #). */
const normalizeHeader = (h: string) => h.trim().replace(/^#/, '').trim().toLowerCase()

export function presetColumnsInFile(preset: ImportPreset | undefined, field: ColumnField, headers: string[]): string[] {
  const alternatives = preset?.options.columns?.[field] ?? []
  const known = new Map(headers.map((h) => [normalizeHeader(h), h]))
  return alternatives.flatMap((alt) => {
    if (/^\d+$/.test(alt)) {
      const header = headers[Number(alt) - 1]
      return header !== undefined ? [header || `kolumna ${alt}`] : []
    }
    const header = known.get(normalizeHeader(alt))
    return header !== undefined ? [header] : []
  })
}

export type FileProblem = 'empty' | 'tooLarge'

export function checkFile(file: File): FileProblem | null {
  if (file.size === 0) return 'empty'
  if (file.size > maxFileBytes) return 'tooLarge'
  return null
}

export const fileProblemMessages: Record<FileProblem, string> = {
  empty: 'Plik jest pusty — wybierz wyciąg z operacjami.',
  tooLarge: 'Plik jest większy niż 5 MiB. Wyeksportuj krótszy okres i zaimportuj go w częściach.',
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toLocaleString('pl-PL', { maximumFractionDigits: 1 })} KiB`
  return `${(bytes / 1024 / 1024).toLocaleString('pl-PL', { maximumFractionDigits: 1 })} MiB`
}

/** Pole błędu wiersza po polsku. */
export function rowFieldLabel(field: string): string {
  const column = columnFields.find((c) => c.field === field)
  if (column) return column.label.toLowerCase()
  return field === 'row' ? 'wiersz' : field === 'file' ? 'plik' : field
}

/** Nazwa partii: plik CSV albo połączenie. */
export function batchTitle(batch: ImportBatch): string {
  if (batch.source === 'csv') return batch.source_ref || 'Plik CSV'
  return 'Synchronizacja połączenia'
}

/** Odmiana liczebnika: 1 transakcja, 2 transakcje, 5 transakcji. */
export function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) return one
  const tens = n % 100
  const units = n % 10
  return units >= 2 && units <= 4 && (tens < 12 || tens > 14) ? few : many
}

export const countLabel = (n: number, one: string, few: string, many: string) => `${n} ${plural(n, one, few, many)}`
