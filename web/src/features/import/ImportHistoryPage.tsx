import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
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
import { batchTitle } from './model.ts'
import { BatchStatus } from './parts.tsx'
import styles from './import.module.css'

/** Imiona autorów partii (członkowie; boty i system opisowo). */
export function useActorName() {
  const { t } = useTranslation('import')
  const { household } = useHousehold()
  const members = useMembers(household.id)
  return useMemo(() => {
    const names = new Map((members.data ?? []).map((m) => [m.user_id, m.name]))
    return (actor: Actor) =>
      actor.kind === 'user'
        ? (names.get(actor.id) ?? t('history.member'))
        : actor.kind === 'bot'
          ? t('history.bot')
          : t('history.system')
  }, [members.data, t])
}

/** Historia partii importu z filtrem portfela. */
export function ImportHistoryPage() {
  const { t } = useTranslation('import')
  useDocumentTitle(t('history.documentTitle'))
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
            {t('history.heading')}
          </h2>
          <p className={styles.muted}>{t('history.intro')}</p>
        </div>
        <SelectField
          label={t('history.wallet')}
          value={walletId}
          options={[
            { value: '', label: t('history.allWallets') },
            ...(wallets.data ?? []).map((w) => ({ value: w.id, label: w.name })),
          ]}
          onChange={(e) => setSearch(e.target.value ? { wallet: e.target.value } : {}, { replace: true })}
        />
      </div>
      {imports.isPending ? (
        <Skeleton lines={4} label={t('history.loading')} />
      ) : imports.isError && items.length === 0 ? (
        <SectionError title={t('history.loadFailed')} error={imports.error} onRetry={() => void imports.refetch()} />
      ) : (
        <DataList
          label={t('history.listLabel')}
          variant="cards"
          items={items}
          getKey={(batch) => batch.id}
          hasMore={imports.hasNextPage}
          onLoadMore={() => void imports.fetchNextPage()}
          loadingMore={imports.isFetchingNextPage}
          loadMoreLabel={t('history.older')}
          empty={
            <EmptyState
              icon="upload"
              title={walletId ? t('history.emptyWallet') : t('history.empty')}
              action={
                role !== 'viewer' && (
                  <LinkButton
                    variant="primary"
                    icon="upload"
                    to={householdPath(household.id, walletId ? `import?wallet=${walletId}` : 'import')}
                  >
                    {t('history.importCsv')}
                  </LinkButton>
                )
              }
            >
              <p>{t('history.emptyBody')}</p>
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
  const { t } = useTranslation('import')
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
          {walletName ?? t('history.walletFallback')}
          {batch.preset ? ` · ${batch.preset}` : ''} · {actorName}, {formatDateTime(batch.created_at)}
        </p>
      </div>
      <p className={styles.batchCounts}>
        {t('history.newCount', { count: batch.created_count })} · {t('history.duplicates', { n: batch.duplicate_count })} ·{' '}
        {t('history.errorCount', { count: batch.error_count })}
        {batch.matched_count > 0 && t('history.matched', { n: batch.matched_count })}
      </p>
    </article>
  )
}
