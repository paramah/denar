import { useState } from 'react'
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
import { batchTitle, countLabel } from './model.ts'
import { BatchCounters, BatchStatus, RowErrors } from './parts.tsx'
import styles from './import.module.css'

/** Szczegóły partii importu i jej cofnięcie. */
export function ImportBatchPage() {
  const { importId = '' } = useParams()
  const { household } = useHousehold()
  const batch = useImportBatch(household.id, importId)
  if (batch.isError && classify(batch.error) === 'notFound') {
    return <MissingBatch />
  }
  return (
    <QueryState query={batch} errorTitle="Nie udało się wczytać importu" loading={<Skeleton lines={5} />}>
      {(data) => <BatchView batch={data} />}
    </QueryState>
  )
}

function MissingBatch() {
  const { household } = useHousehold()
  useDocumentTitle('Nie znaleziono importu')
  return (
    <EmptyState
      icon="upload"
      headingLevel={1}
      title="Nie znaleziono importu"
      action={
        <LinkButton to={householdPath(household.id, 'import/history')} variant="primary">
          Historia importów
        </LinkButton>
      }
    >
      <p>Import nie istnieje albo dotyczy portfela, którego nie widzisz.</p>
    </EmptyState>
  )
}

function BatchView({ batch }: { batch: ImportBatch }) {
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
        documentTitle={`Import: ${title}`}
        description={
          <>
            Import do portfela {wallet?.name ?? '…'} · {formatDateTime(batch.created_at)}
          </>
        }
        actions={
          <>
            <LinkButton icon="history" to={householdPath(household.id, 'import/history')}>
              Historia importów
            </LinkButton>
            {canRevert && (
              <Button variant="danger" icon="undo" onClick={() => setConfirming(true)}>
                Cofnij import
              </Button>
            )}
          </>
        }
      />
      <div className={styles.page}>
        <section className={styles.panel} aria-labelledby="batch-facts">
          <h2 id="batch-facts" className={styles.stepTitle}>
            Podsumowanie
          </h2>
          <dl className={styles.facts}>
            <div>
              <dt>Status</dt>
              <dd>
                <BatchStatus batch={batch} />
              </dd>
            </div>
            <div>
              <dt>Źródło</dt>
              <dd>{batch.source === 'csv' ? 'Plik CSV' : 'Połączenie'}</dd>
            </div>
            {batch.preset && (
              <div>
                <dt>Preset</dt>
                <dd>{batch.preset}</dd>
              </div>
            )}
            <div>
              <dt>Zaimportował(a)</dt>
              <dd>
                {actorName(batch.created_by)}, {formatDateTime(batch.created_at)}
              </dd>
            </div>
            {batch.reverted_at && batch.reverted_by && (
              <div>
                <dt>Cofnął(a)</dt>
                <dd>
                  {actorName(batch.reverted_by)}, {formatDateTime(batch.reverted_at)}
                </dd>
              </div>
            )}
          </dl>
          <BatchCounters batch={batch} />
          {batch.status === 'reverted' && (
            <Callout tone="warn" title="Import cofnięty">
              Usunięto {countLabel(batch.reverted_count, 'transakcję', 'transakcje', 'transakcji')}. Ponowny import tego
              samego pliku utworzy je od nowa.
            </Callout>
          )}
          <div className={styles.summaryLinks}>
            {wallet && (
              <LinkButton icon="list" to={householdPath(household.id, `wallets/${wallet.id}`)}>
                Rejestr portfela
              </LinkButton>
            )}
            {batch.matched_count > 0 && (
              <LinkButton icon="repeat" to={householdPath(household.id, 'subscriptions/charges')}>
                Kolejka dopasowań subskrypcji
              </LinkButton>
            )}
          </div>
        </section>
        <RowErrors errors={batch.errors} total={batch.error_count} label="Pominięte wiersze" />
      </div>
      <ConfirmDialog
        open={confirming}
        title="Cofnąć ten import?"
        confirmLabel="Cofnij import"
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
                message: `Cofnięto import: usunięto ${countLabel(done.reverted_count, 'transakcję', 'transakcje', 'transakcji')}.`,
              })
            },
          })
        }
      >
        <p>
          Usuniemy {countLabel(batch.created_count, 'transakcję', 'transakcje', 'transakcji')} z tego importu (także te
          zmienione później). Płatności subskrypcji, które opłacały, wrócą do kolejki dopasowań.
        </p>
      </ConfirmDialog>
    </>
  )
}
