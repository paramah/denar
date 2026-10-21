import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useImports } from '../../api/hooks/imports.ts'
import { useMembers } from '../../api/hooks/members.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { Actor, ImportBatch } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { LinkButton } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { batchTitle, countLabel } from './model.ts'
import { BatchStatus } from './parts.tsx'
import styles from './import.module.css'

/** Imiona autorów partii (członkowie; boty i system opisowo). */
export function useActorName() {
  const { household } = useHousehold()
  const members = useMembers(household.id)
  return useMemo(() => {
    const names = new Map((members.data ?? []).map((m) => [m.user_id, m.name]))
    return (actor: Actor) =>
      actor.kind === 'user' ? (names.get(actor.id) ?? 'Członek gospodarstwa') : actor.kind === 'bot' ? 'Bot' : 'System'
  }, [members.data])
}

/** Historia partii importu z filtrem portfela. */
export function ImportHistoryPage() {
  useDocumentTitle('Historia importów')
  const { household, role } = useHousehold()
  const [search, setSearch] = useSearchParams()
  const walletId = search.get('wallet') ?? ''
  const wallets = useWallets(household.id, { includeArchived: true })
  const imports = useImports(household.id, { walletId })
  const actorName = useActorName()
  const walletNames = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w.name])), [wallets.data])
  const items = imports.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <section className={styles.panel} aria-labelledby="history-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="history-heading" className={styles.stepTitle}>
            Historia importów
          </h2>
          <p className={styles.muted}>Każdy wgrany plik to partia — możesz ją sprawdzić i cofnąć. Od najnowszych.</p>
        </div>
        <SelectField
          label="Portfel"
          value={walletId}
          options={[
            { value: '', label: 'Wszystkie portfele' },
            ...(wallets.data ?? []).map((w) => ({ value: w.id, label: w.name })),
          ]}
          onChange={(e) => setSearch(e.target.value ? { wallet: e.target.value } : {}, { replace: true })}
        />
      </div>
      {imports.isPending ? (
        <Skeleton lines={4} label="Wczytywanie historii…" />
      ) : imports.isError && items.length === 0 ? (
        <SectionError title="Nie udało się wczytać historii" error={imports.error} onRetry={() => void imports.refetch()} />
      ) : (
        <DataList
          label="Partie importu"
          variant="cards"
          items={items}
          getKey={(batch) => batch.id}
          hasMore={imports.hasNextPage}
          onLoadMore={() => void imports.fetchNextPage()}
          loadingMore={imports.isFetchingNextPage}
          loadMoreLabel="Pokaż starsze"
          empty={
            <EmptyState
              icon="upload"
              title={walletId ? 'Brak importów w tym portfelu' : 'Nie ma jeszcze importów'}
              action={
                role !== 'viewer' && (
                  <LinkButton
                    variant="primary"
                    icon="upload"
                    to={householdPath(household.id, walletId ? `import?wallet=${walletId}` : 'import')}
                  >
                    Importuj CSV
                  </LinkButton>
                )
              }
            >
              <p>Wgraj wyciąg z banku, a transakcje pojawią się w portfelu bez przepisywania.</p>
            </EmptyState>
          }
          renderItem={(batch) => (
            <BatchRow batch={batch} walletName={walletNames.get(batch.wallet_id)} actorName={actorName(batch.created_by)} />
          )}
        />
      )}
    </section>
  )
}

function BatchRow({ batch, walletName, actorName }: { batch: ImportBatch; walletName?: string; actorName: string }) {
  const { household } = useHousehold()
  const title = batchTitle(batch)
  return (
    <article className={styles.batch} aria-label={title}>
      <div className={styles.batchMain}>
        <p className={styles.batchTitle}>
          <Link to={householdPath(household.id, `import/${batch.id}`)}>{title}</Link>
          <BatchStatus batch={batch} />
        </p>
        <p className={styles.muted}>
          {walletName ?? 'Portfel'}
          {batch.preset ? ` · ${batch.preset}` : ''} · {actorName}, {formatDateTime(batch.created_at)}
        </p>
      </div>
      <p className={styles.batchCounts}>
        {countLabel(batch.created_count, 'nowa', 'nowe', 'nowych')} · {batch.duplicate_count} dupl. ·{' '}
        {countLabel(batch.error_count, 'błąd', 'błędy', 'błędów')}
        {batch.matched_count > 0 && ` · ${batch.matched_count} subskr.`}
      </p>
    </article>
  )
}
