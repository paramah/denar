import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useOutletContext } from 'react-router'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { SubscriptionTabs } from './parts.tsx'
import { SubscriptionDialog } from './SubscriptionDialog.tsx'
import styles from './subscriptions.module.css'

type Context = { create: () => void; canWrite: boolean }

/** Akcje wspólne dla zakładek subskrypcji (np. przycisk w pustym stanie). */
export function useSubscriptionsLayout(): Context {
  return useOutletContext<Context>()
}

/** Nagłówek, zakładki (przegląd, wszystkie, do dopasowania) i nowa subskrypcja. */
export function SubscriptionsLayout() {
  const { household, role } = useHousehold()
  const { t } = useTranslation('subscriptions')
  const canWrite = role !== 'viewer'
  const [creating, setCreating] = useState(false)
  return (
    <>
      <PageHeader
        title={t('layout.title')}
        description={t('layout.description', { name: household.name, currency: household.base_currency })}
        actions={
          canWrite && (
            <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
              {t('layout.new')}
            </Button>
          )
        }
      />
      <SubscriptionTabs />
      <div className={styles.page}>
        <Outlet context={{ create: () => setCreating(true), canWrite } satisfies Context} />
      </div>
      <SubscriptionDialog open={creating} onClose={() => setCreating(false)} />
    </>
  )
}
