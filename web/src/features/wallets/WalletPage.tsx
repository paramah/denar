import { useState } from 'react'
import { useParams } from 'react-router'
import { classify } from '../../api/errors.ts'
import { useUpdateWallet, useWallet } from '../../api/hooks/wallets.ts'
import type { Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { PageHeader, useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { walletTypeLabels } from '../../lib/labels.ts'
import { useHotkey } from '../../lib/useHotkey.ts'
import { Register } from '../transactions/Register.tsx'
import { TransactionDialog } from '../transactions/TransactionDialog.tsx'
import { TransferDialog } from '../transactions/TransferDialog.tsx'
import styles from './wallets.module.css'
import txStyles from '../transactions/transactions.module.css'
import { WalletBadges } from './WalletCard.tsx'
import { WalletDialog } from './WalletDialog.tsx'

export function WalletPage() {
  const { walletId = '' } = useParams()
  const { household } = useHousehold()
  const wallet = useWallet(household.id, walletId)

  if (wallet.isError && classify(wallet.error) === 'notFound') {
    return <MissingWallet />
  }
  return (
    <QueryState query={wallet} errorTitle="Nie udało się wczytać portfela" loading={<Skeleton lines={4} />}>
      {(data) => <WalletView wallet={data} />}
    </QueryState>
  )
}

function MissingWallet() {
  const { household } = useHousehold()
  useDocumentTitle('Nie znaleziono portfela')
  return (
    <EmptyState
      icon="wallet"
      headingLevel={1}
      title="Nie znaleziono portfela"
      action={
        <LinkButton to={householdPath(household.id, 'wallets')} variant="primary">
          Wróć do portfeli
        </LinkButton>
      }
    >
      <p>Portfel nie istnieje albo jest prywatny i nie masz do niego dostępu.</p>
    </EmptyState>
  )
}

function WalletView({ wallet }: { wallet: Wallet }) {
  const { household, role } = useHousehold()
  const canWrite = role !== 'viewer'
  const archived = !!wallet.archived_at
  const [adding, setAdding] = useState(false)
  const [transferring, setTransferring] = useState(false)
  const [editing, setEditing] = useState(false)
  const [archiving, setArchiving] = useState(false)
  const update = useUpdateWallet(household.id)
  const toast = useToast()
  useHotkey('n', () => setAdding(true), canWrite && !archived)
  const foreign = wallet.currency !== household.base_currency

  return (
    <>
      <PageHeader
        title={wallet.name}
        description={
          <>
            {walletTypeLabels[wallet.type]} · {wallet.currency}
            {canWrite && !archived && (
              <>
                {' '}
                · Nowa transakcja: <kbd className={txStyles.kbd}>N</kbd>
              </>
            )}
          </>
        }
        actions={
          canWrite && (
            <>
              <Button icon="edit" onClick={() => setEditing(true)}>
                Edytuj
              </Button>
              {archived ? (
                <Button
                  icon="refresh"
                  busy={update.isPending}
                  onClick={() =>
                    update.mutate(
                      { walletId: wallet.id, body: { archived: false } },
                      { onSuccess: () => toast.show({ message: `Przywrócono portfel ${wallet.name}.` }) },
                    )
                  }
                >
                  Przywróć
                </Button>
              ) : (
                <>
                  <Button icon="archive" onClick={() => setArchiving(true)}>
                    Archiwizuj
                  </Button>
                  <Button icon="transfer" onClick={() => setTransferring(true)}>
                    Przelew
                  </Button>
                  <Button variant="primary" icon="plus" onClick={() => setAdding(true)} aria-keyshortcuts="n">
                    Dodaj transakcję
                  </Button>
                </>
              )}
            </>
          )
        }
      />
      <div className={styles.header}>
        <dl className={styles.facts} aria-label="Salda portfela">
          <div>
            <dt>Saldo</dt>
            <dd>
              <Money amount={wallet.balance.amount} currency={wallet.currency} size="xl" />
            </dd>
          </div>
          <div>
            <dt>Rozliczone</dt>
            <dd>
              <Money amount={wallet.cleared_balance.amount} currency={wallet.currency} size="lg" />
            </dd>
          </div>
          {foreign && (
            <div>
              <dt>W walucie {household.base_currency}</dt>
              <dd>
                <Money amount={wallet.base_balance.amount} currency={wallet.base_balance.currency} size="lg" />
              </dd>
            </div>
          )}
        </dl>
        <WalletBadges wallet={wallet} />
        {archived && (
          <Callout tone="warn">Portfel jest zarchiwizowany: nie przyjmuje nowych transakcji ani przelewów.</Callout>
        )}
      </div>
      <Register walletId={wallet.id} onAdd={archived ? undefined : () => setAdding(true)} />
      <TransactionDialog open={adding} onClose={() => setAdding(false)} walletId={wallet.id} />
      <TransferDialog open={transferring} onClose={() => setTransferring(false)} fromWalletId={wallet.id} />
      <WalletDialog open={editing} onClose={() => setEditing(false)} wallet={wallet} />
      <ConfirmDialog
        open={archiving}
        title={`Zarchiwizować portfel ${wallet.name}?`}
        confirmLabel="Archiwizuj"
        danger={false}
        busy={update.isPending}
        error={<FormAlert error={update.error} />}
        onCancel={() => setArchiving(false)}
        onConfirm={() =>
          update.mutate(
            { walletId: wallet.id, body: { archived: true } },
            {
              onSuccess: () => {
                setArchiving(false)
                toast.show({ message: `Zarchiwizowano portfel ${wallet.name}.` })
              },
            },
          )
        }
      >
        <p>Historia zostaje, ale portfel znika z list i sum. Możesz go przywrócić w każdej chwili.</p>
      </ConfirmDialog>
    </>
  )
}
