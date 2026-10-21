import { useMemo, useState } from 'react'
import { useAudit } from '../../api/hooks/audit.ts'
import { useBots } from '../../api/hooks/bots.ts'
import { useMembers } from '../../api/hooks/members.ts'
import type { AuditFilters } from '../../api/keys.ts'
import type { ActorKind, AuditEntry } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { actionLabel, actorKindLabels, fieldLabel, resourceTypeLabel, resourceTypeLabels } from '../../lib/labels.ts'
import { cycleLabel } from '../../lib/subscriptions.ts'
import type { Cycle } from '../../api/types.ts'
import styles from './settings.module.css'

const actorOptions = [
  { value: '', label: 'Wszyscy' },
  ...(Object.keys(actorKindLabels) as ActorKind[]).map((kind) => ({ value: kind, label: actorKindLabels[kind] })),
]

const resourceOptions = [
  { value: '', label: 'Wszystkie' },
  ...Object.entries(resourceTypeLabels).map(([value, label]) => ({ value, label })),
]

const auditCycle = /^(\d+) (day|week|month|year) from (\d{4}-\d{2}-\d{2})$/

function showValue(value: unknown): string {
  // Cykl subskrypcji zapisany przez serwer jako „1 month from 2026-10-21”.
  const cycle = typeof value === 'string' ? auditCycle.exec(value) : null
  if (cycle?.[1] && cycle[2] && cycle[3]) {
    return cycleLabel({ every: Number(cycle[1]), unit: cycle[2] as Cycle['unit'], anchor: cycle[3] })
  }
  if (value === null || value === undefined || value === '') {
    return '—'
  }
  if (typeof value === 'boolean') {
    return value ? 'tak' : 'nie'
  }
  if (Array.isArray(value)) {
    return value.length ? value.map(showValue).join(', ') : '—'
  }
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    // Kwota: { amount: "49.00", currency: "PLN" } → „49.00 PLN”.
    if (typeof record.amount === 'string' && typeof record.currency === 'string') {
      return `${record.amount} ${record.currency}`
    }
    return JSON.stringify(value)
  }
  return String(value)
}

export function AuditTab() {
  useDocumentTitle('Dziennik zmian')
  const { household } = useHousehold()
  const [filters, setFilters] = useState<AuditFilters>({})
  const audit = useAudit(household.id, filters)
  const members = useMembers(household.id)
  const bots = useBots(household.id)

  const names = useMemo(() => {
    const map = new Map<string, string>()
    for (const member of members.data ?? []) {
      map.set(`user:${member.user_id}`, member.name)
    }
    for (const bot of bots.data ?? []) {
      map.set(`bot:${bot.id}`, bot.name)
    }
    return map
  }, [members.data, bots.data])

  const actorName = (entry: AuditEntry) => {
    if (entry.actor.kind === 'system') {
      return 'System'
    }
    const name = names.get(`${entry.actor.kind}:${entry.actor.id}`)
    return name ?? `${actorKindLabels[entry.actor.kind]} ${entry.actor.id.slice(-6)}`
  }

  const entries = audit.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <section className={styles.section} aria-labelledby="audit-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="audit-heading">Dziennik zmian</h2>
          <p>Kto i kiedy zmienił coś w gospodarstwie — ludzie, boty i zadania systemowe. Od najnowszych.</p>
        </div>
      </div>
      <div className={styles.filters} role="group" aria-label="Filtry dziennika">
        <SelectField
          label="Kto"
          value={filters.actor_kind ?? ''}
          options={actorOptions}
          onChange={(event) =>
            setFilters({ ...filters, actor_kind: (event.target.value || undefined) as ActorKind | undefined })
          }
        />
        <SelectField
          label="Czego dotyczy"
          value={filters.resource_type ?? ''}
          options={resourceOptions}
          onChange={(event) => setFilters({ ...filters, resource_type: event.target.value || undefined })}
        />
      </div>
      {audit.isPending ? (
        <Skeleton lines={6} label="Wczytywanie dziennika…" />
      ) : audit.isError && entries.length === 0 ? (
        <SectionError title="Nie udało się wczytać dziennika" error={audit.error} onRetry={() => void audit.refetch()} />
      ) : (
        <DataList
          label="Wpisy dziennika"
          items={entries}
          getKey={(entry) => entry.id}
          hasMore={audit.hasNextPage}
          onLoadMore={() => void audit.fetchNextPage()}
          loadingMore={audit.isFetchingNextPage}
          loadMoreLabel="Pokaż starsze"
          empty={
            <EmptyState icon="history" title="Brak wpisów" headingLevel={3}>
              <p>
                {filters.actor_kind || filters.resource_type
                  ? 'Żaden wpis nie pasuje do filtrów.'
                  : 'Tu pojawi się każda zmiana w gospodarstwie.'}
              </p>
            </EmptyState>
          }
          renderItem={(entry) => <AuditRow entry={entry} actor={actorName(entry)} />}
        />
      )}
    </section>
  )
}

function AuditRow({ entry, actor }: { entry: AuditEntry; actor: string }) {
  const changes = Object.entries(entry.changes)
  return (
    <div className={styles.audit}>
      <time className={styles.auditTime} dateTime={entry.at}>
        {formatDateTime(entry.at)}
      </time>
      <div className={styles.rowMain}>
        <span className={styles.rowTitle}>
          {actionLabel(entry.action)}
          <Badge>{resourceTypeLabel(entry.resource_type)}</Badge>
        </span>
        <span className={styles.rowMeta}>
          {entry.actor.kind === 'bot' ? 'Bot ' : ''}
          {actor}
        </span>
        {changes.length > 0 && (
          <details className={styles.changes}>
            <summary>Zmiany ({changes.length})</summary>
            <dl>
              {changes.map(([field, change]) => (
                <div key={field}>
                  <dt>{fieldLabel(field)}</dt>
                  <dd>
                    {change.from === null || change.from === undefined ? (
                      showValue(change.to)
                    ) : (
                      <>
                        <span className={styles.strike}>{showValue(change.from)}</span>
                        <span aria-label="zmienione na"> → </span>
                        {showValue(change.to)}
                      </>
                    )}
                  </dd>
                </div>
              ))}
            </dl>
          </details>
        )}
      </div>
    </div>
  )
}
