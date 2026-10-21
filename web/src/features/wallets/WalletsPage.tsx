import { useState } from 'react'
import { useWallets } from '../../api/hooks/wallets.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Checkbox } from '../../components/Field/Field.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { readStored, writeStored } from '../../lib/storage.ts'
import { TransferDialog } from '../transactions/TransferDialog.tsx'
import { useDecimals } from '../transactions/money.ts'
import { walletTotals } from './totals.ts'
import { WalletCard, WalletSummary } from './WalletCard.tsx'
import { WalletDialog } from './WalletDialog.tsx'
import styles from './wallets.module.css'

const archivedKey = 'denar:wallets:archived'

export function WalletsPage() {
  const { household, role } = useHousehold()
  const canWrite = role !== 'viewer'
  const [showArchived, setShowArchived] = useState(() => readStored(archivedKey) === '1')
  const wallets = useWallets(household.id, { includeArchived: showArchived })
  const decimalsFor = useDecimals()
  const [creating, setCreating] = useState(false)
  const [transferring, setTransferring] = useState(false)

  return (
    <>
      <PageHeader
        title="Portfele"
        description="Konta, gotówka, karty i oszczędności — każde w swojej walucie."
        actions={
          canWrite && (
            <>
              <Button icon="transfer" onClick={() => setTransferring(true)}>
                Przelew
              </Button>
              <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                Nowy portfel
              </Button>
            </>
          )
        }
      />
      <QueryState query={wallets} errorTitle="Nie udało się wczytać portfeli" loading={<Skeleton lines={4} />}>
        {(items) =>
          items.length === 0 && !showArchived ? (
            <EmptyState
              icon="wallet"
              title="Nie ma jeszcze portfeli"
              action={
                canWrite && (
                  <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                    Dodaj pierwszy portfel
                  </Button>
                )
              }
            >
              <p>Portfel to konto w banku, gotówka w szufladzie, karta albo lokata. Zacznij od tego, którego używasz najczęściej.</p>
            </EmptyState>
          ) : (
            <div className={styles.form}>
              <WalletSummary
                totals={walletTotals(items, decimalsFor(household.base_currency))}
                currency={household.base_currency}
              />
              <div className={styles.toolbar}>
                <h2>Twoje portfele</h2>
                <Checkbox
                  label="Pokaż zarchiwizowane"
                  checked={showArchived}
                  onChange={(event) => {
                    setShowArchived(event.target.checked)
                    writeStored(archivedKey, event.target.checked ? '1' : null)
                  }}
                />
              </div>
              <ul className={styles.grid} aria-label="Portfele">
                {items.map((wallet) => (
                  <li key={wallet.id}>
                    <WalletCard wallet={wallet} />
                  </li>
                ))}
              </ul>
            </div>
          )
        }
      </QueryState>
      <WalletDialog open={creating} onClose={() => setCreating(false)} />
      <TransferDialog open={transferring} onClose={() => setTransferring(false)} />
    </>
  )
}
