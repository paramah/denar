import { useId, useRef, useState, type KeyboardEvent } from 'react'
import { errorMessage } from '../../api/errors.ts'
import { useSetAssignment } from '../../api/hooks/budget.ts'
import type { BudgetEnvelope, BudgetGroup, BudgetMonth } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { IconButton } from '../../components/Button/Button.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatMoney, parseAmountInput, toMinor } from '../../lib/money.ts'
import { describeGoal } from './goal.ts'
import { assignedForInput, availableState } from './model.ts'
import styles from './budget.module.css'

export type GridActions = {
  onDetails: (envelope: BudgetEnvelope) => void
  onAddEnvelope: (group: BudgetGroup) => void
  onEditGroup: (group: BudgetGroup) => void
  onDeleteGroup: (group: BudgetGroup) => void
  onMoveGroup: (index: number, delta: -1 | 1) => void
  onMoveEnvelope: (group: BudgetGroup, index: number, delta: -1 | 1) => void
}

type Props = {
  month: BudgetMonth
  decimals: number
  canWrite: boolean
  /** Tryb układu: przyciski kolejności i usuwania grup. */
  arranging: boolean
  showHidden: boolean
  actions: GridActions
}

/** Siatka grup → kopert: przydzielono (edycja w miejscu), aktywność, dostępne, cel. */
export function BudgetGrid({ month, decimals, canWrite, arranging, showHidden, actions }: Props) {
  const groups = month.groups
  return (
    <div className={styles.grid}>
      <div className={styles.head} aria-hidden="true">
        <span>Koperta</span>
        <span>Przydzielono</span>
        <span>Aktywność</span>
        <span>Dostępne</span>
      </div>
      {groups.map((group, groupIndex) => {
        const visible = group.envelopes.filter((envelope) => showHidden || !envelope.hidden)
        const hiddenCount = group.envelopes.length - visible.length
        // Grupa z samymi ukrytymi kopertami znika razem z nimi (poza trybem układu).
        if (visible.length === 0 && hiddenCount > 0 && !arranging) {
          return null
        }
        return (
          <GroupSection
            key={group.id}
            group={group}
            index={groupIndex}
            count={groups.length}
            visible={visible}
            hiddenCount={hiddenCount}
            month={month}
            decimals={decimals}
            canWrite={canWrite}
            arranging={arranging}
            actions={actions}
          />
        )
      })}
    </div>
  )
}

function GroupSection({
  group,
  index,
  count,
  visible,
  hiddenCount,
  month,
  decimals,
  canWrite,
  arranging,
  actions,
}: {
  group: BudgetGroup
  index: number
  count: number
  visible: BudgetEnvelope[]
  hiddenCount: number
  month: BudgetMonth
  decimals: number
  canWrite: boolean
  arranging: boolean
  actions: GridActions
}) {
  const headingId = useId()
  return (
    <section className={styles.group} aria-labelledby={headingId}>
      <div className={styles.groupHead}>
        <div className={styles.groupTitle}>
          <h3 id={headingId}>{group.name}</h3>
          {hiddenCount > 0 && <Badge>Ukryte: {hiddenCount}</Badge>}
          {canWrite && (
            <span className={styles.groupActions}>
              <IconButton icon="plus" label={`Dodaj kopertę do grupy ${group.name}`} onClick={() => actions.onAddEnvelope(group)} />
              <IconButton icon="edit" label={`Zmień nazwę grupy ${group.name}`} onClick={() => actions.onEditGroup(group)} />
              {arranging && (
                <>
                  <IconButton
                    icon="chevronUp"
                    label={`Przesuń grupę ${group.name} w górę`}
                    disabled={index === 0}
                    data-reorder={`${group.id}:up`}
                    onClick={() => actions.onMoveGroup(index, -1)}
                  />
                  <IconButton
                    icon="chevronDown"
                    label={`Przesuń grupę ${group.name} w dół`}
                    disabled={index === count - 1}
                    data-reorder={`${group.id}:down`}
                    onClick={() => actions.onMoveGroup(index, 1)}
                  />
                  <IconButton icon="trash" label={`Usuń grupę ${group.name}`} onClick={() => actions.onDeleteGroup(group)} />
                </>
              )}
            </span>
          )}
        </div>
        <span className={styles.groupSum}>
          <span className="visually-hidden">Przydzielono w grupie: </span>
          <Money amount={group.assigned.amount} currency={month.currency} />
        </span>
        <span className={styles.groupSum}>
          <span className="visually-hidden">Aktywność w grupie: </span>
          <Money amount={group.activity.amount} currency={month.currency} />
        </span>
        <span className={styles.groupSum}>
          <span className="visually-hidden">Dostępne w grupie: </span>
          <Money amount={group.available.amount} currency={month.currency} />
        </span>
      </div>
      {visible.length === 0 ? (
        <p className={styles.emptyGroup}>
          {hiddenCount > 0 ? 'Wszystkie koperty tej grupy są ukryte.' : 'Grupa nie ma jeszcze kopert.'}
        </p>
      ) : (
        <ul className={styles.rows} aria-label={`Koperty: ${group.name}`}>
          {visible.map((envelope) => {
            const position = group.envelopes.indexOf(envelope)
            return (
              <li key={envelope.id}>
                <EnvelopeRow
                  envelope={envelope}
                  month={month}
                  decimals={decimals}
                  canWrite={canWrite}
                  reorder={
                    arranging && canWrite
                      ? {
                          first: position === 0,
                          last: position === group.envelopes.length - 1,
                          move: (delta) => actions.onMoveEnvelope(group, position, delta),
                        }
                      : undefined
                  }
                  onDetails={() => actions.onDetails(envelope)}
                />
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

function EnvelopeRow({
  envelope,
  month,
  decimals,
  canWrite,
  reorder,
  onDetails,
}: {
  envelope: BudgetEnvelope
  month: BudgetMonth
  decimals: number
  canWrite: boolean
  reorder?: { first: boolean; last: boolean; move: (delta: -1 | 1) => void }
  onDetails: () => void
}) {
  const state = availableState(envelope)
  return (
    <article className={styles.row} aria-label={`Koperta ${envelope.name}`} data-hidden={envelope.hidden}>
      <div className={styles.nameCell}>
        <div className={styles.nameLine}>
          <button type="button" className={styles.name} onClick={onDetails} aria-haspopup="dialog">
            {envelope.name}
          </button>
          {envelope.hidden && <Badge>Ukryta</Badge>}
          {reorder && (
            <span className={styles.reorder}>
              <IconButton
                icon="chevronUp"
                label={`Przesuń kopertę ${envelope.name} w górę`}
                disabled={reorder.first}
                data-reorder={`${envelope.id}:up`}
                onClick={() => reorder.move(-1)}
              />
              <IconButton
                icon="chevronDown"
                label={`Przesuń kopertę ${envelope.name} w dół`}
                disabled={reorder.last}
                data-reorder={`${envelope.id}:down`}
                onClick={() => reorder.move(1)}
              />
            </span>
          )}
        </div>
        <GoalProgress envelope={envelope} currency={month.currency} />
      </div>
      <div className={styles.cell} data-col="assigned">
        <span className={styles.cellLabel}>Przydzielono</span>
        {canWrite ? (
          <AssignedInput key={`${month.month}:${envelope.id}`} envelope={envelope} month={month} decimals={decimals} />
        ) : (
          <Money amount={envelope.assigned.amount} currency={month.currency} />
        )}
      </div>
      <div className={styles.cell} data-col="activity">
        <span className={styles.cellLabel}>Aktywność</span>
        <Money amount={envelope.activity.amount} currency={month.currency} tone="auto" signed={false} />
      </div>
      <div className={styles.cell} data-col="available">
        <span className={styles.cellLabel}>Dostępne</span>
        <span className={styles.available} data-state={state} title={stateTitle(state)}>
          <Money amount={envelope.available.amount} currency={month.currency} />
          {state === 'negative' && <span className="visually-hidden"> (przekroczono)</span>}
        </span>
      </div>
    </article>
  )
}

function stateTitle(state: ReturnType<typeof availableState>): string {
  switch (state) {
    case 'negative':
      return 'Przekroczono — przenieś środki, żeby pokryć wydatki'
    case 'underfunded':
      return 'Cel tego miesiąca nie jest jeszcze pokryty'
    case 'zero':
      return 'Nic nie zostało'
    default:
      return 'Do wydania'
  }
}

/** Pasek postępu celu i jego stan w miesiącu. */
export function GoalProgress({ envelope, currency }: { envelope: BudgetEnvelope; currency: string }) {
  const { goal, goal_status: status } = envelope
  if (!goal || !status) {
    return null
  }
  const text = status.funded
    ? 'Cel pokryty'
    : `Brakuje ${formatMoney(status.underfunded.amount, currency) ?? status.underfunded.amount}`
  return (
    <div className={styles.goal}>
      <div
        className={styles.bar}
        role="progressbar"
        aria-label={`Postęp celu ${envelope.name}`}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={status.progress}
        aria-valuetext={`${status.progress}% · ${describeGoal(goal)}`}
        data-funded={status.funded}
      >
        <span style={{ width: `${status.progress}%` }} />
      </div>
      <span className={styles.goalText} data-funded={status.funded}>
        {text}
        <span className="visually-hidden"> · cel: {describeGoal(goal)}</span>
      </span>
    </div>
  )
}

const assignSelector = '[data-assign]'

/** Następne/poprzednie pole przydziału w kolejności strony. */
function siblingInput(current: HTMLInputElement, delta: 1 | -1): HTMLInputElement | null {
  const all = Array.from(document.querySelectorAll<HTMLInputElement>(assignSelector))
  return all[all.indexOf(current) + delta] ?? null
}

/**
 * Przydział edytowany w miejscu: Enter zapisuje, Esc przywraca, Tab zapisuje
 * i przechodzi do następnej koperty (Shift+Tab — do poprzedniej), wyjście
 * z pola zapisuje zmianę. Kwota może być ujemna, 0 usuwa przydział.
 */
function AssignedInput({ envelope, month, decimals }: { envelope: BudgetEnvelope; month: BudgetMonth; decimals: number }) {
  const { household } = useHousehold()
  const assign = useSetAssignment(household.id)
  const toast = useToast()
  const errorId = useId()
  const current = assignedForInput(envelope.assigned.amount)
  const [draft, setDraft] = useState<string | null>(null)
  const [saving, setSaving] = useState<string | null>(null)
  const [error, setError] = useState<string>()
  // Lustro stanu: blur po Tab przychodzi przed ponownym renderem.
  const draftRef = useRef<string | null>(null)

  const edit = (value: string | null) => {
    draftRef.current = value
    setDraft(value)
  }

  /** true, gdy można opuścić pole (zapis ruszył albo nie było zmian). */
  const commit = (): boolean => {
    const value = draftRef.current
    if (value === null) {
      return true
    }
    const parsed = parseAmountInput(value, decimals, { allowNegative: true, allowZero: true })
    if (!parsed.ok) {
      setError(parsed.error)
      return false
    }
    setError(undefined)
    edit(null)
    if (toMinor(parsed.value, decimals) === toMinor(envelope.assigned.amount, decimals)) {
      return true
    }
    setSaving(assignedForInput(parsed.value))
    assign.mutate(
      { month: month.month, envelopeId: envelope.id, amount: { amount: parsed.value, currency: month.currency } },
      {
        onError: (err) => toast.show({ message: `Nie zapisano przydziału „${envelope.name}”: ${errorMessage(err)}`, kind: 'error' }),
        onSettled: () => setSaving(null),
      },
    )
    return true
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault()
      commit()
    } else if (event.key === 'Escape') {
      if (draftRef.current !== null || error) {
        event.preventDefault()
        setError(undefined)
        edit(null)
      }
    } else if (event.key === 'Tab') {
      const target = siblingInput(event.currentTarget, event.shiftKey ? -1 : 1)
      if (!commit()) {
        event.preventDefault()
        return
      }
      if (target) {
        event.preventDefault()
        target.focus()
      }
    }
  }

  return (
    <>
      <input
        className={styles.assign}
        data-assign
        inputMode="decimal"
        autoComplete="off"
        aria-label={`Przydzielono: ${envelope.name}`}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        aria-busy={saving !== null || undefined}
        value={draft ?? saving ?? current}
        onFocus={(event) => event.currentTarget.select()}
        onChange={(event) => {
          setError(undefined)
          edit(event.target.value)
        }}
        onKeyDown={onKeyDown}
        onBlur={() => {
          if (!commit()) {
            // Błędna kwota przy wyjściu z pola: wracamy do zapisanej.
            setError(undefined)
            edit(null)
          }
        }}
      />
      {error && (
        <p id={errorId} className={styles.assignError} role="alert">
          {error}
        </p>
      )}
    </>
  )
}
