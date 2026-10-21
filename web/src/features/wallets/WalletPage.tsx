import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('wallets')
  const { walletId = '' } = useParams()
  const { household } = useHousehold()
  const wallet = useWallet(household.id, walletId)

  if (wallet.isError && classify(wallet.error) === 'notFound') {
    return <MissingWallet />
  }
  return (
    <QueryState query={wallet} errorTitle={t('page.loadFailed')} loading={<Skeleton lines={4} />}>
      {(data) => <WalletView wallet={data} />}
    </QueryState>
  )
}

function MissingWallet() {
  const { t } = useTranslation('wallets')
  const { household } = useHousehold()
  useDocumentTitle(t('page.notFound'))
  return (
    <EmptyState
      icon="wallet"
      headingLevel={1}
      title={t('page.notFound')}
      action={
        <LinkButton to={householdPath(household.id, 'wallets')} variant="primary">
          {t('page.back')}
        </LinkButton>
      }
    >
      <p>{t('page.notFoundBody')}</p>
    </EmptyState>
  )
}

function WalletView({ wallet }: { wallet: Wallet }) {
  const { t } = useTranslation('wallets')
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
                · {t('page.newTransactionShortcut')} <kbd className={txStyles.kbd}>N</kbd>
              </>
            )}
          </>
        }
        actions={
          canWrite && (
            <>
              <Button icon="edit" onClick={() => setEditing(true)}>
                {t('page.edit')}
              </Button>
              {archived ? (
                <Button
                  icon="refresh"
                  busy={update.isPending}
                  onClick={() =>
                    update.mutate(
                      { walletId: wallet.id, body: { archived: false } },
                      { onSuccess: () => toast.show({ message: t('page.restored', { name: wallet.name }) }) },
                    )
                  }
                >
                  {t('page.restore')}
                </Button>
              ) : (
                <>
                  <Button icon="archive" onClick={() => setArchiving(true)}>
                    {t('page.archive')}
                  </Button>
                  <Button icon="transfer" onClick={() => setTransferring(true)}>
                    {t('page.transfer')}
                  </Button>
                  <LinkButton icon="upload" to={householdPath(household.id, `import?wallet=${wallet.id}`)}>
                    {t('page.import')}
                  </LinkButton>
                  <Button variant="primary" icon="plus" onClick={() => setAdding(true)} aria-keyshortcuts="n">
                    {t('page.addTransaction')}
                  </Button>
                </>
              )}
            </>
          )
        }
      />
      <div className={styles.header}>
        <dl className={styles.facts} aria-label={t('page.balances')}>
          <div>
            <dt>{t('page.balance')}</dt>
            <dd>
              <Money amount={wallet.balance.amount} currency={wallet.currency} size="xl" />
            </dd>
          </div>
          <div>
            <dt>{t('page.cleared')}</dt>
            <dd>
              <Money amount={wallet.cleared_balance.amount} currency={wallet.currency} size="lg" />
            </dd>
          </div>
          {foreign && (
            <div>
              <dt>{t('page.inBase', { currency: household.base_currency })}</dt>
              <dd>
                <Money amount={wallet.base_balance.amount} currency={wallet.base_balance.currency} size="lg" />
              </dd>
            </div>
          )}
        </dl>
        <WalletBadges wallet={wallet} />
        {archived && (
          <Callout tone="warn">{t('page.archivedNotice')}</Callout>
        )}
      </div>
      <Register walletId={wallet.id} onAdd={archived ? undefined : () => setAdding(true)} />
      <TransactionDialog open={adding} onClose={() => setAdding(false)} walletId={wallet.id} />
      <TransferDialog open={transferring} onClose={() => setTransferring(false)} fromWalletId={wallet.id} />
      <WalletDialog open={editing} onClose={() => setEditing(false)} wallet={wallet} />
      <ConfirmDialog
        open={archiving}
        title={t('page.archiveTitle', { name: wallet.name })}
        confirmLabel={t('page.archive')}
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
                toast.show({ message: t('page.archived', { name: wallet.name }) })
              },
            },
          )
        }
      >
        <p>{t('page.archiveBody')}</p>
      </ConfirmDialog>
    </>
  )
}
