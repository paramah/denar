import { useId, useState } from 'react'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import {
  useCreateEnvelope,
  useCreateEnvelopeGroup,
  useDeleteEnvelope,
  useDeleteEnvelopeGroup,
  useUpdateEnvelope,
  useUpdateEnvelopeGroup,
} from '../../api/hooks/budget.ts'
import type { BudgetEnvelope, BudgetGroup, BudgetMonth, EnvelopeCreate, EnvelopeUpdate } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { Checkbox, SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { todayIn } from '../../lib/format.ts'
import { AmountField } from '../transactions/fields.tsx'
import { pickError } from '../transactions/money.ts'
import { checkGoal, goalDraft, goalTypeLabels, type GoalChoice, type GoalDraft } from './goal.ts'
import styles from './budget.module.css'

// ——— Grupa ———

export function GroupDialog({
  open,
  onClose,
  group,
  position,
}: {
  open: boolean
  onClose: () => void
  group?: BudgetGroup
  /** Pozycja nowej grupy (na końcu listy). */
  position: number
}) {
  return (
    <Dialog open={open} title={group ? `Zmiana nazwy grupy ${group.name}` : 'Nowa grupa kopert'} onClose={onClose} size="sm">
      <GroupForm group={group} position={position} onDone={onClose} />
    </Dialog>
  )
}

function GroupForm({ group, position, onDone }: { group?: BudgetGroup; position: number; onDone: () => void }) {
  const { household } = useHousehold()
  const create = useCreateEnvelopeGroup(household.id)
  const update = useUpdateEnvelopeGroup(household.id)
  const toast = useToast()
  const mutation = group ? update : create
  const [name, setName] = useState(group?.name ?? '')
  const [error, setError] = useState<string>()
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'
  const server = fieldErrors(mutation.error)

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Podaj nazwę grupy.')
      return
    }
    setError(undefined)
    try {
      if (group) {
        await update.mutateAsync({ groupId: group.id, body: { name: trimmed } })
      } else {
        await create.mutateAsync({ name: trimmed, position })
      }
      toast.show({ message: group ? `Zmieniono nazwę grupy na ${trimmed}.` : `Dodano grupę ${trimmed}.` })
      onDone()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <form
      noValidate
      aria-label={group ? 'Zmiana nazwy grupy' : 'Nowa grupa kopert'}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert error={duplicate ? null : mutation.error} shownFields={['name']} />
      <TextField
        label="Nazwa grupy"
        value={name}
        onChange={(event) => setName(event.target.value)}
        error={error ?? (duplicate ? 'Grupa o tej nazwie już istnieje.' : server.name)}
        hint={group ? undefined : 'Np. Rachunki, Codzienne, Cele'}
        maxLength={100}
        autoComplete="off"
        data-autofocus
      />
      <div className={styles.actions}>
        <Button onClick={onDone}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {group ? 'Zapisz' : 'Dodaj grupę'}
        </Button>
      </div>
    </form>
  )
}

// ——— Koperta ———

export function EnvelopeDialog({
  open,
  onClose,
  month,
  decimals,
  envelope,
  groupId,
}: {
  open: boolean
  onClose: () => void
  month: BudgetMonth
  decimals: number
  envelope?: BudgetEnvelope
  /** Grupa nowej koperty. */
  groupId?: string
}) {
  return (
    <Dialog open={open} title={envelope ? `Edycja koperty ${envelope.name}` : 'Nowa koperta'} onClose={onClose} size="md">
      <EnvelopeForm month={month} decimals={decimals} envelope={envelope} groupId={groupId} onDone={onClose} />
    </Dialog>
  )
}

function EnvelopeForm({
  month,
  decimals,
  envelope,
  groupId,
  onDone,
}: {
  month: BudgetMonth
  decimals: number
  envelope?: BudgetEnvelope
  groupId?: string
  onDone: () => void
}) {
  const { household } = useHousehold()
  const create = useCreateEnvelope(household.id)
  const update = useUpdateEnvelope(household.id)
  const toast = useToast()
  const mutation = envelope ? update : create
  const goalName = useId()
  const [name, setName] = useState(envelope?.name ?? '')
  const [group, setGroup] = useState(envelope?.group_id ?? groupId ?? month.groups[0]?.id ?? '')
  const [hidden, setHidden] = useState(envelope?.hidden ?? false)
  const [goal, setGoal] = useState<GoalDraft>(() => goalDraft(envelope?.goal))
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const duplicate = mutation.error instanceof ApiError && mutation.error.reason === 'duplicate-name'
  const server = fieldErrors(mutation.error)
  const original = goalDraft(envelope?.goal)

  const positionIn = (target: string) => month.groups.find((g) => g.id === target)?.envelopes.length ?? 0

  const submit = async () => {
    const next: Record<string, string | undefined> = {}
    const trimmed = name.trim()
    if (!trimmed) {
      next.name = 'Podaj nazwę koperty.'
    }
    if (!group) {
      next.group_id = 'Wybierz grupę.'
    }
    // Termin w przeszłości odrzucamy tylko przy nowej dacie — stary cel po terminie zostaje edytowalny.
    const today = goal.targetDate !== original.targetDate ? todayIn(household.timezone) : undefined
    const checked = checkGoal(goal, month.currency, decimals, today)
    if (!checked.ok) {
      next['goal.amount'] = checked.errors.amount
      next['goal.target_date'] = checked.errors.target_date
    }
    setErrors(next)
    if (Object.values(next).some(Boolean) || !checked.ok) {
      return
    }
    try {
      if (envelope) {
        const body: EnvelopeUpdate = { name: trimmed, hidden }
        if (group !== envelope.group_id) {
          body.group_id = group
          body.position = positionIn(group)
        }
        if (checked.goal || envelope.goal) {
          body.goal = checked.goal
        }
        await update.mutateAsync({ envelopeId: envelope.id, body })
      } else {
        const body: EnvelopeCreate = {
          group_id: group,
          name: trimmed,
          position: positionIn(group),
          ...(checked.goal ? { goal: checked.goal } : {}),
        }
        await create.mutateAsync(body)
      }
      toast.show({ message: envelope ? `Zapisano kopertę ${trimmed}.` : `Dodano kopertę ${trimmed}.` })
      onDone()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  const setType = (type: GoalChoice) => setGoal((current) => ({ ...current, type }))

  return (
    <form
      noValidate
      aria-label={envelope ? 'Edycja koperty' : 'Nowa koperta'}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert
        error={duplicate ? null : mutation.error}
        shownFields={['name', 'group_id', 'goal', 'goal.type', 'goal.amount', 'goal.amount.amount', 'goal.amount.currency', 'goal.target_date']}
      />
      <div className={styles.pair}>
        <TextField
          label="Nazwa koperty"
          value={name}
          onChange={(event) => setName(event.target.value)}
          error={errors.name ?? (duplicate ? 'Koperta o tej nazwie już istnieje.' : server.name)}
          maxLength={100}
          autoComplete="off"
          data-autofocus
        />
        <SelectField
          label="Grupa"
          value={group}
          onChange={(event) => setGroup(event.target.value)}
          options={month.groups.map((g) => ({ value: g.id, label: g.name }))}
          error={errors.group_id ?? server.group_id}
        />
      </div>
      <fieldset className={styles.goalChoice}>
        <legend>Cel</legend>
        <div className={styles.radios}>
          {(['none', 'monthly', 'target_by_date'] as GoalChoice[]).map((type) => (
            <label key={type}>
              <input type="radio" name={goalName} checked={goal.type === type} onChange={() => setType(type)} />
              {goalTypeLabels[type]}
            </label>
          ))}
        </div>
        {goal.type === 'monthly' && (
          <p className={styles.muted}>Ile przydzielać do tej koperty co miesiąc, np. na jedzenie albo rachunki.</p>
        )}
        {goal.type === 'target_by_date' && (
          <p className={styles.muted}>
            Ile zebrać do wybranego dnia — brakującą kwotę rozłożymy równo na pozostałe miesiące.
          </p>
        )}
        {goal.type !== 'none' && (
          <div className={styles.pair}>
            <AmountField
              label={goal.type === 'monthly' ? 'Kwota miesięcznie' : 'Kwota celu'}
              value={goal.amount}
              onChange={(amount) => setGoal((current) => ({ ...current, amount }))}
              currency={month.currency}
              error={errors['goal.amount'] ?? pickError(server, 'goal.amount', 'goal.amount.amount', 'goal.amount.currency', 'goal')}
            />
            {goal.type === 'target_by_date' && (
              <TextField
                label="Do dnia"
                type="date"
                value={goal.targetDate}
                onChange={(event) => setGoal((current) => ({ ...current, targetDate: event.target.value }))}
                error={errors['goal.target_date'] ?? server['goal.target_date']}
              />
            )}
          </div>
        )}
      </fieldset>
      {envelope && (
        <Checkbox
          label="Ukryj kopertę"
          hint="Ukryta koperta znika z siatki, ale jej kwoty dalej liczą się w budżecie i można jej używać w transakcjach."
          checked={hidden}
          onChange={(event) => setHidden(event.target.checked)}
        />
      )}
      <div className={styles.actions}>
        <Button onClick={onDone}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {envelope ? 'Zapisz' : 'Dodaj kopertę'}
        </Button>
      </div>
    </form>
  )
}

// ——— Usuwanie ———

export function DeleteEnvelopeDialog({ envelope, onDone }: { envelope: BudgetEnvelope; onDone: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteEnvelope(household.id)
  const hide = useUpdateEnvelope(household.id)
  const toast = useToast()
  const inUse = remove.error instanceof ApiError && remove.error.reason === 'envelope-in-use'

  return (
    <ConfirmDialog
      open
      title={`Usunąć kopertę ${envelope.name}?`}
      confirmLabel="Usuń kopertę"
      busy={remove.isPending}
      error={
        inUse ? (
          <Callout tone="warn" title="Tej koperty nie da się usunąć">
            <p>Są w niej transakcje — historia budżetu musi zostać. Możesz ją ukryć: zniknie z siatki, a kwoty zostaną.</p>
            <Button
              size="sm"
              icon="eyeOff"
              busy={hide.isPending}
              onClick={() =>
                hide.mutate(
                  { envelopeId: envelope.id, body: { hidden: true } },
                  {
                    onSuccess: () => {
                      toast.show({ message: `Ukryto kopertę ${envelope.name}.` })
                      onDone()
                    },
                  },
                )
              }
            >
              Ukryj kopertę
            </Button>
          </Callout>
        ) : (
          <FormAlert error={remove.error ?? hide.error} />
        )
      }
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(envelope.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto kopertę ${envelope.name}.` })
            onDone()
          },
        })
      }
    >
      <p>
        Przydziały koperty znikną, a ich kwota wróci do puli „Do przydzielenia”. Kontrahenci z tą kopertą jako domyślną
        stracą to ustawienie.
      </p>
    </ConfirmDialog>
  )
}

export function DeleteGroupDialog({ group, onDone }: { group: BudgetGroup; onDone: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteEnvelopeGroup(household.id)
  const hide = useUpdateEnvelope(household.id)
  const toast = useToast()
  const notEmpty = remove.error instanceof ApiError && remove.error.reason === 'group-not-empty'
  const visible = group.envelopes.filter((e) => !e.hidden)

  const hideAll = async () => {
    try {
      for (const envelope of visible) {
        await hide.mutateAsync({ envelopeId: envelope.id, body: { hidden: true } })
      }
      toast.show({ message: `Ukryto koperty grupy ${group.name}.` })
      onDone()
    } catch {
      // Błąd pokazuje dialog.
    }
  }

  return (
    <ConfirmDialog
      open
      title={`Usunąć grupę ${group.name}?`}
      confirmLabel="Usuń grupę"
      busy={remove.isPending}
      error={
        notEmpty ? (
          <Callout tone="warn" title="W grupie są koperty">
            <p>
              Usunąć można tylko pustą grupę. Przenieś koperty do innej grupy albo je usuń; koperty z transakcjami możesz
              ukryć.
            </p>
            {visible.length > 0 && (
              <Button size="sm" icon="eyeOff" busy={hide.isPending} onClick={() => void hideAll()}>
                Ukryj koperty grupy ({visible.length})
              </Button>
            )}
          </Callout>
        ) : (
          <FormAlert error={remove.error ?? hide.error} />
        )
      }
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(group.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto grupę ${group.name}.` })
            onDone()
          },
        })
      }
    >
      <p>{group.envelopes.length === 0 ? 'Grupa jest pusta.' : `Koperty w grupie: ${group.envelopes.length}.`}</p>
    </ConfirmDialog>
  )
}
