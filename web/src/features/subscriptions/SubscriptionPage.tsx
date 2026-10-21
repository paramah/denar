import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('subscriptions')
  if (subscription.error instanceof ApiError && subscription.error.status === 404) {
    return (
      <EmptyState
        icon="search"
        title={t('page.notFoundTitle')}
        headingLevel={1}
        action={<LinkButton to={householdPath(household.id, 'subscriptions/list')}>{t('page.all')}</LinkButton>}
      >
        <p>{t('page.notFoundBody')}</p>
      </EmptyState>
    )
  }
  return (
    <QueryState query={subscription} errorTitle={t('page.loadError')} loading={<Skeleton lines={6} />}>
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
  const { t } = useTranslation('subscriptions')
  const inactive = s.status === 'paused' || s.status === 'cancelled'

  const setStatus = (status: SubscriptionStatus, message: string) =>
    update.mutate(
      { subscriptionId: s.id, body: { status } },
      {
        onSuccess: () => toast.show({ message }),
        onError: () => toast.show({ message: t('page.statusFailed'), kind: 'error' }),
      },
    )
  const resumeStatus: SubscriptionStatus = s.trial_ends_at && s.trial_ends_at >= today ? 'trial' : 'active'

  return (
    <>
      <Link to={householdPath(household.id, 'subscriptions/list')} className={styles.back}>
        <Icon name="chevronLeft" size={16} />
        {t('page.all')}
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
                {t('page.edit')}
              </Button>
              {s.status === 'paused' || s.status === 'cancelled' ? (
                <Button icon="play" busy={update.isPending} onClick={() => setStatus(resumeStatus, t('page.resumed', { name: s.name }))}>
                  {t('page.resume')}
                </Button>
              ) : (
                <Button icon="pause" busy={update.isPending} onClick={() => setStatus('paused', t('page.paused', { name: s.name }))}>
                  {t('page.pause')}
                </Button>
              )}
              {s.status !== 'cancelled' && (
                <Button icon="close" onClick={() => setModal('cancel')}>
                  {t('page.cancel')}
                </Button>
              )}
              <Button icon="trash" variant="danger" onClick={() => setModal('delete')}>
                {t('page.delete')}
              </Button>
            </>
          )
        }
      />
      <div className={styles.page}>
        <section className={styles.detailHero} aria-label={t('page.hero')}>
          <VendorCoin vendor={s.vendor} name={coinName(s.vendor, s.vendor_name, s.name)} size="lg" />
          <div className={styles.detailPrice}>
            <Money amount={s.price.amount} currency={s.price.currency} size="xl" />
            <span className={styles.detailCycle}>{cycleLabel(s.cycle)}</span>
            {cycleNote(s.cycle) && <span className={styles.muted}>{cycleNote(s.cycle)}</span>}
          </div>
          <dl className={styles.detailFacts}>
            <div>
              <dt>{t('page.nextDue')}</dt>
              <dd>
                {s.next_due && !inactive ? (
                  <>
                    {formatDay(s.next_due)} <span className={styles.muted}>({relativeDay(s.next_due, today)})</span>
                  </>
                ) : (
                  t('page.none')
                )}
              </dd>
            </div>
            <div>
              <dt>{t('page.monthly')}</dt>
              <dd>
                <Money amount={s.monthly.amount} currency={s.monthly.currency} /> <BaseEstimate value={s.monthly} />
              </dd>
            </div>
            <div>
              <dt>{t('page.yearly')}</dt>
              <dd>
                <Money amount={s.yearly.amount} currency={s.yearly.currency} /> <BaseEstimate value={s.yearly} />
              </dd>
            </div>
          </dl>
        </section>
        {s.status === 'cancelled' && s.ends_at && (
          <Callout title={t('page.cancelledTitle')}>{t('page.cancelledBody', { date: formatDay(s.ends_at) })}</Callout>
        )}
        {s.status === 'paused' && <Callout title={t('page.pausedTitle')}>{t('page.pausedBody')}</Callout>}

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
  const { t } = useTranslation('subscriptions')
  const wallet = s.wallet_id ? wallets.data?.find((w) => w.id === s.wallet_id) : undefined
  const rule = s.match_rule
  return (
    <section className={styles.panel} aria-labelledby="details-heading">
      <h2 id="details-heading">{t('details.heading')}</h2>
      <dl className={styles.facts}>
        <div>
          <dt>{t('details.wallet')}</dt>
          <dd>
            {s.wallet_id ? (
              wallet ? (
                <Link to={householdPath(household.id, `wallets/${wallet.id}`)}>{wallet.name}</Link>
              ) : (
                t('details.walletUnknown')
              )
            ) : (
              t('details.noWallet')
            )}
          </dd>
        </div>
        <div>
          <dt>{t('details.envelope')}</dt>
          <dd>{s.envelope_id ? (envelopes.get(s.envelope_id) ?? t('details.envelopeUnknown')) : t('details.noEnvelope')}</dd>
        </div>
        {s.trial_ends_at && (
          <div>
            <dt>{t('details.trial')}</dt>
            <dd>{t('details.trialUntil', { date: formatDay(s.trial_ends_at) })}</dd>
          </div>
        )}
        <div>
          <dt>{t('details.reminders')}</dt>
          <dd>{s.reminder_offsets.length > 0 ? [...s.reminder_offsets].sort((a, b) => a - b).map(reminderLabel).join(', ') : t('details.remindersOff')}</dd>
        </div>
        <div>
          <dt>{t('details.matching')}</dt>
          <dd>
            {t('details.matchingValue', {
              pattern: rule.payee_pattern ? t('details.patternQuoted', { pattern: rule.payee_pattern }) : t('details.patternDefault'),
              tolerance: rule.amount_tolerance_pct,
              days: rule.date_window_days,
            })}
          </dd>
        </div>
        {s.manage_url && (
          <div>
            <dt>{t('details.manage')}</dt>
            <dd>
              <a href={s.manage_url} target="_blank" rel="noopener noreferrer">
                {new URL(s.manage_url).host}
                <span className="visually-hidden">{t('details.newTab')}</span>
              </a>
            </dd>
          </div>
        )}
        {s.note && (
          <div>
            <dt>{t('details.note')}</dt>
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
  const { t } = useTranslation('subscriptions')
  return (
    <section className={styles.panel} aria-labelledby="prices-heading">
      <div className={styles.sectionHead}>
        <h2 id="prices-heading">{t('prices.heading')}</h2>
        {canWrite && (
          <Button size="sm" icon="rate" onClick={onChange}>
            {t('prices.change')}
          </Button>
        )}
      </div>
      <QueryState query={prices} loading={<Skeleton lines={2} />} errorTitle={t('prices.loadError')}>
        {(items) => (
          <ol className={styles.priceList} aria-label={t('prices.label')}>
            {[...items].reverse().map((point, index, list) => {
              const delta = priceDelta(list[index + 1], point)
              const current = point.effective_from <= today && (index === 0 || list[index - 1]!.effective_from > today)
              return (
                <li key={point.id} className={styles.pricePoint} data-current={current || undefined} data-future={point.effective_from > today || undefined}>
                  <span>
                    {t('prices.from')} <time dateTime={point.effective_from}>{formatDay(point.effective_from)}</time>
                    {point.effective_from > today && <span className={styles.muted}>{t('prices.planned')}</span>}
                    {current && <span className={styles.muted}>{t('prices.current')}</span>}
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
  const { t } = useTranslation('subscriptions')
  return (
    <section className={styles.panel} aria-labelledby="schedule-heading">
      <h2 id="schedule-heading">{t('schedule.heading')}</h2>
      <p className={styles.muted}>{t('schedule.intro')}</p>
      <QueryState query={charges} loading={<Skeleton lines={3} />} errorTitle={t('schedule.loadError')}>
        {(data) => (
          <DataList
            label={t('schedule.label')}
            items={data.pages.flatMap((page) => page.items)}
            getKey={(c) => c.id}
            hasMore={charges.hasNextPage}
            onLoadMore={() => void charges.fetchNextPage()}
            loadingMore={charges.isFetchingNextPage}
            empty={<p className={styles.muted}>{t('schedule.empty')}</p>}
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
  const { t } = useTranslation('subscriptions')
  const [from, setFrom] = useState(today)
  const [currency, setCurrency] = useState(s.price.currency)
  const [amount, setAmount] = useState(() => amountForInput(s.price.amount))
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const server = fieldErrors(change.error)

  const submit = () => {
    const next: Record<string, string | undefined> = {}
    const parsed = parseAmountInput(amount, decimalsOf(currency, currencies.data))
    if (!parsed.ok) next.amount = parsed.error
    if (!/^\d{4}-\d{2}-\d{2}$/.test(from)) next.effective_from = t('priceDialog.effectiveRequired')
    setErrors(next)
    if (!parsed.ok || next.effective_from) {
      return
    }
    change.mutate(
      { subscriptionId: s.id, body: { effective_from: from, price: { amount: parsed.value, currency } } },
      {
        onSuccess: () => {
          toast.show({ message: t('priceDialog.saved', { name: s.name, date: formatDay(from) }) })
          onClose()
        },
      },
    )
  }

  return (
    <Dialog
      open
      title={t('priceDialog.title', { name: s.name })}
      onClose={onClose}
      onSubmit={submit}
      footer={
        <>
          <Button onClick={onClose}>{t('actions.cancel')}</Button>
          <Button type="submit" variant="primary" busy={change.isPending}>
            {t('priceDialog.save')}
          </Button>
        </>
      }
    >
      <p className={styles.muted}>
        <Trans t={t} i18nKey="priceDialog.now" components={{ money: <Money amount={s.price.amount} currency={s.price.currency} /> }} />
      </p>
      <TextField
        label={t('priceDialog.from')}
        type="date"
        value={from}
        onChange={(event) => setFrom(event.target.value)}
        error={errors.effective_from ?? pickError(server, 'effective_from')}
        hint={t('priceDialog.fromHint')}
      />
      <div className={styles.formGrid}>
        <AmountField
          label={t('priceDialog.newPrice')}
          value={amount}
          onChange={setAmount}
          currency={currency}
          error={errors.amount ?? pickError(server, 'price', 'price.amount', 'amount')}
          data-autofocus
        />
        <SelectField
          label={t('priceDialog.currency')}
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
  const { t } = useTranslation('subscriptions')
  return (
    <ConfirmDialog
      open
      title={t('cancelDialog.title', { name: s.name })}
      confirmLabel={t('cancelDialog.confirm')}
      busy={update.isPending}
      error={<FormAlert error={update.error} />}
      onCancel={onClose}
      onConfirm={() =>
        update.mutate(
          { subscriptionId: s.id, body: { status: 'cancelled' } },
          {
            onSuccess: () => {
              toast.show({ message: t('cancelDialog.done', { name: s.name }) })
              onClose()
            },
          },
        )
      }
    >
      <p>{s.next_due ? t('cancelDialog.bodyUntil', { date: formatDay(s.next_due) }) : t('cancelDialog.body')}</p>
      <p>{t('cancelDialog.vendor')}</p>
    </ConfirmDialog>
  )
}

function DeleteDialog({ subscription: s, onClose }: { subscription: Subscription; onClose: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteSubscription(household.id)
  const toast = useToast()
  const navigate = useNavigate()
  const { t } = useTranslation('subscriptions')
  return (
    <ConfirmDialog
      open
      title={t('deleteDialog.title', { name: s.name })}
      confirmLabel={t('deleteDialog.confirm')}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onClose}
      onConfirm={() =>
        remove.mutate(s.id, {
          onSuccess: () => {
            toast.show({ message: t('deleteDialog.done', { name: s.name }) })
            navigate(householdPath(household.id, 'subscriptions/list'))
          },
        })
      }
    >
      <p>{t('deleteDialog.body')}</p>
      <p>{t('deleteDialog.hint')}</p>
    </ConfirmDialog>
  )
}
