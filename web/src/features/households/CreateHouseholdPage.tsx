import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { fieldErrors } from '../../api/errors.ts'
import { useCreateHousehold } from '../../api/hooks/households.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import { PlainLayout } from '../../app/AppShell.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { Form, FormActions, FormAlert } from '../../components/Form/Form.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { browserTimeZone } from '../../lib/format.ts'
import { HouseholdFields, type HouseholdValues } from './HouseholdFields.tsx'
import styles from './households.module.css'

const shownFields = ['name', 'base_currency', 'timezone', 'month_start_day']

export function CreateHouseholdPage() {
  const { t } = useTranslation('households')
  const create = useCreateHousehold()
  const navigate = useNavigate()
  const toast = useToast()
  const { households } = useSessionUser()
  const [values, setValues] = useState<HouseholdValues>(() => ({
    name: '',
    base_currency: 'PLN',
    timezone: browserTimeZone(),
    month_start_day: '1',
  }))
  const [localError, setLocalError] = useState<string | undefined>()
  const server = fieldErrors(create.error)

  const submit = () => {
    if (!values.name.trim()) {
      setLocalError(t('create.nameRequired'))
      return
    }
    setLocalError(undefined)
    create.mutate(
      {
        name: values.name.trim(),
        base_currency: values.base_currency,
        timezone: values.timezone,
        month_start_day: Number(values.month_start_day),
      },
      {
        onSuccess: (household) => {
          toast.show({ message: t('create.created', { name: household.name }) })
          void navigate(householdPath(household.id), { replace: true })
        },
      },
    )
  }

  return (
    <PlainLayout>
      <PageHeader
        title={t('create.title')}
        description={t('create.description')}
      />
      <div className={styles.card}>
        <Form onSubmit={submit} label={t('create.title')}>
          <FormAlert error={create.error} shownFields={shownFields} />
          <HouseholdFields
            values={values}
            onChange={setValues}
            errors={{ ...server, name: localError ?? server.name }}
          />
          <FormActions>
            <Button type="submit" variant="primary" busy={create.isPending}>
              {t('create.submit')}
            </Button>
            {households.length > 0 && <LinkButton to="/">{t('create.cancel')}</LinkButton>}
          </FormActions>
        </Form>
      </div>
    </PlainLayout>
  )
}
