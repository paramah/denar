import { useId } from 'react'
import type { ImportPreset } from '../../api/types.ts'
import { Button, IconButton } from '../../components/Button/Button.tsx'
import { Checkbox, SelectField, TextField, selectClass, inputClass } from '../../components/Field/Field.tsx'
import {
  columnFields,
  decimalOptions,
  delimiterLabel,
  delimiterOptions,
  emptyParserForm,
  encodingLabel,
  encodingOptions,
  isCustomized,
  presetColumnsInFile,
  type ColumnField,
  type ParserForm,
} from './model.ts'
import styles from './import.module.css'

type Props = {
  preset?: ImportPreset
  /** Nagłówki wykryte w pliku (puste: plik bez nagłówka albo nieudany podgląd). */
  headers: string[]
  form: ParserForm
  onChange: (form: ParserForm) => void
  /** Błędy 422 pól `options.*` (bez prefiksu). */
  errors: Record<string, string>
  busy: boolean
  onApply: (form: ParserForm) => void
}

const presetValue = (value: string | undefined, describe: (v: string) => string = (v) => v) =>
  value ? `Jak w presecie: ${describe(value)}` : 'Wykryj automatycznie'

/** Panel „Dostosuj”: opcje parsera i mapowanie kolumn; zatwierdzenie ponawia podgląd. */
export function ParserPanel({ preset, headers, form, onChange, errors, busy, onApply }: Props) {
  const o = preset?.options ?? {}
  const set = <K extends keyof ParserForm>(key: K, value: ParserForm[K]) => onChange({ ...form, [key]: value })
  const setColumn = (field: ColumnField, value: string[] | undefined) => {
    const columns = { ...form.columns }
    if (value === undefined) delete columns[field]
    else columns[field] = value
    onChange({ ...form, columns })
  }

  return (
    <form
      className={styles.parser}
      aria-label="Dostosuj odczyt pliku"
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        onApply(form)
      }}
    >
      <fieldset className={styles.fieldset}>
        <legend>Format pliku</legend>
        <div className={styles.grid}>
          <SelectField
            label="Separator pól"
            value={form.delimiter}
            error={errors.delimiter}
            options={[{ value: '', label: presetValue(o.delimiter, delimiterLabel) }, ...delimiterOptions]}
            onChange={(e) => set('delimiter', e.target.value)}
          />
          <SelectField
            label="Kodowanie znaków"
            value={form.encoding}
            error={errors.encoding}
            hint="Krzaki zamiast „ąę” — zmień kodowanie."
            options={[{ value: '', label: presetValue(o.encoding, encodingLabel) }, ...encodingOptions]}
            onChange={(e) => set('encoding', e.target.value)}
          />
          <TextField
            label="Format daty"
            value={form.dateFormat}
            error={errors.date_format}
            placeholder={o.date_format || 'wykrywany'}
            hint="Np. DD.MM.YYYY albo YYYY-MM-DD. Puste — jak w presecie."
            onChange={(e) => set('dateFormat', e.target.value)}
          />
          <SelectField
            label="Separator dziesiętny"
            value={form.decimal}
            error={errors.decimal_separator}
            options={[{ value: '', label: presetValue(o.decimal_separator) }, ...decimalOptions]}
            onChange={(e) => set('decimal', e.target.value)}
          />
          <TextField
            label="Pomiń wiersze na początku"
            type="number"
            inputMode="numeric"
            min={0}
            max={1000}
            value={form.skipRows}
            error={errors.skip_rows}
            hint="Nagłówek jest szukany dopiero po nich."
            onChange={(e) => set('skipRows', e.target.value)}
          />
          <TextField
            label="Importowane stany"
            value={form.acceptStates}
            error={errors.accept_states}
            placeholder="np. COMPLETED"
            hint="Z kolumną „Stan”: tylko te wartości, po przecinku. Puste — wszystkie."
            onChange={(e) => set('acceptStates', e.target.value)}
          />
        </div>
        <div className={styles.checks}>
          <Checkbox
            label="Plik ma wiersz nagłówka"
            hint="Bez nagłówka kolumny wskazuje się numerami (1, 2, …)."
            checked={form.hasHeader}
            onChange={(e) => set('hasHeader', e.target.checked)}
          />
          <Checkbox
            label="Odwróć znak kwot"
            hint="Gdy wydatki w pliku są dodatnie (np. wyciąg karty kredytowej)."
            checked={form.invertAmount}
            onChange={(e) => set('invertAmount', e.target.checked)}
          />
        </div>
      </fieldset>

      <fieldset className={styles.fieldset}>
        <legend>Kolumny</legend>
        <p className={styles.muted}>
          Każda pozycja to lista alternatyw — pierwsza, która jest w pliku, wygrywa (kontrahent: pierwsza niepusta; opis:
          wszystkie złączone).
        </p>
        {errors.columns && (
          <p className={styles.fieldError} role="alert">
            {errors.columns}
          </p>
        )}
        <div className={styles.columns}>
          {columnFields.map(({ field, label, hint }) => (
            <ColumnPicker
              key={field}
              label={label}
              hint={hint}
              headers={form.hasHeader ? headers : []}
              matched={presetColumnsInFile(preset, field, headers)}
              presetAlternatives={o.columns?.[field] ?? []}
              value={form.columns[field]}
              text={form.columnText[field] ?? ''}
              error={errors[`columns.${field}`]}
              onChange={(value) => setColumn(field, value)}
              onText={(text) => onChange({ ...form, columnText: { ...form.columnText, [field]: text } })}
            />
          ))}
        </div>
      </fieldset>

      <div className={styles.parserActions}>
        <Button type="submit" variant="primary" icon="refresh" busy={busy}>
          Zastosuj i odśwież podgląd
        </Button>
        {isCustomized(form, preset) && (
          <Button onClick={() => onChange(emptyParserForm(preset))}>Przywróć ustawienia presetu</Button>
        )}
      </div>
    </form>
  )
}

const presetOption = '__preset__'
const noneOption = '__none__'

type PickerProps = {
  label: string
  hint: string
  headers: string[]
  /** Kolumny presetu znalezione w pliku. */
  matched: string[]
  presetAlternatives: string[]
  /** undefined — jak w presecie; [] — nie używaj. */
  value: string[] | undefined
  /** Wpis ręczny (bez nagłówków). */
  text: string
  error?: string
  onChange: (value: string[] | undefined) => void
  onText: (text: string) => void
}

/** Mapowanie jednej kolumny: wartość presetu, „nie używaj” albo lista alternatyw z nagłówków. */
function ColumnPicker({ label, hint, headers, matched, presetAlternatives, value, text, error, onChange, onText }: PickerProps) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const described = [error ? errorId : '', hintId].filter(Boolean).join(' ')
  const presetText = matched.length
    ? `Jak w presecie: ${matched.join(' / ')}`
    : presetAlternatives.length
      ? 'Jak w presecie (nie ma w pliku)'
      : 'Jak w presecie (brak)'

  const inner = headers.length === 0 ? (
    // Bez wykrytych nagłówków: nazwy lub numery kolumn wpisane ręcznie.
    <input
      id={id}
      className={inputClass}
      aria-describedby={described}
      aria-invalid={error ? true : undefined}
      value={text}
      placeholder={presetAlternatives.join(', ') || 'np. 1 albo Data'}
      onChange={(e) => onText(e.target.value)}
    />
  ) : (
    <div className={styles.alternatives}>
      <select
        id={id}
        className={selectClass}
        aria-describedby={described}
        aria-invalid={error ? true : undefined}
        value={value === undefined ? presetOption : value.length === 0 ? noneOption : value[0]}
        onChange={(e) => {
          const v = e.target.value
          onChange(v === presetOption ? undefined : v === noneOption ? [] : [v, ...(value ?? []).slice(1)])
        }}
      >
        <option value={presetOption}>{presetText}</option>
        <option value={noneOption}>Nie używaj</option>
        {headers.map((h, i) => (
          <option key={`${i}:${h}`} value={h || String(i + 1)}>
            {h || `Kolumna ${i + 1} (bez nazwy)`}
          </option>
        ))}
      </select>
      {(value ?? []).slice(1).map((alt, index) => (
        <div key={index} className={styles.alternative}>
          <select
            className={selectClass}
            aria-label={`${label} — alternatywa ${index + 2}`}
            value={alt}
            onChange={(e) => {
              const next = [...(value ?? [])]
              next[index + 1] = e.target.value
              onChange(next)
            }}
          >
            {headers.map((h, i) => (
              <option key={`${i}:${h}`} value={h || String(i + 1)}>
                {h || `Kolumna ${i + 1} (bez nazwy)`}
              </option>
            ))}
          </select>
          <IconButton
            icon="close"
            label={`Usuń alternatywę ${index + 2}: ${label}`}
            onClick={() => onChange((value ?? []).filter((_, i) => i !== index + 1))}
          />
        </div>
      ))}
      {value && value.length > 0 && (
        <Button
          size="sm"
          variant="ghost"
          icon="plus"
          aria-label={`Dodaj alternatywę: ${label}`}
          onClick={() => {
            const unused = headers.find((h) => h && !value.includes(h)) ?? headers[0] ?? ''
            onChange([...value, unused])
          }}
        >
          Alternatywa
        </Button>
      )}
    </div>
  )

  return (
    <div className={styles.column}>
      <label htmlFor={id} className={styles.columnLabel}>
        {label}
      </label>
      {inner}
      <p id={hintId} className={styles.hint}>
        {headers.length === 0 ? `${hint} Nazwy lub numery kolumn, po przecinku; puste — jak w presecie.` : hint}
      </p>
      {error && (
        <p id={errorId} className={styles.fieldError}>
          {error}
        </p>
      )}
    </div>
  )
}
