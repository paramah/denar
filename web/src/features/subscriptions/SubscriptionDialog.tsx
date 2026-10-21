import { useId, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useCurrencies } from '../../api/hooks/households.ts'
import { useCreateSubscription, useUpdateSubscription, useVendors } from '../../api/hooks/subscriptions.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { CycleUnit, Subscription, SubscriptionCreate, SubscriptionUpdate, Vendor } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { Checkbox, Field, inputClass, SelectField, TextField } from '../../components/Field/Field.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDay } from '../../lib/format.ts'
import { amountForInput, parseAmountInput } from '../../lib/money.ts'
import {
  cycleLabel,
  cycleNote,
  isHttpUrl,
  maxReminders,
  nextDues,
  plural,
  reminderChoices,
  reminderLabel,
} from '../../lib/subscriptions.ts'
import { envelopesEnabled, useEnvelopeOptions } from '../transactions/envelopes.ts'
import { AmountField, EnvelopeField, TextAreaField } from '../transactions/fields.tsx'
import { pickError, useDecimals } from '../transactions/money.ts'
import { useToday } from './parts.tsx'
import styles from './subscriptions.module.css'

type Props = {
  open: boolean
  onClose: () => void
  /** Edytowana subskrypcja; bez niej — nowa. */
  subscription?: Subscription
}

export function SubscriptionDialog({ open, onClose, subscription }: Props) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={subscription ? `Edycja: ${subscription.name}` : 'Nowa subskrypcja'}
      size="lg"
    >
      <SubscriptionForm onClose={onClose} subscription={subscription} />
    </Dialog>
  )
}

const units: CycleUnit[] = ['month', 'year', 'week', 'day']
const unitForms: Record<CycleUnit, [string, string, string]> = {
  day: ['dzień', 'dni', 'dni'],
  week: ['tydzień', 'tygodnie', 'tygodni'],
  month: ['miesiąc', 'miesiące', 'miesięcy'],
  year: ['rok', 'lata', 'lat'],
}

/** Dostawca po nazwie lub kluczu (bez wielkości liter). */
export function findVendor(vendors: Vendor[], text: string): Vendor | undefined {
  const wanted = text.trim().toLocaleLowerCase('pl')
  return wanted ? vendors.find((v) => v.name.toLocaleLowerCase('pl') === wanted || v.key === wanted) : undefined
}

type Draft = {
  name: string
  vendor: string
  amount: string
  currency: string
  unit: CycleUnit
  every: string
  anchor: string
  trial: boolean
  trialEndsAt: string
  walletId: string
  envelopeId: string
  reminders: number[]
  payeePattern: string
  tolerance: string
  window: string
  note: string
  manageUrl: string
}

function initialDraft(subscription: Subscription | undefined, vendors: Vendor[], today: string, base: string): Draft {
  if (!subscription) {
    return {
      name: '',
      vendor: '',
      amount: '',
      currency: base,
      unit: 'month',
      every: '1',
      anchor: today,
      trial: false,
      trialEndsAt: '',
      walletId: '',
      envelopeId: '',
      reminders: [3],
      payeePattern: '',
      tolerance: '10',
      window: '3',
      note: '',
      manageUrl: '',
    }
  }
  return {
    name: subscription.name,
    vendor: vendors.find((v) => v.key === subscription.vendor)?.name ?? subscription.vendor_name,
    amount: amountForInput(subscription.price.amount),
    currency: subscription.price.currency,
    unit: subscription.cycle.unit,
    every: String(subscription.cycle.every),
    anchor: subscription.cycle.anchor,
    trial: !!subscription.trial_ends_at,
    trialEndsAt: subscription.trial_ends_at ?? '',
    walletId: subscription.wallet_id ?? '',
    envelopeId: subscription.envelope_id ?? '',
    reminders: [...subscription.reminder_offsets],
    payeePattern: subscription.match_rule.payee_pattern,
    tolerance: String(subscription.match_rule.amount_tolerance_pct),
    window: String(subscription.match_rule.date_window_days),
    note: subscription.note,
    manageUrl: subscription.manage_url ?? '',
  }
}

const isDay = (value: string) => /^\d{4}-\d{2}-\d{2}$/.test(value)

function intIn(text: string, min: number, max: number): number | null {
  if (!/^\d+$/.test(text.trim())) {
    return null
  }
  const n = Number(text.trim())
  return n >= min && n <= max ? n : null
}

function SubscriptionForm({ onClose, subscription }: { onClose: () => void; subscription?: Subscription }) {
  const { household } = useHousehold()
  const hid = household.id
  const today = useToday()
  const vendors = useVendors()
  const currencies = useCurrencies()
  const wallets = useWallets(hid)
  const envelopeOptions = useEnvelopeOptions(hid)
  const decimalsFor = useDecimals()
  const create = useCreateSubscription(hid)
  const update = useUpdateSubscription(hid)
  const toast = useToast()
  const navigate = useNavigate()
  const vendorList = useId()
  const editing = !!subscription

  const [draft, setDraft] = useState<Draft>(() => initialDraft(subscription, vendors.data ?? [], today, household.base_currency))
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft((d) => ({ ...d, [key]: value }))

  const mutation = editing ? update : create
  const server = fieldErrors(mutation.error)
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'
  const error = (field: string, ...serverFields: string[]) =>
    errors[field] ?? pickError(server, field, ...serverFields) ?? (field === 'name' && duplicate ? 'Masz już subskrypcję o tej nazwie.' : undefined)

  const every = intIn(draft.every, 1, 365)
  const preview = every && isDay(draft.anchor) ? { unit: draft.unit, every, anchor: draft.anchor } : null
  const upcoming = preview ? nextDues(preview, today, 3) : []
  const activeWallets = (wallets.data ?? []).filter((w) => !w.archived_at || w.id === draft.walletId)
  const reminderOptions = [...new Set([...reminderChoices, ...draft.reminders])].sort((a, b) => a - b)

  const onVendor = (text: string) => {
    const before = findVendor(vendors.data ?? [], draft.vendor)
    const after = findVendor(vendors.data ?? [], text)
    setDraft((d) => ({
      ...d,
      vendor: text,
      // Nazwa idzie za dostawcą, dopóki użytkownik jej nie zmienił.
      name: after && after.key !== 'other' && (d.name === '' || d.name === before?.name) ? after.name : d.name,
    }))
  }

  const toggleReminder = (days: number) =>
    setDraft((d) => ({
      ...d,
      reminders: d.reminders.includes(days) ? d.reminders.filter((x) => x !== days) : [...d.reminders, days],
    }))

  const submit = async () => {
    const next: Record<string, string | undefined> = {}
    const name = draft.name.trim()
    if (!name) next.name = 'Podaj nazwę.'
    else if (name.length > 100) next.name = 'Najwyżej 100 znaków.'
    const vendor = findVendor(vendors.data ?? [], draft.vendor)
    if (!vendor) next.vendor = 'Wybierz dostawcę z listy — albo „Inne”.'
    const price = parseAmountInput(draft.amount, decimalsFor(draft.currency))
    if (!editing && !price.ok) next.price = price.error
    if (!every) next.every = 'Od 1 do 365.'
    if (!isDay(draft.anchor)) next.anchor = 'Podaj datę płatności.'
    if (draft.trial && !isDay(draft.trialEndsAt)) next.trial_ends_at = 'Podaj koniec okresu próbnego.'
    if (draft.reminders.length > maxReminders) next.reminder_offsets = `Najwyżej ${maxReminders} przypomnień.`
    const tolerance = intIn(draft.tolerance, 0, 50)
    if (tolerance === null) next.tolerance = 'Od 0 do 50 %.'
    const window = intIn(draft.window, 0, 15)
    if (window === null) next.window = 'Od 0 do 15 dni.'
    const url = draft.manageUrl.trim()
    if (url && !isHttpUrl(url)) next.manage_url = 'Adres musi zaczynać się od http:// lub https://.'
    setErrors(next)
    if (Object.values(next).some(Boolean) || !vendor || !every || tolerance === null || window === null) {
      return
    }
    const reminders = [...draft.reminders].sort((a, b) => b - a)
    const match = { payee_pattern: draft.payeePattern.trim(), amount_tolerance_pct: tolerance, date_window_days: window }
    const cycle = { unit: draft.unit, every, anchor: draft.anchor }
    try {
      if (!subscription) {
        const body: SubscriptionCreate = {
          name,
          vendor: vendor.key,
          price: { amount: price.ok ? price.value : '0', currency: draft.currency },
          cycle,
          reminder_offsets: reminders,
          match_rule: match,
          ...(draft.trial ? { trial_ends_at: draft.trialEndsAt } : {}),
          ...(draft.walletId ? { wallet_id: draft.walletId } : {}),
          ...(draft.envelopeId ? { envelope_id: draft.envelopeId } : {}),
          ...(draft.note.trim() ? { note: draft.note.trim() } : {}),
          ...(url ? { manage_url: url } : {}),
        }
        const created = await create.mutateAsync(body)
        toast.show({ message: `Dodano subskrypcję ${created.name}.` })
        onClose()
        navigate(householdPath(hid, `subscriptions/${created.id}`))
        return
      }
      const body = changedFields(subscription, {
        name,
        vendor: vendor.key,
        cycle,
        trial_ends_at: draft.trial ? draft.trialEndsAt : null,
        wallet_id: draft.walletId || null,
        envelope_id: draft.envelopeId || null,
        reminder_offsets: reminders,
        match_rule: match,
        note: draft.note.trim(),
        manage_url: url || null,
      })
      if (Object.keys(body).length > 0) {
        await update.mutateAsync({ subscriptionId: subscription.id, body })
      }
      toast.show({ message: `Zapisano subskrypcję ${name}.` })
      onClose()
    } catch {
      // Błąd pokazuje formularz (mutation.error).
    }
  }

  return (
    <form
      noValidate
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <div className={styles.formGrid}>
        <Field label="Dostawca" error={error('vendor')} hint={editing ? undefined : 'Wpisz nazwę albo wybierz z listy.'}>
          {(control) => (
            <>
              <input
                className={inputClass}
                list={vendorList}
                value={draft.vendor}
                onChange={(event) => onVendor(event.target.value)}
                autoComplete="off"
                data-autofocus
                {...control}
              />
              <datalist id={vendorList}>
                {(vendors.data ?? []).map((v) => (
                  <option key={v.key} value={v.name} />
                ))}
              </datalist>
            </>
          )}
        </Field>
        <TextField
          label="Nazwa"
          value={draft.name}
          onChange={(event) => set('name', event.target.value)}
          error={error('name')}
          hint="Np. plan usługi — musi być inna niż pozostałych subskrypcji."
          maxLength={100}
          autoComplete="off"
        />
      </div>

      {editing && subscription ? (
        <div className={styles.priceNow}>
          <span>Cena teraz</span>
          <Money amount={subscription.price.amount} currency={subscription.price.currency} size="lg" />
          <span className={styles.muted}>Zmianę ceny od wybranego dnia zapiszesz na stronie subskrypcji.</span>
        </div>
      ) : (
        <div className={styles.formGrid}>
          <AmountField
            label="Cena"
            value={draft.amount}
            onChange={(value) => set('amount', value)}
            currency={draft.currency}
            error={error('price', 'price.amount', 'amount')}
            hint="Tyle, ile pobiera dostawca, w jego walucie."
          />
          <SelectField
            label="Waluta"
            value={draft.currency}
            onChange={(event) => set('currency', event.target.value)}
            options={(currencies.data ?? [{ code: draft.currency, decimals: 2 }]).map((c) => ({ value: c.code, label: c.code }))}
            hint={draft.currency !== household.base_currency ? `Koszty przeliczymy na ${household.base_currency}.` : undefined}
          />
        </div>
      )}

      <fieldset className={styles.fieldset}>
        <legend>Cykl płatności</legend>
        <div className={styles.cycleRow}>
          <TextField
            label="Co ile"
            inputMode="numeric"
            value={draft.every}
            onChange={(event) => set('every', event.target.value)}
            error={error('every', 'cycle.every')}
            className={styles.everyField}
          />
          <SelectField
            label="Jednostka"
            value={draft.unit}
            onChange={(event) => set('unit', event.target.value as CycleUnit)}
            error={pickError(server, 'cycle.unit')}
            options={units.map((unit) => ({ value: unit, label: plural(every ?? 1, ...unitForms[unit]) }))}
          />
          <TextField
            label="Dzień płatności"
            type="date"
            value={draft.anchor}
            onChange={(event) => set('anchor', event.target.value)}
            error={error('anchor', 'cycle.anchor')}
            hint="Dowolny dzień, w którym przychodzi obciążenie — kolejne liczymy od niego."
          />
        </div>
        <div className={styles.cyclePreview} aria-live="polite">
          {preview ? (
            <>
              <p className={styles.cycleText}>
                <span className="visually-hidden">Cykl: </span>
                {cycleLabel(preview)}
              </p>
              {cycleNote(preview) && <p className={styles.muted}>{cycleNote(preview)}</p>}
              {upcoming.length > 0 && (
                <p className={styles.muted}>Najbliższe płatności: {upcoming.map(formatDay).join(', ')}</p>
              )}
            </>
          ) : (
            <p className={styles.muted}>Uzupełnij cykl, żeby zobaczyć najbliższe płatności.</p>
          )}
        </div>
      </fieldset>

      <div className={styles.formGrid}>
        <div className={styles.stack}>
          <Checkbox
            label="Okres próbny"
            hint="Płatności zaczną się po jego końcu."
            checked={draft.trial}
            onChange={(event) => set('trial', event.target.checked)}
          />
          {draft.trial && (
            <TextField
              label="Koniec okresu próbnego"
              type="date"
              value={draft.trialEndsAt}
              onChange={(event) => set('trialEndsAt', event.target.value)}
              error={error('trial_ends_at')}
            />
          )}
        </div>
        <SelectField
          label="Portfel płatności"
          value={draft.walletId}
          onChange={(event) => set('walletId', event.target.value)}
          error={error('wallet_id')}
          hint={draft.walletId ? 'Widzą ją tylko osoby, które widzą ten portfel.' : 'Bez portfela widzi ją całe gospodarstwo.'}
          options={[
            { value: '', label: 'Bez portfela' },
            ...activeWallets.map((w) => ({ value: w.id, label: `${w.name} (${w.currency})` })),
          ]}
        />
        {envelopesEnabled() && (
          <EnvelopeField
            value={draft.envelopeId}
            onChange={(value) => set('envelopeId', value)}
            options={envelopeOptions}
            error={error('envelope_id')}
          />
        )}
      </div>

      <fieldset className={styles.fieldset}>
        <legend>Przypomnienia</legend>
        <p className={styles.muted} id={`${vendorList}-rem`}>
          Kiedy przypomnieć o płatności — najwyżej {maxReminders}.
        </p>
        <div className={styles.chips} role="group" aria-describedby={`${vendorList}-rem`}>
          {reminderOptions.map((days) => {
            const checked = draft.reminders.includes(days)
            return (
              <label key={days} className={styles.chip} data-checked={checked || undefined}>
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={!checked && draft.reminders.length >= maxReminders}
                  onChange={() => toggleReminder(days)}
                />
                {reminderLabel(days)}
              </label>
            )
          })}
        </div>
        {error('reminder_offsets') && <p className={styles.fieldError}>{error('reminder_offsets')}</p>}
      </fieldset>

      <details className={styles.advanced}>
        <summary>Zaawansowane: dopasowanie płatności</summary>
        <p className={styles.muted}>
          Po tych regułach Denar szuka w rejestrze transakcji, które opłacają subskrypcję.
        </p>
        <div className={styles.formGrid}>
          <TextField
            label="Wzorzec kontrahenta"
            value={draft.payeePattern}
            onChange={(event) => set('payeePattern', event.target.value)}
            hint="Fragment nazwy kontrahenta lub notatki, np. OPENAI *CHATGPT. Pusty — nazwa dostawcy i subskrypcji."
            error={pickError(server, 'match_rule.payee_pattern')}
            maxLength={100}
          />
          <TextField
            label="Tolerancja kwoty (%)"
            inputMode="numeric"
            value={draft.tolerance}
            onChange={(event) => set('tolerance', event.target.value)}
            error={error('tolerance', 'match_rule.amount_tolerance_pct')}
            hint="Różnica kursu lub opłata banku."
          />
          <TextField
            label="Okno dat (dni)"
            inputMode="numeric"
            value={draft.window}
            onChange={(event) => set('window', event.target.value)}
            error={error('window', 'match_rule.date_window_days')}
            hint="Ile dni przed i po terminie szukać płatności."
          />
        </div>
      </details>

      <div className={styles.formGrid}>
        <TextField
          label="Adres zarządzania"
          type="url"
          value={draft.manageUrl}
          onChange={(event) => set('manageUrl', event.target.value)}
          error={error('manage_url')}
          hint="Strona, na której zmienisz plan lub anulujesz."
          maxLength={500}
          placeholder="https://"
        />
        <TextAreaField label="Notatka" value={draft.note} onChange={(value) => set('note', value)} maxLength={1000} error={error('note')} />
      </div>

      <FormAlert
        error={mutation.error}
        shownFields={[
          'name',
          'vendor',
          'price',
          'price.amount',
          'amount',
          'cycle.every',
          'cycle.unit',
          'cycle.anchor',
          'trial_ends_at',
          'wallet_id',
          'envelope_id',
          'reminder_offsets',
          'match_rule.payee_pattern',
          'match_rule.amount_tolerance_pct',
          'match_rule.date_window_days',
          'note',
          'manage_url',
        ]}
      />
      <footer className={styles.formFooter}>
        <Button onClick={onClose}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {editing ? 'Zapisz zmiany' : 'Dodaj subskrypcję'}
        </Button>
      </footer>
    </form>
  )
}

/** Tylko pola, które różnią się od zapisanej subskrypcji. */
export function changedFields(subscription: Subscription, next: Required<Omit<SubscriptionUpdate, 'status' | 'ends_at' | 'external_ref'>>): SubscriptionUpdate {
  const current: typeof next = {
    name: subscription.name,
    vendor: subscription.vendor,
    cycle: { unit: subscription.cycle.unit, every: subscription.cycle.every, anchor: subscription.cycle.anchor },
    trial_ends_at: subscription.trial_ends_at,
    wallet_id: subscription.wallet_id,
    envelope_id: subscription.envelope_id,
    reminder_offsets: [...subscription.reminder_offsets].sort((a, b) => b - a),
    match_rule: { ...subscription.match_rule },
    note: subscription.note,
    manage_url: subscription.manage_url,
  }
  const out: Record<string, unknown> = {}
  for (const key of Object.keys(next) as (keyof typeof next)[]) {
    if (JSON.stringify(next[key]) !== JSON.stringify(current[key])) {
      out[key] = next[key]
    }
  }
  return out as SubscriptionUpdate
}
