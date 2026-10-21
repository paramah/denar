import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { useHotkey } from '../../lib/useHotkey.ts'
import { Register } from './Register.tsx'
import { TransactionDialog } from './TransactionDialog.tsx'
import { TransferDialog } from './TransferDialog.tsx'
import styles from './transactions.module.css'

/** Rejestr wszystkich widocznych portfeli. */
export function TransactionsPage() {
  const { t } = useTranslation('transactions')
  const { household, role } = useHousehold()
  const canWrite = role !== 'viewer'
  const [adding, setAdding] = useState(false)
  const [transferring, setTransferring] = useState(false)
  useHotkey('n', () => setAdding(true), canWrite)

  return (
    <>
      <PageHeader
        title={t('page.title')}
        description={
          canWrite ? (
            <>
              {t('page.description')} {t('page.newTransactionShortcut')} <kbd className={styles.kbd}>N</kbd>
            </>
          ) : (
            t('page.description')
          )
        }
        actions={
          canWrite && (
            <>
              <LinkButton icon="upload" to={householdPath(household.id, 'import')}>
                {t('page.import')}
              </LinkButton>
              <Button icon="transfer" onClick={() => setTransferring(true)}>
                {t('page.transfer')}
              </Button>
              <Button variant="primary" icon="plus" onClick={() => setAdding(true)} aria-keyshortcuts="n">
                {t('page.add')}
              </Button>
            </>
          )
        }
      />
      <Register onAdd={() => setAdding(true)} />
      <TransactionDialog open={adding} onClose={() => setAdding(false)} />
      <TransferDialog open={transferring} onClose={() => setTransferring(false)} />
    </>
  )
}
