import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useCurrencies } from '../../api/hooks/households.ts'
import { useMembers } from '../../api/hooks/members.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import { useCreateWallet, useUpdateWallet } from '../../api/hooks/wallets.ts'
import type { Visibility, Wallet, WalletCreate, WalletType, WalletUpdate } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { Checkbox, SelectField, TextField } from '../../components/Field/Field.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { todayIn } from '../../lib/format.ts'
import { walletTypeLabels, walletTypeOrder } from '../../lib/labels.ts'
import { decimalsOf, parseAmountInput, toMinor } from '../../lib/money.ts'
import { AmountField } from '../transactions/fields.tsx'
import { pickError } from '../transactions/money.ts'
import styles from './wallets.module.css'

/** Czy zalogowany może zmienić widoczność i członków portfela. */
export function canChangeVisibility(wallet: Wallet, userId: string, role: Parameters<typeof canManage>[0]): boolean {
  return wallet.created_by === userId || (wallet.visibility === 'shared' && canManage(role))
}

type Props = {
  open: boolean
  onClose: () => void
  /** Edytowany portfel; bez niego — nowy. */
  wallet?: Wallet
  onSaved?: (wallet: Wallet) => void
}

export function WalletDialog({ open, onClose, wallet, onSaved }: Props) {
  const { t } = useTranslation('wallets')
  return (
    <Dialog
      open={open}
      title={wallet ? t('dialog.editTitle', { name: wallet.name }) : t('dialog.newTitle')}
      onClose={onClose}
      size="md"
    >
      <WalletForm wallet={wallet} onClose={onClose} onSaved={onSaved} />
    </Dialog>
  )
}

function WalletForm({ wallet, onClose, onSaved }: { wallet?: Wallet; onClose: () => void; onSaved?: (w: Wallet) => void }) {
  const { t } = useTranslation('wallets')
  const { household, role } = useHousehold()
  const { user } = useSessionUser()
  const currencies = useCurrencies()
  const members = useMembers(household.id)
  const create = useCreateWallet(household.id)
  const update = useUpdateWallet(household.id)
  const toast = useToast()
  const mutation = wallet ? update : create

  const [name, setName] = useState(wallet?.name ?? '')
  const [type, setType] = useState<WalletType>(wallet?.type ?? 'checking')
  const [currency, setCurrency] = useState(wallet?.currency ?? household.base_currency)
  const [opening, setOpening] = useState('')
  const [openedOn, setOpenedOn] = useState(() => todayIn(household.timezone))
  const [onBudget, setOnBudget] = useState(wallet?.on_budget ?? true)
  const [visibility, setVisibility] = useState<Visibility>(wallet?.visibility ?? 'shared')
  const [memberIds, setMemberIds] = useState<string[]>(wallet?.member_ids ?? [])
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})

  const creator = wallet?.created_by ?? user.id
  const visibilityEditable = !wallet || canChangeVisibility(wallet, user.id, role)
  const server = fieldErrors(mutation.error)
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'
  const decimals = decimalsOf(currency, currencies.data)

  const submit = async () => {
    const next: Record<string, string | undefined> = {}
    if (!name.trim()) {
      next.name = t('dialog.nameRequired')
    }
    let openingValue: string | null = null
    if (!wallet && opening.trim()) {
      const parsed = parseAmountInput(opening, decimals, { allowNegative: true, allowZero: true })
      if (parsed.ok) {
        openingValue = toMinor(parsed.value, decimals) === 0n ? null : parsed.value
      } else {
        next.opening_balance = parsed.error
      }
    }
    setErrors(next)
    if (Object.values(next).some(Boolean)) {
      return
    }
    const privacy =
      visibilityEditable ? { visibility, member_ids: visibility === 'private' ? memberIds.filter((id) => id !== creator) : [] } : {}
    try {
      let saved: Wallet
      if (wallet) {
        const body: WalletUpdate = { name: name.trim(), type, on_budget: onBudget, ...privacy }
        saved = await update.mutateAsync({ walletId: wallet.id, body })
        toast.show({ message: t('dialog.saved', { name: saved.name }) })
      } else {
        const body: WalletCreate = {
          name: name.trim(),
          type,
          currency,
          on_budget: onBudget,
          ...privacy,
          ...(openingValue ? { opening_balance: { amount: openingValue, currency }, opened_on: openedOn } : {}),
        }
        saved = await create.mutateAsync(body)
        toast.show({ message: t('dialog.added', { name: saved.name }) })
      }
      onSaved?.(saved)
      onClose()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <form
      noValidate
      aria-label={wallet ? t('dialog.editForm') : t('dialog.newTitle')}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert
        error={duplicate ? null : mutation.error}
        shownFields={['name', 'type', 'currency', 'on_budget', 'visibility', 'member_ids', 'opening_balance', 'opening_balance.amount', 'opened_on']}
      />
      <TextField
        label={t('dialog.name')}
        value={name}
        onChange={(event) => setName(event.target.value)}
        error={errors.name ?? (duplicate ? t('dialog.duplicate') : server.name)}
        maxLength={100}
        autoComplete="off"
        data-autofocus
        required
      />
      <div className={styles.pair}>
        <SelectField
          label={t('dialog.type')}
          value={type}
          onChange={(event) => setType(event.target.value as WalletType)}
          options={walletTypeOrder.map((wt) => ({ value: wt, label: walletTypeLabels[wt] }))}
          error={server.type}
        />
        {wallet ? (
          <TextField label={t('dialog.currency')} value={wallet.currency} readOnly hint={t('dialog.currencyLocked')} />
        ) : (
          <SelectField
            label={t('dialog.currency')}
            value={currency}
            onChange={(event) => setCurrency(event.target.value)}
            options={(currencies.data ?? [{ code: household.base_currency, decimals: 2 }]).map((c) => ({
              value: c.code,
              label: c.code === household.base_currency ? t('dialog.baseCurrency', { code: c.code }) : c.code,
            }))}
            error={server.currency}
          />
        )}
      </div>
      {!wallet && (
        <div className={styles.pair}>
          <AmountField
            label={t('dialog.opening')}
            value={opening}
            onChange={setOpening}
            currency={currency}
            hint={t('dialog.openingHint')}
            error={errors.opening_balance ?? pickError(server, 'opening_balance', 'opening_balance.amount')}
          />
          <TextField
            label={t('dialog.openedOn')}
            type="date"
            value={openedOn}
            onChange={(event) => setOpenedOn(event.target.value)}
            error={server.opened_on}
          />
        </div>
      )}
      <Checkbox
        label={t('dialog.onBudget')}
        checked={onBudget}
        onChange={(event) => setOnBudget(event.target.checked)}
        hint={t('dialog.onBudgetHint')}
      />
      {visibilityEditable && (
        <fieldset className={styles.fieldset}>
          <legend>{t('dialog.visibility')}</legend>
          <label className={styles.radio}>
            <input type="radio" name="visibility" checked={visibility === 'shared'} onChange={() => setVisibility('shared')} />
            <span>
              {t('dialog.shared')}
              <small>{t('dialog.sharedHint')}</small>
            </span>
          </label>
          <label className={styles.radio}>
            <input type="radio" name="visibility" checked={visibility === 'private'} onChange={() => setVisibility('private')} />
            <span>
              {t('dialog.private')}
              <small>{t('dialog.privateHint')}</small>
            </span>
          </label>
          {visibility === 'private' && (
            <ul className={styles.members} aria-label={t('dialog.members')}>
              {(members.data ?? []).map((member) => (
                <li key={member.user_id}>
                  <Checkbox
                    label={member.user_id === creator ? t('dialog.creator', { name: member.name }) : member.name}
                    checked={member.user_id === creator || memberIds.includes(member.user_id)}
                    disabled={member.user_id === creator}
                    onChange={(event) =>
                      setMemberIds((ids) =>
                        event.target.checked ? [...ids, member.user_id] : ids.filter((id) => id !== member.user_id),
                      )
                    }
                  />
                </li>
              ))}
            </ul>
          )}
          {server.member_ids && (
            <p role="alert" className={styles.tileNote}>
              {server.member_ids}
            </p>
          )}
        </fieldset>
      )}
      <div className={styles.footer}>
        <Button onClick={onClose}>{t('dialog.cancel')}</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {wallet ? t('dialog.saveChanges') : t('dialog.add')}
        </Button>
      </div>
    </form>
  )
}
