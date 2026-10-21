import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { classify } from '../../api/errors.ts'
import { useImportBatch, useRevertImport } from '../../api/hooks/imports.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { ImportBatch } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { PageHeader, useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { useActorName } from './ImportHistoryPage.tsx'
import { batchTitle } from './model.ts'
import { BatchCounters, BatchStatus, RowErrors } from './parts.tsx'
import styles from './import.module.css'

/** Szczegóły partii importu i jej cofnięcie. */
export function ImportBatchPage() {
  const { t } = useTranslation('import')
  const { importId = '' } = useParams()
  const { household } = useHousehold()
  const batch = useImportBatch(household.id, importId)
  if (batch.isError && classify(batch.error) === 'notFound') {
    return <MissingBatch />
  }
  return (
    <QueryState query={batch} errorTitle={t('batch.loadFailed')} loading={<Skeleton lines={5} />}>
      {(data) => <BatchView batch={data} />}
    </QueryState>
  )
}

function MissingBatch() {
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  useDocumentTitle(t('batch.notFound'))
  return (
    <EmptyState
      icon="upload"
      headingLevel={1}
      title={t('batch.notFound')}
      action={
        <LinkButton to={householdPath(household.id, 'import/history')} variant="primary">
          {t('batch.history')}
        </LinkButton>
      }
    >
      <p>{t('batch.notFoundBody')}</p>
    </EmptyState>
  )
}

function BatchView({ batch }: { batch: ImportBatch }) {
  const { t } = useTranslation('import')
  const { household, role } = useHousehold()
  const wallets = useWallets(household.id, { includeArchived: true })
  const wallet = wallets.data?.find((w) => w.id === batch.wallet_id)
  const actorName = useActorName()
  const revert = useRevertImport(household.id)
  const toast = useToast()
  const [confirming, setConfirming] = useState(false)
  const canRevert = role !== 'viewer' && batch.status === 'completed'
  const title = batchTitle(batch)

  return (
    <>
      <PageHeader
        title={title}
        documentTitle={t('batch.documentTitle', { title })}
        description={t('batch.description', { wallet: wallet?.name ?? '…', date: formatDateTime(batch.created_at) })}
        actions={
          <>
            <LinkButton icon="history" to={householdPath(household.id, 'import/history')}>
              {t('batch.history')}
            </LinkButton>
            {canRevert && (
              <Button variant="danger" icon="undo" onClick={() => setConfirming(true)}>
                {t('batch.revert')}
              </Button>
            )}
          </>
        }
      />
      <div className={styles.page}>
        <section className={styles.panel} aria-labelledby="batch-facts">
          <h2 id="batch-facts" className={styles.stepTitle}>
            {t('batch.summary')}
          </h2>
          <dl className={styles.facts}>
            <div>
              <dt>{t('batch.status')}</dt>
              <dd>
                <BatchStatus batch={batch} />
              </dd>
            </div>
            <div>
              <dt>{t('batch.source')}</dt>
              <dd>{batch.source === 'csv' ? t('batch.sourceCsv') : t('batch.sourceConnection')}</dd>
            </div>
            {batch.preset && (
              <div>
                <dt>{t('batch.preset')}</dt>
                <dd>{batch.preset}</dd>
              </div>
            )}
            <div>
              <dt>{t('batch.createdBy')}</dt>
              <dd>{t('batch.actorAt', { actor: actorName(batch.created_by), date: formatDateTime(batch.created_at) })}</dd>
            </div>
            {batch.reverted_at && batch.reverted_by && (
              <div>
                <dt>{t('batch.revertedBy')}</dt>
                <dd>{t('batch.actorAt', { actor: actorName(batch.reverted_by), date: formatDateTime(batch.reverted_at) })}</dd>
              </div>
            )}
          </dl>
          <BatchCounters batch={batch} />
          {batch.status === 'reverted' && (
            <Callout tone="warn" title={t('batch.revertedTitle')}>
              {t('batch.revertedBody', { count: batch.reverted_count })}
            </Callout>
          )}
          <div className={styles.summaryLinks}>
            {wallet && (
              <LinkButton icon="list" to={householdPath(household.id, `wallets/${wallet.id}`)}>
                {t('batch.register')}
              </LinkButton>
            )}
            {batch.matched_count > 0 && (
              <LinkButton icon="repeat" to={householdPath(household.id, 'subscriptions/charges')}>
                {t('batch.chargesQueue')}
              </LinkButton>
            )}
          </div>
        </section>
        <RowErrors errors={batch.errors} total={batch.error_count} label={t('batch.skippedRows')} />
      </div>
      <ConfirmDialog
        open={confirming}
        title={t('batch.confirmTitle')}
        confirmLabel={t('batch.confirmLabel')}
        busy={revert.isPending}
        error={<FormAlert error={revert.error} />}
        onCancel={() => {
          setConfirming(false)
          revert.reset()
        }}
        onConfirm={() =>
          revert.mutate(batch.id, {
            onSuccess: (done) => {
              setConfirming(false)
              toast.show({
                message: t('batch.revertedToast', { count: done.reverted_count }),
              })
            },
          })
        }
      >
        <p>{t('batch.confirmBody', { count: batch.created_count })}</p>
      </ConfirmDialog>
    </>
  )
}
