import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { ImportPreset } from '../../api/types.ts'
import { tk } from '../../i18n/translate.ts'
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
  value ? tk('import:parser.presetValue', { value: describe(value) }) : tk('import:parser.detect')

/** Panel „Dostosuj”: opcje parsera i mapowanie kolumn; zatwierdzenie ponawia podgląd. */
export function ParserPanel({ preset, headers, form, onChange, errors, busy, onApply }: Props) {
  const { t } = useTranslation('import')
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
      aria-label={t('parser.formLabel')}
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        onApply(form)
      }}
    >
      <fieldset className={styles.fieldset}>
        <legend>{t('parser.formatLegend')}</legend>
        <div className={styles.grid}>
          <SelectField
            label={t('parser.delimiter')}
            value={form.delimiter}
            error={errors.delimiter}
            options={[{ value: '', label: presetValue(o.delimiter, delimiterLabel) }, ...delimiterOptions]}
            onChange={(e) => set('delimiter', e.target.value)}
          />
          <SelectField
            label={t('parser.encoding')}
            value={form.encoding}
            error={errors.encoding}
            hint={t('parser.encodingHint')}
            options={[{ value: '', label: presetValue(o.encoding, encodingLabel) }, ...encodingOptions]}
            onChange={(e) => set('encoding', e.target.value)}
          />
          <TextField
            label={t('parser.dateFormat')}
            value={form.dateFormat}
            error={errors.date_format}
            placeholder={o.date_format || t('parser.dateFormatPlaceholder')}
            hint={t('parser.dateFormatHint')}
            onChange={(e) => set('dateFormat', e.target.value)}
          />
          <SelectField
            label={t('parser.decimal')}
            value={form.decimal}
            error={errors.decimal_separator}
            options={[
              { value: '', label: presetValue(o.decimal_separator, (v) => (v === ',' ? t('parser.decimalComma') : t('parser.decimalDot'))) },
              ...decimalOptions,
            ]}
            onChange={(e) => set('decimal', e.target.value)}
          />
          <TextField
            label={t('parser.skipRows')}
            type="number"
            inputMode="numeric"
            min={0}
            max={1000}
            value={form.skipRows}
            error={errors.skip_rows}
            hint={t('parser.skipRowsHint')}
            onChange={(e) => set('skipRows', e.target.value)}
          />
          <TextField
            label={t('parser.acceptStates')}
            value={form.acceptStates}
            error={errors.accept_states}
            placeholder={t('parser.acceptStatesPlaceholder')}
            hint={t('parser.acceptStatesHint')}
            onChange={(e) => set('acceptStates', e.target.value)}
          />
        </div>
        <div className={styles.checks}>
          <Checkbox
            label={t('parser.hasHeader')}
            hint={t('parser.hasHeaderHint')}
            checked={form.hasHeader}
            onChange={(e) => set('hasHeader', e.target.checked)}
          />
          <Checkbox
            label={t('parser.invert')}
            hint={t('parser.invertHint')}
            checked={form.invertAmount}
            onChange={(e) => set('invertAmount', e.target.checked)}
          />
        </div>
      </fieldset>

      <fieldset className={styles.fieldset}>
        <legend>{t('parser.columnsLegend')}</legend>
        <p className={styles.muted}>{t('parser.columnsIntro')}</p>
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
          {t('parser.apply')}
        </Button>
        {isCustomized(form, preset) && (
          <Button onClick={() => onChange(emptyParserForm(preset))}>{t('parser.reset')}</Button>
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
  const { t } = useTranslation('import')
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const described = [error ? errorId : '', hintId].filter(Boolean).join(' ')
  const presetText = matched.length
    ? t('parser.presetMatched', { columns: matched.join(' / ') })
    : presetAlternatives.length
      ? t('parser.presetMissing')
      : t('parser.presetNone')

  const inner = headers.length === 0 ? (
    // Bez wykrytych nagłówków: nazwy lub numery kolumn wpisane ręcznie.
    <input
      id={id}
      className={inputClass}
      aria-describedby={described}
      aria-invalid={error ? true : undefined}
      value={text}
      placeholder={presetAlternatives.join(', ') || t('parser.manualPlaceholder')}
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
        <option value={noneOption}>{t('parser.none')}</option>
        {headers.map((h, i) => (
          <option key={`${i}:${h}`} value={h || String(i + 1)}>
            {h || t('parser.unnamed', { n: i + 1 })}
          </option>
        ))}
      </select>
      {(value ?? []).slice(1).map((alt, index) => (
        <div key={index} className={styles.alternative}>
          <select
            className={selectClass}
            aria-label={t('parser.alternativeLabel', { label, n: index + 2 })}
            value={alt}
            onChange={(e) => {
              const next = [...(value ?? [])]
              next[index + 1] = e.target.value
              onChange(next)
            }}
          >
            {headers.map((h, i) => (
              <option key={`${i}:${h}`} value={h || String(i + 1)}>
                {h || t('parser.unnamed', { n: i + 1 })}
              </option>
            ))}
          </select>
          <IconButton
            icon="close"
            label={t('parser.removeAlternative', { label, n: index + 2 })}
            onClick={() => onChange((value ?? []).filter((_, i) => i !== index + 1))}
          />
        </div>
      ))}
      {value && value.length > 0 && (
        <Button
          size="sm"
          variant="ghost"
          icon="plus"
          aria-label={t('parser.addAlternative', { label })}
          onClick={() => {
            const unused = headers.find((h) => h && !value.includes(h)) ?? headers[0] ?? ''
            onChange([...value, unused])
          }}
        >
          {t('parser.alternative')}
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
        {headers.length === 0 ? t('parser.manualHint', { hint }) : hint}
      </p>
      {error && (
        <p id={errorId} className={styles.fieldError}>
          {error}
        </p>
      )}
    </div>
  )
}
