import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
import { tk } from '../../i18n/translate.ts'
import { formatDateTime } from '../../lib/format.ts'
import { actionLabel, actorKindLabels, fieldLabel, resourceTypeLabel, resourceTypeLabels } from '../../lib/labels.ts'
import { cycleLabel } from '../../lib/subscriptions.ts'
import type { Cycle } from '../../api/types.ts'
import styles from './settings.module.css'

// Opcje liczone przy każdym renderze: etykiety są w aktywnym języku.
function actorOptions() {
  return [
    { value: '', label: tk('settings:audit.actorAll') },
    ...(Object.keys(actorKindLabels) as ActorKind[]).map((kind) => ({ value: kind, label: actorKindLabels[kind] })),
  ]
}

function resourceOptions() {
  return [
    { value: '', label: tk('settings:audit.resourceAll') },
    ...Object.entries(resourceTypeLabels).map(([value, label]) => ({ value, label })),
  ]
}

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
    return value ? tk('settings:audit.yes') : tk('settings:audit.no')
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
  const { t } = useTranslation('settings')
  useDocumentTitle(t('audit.title'))
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
      return t('audit.system')
    }
    const name = names.get(`${entry.actor.kind}:${entry.actor.id}`)
    return name ?? `${actorKindLabels[entry.actor.kind]} ${entry.actor.id.slice(-6)}`
  }

  const entries = audit.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <section className={styles.section} aria-labelledby="audit-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="audit-heading">{t('audit.title')}</h2>
          <p>{t('audit.lead')}</p>
        </div>
      </div>
      <div className={styles.filters} role="group" aria-label={t('audit.filters')}>
        <SelectField
          label={t('audit.actor')}
          value={filters.actor_kind ?? ''}
          options={actorOptions()}
          onChange={(event) =>
            setFilters({ ...filters, actor_kind: (event.target.value || undefined) as ActorKind | undefined })
          }
        />
        <SelectField
          label={t('audit.resource')}
          value={filters.resource_type ?? ''}
          options={resourceOptions()}
          onChange={(event) => setFilters({ ...filters, resource_type: event.target.value || undefined })}
        />
      </div>
      {audit.isPending ? (
        <Skeleton lines={6} label={t('audit.loading')} />
      ) : audit.isError && entries.length === 0 ? (
        <SectionError title={t('audit.loadFailed')} error={audit.error} onRetry={() => void audit.refetch()} />
      ) : (
        <DataList
          label={t('audit.listLabel')}
          items={entries}
          getKey={(entry) => entry.id}
          hasMore={audit.hasNextPage}
          onLoadMore={() => void audit.fetchNextPage()}
          loadingMore={audit.isFetchingNextPage}
          loadMoreLabel={t('audit.loadMore')}
          empty={
            <EmptyState icon="history" title={t('audit.empty.title')} headingLevel={3}>
              <p>
                {filters.actor_kind || filters.resource_type ? t('audit.empty.filtered') : t('audit.empty.body')}
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
  const { t } = useTranslation('settings')
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
          {entry.actor.kind === 'bot' ? t('audit.botPrefix') : ''}
          {actor}
        </span>
        {changes.length > 0 && (
          <details className={styles.changes}>
            <summary>{t('audit.changes', { count: changes.length })}</summary>
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
                        <span aria-label={t('audit.changedTo')}> → </span>
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
