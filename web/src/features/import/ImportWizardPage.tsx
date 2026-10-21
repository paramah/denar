import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useImportCsv, useImportPresets, usePreviewCsvImport } from '../../api/hooks/imports.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { ImportBatch, ImportPreset, ImportPreview, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { ParserPanel } from './ParserPanel.tsx'
import { PreviewView } from './PreviewView.tsx'
import { BatchCounters } from './parts.tsx'
import {
  checkFile,
  emptyParserForm,
  fileProblemMessage,
  formatFileSize,
  importOptions,
  type ParserForm,
} from './model.ts'
import styles from './import.module.css'

type Step = 'file' | 'preview' | 'done'

const steps = [
  { key: 'file', label: 'stepFile' },
  { key: 'preview', label: 'stepPreview' },
  { key: 'done', label: 'stepDone' },
] as const satisfies readonly { key: Step; label: string }[]

/** Kreator importu CSV: plik → podgląd z dostosowaniem → podsumowanie. */
export function ImportWizardPage() {
  const { t } = useTranslation('import')
  useDocumentTitle(t('wizard.documentTitle'))
  const { household, role } = useHousehold()
  const wallets = useWallets(household.id)
  const presets = useImportPresets()

  if (role === 'viewer') {
    return (
      <EmptyState icon="lock" title={t('wizard.viewerTitle')}>
        <p>{t('wizard.viewerBody')}</p>
      </EmptyState>
    )
  }
  if (wallets.isPending || presets.isPending) {
    return <Skeleton lines={5} label={t('wizard.loading')} />
  }
  if (wallets.isError || presets.isError) {
    const failed = wallets.isError ? wallets : presets
    return (
      <SectionError
        title={t('wizard.loadFailed')}
        error={failed.error}
        onRetry={() => void failed.refetch()}
      />
    )
  }
  const open = wallets.data.filter((w) => !w.archived_at)
  if (open.length === 0) {
    return (
      <EmptyState
        icon="wallet"
        title={t('wizard.noWalletTitle')}
        action={
          <LinkButton to={householdPath(household.id, 'wallets')} variant="primary">
            {t('wizard.noWalletAction')}
          </LinkButton>
        }
      >
        <p>{t('wizard.noWalletBody')}</p>
      </EmptyState>
    )
  }
  return <Wizard wallets={open} presets={presets.data} />
}

function Wizard({ wallets, presets }: { wallets: Wallet[]; presets: ImportPreset[] }) {
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  const [search] = useSearchParams()
  const requested = search.get('wallet') ?? ''
  const [step, setStep] = useState<Step>('file')
  const [walletId, setWalletId] = useState(() =>
    wallets.some((w) => w.id === requested) ? requested : (wallets[0]?.id ?? ''),
  )
  const [file, setFile] = useState<File | null>(null)
  const [fileProblem, setFileProblem] = useState<string | null>(null)
  const [presetKey, setPresetKey] = useState(presets[0]?.key ?? 'generic')
  const preset = presets.find((p) => p.key === presetKey)
  const [draft, setDraft] = useState<ParserForm>(() => emptyParserForm(preset))
  const [adjusting, setAdjusting] = useState(false)
  const [shown, setShown] = useState<{ preview: ImportPreview; form: ParserForm } | null>(null)
  const [batch, setBatch] = useState<ImportBatch | null>(null)
  const preview = usePreviewCsvImport(household.id)
  const run = useImportCsv(household.id)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const firstRender = useRef(true)

  // Nowy krok: focus na jego nagłówek (czytnik ogłasza, gdzie jesteśmy).
  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false
      return
    }
    headingRef.current?.focus()
  }, [step])

  const wallet = wallets.find((w) => w.id === walletId)
  const errors = fieldErrors(preview.error)
  const optionErrors = Object.fromEntries(
    Object.entries(errors)
      .filter(([field]) => field.startsWith('options.') && field !== 'options.wallet_id')
      .map(([field, message]) => [field.slice('options.'.length), message]),
  )
  const hasOptionErrors = Object.keys(optionErrors).length > 0

  const choosePreset = (key: string) => {
    setPresetKey(key)
    setDraft(emptyParserForm(presets.find((p) => p.key === key)))
    setShown(null)
  }

  const runPreview = (form: ParserForm) => {
    if (!file) return
    preview.mutate(
      { file, options: importOptions(walletId, presetKey, form, preset) },
      {
        onSuccess: (data) => {
          setShown({ preview: data, form })
          setStep('preview')
        },
        onError: (err) => {
          // Złe opcje parsera (np. brak kolumny daty) poprawia się w „Dostosuj”.
          const fields = Object.keys(fieldErrors(err))
          if (err instanceof ApiError && err.status === 422 && fields.some((f) => f.startsWith('options.'))) {
            setStep('preview')
            setAdjusting(true)
          }
        },
      },
    )
  }

  const runImport = () => {
    if (!file || !shown) return
    run.mutate(
      { file, options: importOptions(walletId, presetKey, shown.form, preset) },
      {
        onSuccess: (data) => {
          setBatch(data)
          setStep('done')
        },
      },
    )
  }

  const restart = () => {
    setStep('file')
    setFile(null)
    setShown(null)
    setBatch(null)
    setAdjusting(false)
    preview.reset()
    run.reset()
  }

  const current = steps.findIndex((s) => s.key === step)
  return (
    <div className={styles.wizard}>
      <ol className={styles.steps} aria-label={t('wizard.steps')}>
        {steps.map((s, index) => (
          <li
            key={s.key}
            className={index === current ? styles.stepCurrent : index < current ? styles.stepDone : styles.step}
            aria-current={index === current ? 'step' : undefined}
          >
            <span className={styles.stepNumber} aria-hidden="true">
              {index < current ? <Icon name="check" size={14} /> : index + 1}
            </span>
            {t(`wizard.${s.label}`)}
            {index < current && <span className="visually-hidden">{t('wizard.stepCompleted')}</span>}
          </li>
        ))}
      </ol>

      {step === 'file' && (
        <section className={styles.panel} aria-labelledby="import-step">
          <h2 id="import-step" ref={headingRef} tabIndex={-1} className={styles.stepTitle}>
            {t('wizard.fileHeading')}
          </h2>
          <form
            className={styles.stepForm}
            noValidate
            onSubmit={(event) => {
              event.preventDefault()
              if (!file) {
                setFileProblem(t('wizard.noFile'))
                return
              }
              runPreview(draft)
            }}
          >
            <SelectField
              label={t('wizard.wallet')}
              value={walletId}
              error={errors['options.wallet_id']}
              hint={t('wizard.walletHint')}
              options={wallets.map((w) => ({ value: w.id, label: t('wizard.walletOption', { name: w.name, currency: w.currency }) }))}
              onChange={(e) => {
                setWalletId(e.target.value)
                setShown(null)
              }}
            />
            <FileDrop
              file={file}
              problem={fileProblem ?? errors.file ?? null}
              onFile={(next) => {
                const problem = next ? checkFile(next) : null
                setFileProblem(problem ? fileProblemMessage(problem) : null)
                setFile(problem ? null : next)
                setShown(null)
                preview.reset()
              }}
            />
            <PresetPicker presets={presets} value={presetKey} onChange={choosePreset} />
            <FormAlert error={hasOptionErrors ? null : preview.error} shownFields={['file', 'options.wallet_id']} />
            <div className={styles.stepActions}>
              <Button type="submit" variant="primary" icon="eye" busy={preview.isPending}>
                {t('wizard.next')}
              </Button>
            </div>
          </form>
        </section>
      )}

      {step === 'preview' && (
        <section className={styles.panel} aria-labelledby="import-step">
          <div className={styles.stepHead}>
            <h2 id="import-step" ref={headingRef} tabIndex={-1} className={styles.stepTitle}>
              {t('wizard.previewHeading')}
            </h2>
            <p className={styles.muted}>
              {t('wizard.previewMeta', { file: file?.name ?? '', wallet: wallet?.name ?? '', preset: preset?.name ?? presetKey })}
            </p>
          </div>
          {preview.error && !hasOptionErrors && <FormAlert error={preview.error} />}
          {hasOptionErrors && (
            <Callout tone="warn" title={t('wizard.optionErrorsTitle')}>
              {t('wizard.optionErrorsBody')}
            </Callout>
          )}
          {shown && !hasOptionErrors && <PreviewView preview={shown.preview} currency={wallet?.currency ?? ''} />}

          <div className={styles.adjust}>
            <Button
              icon="settings"
              aria-expanded={adjusting}
              aria-controls="import-adjust"
              onClick={() => setAdjusting((v) => !v)}
            >
              {t('wizard.adjust')}
            </Button>
            <div id="import-adjust" hidden={!adjusting}>
              {adjusting && (
                <ParserPanel
                  preset={preset}
                  headers={shown?.preview.detected.headers ?? []}
                  form={draft}
                  onChange={setDraft}
                  errors={optionErrors}
                  busy={preview.isPending}
                  onApply={runPreview}
                />
              )}
            </div>
          </div>

          <FormAlert error={run.error} />
          <div className={styles.stepActions}>
            <Button icon="undo" onClick={() => setStep('file')}>
              {t('wizard.back')}
            </Button>
            <ImportButton preview={hasOptionErrors ? null : (shown?.preview ?? null)} busy={run.isPending} onImport={runImport} />
          </div>
        </section>
      )}

      {step === 'done' && batch && (
        <section className={styles.panel} aria-labelledby="import-step">
          <h2 id="import-step" ref={headingRef} tabIndex={-1} className={styles.stepTitle}>
            {t('wizard.doneHeading')}
          </h2>
          <Summary batch={batch} wallet={wallet} onRestart={restart} />
        </section>
      )}
    </div>
  )
}

function ImportButton({ preview, busy, onImport }: { preview: ImportPreview | null; busy: boolean; onImport: () => void }) {
  const { t } = useTranslation('import')
  if (!preview) {
    return null
  }
  if (preview.new_count === 0) {
    return (
      <p className={styles.muted} role="status">
        {preview.duplicate_count > 0 ? t('wizard.nothingNewDuplicates') : t('wizard.nothingNew')}
      </p>
    )
  }
  return (
    <Button variant="primary" icon="upload" busy={busy} onClick={onImport}>
      {t('wizard.importCount', { count: preview.new_count })}
    </Button>
  )
}

function FileDrop({
  file,
  problem,
  onFile,
}: {
  file: File | null
  problem: string | null
  onFile: (file: File | null) => void
}) {
  const { t } = useTranslation('import')
  const id = useId()
  const [over, setOver] = useState(false)
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  return (
    <div className={styles.fileField}>
      <span className={styles.columnLabel} id={`${id}-label`}>
        {t('wizard.fileLabel')}
      </span>
      <div
        className={[styles.drop, over ? styles.dropOver : '', problem ? styles.dropInvalid : ''].filter(Boolean).join(' ')}
        onDragOver={(event) => {
          event.preventDefault()
          setOver(true)
        }}
        onDragLeave={() => setOver(false)}
        onDrop={(event) => {
          event.preventDefault()
          setOver(false)
          onFile(event.dataTransfer.files[0] ?? null)
        }}
      >
        <Icon name="upload" size={28} />
        {file ? (
          <p className={styles.fileName}>
            <strong>{file.name}</strong> · {formatFileSize(file.size)}
          </p>
        ) : (
          <p>{t('wizard.drop')}</p>
        )}
        <input
          id={id}
          type="file"
          accept=".csv,.txt,text/csv,text/plain"
          className={`visually-hidden ${styles.fileInput}`}
          aria-labelledby={`${id}-label`}
          aria-describedby={[problem ? errorId : '', hintId].filter(Boolean).join(' ')}
          aria-invalid={problem ? true : undefined}
          onChange={(event) => {
            onFile(event.target.files?.[0] ?? null)
            event.target.value = ''
          }}
        />
        {file ? (
          <label htmlFor={id} className={styles.fileButton}>
            {t('wizard.chooseOther')}
          </label>
        ) : (
          <label htmlFor={id} className={styles.fileButton}>
            {t('wizard.choose')}
          </label>
        )}
      </div>
      <p id={hintId} className={styles.hint}>
        {t('wizard.fileHint')}
      </p>
      {problem && (
        <p id={errorId} className={styles.fieldError}>
          {problem}
        </p>
      )}
    </div>
  )
}

function PresetPicker({
  presets,
  value,
  onChange,
}: {
  presets: ImportPreset[]
  value: string
  onChange: (key: string) => void
}) {
  const { t } = useTranslation('import')
  return (
    <fieldset className={styles.presets}>
      <legend className={styles.columnLabel}>{t('wizard.presetLegend')}</legend>
      <div className={styles.presetGrid}>
        {presets.map((preset) => (
          <label key={preset.key} className={styles.preset} data-checked={preset.key === value || undefined}>
            <input
              type="radio"
              name="preset"
              value={preset.key}
              checked={preset.key === value}
              onChange={() => onChange(preset.key)}
            />
            <span className={styles.presetName}>{preset.name}</span>
            <span className={styles.presetDescription}>{preset.description}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}

function Summary({ batch, wallet, onRestart }: { batch: ImportBatch; wallet?: Wallet; onRestart: () => void }) {
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  const lead = useMemo(() => {
    if (batch.created_count === 0) {
      return t('wizard.noneAdded')
    }
    return t('wizard.added', { count: batch.created_count, wallet: wallet?.name ?? '' })
  }, [batch, wallet, t])
  return (
    <div className={styles.summary}>
      <p role="status" className={styles.summaryLead}>
        {lead}
      </p>
      <BatchCounters batch={batch} />
      {batch.matched_count > 0 && (
        <Callout title={t('wizard.matchedTitle')}>{t('wizard.matched', { count: batch.matched_count })}</Callout>
      )}
      {batch.error_count > 0 && (
        <Callout tone="warn" title={t('wizard.skippedTitle')}>
          {t('wizard.skippedBody')}
        </Callout>
      )}
      <div className={styles.summaryLinks}>
        {wallet && (
          <LinkButton variant="primary" icon="list" to={householdPath(household.id, `wallets/${wallet.id}`)}>
            {t('wizard.viewRegister')}
          </LinkButton>
        )}
        <LinkButton icon="repeat" to={householdPath(household.id, 'subscriptions/charges')}>
          {t('wizard.chargesQueue')}
        </LinkButton>
        <LinkButton icon="history" to={householdPath(household.id, `import/${batch.id}`)}>
          {t('wizard.details')}
        </LinkButton>
        <Button icon="upload" onClick={onRestart}>
          {t('wizard.another')}
        </Button>
      </div>
    </div>
  )
}
