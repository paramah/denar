import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { usePayees } from '../../api/hooks/payees.ts'
import { useTransactions } from '../../api/hooks/transactions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { TransactionFilters } from '../../api/keys.ts'
import type { Transaction, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { kindLabels, statusLabels } from '../../lib/labels.ts'
import { useDebouncedValue } from '../../lib/useDebouncedValue.ts'
import { DeleteTransactionDialog, TransactionRow, transactionTitle } from './TransactionRow.tsx'
import { TransactionDialog } from './TransactionDialog.tsx'
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
  const { household, role } = useHousehold()
  const [params] = useSearchParams()
  const filters = filtersFromParams(params, walletId)
  const transactions = useTransactions(household.id, filters)
  const allWallets = useWallets(household.id, { includeArchived: true })
  const wallets = useMemo(() => new Map((allWallets.data ?? []).map((w) => [w.id, w])), [allWallets.data])
  const canWrite = role !== 'viewer'
  const [editing, setEditing] = useState<Transaction | null>(null)
  const [deleting, setDeleting] = useState<Transaction | null>(null)
  const filtered = Object.keys(filters).some((key) => key !== 'wallet_id' || !walletId)

  return (
    <section className={styles.register} aria-labelledby="register-heading">
      <h2 id="register-heading" className="visually-hidden">
        Rejestr transakcji
      </h2>
      <RegisterFilters walletId={walletId} wallets={allWallets.data ?? []} />
      <QueryState query={transactions} errorTitle="Nie udało się wczytać transakcji" loading={<Skeleton lines={5} />}>
        {(data) => (
          <DataList
            label="Transakcje"
            items={data.pages.flatMap((page) => page.items)}
            getKey={(t) => t.id}
            hasMore={transactions.hasNextPage}
            onLoadMore={() => void transactions.fetchNextPage()}
            loadingMore={transactions.isFetchingNextPage}
            empty={
              filtered ? (
                <EmptyState icon="search" title="Brak transakcji dla tych filtrów" headingLevel={3}>
                  <p>Zmień albo wyczyść filtry.</p>
                </EmptyState>
              ) : (
                <EmptyState
                  icon="list"
                  title="Nie ma jeszcze transakcji"
                  headingLevel={3}
                  action={
                    canWrite && onAdd ? (
                      <Button variant="primary" icon="plus" onClick={onAdd}>
                        Dodaj pierwszą transakcję
                      </Button>
                    ) : undefined
                  }
                >
                  <p>Zapisuj wydatki i wpływy — saldo portfela policzy się samo.</p>
                </EmptyState>
              )
            }
            renderItem={(t) => (
              <TransactionRow
                transaction={t}
                wallets={wallets}
                showWallet={!walletId}
                canWrite={canWrite}
                onEdit={() => setEditing(t)}
                onDelete={() => setDeleting(t)}
              />
            )}
          />
        )}
      </QueryState>
      <TransactionDialog open={!!editing} onClose={() => setEditing(null)} transaction={editing ?? undefined} />
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

function RegisterFilters({ walletId, wallets }: { walletId?: string; wallets: Wallet[] }) {
  const { household } = useHousehold()
  const payees = usePayees(household.id)
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

  return (
    <form role="search" aria-label="Filtry transakcji" className={styles.filters} onSubmit={(event) => event.preventDefault()}>
      <div className={styles.filterRow}>
        <TextField
          label="Szukaj"
          type="search"
          value={q}
          onChange={(event) => setQ(event.target.value)}
          placeholder="Notatka lub kontrahent"
          className={styles.filterSearch}
          maxLength={100}
        />
        {!walletId && (
          <SelectField
            label="Portfel"
            value={params.get('wallet') ?? ''}
            onChange={(event) => set({ wallet: event.target.value })}
            options={[
              { value: '', label: 'Wszystkie' },
              ...wallets.map((w) => ({ value: w.id, label: w.archived_at ? `${w.name} (archiwum)` : w.name })),
            ]}
          />
        )}
        <SelectField
          label="Kontrahent"
          value={params.get('payee') ?? ''}
          onChange={(event) => set({ payee: event.target.value })}
          options={[{ value: '', label: 'Wszyscy' }, ...(payees.data ?? []).map((p) => ({ value: p.id, label: p.name }))]}
        />
      </div>
      <div className={styles.filterRow}>
        <TextField label="Od dnia" type="date" value={params.get('from') ?? ''} onChange={(event) => set({ from: event.target.value })} />
        <TextField label="Do dnia" type="date" value={params.get('to') ?? ''} onChange={(event) => set({ to: event.target.value })} />
        <SelectField
          label="Rodzaj"
          value={params.get('kind') ?? ''}
          onChange={(event) => set({ kind: event.target.value })}
          options={[
            { value: '', label: 'Każdy' },
            ...(['expense', 'income', 'transfer'] as const).map((k) => ({ value: k, label: kindLabels[k] })),
          ]}
        />
        <SelectField
          label="Status"
          value={params.get('status') ?? ''}
          onChange={(event) => set({ status: event.target.value })}
          options={[
            { value: '', label: 'Każdy' },
            ...(['pending', 'cleared', 'reconciled'] as const).map((s) => ({ value: s, label: statusLabels[s] })),
          ]}
        />
        <TextField label="Tag" value={tag} onChange={(event) => setTag(event.target.value)} autoComplete="off" maxLength={40} />
      </div>
      {active.length > 0 && (
        <div className={styles.filterFoot}>
          <span aria-live="polite">Aktywne filtry: {active.length}</span>
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
            Wyczyść filtry
          </Button>
        </div>
      )}
    </form>
  )
}
