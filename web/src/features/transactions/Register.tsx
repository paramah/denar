import { useEffect, useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { usePayees } from '../../api/hooks/payees.ts'
import { useLinkedCharges } from '../../api/hooks/subscriptions.ts'
import { useTransactions } from '../../api/hooks/transactions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { TransactionFilters } from '../../api/keys.ts'
import type { Charge, Transaction, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { kindLabels, statusLabels } from '../../lib/labels.ts'
import { addDaysTo } from '../../lib/subscriptions.ts'
import { useDebouncedValue } from '../../lib/useDebouncedValue.ts'
import { LinkChargeDialog } from '../subscriptions/LinkChargeDialog.tsx'
import { DeleteTransactionDialog, TransactionRow, transactionTitle } from './TransactionRow.tsx'
import { TransactionDialog } from './TransactionDialog.tsx'
import { envelopesEnabled, useEnvelopeNames, useEnvelopeOptions } from './envelopes.ts'
import styles from './transactions.module.css'

/** Parametry adresu → filtry API. */
const paramNames = {
  wallet: 'wallet_id',
  from: 'from',
  to: 'to',
  payee: 'payee_id',
  q: 'q',
  kind: 'kind',
  status: 'status',
  tag: 'tag',
  envelope: 'envelope_id',
} as const

type Param = keyof typeof paramNames

export function filtersFromParams(params: URLSearchParams, walletId?: string): TransactionFilters {
  const filters: TransactionFilters = {}
  for (const [param, field] of Object.entries(paramNames) as [Param, keyof TransactionFilters][]) {
    const value = params.get(param)?.trim()
    if (value) {
      filters[field] = value
    }
  }
  if (walletId) {
    filters.wallet_id = walletId
  }
  return filters
}

type Props = {
  /** Rejestr jednego portfela (bez filtra portfela). */
  walletId?: string
  /** Akcja pustego stanu bez filtrów. */
  onAdd?: () => void
}

/** Rejestr transakcji z filtrami w adresie i stronicowaniem kursorem. */
export function Register({ walletId, onAdd }: Props) {
  const { t } = useTranslation('transactions')
  const { household, role } = useHousehold()
  const [params] = useSearchParams()
  const filters = filtersFromParams(params, walletId)
  const transactions = useTransactions(household.id, filters)
  const allWallets = useWallets(household.id, { includeArchived: true })
  const wallets = useMemo(() => new Map((allWallets.data ?? []).map((w) => [w.id, w])), [allWallets.data])
  const canWrite = role !== 'viewer'
  const [editing, setEditing] = useState<Transaction | null>(null)
  const [deleting, setDeleting] = useState<Transaction | null>(null)
  const envelopeNames = useEnvelopeNames(household.id)
  const [linking, setLinking] = useState<Transaction | null>(null)
  const linked = useLinkedChargeMap(transactions.data?.pages.flatMap((page) => page.items) ?? [])
  const filtered = Object.keys(filters).some((key) => key !== 'wallet_id' || !walletId)

  return (
    <section className={styles.register} aria-labelledby="register-heading">
      <h2 id="register-heading" className="visually-hidden">
        {t('register.heading')}
      </h2>
      <RegisterFilters walletId={walletId} wallets={allWallets.data ?? []} />
      <QueryState query={transactions} errorTitle={t('register.loadFailed')} loading={<Skeleton lines={5} />}>
        {(data) => (
          <DataList
            label={t('register.label')}
            items={data.pages.flatMap((page) => page.items)}
            getKey={(tx) => tx.id}
            hasMore={transactions.hasNextPage}
            onLoadMore={() => void transactions.fetchNextPage()}
            loadingMore={transactions.isFetchingNextPage}
            empty={
              filtered ? (
                <EmptyState icon="search" title={t('register.noMatchTitle')} headingLevel={3}>
                  <p>{t('register.noMatchBody')}</p>
                </EmptyState>
              ) : (
                <EmptyState
                  icon="list"
                  title={t('register.emptyTitle')}
                  headingLevel={3}
                  action={
                    canWrite && onAdd ? (
                      <Button variant="primary" icon="plus" onClick={onAdd}>
                        {t('register.addFirst')}
                      </Button>
                    ) : undefined
                  }
                >
                  <p>{t('register.emptyBody')}</p>
                </EmptyState>
              )
            }
            renderItem={(tx) => (
              <TransactionRow
                transaction={tx}
                wallets={wallets}
                showWallet={!walletId}
                canWrite={canWrite}
                onEdit={() => setEditing(tx)}
                onDelete={() => setDeleting(tx)}
                envelopes={envelopeNames}
                charge={linked.get(tx.id)}
                onLinkCharge={() => setLinking(tx)}
              />
            )}
          />
        )}
      </QueryState>
      <TransactionDialog open={!!editing} onClose={() => setEditing(null)} transaction={editing ?? undefined} />
      {linking && (
        <LinkChargeDialog transaction={linking} title={transactionTitle(linking, wallets)} onClose={() => setLinking(null)} />
      )}
      {deleting && (
        <DeleteTransactionDialog
          transaction={deleting}
          title={transactionTitle(deleting, wallets)}
          onDone={() => setDeleting(null)}
        />
      )}
    </section>
  )
}

/**
 * Obciążenia subskrypcji powiązane z widocznymi transakcjami (po id
 * transakcji). API transakcji nie zwraca obciążenia, więc pytamy o dopasowane
 * i zaksięgowane z okna dat widocznych wierszy (±31 dni na rozjazd terminu).
 */
function useLinkedChargeMap(items: Transaction[]): Map<string, Charge> {
  const { household } = useHousehold()
  const dates = items.map((t) => t.date).sort()
  const from = dates.length ? addDaysTo(dates[0]!, -31) : ''
  const to = dates.length ? addDaysTo(dates[dates.length - 1]!, 31) : ''
  const charges = useLinkedCharges(household.id, from, to)
  return useMemo(
    () => new Map((charges.data ?? []).flatMap((c) => (c.transaction_id ? [[c.transaction_id, c] as const] : []))),
    [charges.data],
  )
}

function RegisterFilters({ walletId, wallets }: { walletId?: string; wallets: Wallet[] }) {
  const { t } = useTranslation('transactions')
  const { household } = useHousehold()
  const payees = usePayees(household.id)
  const envelopeOptions = useEnvelopeOptions(household.id)
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState(params.get('q') ?? '')
  const [tag, setTag] = useState(params.get('tag') ?? '')
  const debouncedQ = useDebouncedValue(q)
  const debouncedTag = useDebouncedValue(tag)

  const set = (patch: Partial<Record<Param, string>>) => {
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        for (const [name, value] of Object.entries(patch)) {
          if (value) {
            next.set(name, value)
          } else {
            next.delete(name)
          }
        }
        return next
      },
      { replace: true },
    )
  }

  useEffect(() => {
    if ((params.get('q') ?? '') !== debouncedQ.trim()) {
      set({ q: debouncedQ.trim() })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tylko po zmianie wpisanego tekstu
  }, [debouncedQ])

  useEffect(() => {
    if ((params.get('tag') ?? '') !== debouncedTag.trim()) {
      set({ tag: debouncedTag.trim() })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tylko po zmianie wpisanego tekstu
  }, [debouncedTag])

  const active = (Object.keys(paramNames) as Param[]).filter((name) => params.get(name) && !(walletId && name === 'wallet'))
  const [expanded, setExpanded] = useState(false)
  const advancedId = useId()

  return (
    <form role="search" aria-label={t('filters.label')} className={styles.filters} onSubmit={(event) => event.preventDefault()}>
      <div className={styles.filterRow}>
        <TextField
          label={t('filters.search')}
          type="search"
          value={q}
          onChange={(event) => setQ(event.target.value)}
          placeholder={t('filters.searchPlaceholder')}
          className={styles.filterSearch}
          maxLength={100}
        />
        <Button
          className={styles.filterToggle}
          icon="search"
          aria-expanded={expanded}
          aria-controls={advancedId}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? t('filters.less') : t('filters.more')}
          {active.filter((name) => name !== 'q').length > 0 && ` (${active.filter((name) => name !== 'q').length})`}
        </Button>
      </div>
      <div id={advancedId} className={styles.advanced} data-expanded={expanded}>
        <div className={styles.filterRow}>
          {!walletId && (
            <SelectField
              label={t('filters.wallet')}
              value={params.get('wallet') ?? ''}
              onChange={(event) => set({ wallet: event.target.value })}
              options={[
                { value: '', label: t('filters.allWallets') },
                ...wallets.map((w) => ({
                  value: w.id,
                  label: w.archived_at ? t('filters.archivedWallet', { name: w.name }) : w.name,
                })),
              ]}
            />
          )}
          <SelectField
            label={t('filters.payee')}
            value={params.get('payee') ?? ''}
            onChange={(event) => set({ payee: event.target.value })}
            options={[{ value: '', label: t('filters.allPayees') }, ...(payees.data ?? []).map((p) => ({ value: p.id, label: p.name }))]}
          />
          {envelopesEnabled() && (
            <SelectField
              label={t('filters.envelope')}
              value={params.get('envelope') ?? ''}
              onChange={(event) => set({ envelope: event.target.value })}
              options={[{ value: '', label: t('filters.allEnvelopes') }, ...envelopeOptions]}
            />
          )}
        </div>
        <div className={styles.filterRow}>
          <TextField label={t('filters.from')} type="date" value={params.get('from') ?? ''} onChange={(event) => set({ from: event.target.value })} />
          <TextField label={t('filters.to')} type="date" value={params.get('to') ?? ''} onChange={(event) => set({ to: event.target.value })} />
          <SelectField
            label={t('filters.kind')}
            value={params.get('kind') ?? ''}
            onChange={(event) => set({ kind: event.target.value })}
            options={[
              { value: '', label: t('filters.any') },
              ...(['expense', 'income', 'transfer'] as const).map((k) => ({ value: k, label: kindLabels[k] })),
            ]}
          />
          <SelectField
            label={t('filters.status')}
            value={params.get('status') ?? ''}
            onChange={(event) => set({ status: event.target.value })}
            options={[
              { value: '', label: t('filters.any') },
              ...(['pending', 'cleared', 'reconciled'] as const).map((s) => ({ value: s, label: statusLabels[s] })),
            ]}
          />
          <TextField label={t('filters.tag')} value={tag} onChange={(event) => setTag(event.target.value)} autoComplete="off" maxLength={40} />
        </div>
      </div>
      {active.length > 0 && (
        <div className={styles.filterFoot}>
          <span aria-live="polite">{t('filters.active', { n: active.length })}</span>
          <Button
            size="sm"
            variant="ghost"
            icon="close"
            onClick={() => {
              setQ('')
              setTag('')
              set(Object.fromEntries(active.map((name) => [name, ''])))
            }}
          >
            {t('filters.clear')}
          </Button>
        </div>
      )}
    </form>
  )
}
