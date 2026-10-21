import { useState } from 'react'
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
import { TextAreaField } from '../transactions/fields.tsx'
import styles from './settings.module.css'

/** Wzorce wpisane po jednym w linii. */
export function parsePatterns(text: string): string[] {
  return [...new Set(text.split('\n').map((line) => line.trim()).filter(Boolean))]
}

export function PayeesTab() {
  useDocumentTitle('Kontrahenci')
  const { household, role } = useHousehold()
  const payees = usePayees(household.id)
  const canWrite = role !== 'viewer'
  const [editing, setEditing] = useState<Payee | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Payee | null>(null)

  return (
    <section className={styles.section} aria-labelledby="payees-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="payees-heading">Kontrahenci</h2>
          <p>
            Sklepy, firmy i osoby, z którymi się rozliczasz. Nowy kontrahent powstaje sam, gdy wpiszesz go w transakcji.
            Wzorce rozpoznają go w opisach z banku (np. „BIEDRONKA” w „BIEDRONKA 1234 WARSZAWA”).
          </p>
        </div>
        {canWrite && (
          <Button variant="primary" icon="plus" onClick={() => setEditing('new')}>
            Nowy kontrahent
          </Button>
        )}
      </div>
      <QueryState query={payees} errorTitle="Nie udało się wczytać kontrahentów">
        {(items) => (
          <DataList
            label="Kontrahenci"
            items={items}
            getKey={(payee) => payee.id}
            empty={
              <EmptyState icon="users" title="Nie ma jeszcze kontrahentów" headingLevel={3}>
                <p>Pojawią się, gdy wpiszesz kontrahenta w transakcji, albo dodaj ich tutaj.</p>
              </EmptyState>
            }
            renderItem={(payee) => (
              <article className={styles.row} aria-label={`Kontrahent ${payee.name}`}>
                <div className={styles.rowMain}>
                  <h3 className={styles.rowTitle}>{payee.name}</h3>
                  {payee.patterns.length > 0 ? (
                    <ul className={styles.chips} aria-label="Wzorce">
                      {payee.patterns.map((pattern) => (
                        <li key={pattern}>
                          <Badge>{pattern}</Badge>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <span className={styles.rowMeta}>Bez wzorców — rozpoznawany po nazwie</span>
                  )}
                </div>
                {canWrite && (
                  <div className={styles.rowActions}>
                    <Button size="sm" icon="edit" onClick={() => setEditing(payee)} aria-label={`Edytuj ${payee.name}`}>
                      Edytuj
                    </Button>
                    <Button size="sm" variant="danger" icon="trash" onClick={() => setDeleting(payee)} aria-label={`Usuń ${payee.name}`}>
                      Usuń
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
        title={editing === 'new' ? 'Nowy kontrahent' : `Edycja: ${editing?.name ?? ''}`}
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
  const create = useCreatePayee(household.id)
  const update = useUpdatePayee(household.id)
  const toast = useToast()
  const mutation = payee ? update : create
  const [name, setName] = useState(payee?.name ?? '')
  const [patterns, setPatterns] = useState(payee?.patterns.join('\n') ?? '')
  const [error, setError] = useState<string>()
  const server = fieldErrors(mutation.error)
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'

  const submit = async () => {
    if (!name.trim()) {
      setError('Podaj nazwę.')
      return
    }
    setError(undefined)
    const body = { name: name.trim(), patterns: parsePatterns(patterns) }
    try {
      if (payee) {
        await update.mutateAsync({ payeeId: payee.id, body })
      } else {
        await create.mutateAsync(body)
      }
      toast.show({ message: `Zapisano kontrahenta ${body.name}.` })
      onDone()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <form
      noValidate
      aria-label={payee ? 'Edycja kontrahenta' : 'Nowy kontrahent'}
      className={styles.section}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert error={duplicate ? null : mutation.error} shownFields={['name', 'patterns']} />
      <TextField
        label="Nazwa"
        value={name}
        onChange={(event) => setName(event.target.value)}
        error={error ?? (duplicate ? 'Kontrahent o tej nazwie już istnieje.' : server.name)}
        maxLength={100}
        autoComplete="off"
        data-autofocus
      />
      <TextAreaField
        label="Wzorce"
        value={patterns}
        onChange={setPatterns}
        rows={4}
        hint="Jeden w linii. Fragment tekstu bez względu na wielkość liter, np. ORLEN."
        error={server.patterns}
      />
      <div className={styles.rowActions}>
        <Button onClick={onDone}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          Zapisz
        </Button>
      </div>
    </form>
  )
}

function DeletePayeeDialog({ payee, onDone }: { payee: Payee; onDone: () => void }) {
  const { household } = useHousehold()
  const remove = useDeletePayee(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={`Usunąć kontrahenta ${payee.name}?`}
      confirmLabel="Usuń kontrahenta"
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(payee.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto kontrahenta ${payee.name}.` })
            onDone()
          },
        })
      }
    >
      <p>Transakcje zostaną, ale bez kontrahenta.</p>
    </ConfirmDialog>
  )
}
