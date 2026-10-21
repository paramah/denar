import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { fieldErrors } from '../../api/errors.ts'
import {
  useChargeSuggestions,
  useMatchCharge,
  usePostCharge,
  useSkipCharge,
  useUnmatchCharge,
} from '../../api/hooks/subscriptions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { Charge, ChargePost, TransactionSuggestion, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDay } from '../../lib/format.ts'
import { amountForInput, formatMoney, parseAmountInput } from '../../lib/money.ts'
import { relativeDay } from '../../lib/subscriptions.ts'
import { AmountField } from '../transactions/fields.tsx'
import { pickError, useDecimals } from '../transactions/money.ts'
import { transactionTitle } from '../transactions/TransactionRow.tsx'
import { ChargeStatusBadge, coinName, useToday, useVendorNames, VendorCoin } from './parts.tsx'
import styles from './subscriptions.module.css'

export type ChargeModal = { kind: 'match' | 'post' | 'skip'; charge: Charge } | null

type RowProps = {
  charge: Charge
  canWrite: boolean
  /** Pokazuj subskrypcję (kolejka); na stronie subskrypcji zbędne. */
  showSubscription?: boolean
  onAction: (modal: ChargeModal) => void
}

/** Wiersz harmonogramu: termin, kwota, status i akcje zależne od statusu. */
export function ChargeRow({ charge, canWrite, showSubscription, onAction }: RowProps) {
  const { household } = useHousehold()
  const today = useToday()
  const vendors = useVendorNames()
  const unmatch = useUnmatchCharge(household.id)
  const toast = useToast()
  const { t } = useTranslation('subscriptions')
  const open = charge.status === 'expected' || charge.status === 'missed'
  const linked = charge.status === 'matched' || charge.status === 'posted'
  const past = charge.due_date < today
  const label = showSubscription
    ? t('charge.labelWithName', { name: charge.subscription_name, date: formatDay(charge.due_date) })
    : t('charge.label', { date: formatDay(charge.due_date) })

  return (
    <article className={styles.chargeRow} data-status={charge.status} data-past={past || undefined} aria-label={label}>
      <div className={styles.chargeWhen}>
        <time dateTime={charge.due_date}>{formatDay(charge.due_date)}</time>
        <span className={styles.muted}>{relativeDay(charge.due_date, today)}</span>
      </div>
      <div className={styles.chargeMain}>
        {showSubscription && (
          <Link to={householdPath(household.id, `subscriptions/${charge.subscription_id}`)} className={styles.chargeSub}>
            <VendorCoin vendor={charge.vendor} name={coinName(charge.vendor, vendors.get(charge.vendor) ?? '', charge.subscription_name)} size="sm" />
            {charge.subscription_name}
          </Link>
        )}
        <ChargeStatusBadge charge={charge} />
      </div>
      <div className={styles.chargeAmount}>
        <Money amount={charge.amount.amount} currency={charge.amount.currency} />
      </div>
      {canWrite && (
        <div className={styles.chargeActions}>
          {open && (
            <>
              <Button size="sm" variant={past ? 'primary' : 'secondary'} icon="link" onClick={() => onAction({ kind: 'match', charge })}>
                {t('charge.match')}
              </Button>
              <Button size="sm" onClick={() => onAction({ kind: 'post', charge })}>
                {t('charge.post')}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => onAction({ kind: 'skip', charge })}>
                {t('charge.skip')}
              </Button>
            </>
          )}
          {linked && (
            <Button
              size="sm"
              variant="ghost"
              busy={unmatch.isPending}
              onClick={() =>
                unmatch.mutate(charge.id, {
                  onSuccess: () => toast.show({ message: t('charge.unmatched', { date: formatDay(charge.due_date) }) }),
                  onError: () => toast.show({ message: t('charge.unmatchFailed'), kind: 'error' }),
                })
              }
            >
              {t('charge.unmatch')}
            </Button>
          )}
        </div>
      )}
    </article>
  )
}

/** Dialogi akcji obciążenia; stan trzyma strona (jeden naraz). */
export function ChargeDialogs({ modal, onClose, onChange }: { modal: ChargeModal; onClose: () => void; onChange: (m: ChargeModal) => void }) {
  if (!modal) {
    return null
  }
  if (modal.kind === 'match') {
    return <MatchDialog charge={modal.charge} onClose={onClose} onPost={() => onChange({ kind: 'post', charge: modal.charge })} />
  }
  if (modal.kind === 'post') {
    return <PostDialog charge={modal.charge} onClose={onClose} />
  }
  return <SkipDialog charge={modal.charge} onClose={onClose} />
}

function ChargeSummary({ charge }: { charge: Charge }) {
  const { t } = useTranslation('subscriptions')
  return (
    <p className={styles.chargeSummary}>
      <strong>{charge.subscription_name}</strong>, {t('charge.label', { date: formatDay(charge.due_date) })}:{' '}
      <Money amount={charge.amount.amount} currency={charge.amount.currency} />
    </p>
  )
}

/** Wynik dopasowania jako krótka skala (kolor nie jest jedynym nośnikiem — liczba obok). */
export function ScoreMeter({ score }: { score: number }) {
  const level = score >= 80 ? 'high' : score >= 50 ? 'mid' : 'low'
  const { t } = useTranslation('subscriptions')
  return (
    <span className={styles.score} data-level={level} title={t('score.title')}>
      <span className={styles.scoreBar} aria-hidden="true">
        <span ref={(el) => el?.style.setProperty('--score', `${score}%`)} />
      </span>
      {t('score.label', { score })}
    </span>
  )
}

/** Lista sugestii transakcji z przyciskiem potwierdzenia. */
export function SuggestionList({
  charge,
  limit = 5,
  onConfirmed,
  compact,
}: {
  charge: Charge
  limit?: number
  onConfirmed?: () => void
  compact?: boolean
}) {
  const { household } = useHousehold()
  const suggestions = useChargeSuggestions(household.id, charge.id, limit)
  const wallets = useWallets(household.id, { includeArchived: true })
  const walletMap = new Map<string, Wallet>((wallets.data ?? []).map((w) => [w.id, w]))
  const match = useMatchCharge(household.id)
  const toast = useToast()
  const { t } = useTranslation('subscriptions')
  const confirm = (item: TransactionSuggestion) =>
    match.mutate(
      { chargeId: charge.id, transactionId: item.transaction.id },
      {
        onSuccess: () => {
          toast.show({ message: t('suggestions.matched', { name: charge.subscription_name, date: formatDay(charge.due_date) }) })
          onConfirmed?.()
        },
        onError: () => toast.show({ message: t('suggestions.failed'), kind: 'error' }),
      },
    )
  return (
    <QueryState query={suggestions} loading={<Skeleton lines={2} />} errorTitle={t('suggestions.loadError')}>
      {(items) =>
        items.length === 0 ? (
          <p className={styles.muted}>
            {compact ? t('suggestions.noneCompact') : t('suggestions.none')}
          </p>
        ) : (
          <ul className={styles.suggestions} aria-label={t('suggestions.label', { name: charge.subscription_name, date: formatDay(charge.due_date) })}>
            {items.map((item) => {
              const tx = item.transaction
              const title = transactionTitle(tx, walletMap)
              return (
                <li key={tx.id} className={styles.suggestion}>
                  <div className={styles.suggestionText}>
                    <span className={styles.suggestionTitle}>{title}</span>
                    <span className={styles.muted}>
                      {formatDay(tx.date)}
                      {walletMap.get(tx.wallet_id) ? `, ${walletMap.get(tx.wallet_id)!.name}` : ''}
                    </span>
                  </div>
                  <Money amount={tx.amount.amount} currency={tx.amount.currency} tone="auto" />
                  <ScoreMeter score={item.score} />
                  <Button
                    size="sm"
                    variant="primary"
                    icon="check"
                    busy={match.isPending && match.variables?.transactionId === tx.id}
                    disabled={match.isPending}
                    onClick={() => confirm(item)}
                    aria-label={t('suggestions.confirmLabel', { title, date: formatDay(tx.date) })}
                  >
                    {t('suggestions.confirm')}
                  </Button>
                </li>
              )
            })}
          </ul>
        )
      }
    </QueryState>
  )
}

function MatchDialog({ charge, onClose, onPost }: { charge: Charge; onClose: () => void; onPost: () => void }) {
  const { t } = useTranslation('subscriptions')
  return (
    <Dialog open title={t('matchDialog.title')} onClose={onClose} size="lg">
      <div className={styles.dialogBody}>
        <ChargeSummary charge={charge} />
        <SuggestionList charge={charge} onConfirmed={onClose} />
        <Callout>
          <Trans
            t={t}
            i18nKey="matchDialog.notFound"
            components={{ post: <button type="button" className={styles.inlineLink} onClick={onPost} /> }}
          />
        </Callout>
      </div>
    </Dialog>
  )
}

function PostDialog({ charge, onClose }: { charge: Charge; onClose: () => void }) {
  const { household } = useHousehold()
  const wallets = useWallets(household.id)
  const decimalsFor = useDecimals()
  const post = usePostCharge(household.id)
  const toast = useToast()
  const { t } = useTranslation('subscriptions')
  const active = (wallets.data ?? []).filter((w) => !w.archived_at)
  const [walletId, setWalletId] = useState(charge.wallet_id ?? '')
  const chosen = active.find((w) => w.id === walletId)
  const sameCurrency = chosen?.currency === charge.amount.currency
  const [amount, setAmount] = useState(() => amountForInput(charge.amount.amount))
  const [amountTouched, setAmountTouched] = useState(false)
  const [date, setDate] = useState(charge.due_date)
  const [payee, setPayee] = useState('')
  const [note, setNote] = useState('')
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const server = fieldErrors(post.error)

  // Inna waluta portfela: kwota z wyciągu, nie kwota dostawcy.
  const shownAmount = chosen && !sameCurrency && !amountTouched ? '' : amount
  const currency = chosen?.currency ?? charge.amount.currency

  const submit = () => {
    const next: Record<string, string | undefined> = {}
    if (!chosen) next.wallet_id = t('postDialog.walletRequired')
    const parsed = parseAmountInput(shownAmount, decimalsFor(currency))
    if (!parsed.ok) next.amount = parsed.error
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) next.date = t('postDialog.dateRequired')
    setErrors(next)
    if (Object.values(next).some(Boolean) || !chosen || !parsed.ok) {
      return
    }
    const body: ChargePost = {
      wallet_id: chosen.id,
      amount: { amount: parsed.value, currency: chosen.currency },
      date,
      ...(payee.trim() ? { payee_name: payee.trim() } : {}),
      ...(note.trim() ? { note: note.trim() } : {}),
    }
    post.mutate(
      { chargeId: charge.id, body },
      {
        onSuccess: (result) => {
          toast.show({
            message: t('postDialog.posted', {
              amount: formatMoney(result.transaction.amount.amount, result.transaction.amount.currency) ?? '',
            }),
          })
          onClose()
        },
      },
    )
  }

  return (
    <Dialog open title={t('postDialog.title')} onClose={onClose} onSubmit={submit} size="md"
      footer={
        <>
          <Button onClick={onClose}>{t('actions.cancel')}</Button>
          <Button type="submit" variant="primary" busy={post.isPending}>
            {t('postDialog.submit')}
          </Button>
        </>
      }
    >
      <ChargeSummary charge={charge} />
      <p className={styles.muted}>{t('postDialog.intro')}</p>
      <SelectField
        label={t('postDialog.wallet')}
        value={walletId}
        onChange={(event) => setWalletId(event.target.value)}
        error={errors.wallet_id ?? pickError(server, 'wallet_id')}
        options={[{ value: '', label: t('postDialog.walletPlaceholder') }, ...active.map((w) => ({ value: w.id, label: `${w.name} (${w.currency})` }))]}
      />
      <AmountField
        label={t('postDialog.amount', { currency })}
        value={shownAmount}
        onChange={(value) => {
          setAmountTouched(true)
          setAmount(value)
        }}
        currency={currency}
        sign="−"
        data-autofocus
        error={errors.amount ?? pickError(server, 'amount', 'amount.amount')}
        hint={
          chosen && !sameCurrency
            ? t('postDialog.otherCurrency', { wallet: chosen.currency, charge: charge.amount.currency })
            : undefined
        }
      />
      <TextField label={t('postDialog.date')} type="date" value={date} onChange={(event) => setDate(event.target.value)} error={errors.date ?? pickError(server, 'date')} />
      <TextField label={t('postDialog.payee')} value={payee} onChange={(event) => setPayee(event.target.value)} hint={t('postDialog.payeeHint')} maxLength={100} autoComplete="off" />
      <TextField label={t('postDialog.note')} value={note} onChange={(event) => setNote(event.target.value)} maxLength={1000} />
      <FormAlert error={post.error} shownFields={['wallet_id', 'amount', 'amount.amount', 'date']} />
    </Dialog>
  )
}

function SkipDialog({ charge, onClose }: { charge: Charge; onClose: () => void }) {
  const { household } = useHousehold()
  const skip = useSkipCharge(household.id)
  const toast = useToast()
  const { t } = useTranslation('subscriptions')
  return (
    <ConfirmDialog
      open
      title={t('skipDialog.title')}
      confirmLabel={t('skipDialog.confirm')}
      danger={false}
      busy={skip.isPending}
      error={<FormAlert error={skip.error} />}
      onCancel={onClose}
      onConfirm={() =>
        skip.mutate(charge.id, {
          onSuccess: () => {
            toast.show({ message: t('skipDialog.skipped', { date: formatDay(charge.due_date) }) })
            onClose()
          },
        })
      }
    >
      <ChargeSummary charge={charge} />
      <p>{t('skipDialog.body')}</p>
    </ConfirmDialog>
  )
}
