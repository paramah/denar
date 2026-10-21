import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useConnectionProviders, useConnections } from '../../api/hooks/imports.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { Connection } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { LinkButton } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { connectionKindLabels, connectionStatusLabels } from '../../lib/labels.ts'
import styles from './settings.module.css'

const statusTone = { active: 'positive', error: 'negative', disabled: 'neutral' } as const

/** Połączenia z bankami i dostawcami (008): tylko odczyt — zarządzanie dojdzie z konektorami. */
export function ConnectionsTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('connections.title'))
  const { household } = useHousehold()
  const connections = useConnections(household.id)
  const providers = useConnectionProviders()
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletNames = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w.name])), [wallets.data])

  return (
    <section className={styles.section} aria-labelledby="connections-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="connections-heading">{t('connections.title')}</h2>
          <p>{t('connections.lead')}</p>
        </div>
      </div>
      {connections.isPending ? (
        <Skeleton lines={3} label={t('connections.loading')} />
      ) : connections.isError ? (
        <SectionError
          title={t('connections.loadFailed')}
          error={connections.error}
          onRetry={() => void connections.refetch()}
        />
      ) : (
        <DataList
          label={t('connections.title')}
          variant="cards"
          items={connections.data}
          getKey={(c) => c.id}
          empty={
            <EmptyState
              icon="plug"
              title={t('connections.empty.title')}
              action={
                <LinkButton variant="primary" icon="upload" to={householdPath(household.id, 'import')}>
                  {t('connections.empty.action')}
                </LinkButton>
              }
            >
              <p>{t('connections.empty.body')}</p>
            </EmptyState>
          }
          renderItem={(connection) => <ConnectionCard connection={connection} walletNames={walletNames} />}
        />
      )}

      <div className={styles.card}>
        <h3 className={styles.cardTitle}>{t('connections.providers.title')}</h3>
        {providers.isPending ? (
          <Skeleton lines={1} />
        ) : providers.isError ? (
          <SectionError title={t('connections.providers.loadFailed')} error={providers.error} onRetry={() => void providers.refetch()} />
        ) : providers.data.length === 0 ? (
          <p className={styles.muted}>{t('connections.providers.none')}</p>
        ) : (
          <ul className={styles.plainList} aria-label={t('connections.providers.listLabel')}>
            {providers.data.map((p) => (
              <li key={p.key}>
                <strong>{p.name}</strong> · {connectionKindLabels[p.kind]}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}

function ConnectionCard({ connection, walletNames }: { connection: Connection; walletNames: Map<string, string> }) {
  const { t } = useTranslation('settings')
  return (
    <article className={styles.connection} aria-label={connection.display_name}>
      <div className={styles.rowMain}>
        <p className={styles.rowTitle}>
          {connection.display_name}
          <Badge tone={statusTone[connection.status]}>{connectionStatusLabels[connection.status]}</Badge>
        </p>
        <p className={styles.rowMeta}>
          {connection.provider_name} · {connectionKindLabels[connection.kind]} ·{' '}
          {connection.last_sync_at
            ? t('connections.card.lastSync', { date: formatDateTime(connection.last_sync_at) })
            : t('connections.card.neverSynced')}
        </p>
        {connection.last_error && (
          <p className={styles.connectionError}>
            <span className="visually-hidden">{t('connections.card.lastError')}</span>
            {connection.last_error}
          </p>
        )}
      </div>
      {connection.accounts.length > 0 && (
        <ul className={styles.accounts} aria-label={t('connections.card.accounts', { name: connection.display_name })}>
          {connection.accounts.map((account) => (
            <li key={account.external_id}>
              <span>
                {account.name || account.external_id}
                {account.currency && ` (${account.currency})`}
              </span>
              <span className={styles.rowMeta}>
                {account.wallet_id
                  ? t('connections.card.wallet', { wallet: walletNames.get(account.wallet_id) ?? t('connections.card.walletFallback') })
                  : t('connections.card.noWallet')}
              </span>
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}
