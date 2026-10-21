import { useState } from 'react'
import { useNavigate } from 'react-router'
import { fieldErrors } from '../../api/errors.ts'
import { useDeleteHousehold, useUpdateHousehold } from '../../api/hooks/households.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { Form, FormActions, FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { storageKeys, writeStored } from '../../lib/storage.ts'
import { HouseholdFields, type HouseholdValues } from '../households/HouseholdFields.tsx'
import styles from './settings.module.css'

const shownFields = ['name', 'timezone', 'month_start_day']

export function GeneralTab() {
  useDocumentTitle('Ustawienia gospodarstwa')
  const { household, role } = useHousehold()
  const editable = canManage(role)
  const update = useUpdateHousehold(household.id)
  const toast = useToast()
  const initial: HouseholdValues = {
    name: household.name,
    base_currency: household.base_currency,
    timezone: household.timezone,
    month_start_day: String(household.month_start_day),
  }
  const [values, setValues] = useState(initial)
  const [nameError, setNameError] = useState<string>()
  const server = fieldErrors(update.error)
  const dirty =
    values.name !== initial.name ||
    values.timezone !== initial.timezone ||
    values.month_start_day !== initial.month_start_day

  const submit = () => {
    if (!values.name.trim()) {
      setNameError('Podaj nazwę gospodarstwa.')
      return
    }
    setNameError(undefined)
    update.mutate(
      { name: values.name.trim(), timezone: values.timezone, month_start_day: Number(values.month_start_day) },
      { onSuccess: () => toast.show({ message: 'Zapisano ustawienia gospodarstwa.' }) },
    )
  }

  return (
    <>
      <section className={styles.section} aria-labelledby="general-heading">
        <div className={styles.sectionHead}>
          <div>
            <h2 id="general-heading">Dane gospodarstwa</h2>
            {!editable && <p>Zmieniać je mogą właściciel i administratorzy.</p>}
          </div>
        </div>
        <div className={styles.card}>
          <Form onSubmit={submit} label="Dane gospodarstwa">
            <FormAlert error={update.error} shownFields={shownFields} />
            <HouseholdFields
              values={values}
              onChange={setValues}
              errors={{ ...server, name: nameError ?? server.name }}
              currencyLocked
              readOnly={!editable}
            />
            {editable && (
              <FormActions>
                <Button type="submit" variant="primary" busy={update.isPending} disabled={!dirty}>
                  Zapisz zmiany
                </Button>
                {dirty && <Button onClick={() => setValues(initial)}>Cofnij zmiany</Button>}
              </FormActions>
            )}
          </Form>
        </div>
      </section>
      {role === 'owner' && <DeleteHousehold />}
    </>
  )
}

function DeleteHousehold() {
  const { household } = useHousehold()
  const remove = useDeleteHousehold(household.id)
  const navigate = useNavigate()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  return (
    <section className={styles.danger} aria-labelledby="delete-heading">
      <h2 id="delete-heading">Usuń gospodarstwo</h2>
      <p>Usunięcie kasuje członkostwa, zaproszenia, boty i dziennik zmian. Tego nie da się cofnąć.</p>
      <Button variant="danger" icon="trash" onClick={() => setOpen(true)}>
        Usuń gospodarstwo
      </Button>
      <ConfirmDialog
        open={open}
        title={`Usunąć gospodarstwo ${household.name}?`}
        confirmLabel="Usuń na zawsze"
        requirePhrase={household.name}
        busy={remove.isPending}
        error={<FormAlert error={remove.error} />}
        onCancel={() => setOpen(false)}
        onConfirm={() =>
          remove.mutate(undefined, {
            onSuccess: () => {
              writeStored(storageKeys.household, null)
              toast.show({ message: `Usunięto gospodarstwo ${household.name}.` })
              void navigate('/', { replace: true })
            },
          })
        }
      >
        <p>Wszyscy członkowie stracą dostęp, a tokeny botów przestaną działać.</p>
      </ConfirmDialog>
    </section>
  )
}
