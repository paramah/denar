import { useMemo } from 'react'
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
  useDocumentTitle('Połączenia')
  const { household } = useHousehold()
  const connections = useConnections(household.id)
  const providers = useConnectionProviders()
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletNames = useMemo(() => new Map((wallets.data ?? []).map((w) => [w.id, w.name])), [wallets.data])

  return (
    <section className={styles.section} aria-labelledby="connections-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="connections-heading">Połączenia</h2>
          <p>
            Automatyczne pobieranie transakcji z banków i rachunków dostawców. Pobrane operacje przechodzą przez ten sam
            import co pliki CSV: duplikaty są pomijane, kontrahenci dopasowani regułami.
          </p>
        </div>
      </div>
      {connections.isPending ? (
        <Skeleton lines={3} label="Wczytywanie połączeń…" />
      ) : connections.isError ? (
        <SectionError
          title="Nie udało się wczytać połączeń"
          error={connections.error}
          onRetry={() => void connections.refetch()}
        />
      ) : (
        <DataList
          label="Połączenia"
          variant="cards"
          items={connections.data}
          getKey={(c) => c.id}
          empty={
            <EmptyState
              icon="plug"
              title="Brak połączeń"
              action={
                <LinkButton variant="primary" icon="upload" to={householdPath(household.id, 'import')}>
                  Importuj CSV
                </LinkButton>
              }
            >
              <p>
                Integracje z bankami i dostawcami (PSD2, OpenAI, Apple) pojawią się w przyszłych wersjach; teraz użyj
                importu CSV — wyciąg z banku trafi do portfela w kilka kliknięć.
              </p>
            </EmptyState>
          }
          renderItem={(connection) => <ConnectionCard connection={connection} walletNames={walletNames} />}
        />
      )}

      <div className={styles.card}>
        <h3 className={styles.cardTitle}>Dostawcy na tym serwerze</h3>
        {providers.isPending ? (
          <Skeleton lines={1} />
        ) : providers.isError ? (
          <SectionError title="Nie udało się wczytać dostawców" error={providers.error} onRetry={() => void providers.refetch()} />
        ) : providers.data.length === 0 ? (
          <p className={styles.muted}>
            Żaden dostawca nie jest jeszcze dostępny. Kolejne wersje Denara dodadzą banki (PSD2 przez agregatora) i rachunki
            OpenAI oraz Apple.
          </p>
        ) : (
          <ul className={styles.plainList} aria-label="Dostawcy połączeń">
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
  return (
    <article className={styles.connection} aria-label={connection.display_name}>
      <div className={styles.rowMain}>
        <p className={styles.rowTitle}>
          {connection.display_name}
          <Badge tone={statusTone[connection.status]}>{connectionStatusLabels[connection.status]}</Badge>
        </p>
        <p className={styles.rowMeta}>
          {connection.provider_name} · {connectionKindLabels[connection.kind]} ·{' '}
          {connection.last_sync_at ? `ostatnia synchronizacja ${formatDateTime(connection.last_sync_at)}` : 'jeszcze nie synchronizowano'}
        </p>
        {connection.last_error && (
          <p className={styles.connectionError}>
            <span className="visually-hidden">Ostatni błąd: </span>
            {connection.last_error}
          </p>
        )}
      </div>
      {connection.accounts.length > 0 && (
        <ul className={styles.accounts} aria-label={`Konta: ${connection.display_name}`}>
          {connection.accounts.map((account) => (
            <li key={account.external_id}>
              <span>
                {account.name || account.external_id}
                {account.currency && ` (${account.currency})`}
              </span>
              <span className={styles.rowMeta}>
                {account.wallet_id ? `→ ${walletNames.get(account.wallet_id) ?? 'portfel'}` : 'nie przypisano do portfela'}
              </span>
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}
