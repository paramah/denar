import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useCreatePayee, useDeletePayee, usePayees, useUpdatePayee } from '../../api/hooks/payees.ts'
import type { Payee } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { TextField } from '../../components/Field/Field.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { envelopesEnabled, useEnvelopeNames, useEnvelopeOptions } from '../transactions/envelopes.ts'
import { EnvelopeField, TextAreaField } from '../transactions/fields.tsx'
import styles from './settings.module.css'

/** Wzorce wpisane po jednym w linii. */
export function parsePatterns(text: string): string[] {
  return [...new Set(text.split('\n').map((line) => line.trim()).filter(Boolean))]
}

export function PayeesTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('payees.title'))
  const { household, role } = useHousehold()
  const payees = usePayees(household.id)
  const canWrite = role !== 'viewer'
  const [editing, setEditing] = useState<Payee | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Payee | null>(null)
  const envelopeNames = useEnvelopeNames(household.id)

  return (
    <section className={styles.section} aria-labelledby="payees-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="payees-heading">{t('payees.title')}</h2>
          <p>{t('payees.lead')}</p>
        </div>
        {canWrite && (
          <Button variant="primary" icon="plus" onClick={() => setEditing('new')}>
            {t('payees.new')}
          </Button>
        )}
      </div>
      <QueryState query={payees} errorTitle={t('payees.loadFailed')}>
        {(items) => (
          <DataList
            label={t('payees.title')}
            items={items}
            getKey={(payee) => payee.id}
            empty={
              <EmptyState icon="users" title={t('payees.empty.title')} headingLevel={3}>
                <p>{t('payees.empty.body')}</p>
              </EmptyState>
            }
            renderItem={(payee) => (
              <article className={styles.row} aria-label={t('payees.rowLabel', { name: payee.name })}>
                <div className={styles.rowMain}>
                  <h3 className={styles.rowTitle}>{payee.name}</h3>
                  {payee.patterns.length > 0 ? (
                    <ul className={styles.chips} aria-label={t('payees.patterns')}>
                      {payee.patterns.map((pattern) => (
                        <li key={pattern}>
                          <Badge>{pattern}</Badge>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <span className={styles.rowMeta}>{t('payees.noPatterns')}</span>
                  )}
                  {envelopesEnabled() && payee.default_envelope_id && (
                    <span className={styles.rowMeta}>
                      {t('payees.defaultEnvelopeMeta', {
                        envelope: envelopeNames.get(payee.default_envelope_id) ?? t('payees.unknownEnvelope'),
                      })}
                    </span>
                  )}
                </div>
                {canWrite && (
                  <div className={styles.rowActions}>
                    <Button size="sm" icon="edit" onClick={() => setEditing(payee)} aria-label={t('payees.editLabel', { name: payee.name })}>
                      {t('payees.edit')}
                    </Button>
                    <Button size="sm" variant="danger" icon="trash" onClick={() => setDeleting(payee)} aria-label={t('payees.deleteLabel', { name: payee.name })}>
                      {t('payees.delete')}
                    </Button>
                  </div>
                )}
              </article>
            )}
          />
        )}
      </QueryState>
      <Dialog
        open={editing !== null}
        title={editing === 'new' ? t('payees.new') : t('payees.editTitle', { name: editing?.name ?? '' })}
        onClose={() => setEditing(null)}
      >
        <PayeeForm payee={editing === 'new' ? undefined : (editing ?? undefined)} onDone={() => setEditing(null)} />
      </Dialog>
      {deleting && <DeletePayeeDialog payee={deleting} onDone={() => setDeleting(null)} />}
    </section>
  )
}

function PayeeForm({ payee, onDone }: { payee?: Payee; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const create = useCreatePayee(household.id)
  const update = useUpdatePayee(household.id)
  const toast = useToast()
  const mutation = payee ? update : create
  const [name, setName] = useState(payee?.name ?? '')
  const [patterns, setPatterns] = useState(payee?.patterns.join('\n') ?? '')
  const [envelopeId, setEnvelopeId] = useState(payee?.default_envelope_id ?? '')
  const envelopeOptions = useEnvelopeOptions(household.id)
  const showEnvelope = envelopesEnabled()
  const [error, setError] = useState<string>()
  const server = fieldErrors(mutation.error)
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'

  const submit = async () => {
    if (!name.trim()) {
      setError(t('payees.form.nameRequired'))
      return
    }
    setError(undefined)
    const body = { name: name.trim(), patterns: parsePatterns(patterns) }
    try {
      if (payee) {
        const envelope =
          showEnvelope && envelopeId !== (payee.default_envelope_id ?? '') ? { default_envelope_id: envelopeId || null } : {}
        await update.mutateAsync({ payeeId: payee.id, body: { ...body, ...envelope } })
      } else {
        await create.mutateAsync({ ...body, ...(showEnvelope && envelopeId ? { default_envelope_id: envelopeId } : {}) })
      }
      toast.show({ message: t('payees.form.saved', { name: body.name }) })
      onDone()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <form
      noValidate
      aria-label={payee ? t('payees.form.editLabel') : t('payees.new')}
      className={styles.section}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert error={duplicate ? null : mutation.error} shownFields={['name', 'patterns', 'default_envelope_id']} />
      <TextField
        label={t('payees.form.name')}
        value={name}
        onChange={(event) => setName(event.target.value)}
        error={error ?? (duplicate ? t('payees.form.duplicate') : server.name)}
        maxLength={100}
        autoComplete="off"
        data-autofocus
      />
      <TextAreaField
        label={t('payees.patterns')}
        value={patterns}
        onChange={setPatterns}
        rows={4}
        hint={t('payees.form.patternsHint')}
        error={server.patterns}
      />
      {showEnvelope && (
        <div>
          <EnvelopeField
            label={t('payees.form.defaultEnvelope')}
            value={envelopeId}
            onChange={setEnvelopeId}
            options={envelopeOptions}
            error={server.default_envelope_id}
          />
          <p className={styles.rowMeta}>{t('payees.form.defaultEnvelopeHint')}</p>
        </div>
      )}
      <div className={styles.rowActions}>
        <Button onClick={onDone}>{t('payees.form.cancel')}</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {t('payees.form.save')}
        </Button>
      </div>
    </form>
  )
}

function DeletePayeeDialog({ payee, onDone }: { payee: Payee; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const remove = useDeletePayee(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={t('payees.deleteDialog.title', { name: payee.name })}
      confirmLabel={t('payees.deleteDialog.confirm')}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(payee.id, {
          onSuccess: () => {
            toast.show({ message: t('payees.deleteDialog.done', { name: payee.name }) })
            onDone()
          },
        })
      }
    >
      <p>{t('payees.deleteDialog.body')}</p>
    </ConfirmDialog>
  )
}
