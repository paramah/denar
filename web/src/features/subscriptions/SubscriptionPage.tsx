import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useCurrencies } from '../../api/hooks/households.ts'
import {
  useChangeSubscriptionPrice,
  useCharges,
  useDeleteSubscription,
  useSubscription,
  useSubscriptionPrices,
  useUpdateSubscription,
} from '../../api/hooks/subscriptions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { PricePoint, Subscription, SubscriptionStatus } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDay } from '../../lib/format.ts'
import { amountForInput, decimalsOf, parseAmountInput, toMinor, fromMinor } from '../../lib/money.ts'
import { cycleLabel, cycleNote, reminderLabel, relativeDay } from '../../lib/subscriptions.ts'
import { useEnvelopeNames } from '../transactions/envelopes.ts'
import { AmountField } from '../transactions/fields.tsx'
import { pickError } from '../transactions/money.ts'
import { ChargeDialogs, ChargeRow, type ChargeModal } from './charges.tsx'
import { BaseEstimate, coinName, SubscriptionStatusBadge, useToday, VendorCoin } from './parts.tsx'
import { SubscriptionDialog } from './SubscriptionDialog.tsx'
import styles from './subscriptions.module.css'

export function SubscriptionPage() {
  const { subscriptionId = '' } = useParams()
  const { household } = useHousehold()
  const subscription = useSubscription(household.id, subscriptionId)
  if (subscription.error instanceof ApiError && subscription.error.status === 404) {
    return (
      <EmptyState
        icon="search"
        title="Nie znaleziono subskrypcji"
        headingLevel={1}
        action={<LinkButton to={householdPath(household.id, 'subscriptions/list')}>Wszystkie subskrypcje</LinkButton>}
      >
        <p>Mogła zostać usunięta albo płacisz za nią z portfela, którego nie widzisz.</p>
      </EmptyState>
    )
  }
  return (
    <QueryState query={subscription} errorTitle="Nie udało się wczytać subskrypcji" loading={<Skeleton lines={6} />}>
      {(s) => <SubscriptionView subscription={s} />}
    </QueryState>
  )
}

type Modal = 'edit' | 'price' | 'cancel' | 'delete' | null

function SubscriptionView({ subscription: s }: { subscription: Subscription }) {
  const { household, role } = useHousehold()
  const canWrite = role !== 'viewer'
  const today = useToday()
  const update = useUpdateSubscription(household.id)
  const toast = useToast()
  const [modal, setModal] = useState<Modal>(null)
  const [chargeModal, setChargeModal] = useState<ChargeModal>(null)
  const inactive = s.status === 'paused' || s.status === 'cancelled'

  const setStatus = (status: SubscriptionStatus, message: string) =>
    update.mutate(
      { subscriptionId: s.id, body: { status } },
      {
        onSuccess: () => toast.show({ message }),
        onError: () => toast.show({ message: 'Nie udało się zmienić statusu.', kind: 'error' }),
      },
    )
  const resumeStatus: SubscriptionStatus = s.trial_ends_at && s.trial_ends_at >= today ? 'trial' : 'active'

  return (
    <>
      <Link to={householdPath(household.id, 'subscriptions/list')} className={styles.back}>
        <Icon name="chevronLeft" size={16} />
        Wszystkie subskrypcje
      </Link>
      <PageHeader
        title={s.name}
        description={
          <span className={styles.headerMeta}>
            <SubscriptionStatusBadge subscription={s} />
            {s.vendor !== 'other' && <span>{s.vendor_name}</span>}
          </span>
        }
        actions={
          canWrite && (
            <>
              <Button icon="edit" onClick={() => setModal('edit')}>
                Edytuj
              </Button>
              {s.status === 'paused' || s.status === 'cancelled' ? (
                <Button icon="play" busy={update.isPending} onClick={() => setStatus(resumeStatus, `Wznowiono subskrypcję ${s.name}.`)}>
                  Wznów
                </Button>
              ) : (
                <Button icon="pause" busy={update.isPending} onClick={() => setStatus('paused', `Wstrzymano subskrypcję ${s.name}.`)}>
                  Wstrzymaj
                </Button>
              )}
              {s.status !== 'cancelled' && (
                <Button icon="close" onClick={() => setModal('cancel')}>
                  Anuluj subskrypcję
                </Button>
              )}
              <Button icon="trash" variant="danger" onClick={() => setModal('delete')}>
                Usuń
              </Button>
            </>
          )
        }
      />
      <div className={styles.page}>
        <section className={styles.detailHero} aria-label="Cena i cykl">
          <VendorCoin vendor={s.vendor} name={coinName(s.vendor, s.vendor_name, s.name)} size="lg" />
          <div className={styles.detailPrice}>
            <Money amount={s.price.amount} currency={s.price.currency} size="xl" />
            <span className={styles.detailCycle}>{cycleLabel(s.cycle)}</span>
            {cycleNote(s.cycle) && <span className={styles.muted}>{cycleNote(s.cycle)}</span>}
          </div>
          <dl className={styles.detailFacts}>
            <div>
              <dt>Następna płatność</dt>
              <dd>
                {s.next_due && !inactive ? (
                  <>
                    {formatDay(s.next_due)} <span className={styles.muted}>({relativeDay(s.next_due, today)})</span>
                  </>
                ) : (
                  'brak'
                )}
              </dd>
            </div>
            <div>
              <dt>Średnio miesięcznie</dt>
              <dd>
                <Money amount={s.monthly.amount} currency={s.monthly.currency} /> <BaseEstimate value={s.monthly} />
              </dd>
            </div>
            <div>
              <dt>Rocznie</dt>
              <dd>
                <Money amount={s.yearly.amount} currency={s.yearly.currency} /> <BaseEstimate value={s.yearly} />
              </dd>
            </div>
          </dl>
        </section>
        {s.status === 'cancelled' && s.ends_at && (
          <Callout title="Subskrypcja anulowana">Dostęp do {formatDay(s.ends_at)}; kolejnych płatności nie będzie.</Callout>
        )}
        {s.status === 'paused' && <Callout title="Subskrypcja wstrzymana">Nie oczekujemy płatności, dopóki jej nie wznowisz.</Callout>}

        <div className={styles.columns}>
          <Details subscription={s} />
          <PriceHistory subscription={s} canWrite={canWrite} onChange={() => setModal('price')} />
        </div>
        <ChargeSchedule subscription={s} canWrite={canWrite} onAction={setChargeModal} />
      </div>

      <SubscriptionDialog open={modal === 'edit'} onClose={() => setModal(null)} subscription={s} />
      {modal === 'price' && <PriceDialog subscription={s} onClose={() => setModal(null)} />}
      {modal === 'cancel' && <CancelDialog subscription={s} onClose={() => setModal(null)} />}
      {modal === 'delete' && <DeleteDialog subscription={s} onClose={() => setModal(null)} />}
      <ChargeDialogs modal={chargeModal} onClose={() => setChargeModal(null)} onChange={setChargeModal} />
    </>
  )
}

function Details({ subscription: s }: { subscription: Subscription }) {
  const { household } = useHousehold()
  const wallets = useWallets(household.id, { includeArchived: true })
  const envelopes = useEnvelopeNames(household.id)
  const wallet = s.wallet_id ? wallets.data?.find((w) => w.id === s.wallet_id) : undefined
  const rule = s.match_rule
  return (
    <section className={styles.panel} aria-labelledby="details-heading">
      <h2 id="details-heading">Szczegóły</h2>
      <dl className={styles.facts}>
        <div>
          <dt>Portfel płatności</dt>
          <dd>
            {s.wallet_id ? (
              wallet ? (
                <Link to={householdPath(household.id, `wallets/${wallet.id}`)}>{wallet.name}</Link>
              ) : (
                'portfel'
              )
            ) : (
              'bez portfela — widzi ją całe gospodarstwo'
            )}
          </dd>
        </div>
        <div>
          <dt>Koperta</dt>
          <dd>{s.envelope_id ? (envelopes.get(s.envelope_id) ?? 'koperta') : 'bez koperty'}</dd>
        </div>
        {s.trial_ends_at && (
          <div>
            <dt>Okres próbny</dt>
            <dd>do {formatDay(s.trial_ends_at)}</dd>
          </div>
        )}
        <div>
          <dt>Przypomnienia</dt>
          <dd>{s.reminder_offsets.length > 0 ? [...s.reminder_offsets].sort((a, b) => a - b).map(reminderLabel).join(', ') : 'wyłączone'}</dd>
        </div>
        <div>
          <dt>Dopasowanie</dt>
          <dd>
            {rule.payee_pattern ? `„${rule.payee_pattern}”` : 'nazwa dostawcy i subskrypcji'}, kwota ±{rule.amount_tolerance_pct}%, data ±
            {rule.date_window_days} dni
          </dd>
        </div>
        {s.manage_url && (
          <div>
            <dt>Zarządzanie</dt>
            <dd>
              <a href={s.manage_url} target="_blank" rel="noopener noreferrer">
                {new URL(s.manage_url).host}
                <span className="visually-hidden"> (otwiera się w nowej karcie)</span>
              </a>
            </dd>
          </div>
        )}
        {s.note && (
          <div>
            <dt>Notatka</dt>
            <dd className={styles.note}>{s.note}</dd>
          </div>
        )}
      </dl>
    </section>
  )
}

/** Różnica między kolejnymi cenami tej samej waluty („+2,00 USD”); inaczej null. */
export function priceDelta(prev: PricePoint | undefined, next: PricePoint): { amount: string; currency: string } | null {
  if (!prev || prev.price.currency !== next.price.currency) {
    return null
  }
  const decimals = Math.max(decimalsOf(next.price.currency), prev.price.amount.split('.')[1]?.length ?? 0)
  const a = toMinor(prev.price.amount, decimals)
  const b = toMinor(next.price.amount, decimals)
  if (a === null || b === null || a === b) {
    return null
  }
  return { amount: fromMinor(b - a, decimals), currency: next.price.currency }
}

function PriceHistory({ subscription: s, canWrite, onChange }: { subscription: Subscription; canWrite: boolean; onChange: () => void }) {
  const { household } = useHousehold()
  const today = useToday()
  const prices = useSubscriptionPrices(household.id, s.id)
  return (
    <section className={styles.panel} aria-labelledby="prices-heading">
      <div className={styles.sectionHead}>
        <h2 id="prices-heading">Historia cen</h2>
        {canWrite && (
          <Button size="sm" icon="rate" onClick={onChange}>
            Zmień cenę
          </Button>
        )}
      </div>
      <QueryState query={prices} loading={<Skeleton lines={2} />} errorTitle="Nie udało się wczytać cen">
        {(items) => (
          <ol className={styles.priceList} aria-label="Ceny od najnowszej">
            {[...items].reverse().map((point, index, list) => {
              const delta = priceDelta(list[index + 1], point)
              const current = point.effective_from <= today && (index === 0 || list[index - 1]!.effective_from > today)
              return (
                <li key={point.id} className={styles.pricePoint} data-current={current || undefined} data-future={point.effective_from > today || undefined}>
                  <span>
                    od <time dateTime={point.effective_from}>{formatDay(point.effective_from)}</time>
                    {point.effective_from > today && <span className={styles.muted}> (zaplanowana)</span>}
                    {current && <span className={styles.muted}> (obowiązuje)</span>}
                  </span>
                  <Money amount={point.price.amount} currency={point.price.currency} />
                  {delta && <Money amount={delta.amount} currency={delta.currency} size="sm" tone="auto" className={styles.delta} />}
                </li>
              )
            })}
          </ol>
        )}
      </QueryState>
    </section>
  )
}

function ChargeSchedule({
  subscription: s,
  canWrite,
  onAction,
}: {
  subscription: Subscription
  canWrite: boolean
  onAction: (m: ChargeModal) => void
}) {
  const { household } = useHousehold()
  const charges = useCharges(household.id, { subscription_id: s.id })
  return (
    <section className={styles.panel} aria-labelledby="schedule-heading">
      <h2 id="schedule-heading">Harmonogram płatności</h2>
      <p className={styles.muted}>
        Oczekiwane płatności powstają na 90 dni naprzód. Dopasuj je do transakcji z rejestru, zaksięguj albo pomiń.
      </p>
      <QueryState query={charges} loading={<Skeleton lines={3} />} errorTitle="Nie udało się wczytać płatności">
        {(data) => (
          <DataList
            label="Płatności"
            items={data.pages.flatMap((page) => page.items)}
            getKey={(c) => c.id}
            hasMore={charges.hasNextPage}
            onLoadMore={() => void charges.fetchNextPage()}
            loadingMore={charges.isFetchingNextPage}
            empty={<p className={styles.muted}>Brak płatności — subskrypcja jest wstrzymana, anulowana albo jeszcze w okresie próbnym.</p>}
            renderItem={(charge) => <ChargeRow charge={charge} canWrite={canWrite} onAction={onAction} />}
          />
        )}
      </QueryState>
    </section>
  )
}

function PriceDialog({ subscription: s, onClose }: { subscription: Subscription; onClose: () => void }) {
  const { household } = useHousehold()
  const today = useToday()
  const currencies = useCurrencies()
  const change = useChangeSubscriptionPrice(household.id)
  const toast = useToast()
  const [from, setFrom] = useState(today)
  const [currency, setCurrency] = useState(s.price.currency)
  const [amount, setAmount] = useState(amountForInput(s.price.amount))
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const server = fieldErrors(change.error)

  const submit = () => {
    const next: Record<string, string | undefined> = {}
    const parsed = parseAmountInput(amount, decimalsOf(currency, currencies.data))
    if (!parsed.ok) next.amount = parsed.error
    if (!/^\d{4}-\d{2}-\d{2}$/.test(from)) next.effective_from = 'Podaj dzień, od którego obowiązuje cena.'
    setErrors(next)
    if (!parsed.ok || next.effective_from) {
      return
    }
    change.mutate(
      { subscriptionId: s.id, body: { effective_from: from, price: { amount: parsed.value, currency } } },
      {
        onSuccess: () => {
          toast.show({ message: `Zapisano nową cenę ${s.name} od ${formatDay(from)}.` })
          onClose()
        },
      },
    )
  }

  return (
    <Dialog
      open
      title={`Zmiana ceny: ${s.name}`}
      onClose={onClose}
      onSubmit={submit}
      footer={
        <>
          <Button onClick={onClose}>Anuluj</Button>
          <Button type="submit" variant="primary" busy={change.isPending}>
            Zapisz cenę
          </Button>
        </>
      }
    >
      <p className={styles.muted}>
        Teraz: <Money amount={s.price.amount} currency={s.price.currency} />. Nowa cena trafi do historii, a przyszłe płatności od tego dnia
        dostaną nową kwotę.
      </p>
      <TextField
        label="Obowiązuje od"
        type="date"
        value={from}
        onChange={(event) => setFrom(event.target.value)}
        error={errors.effective_from ?? pickError(server, 'effective_from')}
        hint="Ten sam dzień co istniejąca cena — poprawia ją."
      />
      <div className={styles.formGrid}>
        <AmountField
          label="Nowa cena"
          value={amount}
          onChange={setAmount}
          currency={currency}
          error={errors.amount ?? pickError(server, 'price', 'price.amount', 'amount')}
          data-autofocus
        />
        <SelectField
          label="Waluta"
          value={currency}
          onChange={(event) => setCurrency(event.target.value)}
          options={(currencies.data ?? [{ code: currency, decimals: 2 }]).map((c) => ({ value: c.code, label: c.code }))}
        />
      </div>
      <FormAlert error={change.error} shownFields={['effective_from', 'price', 'price.amount', 'amount']} />
    </Dialog>
  )
}

function CancelDialog({ subscription: s, onClose }: { subscription: Subscription; onClose: () => void }) {
  const { household } = useHousehold()
  const update = useUpdateSubscription(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={`Anulować ${s.name}?`}
      confirmLabel="Anuluj subskrypcję"
      busy={update.isPending}
      error={<FormAlert error={update.error} />}
      onCancel={onClose}
      onConfirm={() =>
        update.mutate(
          { subscriptionId: s.id, body: { status: 'cancelled' } },
          {
            onSuccess: () => {
              toast.show({ message: `Anulowano subskrypcję ${s.name}.` })
              onClose()
            },
          },
        )
      }
    >
      <p>
        Dostęp zostaje do końca opłaconego okresu{s.next_due ? ` (do ${formatDay(s.next_due)})` : ''}. Przyszłe płatności znikną z
        harmonogramu; historia zostaje.
      </p>
      <p>Pamiętaj, żeby anulować ją także u dostawcy.</p>
    </ConfirmDialog>
  )
}

function DeleteDialog({ subscription: s, onClose }: { subscription: Subscription; onClose: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteSubscription(household.id)
  const toast = useToast()
  const navigate = useNavigate()
  return (
    <ConfirmDialog
      open
      title={`Usunąć ${s.name}?`}
      confirmLabel="Usuń subskrypcję"
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onClose}
      onConfirm={() =>
        remove.mutate(s.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto subskrypcję ${s.name}.` })
            navigate(householdPath(household.id, 'subscriptions/list'))
          },
        })
      }
    >
      <p>Znikną historia cen i harmonogram płatności. Transakcje w rejestrze zostaną.</p>
      <p>Jeśli tylko przestajesz płacić, lepiej ją anulować — zostanie w historii.</p>
    </ConfirmDialog>
  )
}
