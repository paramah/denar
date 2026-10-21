import { useState } from 'react'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { useHotkey } from '../../lib/useHotkey.ts'
import { Register } from './Register.tsx'
import { TransactionDialog } from './TransactionDialog.tsx'
import { TransferDialog } from './TransferDialog.tsx'
import styles from './transactions.module.css'

/** Rejestr wszystkich widocznych portfeli. */
export function TransactionsPage() {
  const { role } = useHousehold()
  const canWrite = role !== 'viewer'
  const [adding, setAdding] = useState(false)
  const [transferring, setTransferring] = useState(false)
  useHotkey('n', () => setAdding(true), canWrite)

  return (
    <>
      <PageHeader
        title="Transakcje"
        description={
          canWrite ? (
            <>
              Wszystkie portfele w jednym rejestrze. Nowa transakcja: <kbd className={styles.kbd}>N</kbd>
            </>
          ) : (
            'Wszystkie portfele w jednym rejestrze.'
          )
        }
        actions={
          canWrite && (
            <>
              <Button icon="transfer" onClick={() => setTransferring(true)}>
                Przelew
              </Button>
              <Button variant="primary" icon="plus" onClick={() => setAdding(true)} aria-keyshortcuts="n">
                Dodaj transakcję
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
