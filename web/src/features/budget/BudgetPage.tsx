import { useRef, useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { errorMessage } from '../../api/errors.ts'
import { reorderPatches, useAutoAssign, useBudgetMonth, useUpdateEnvelope, useUpdateEnvelopeGroup } from '../../api/hooks/budget.ts'
import type { AutoAssignResult, BudgetEnvelope, BudgetGroup, BudgetMonth } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Checkbox } from '../../components/Field/Field.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { currentBudgetMonth, firstMonth, formatMonth, isMonth, lastMonth, shiftMonth } from '../../lib/budgetMonth.ts'
import { absDecimal, amountForInput, toneOf } from '../../lib/money.ts'
import { readStored, writeStored } from '../../lib/storage.ts'
import { useDecimals } from '../transactions/money.ts'
import { BudgetGrid, type GridActions } from './BudgetGrid.tsx'
import { BudgetFlows, BudgetSummary } from './BudgetSummary.tsx'
import { EnvelopeDetails } from './EnvelopeDetails.tsx'
import { MoveDialog, type MovePreset } from './MoveDialog.tsx'
import { envelopesOf } from './model.ts'
import { DeleteEnvelopeDialog, DeleteGroupDialog, EnvelopeDialog, GroupDialog } from './StructureDialogs.tsx'
import styles from './budget.module.css'

type Modal =
  | { kind: 'move'; preset?: MovePreset }
  | { kind: 'auto'; result: AutoAssignResult }
  | { kind: 'group'; group?: BudgetGroup }
  | { kind: 'envelope'; envelope?: BudgetEnvelope; groupId?: string }
  | { kind: 'deleteGroup'; group: BudgetGroup }
  | { kind: 'deleteEnvelope'; envelope: BudgetEnvelope }
  | { kind: 'details'; envelopeId: string }

const hiddenKey = 'denar:budget:hidden'

/** Budżet kopertowy miesiąca: /budget (bieżący miesiąc) i /budget/:month. */
export function BudgetPage() {
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const params = useParams()
  const current = currentBudgetMonth(household.timezone, household.month_start_day)
  const month = params.month ?? current

  if (!isMonth(month)) {
    return (
      <>
        <PageHeader title={t('page.title')} />
        <EmptyState
          icon="budget"
          title={t('page.badMonthTitle')}
          action={
            <LinkButton to={householdPath(household.id, 'budget')} variant="primary">
              {t('page.badMonthAction')}
            </LinkButton>
          }
        >
          <p>{t('page.badMonthHint')}</p>
        </EmptyState>
      </>
    )
  }
  return <BudgetMonthView key={month} month={month} current={current} />
}

function MonthNav({ month, current }: { month: string; current: string }) {
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const to = (m: string) => householdPath(household.id, `budget/${m}`)
  const prev = shiftMonth(month, -1)
  const next = shiftMonth(month, 1)
  return (
    <nav className={styles.monthNav} aria-label={t('monthNav.label')}>
      <LinkButton
        to={to(prev)}
        icon="chevronLeft"
        aria-label={t('monthNav.previous', { month: formatMonth(prev) })}
        aria-disabled={month <= firstMonth || undefined}
        onClick={(event) => month <= firstMonth && event.preventDefault()}
      />
      <h2 className={styles.monthName} aria-live="polite">
        {formatMonth(month)}
      </h2>
      <LinkButton
        to={to(next)}
        icon="chevronRight"
        aria-label={t('monthNav.next', { month: formatMonth(next) })}
        aria-disabled={month >= lastMonth || undefined}
        onClick={(event) => month >= lastMonth && event.preventDefault()}
      />
      {month !== current && (
        <LinkButton to={householdPath(household.id, 'budget')} size="sm" variant="ghost">
          {t('monthNav.current')}
        </LinkButton>
      )}
    </nav>
  )
}

function BudgetMonthView({ month, current }: { month: string; current: string }) {
  const { t } = useTranslation('budget')
  const { household, role } = useHousehold()
  const budget = useBudgetMonth(household.id, month)
  const canWrite = role !== 'viewer'
  const decimals = useDecimals()(household.base_currency)

  return (
    <>
      <PageHeader
        title={t('page.title')}
        documentTitle={t('page.documentTitle', { month: formatMonth(month) })}
        description={<MonthNav month={month} current={current} />}
      />
      <QueryState query={budget} errorTitle={t('page.loadFailed')} loading={<Skeleton lines={6} />}>
        {(data) => <BudgetBody data={data} decimals={decimals} canWrite={canWrite} />}
      </QueryState>
    </>
  )
}

function BudgetBody({ data, decimals, canWrite }: { data: BudgetMonth; decimals: number; canWrite: boolean }) {
  const { t } = useTranslation('budget')
  const { household } = useHousehold()
  const auto = useAutoAssign(household.id)
  const updateGroup = useUpdateEnvelopeGroup(household.id)
  const updateEnvelope = useUpdateEnvelope(household.id)
  const toast = useToast()
  const [modal, setModal] = useState<Modal | null>(null)
  const [arranging, setArranging] = useState(false)
  const [showHidden, setShowHidden] = useState(() => readStored(hiddenKey) === '1')
  const [reordering, setReordering] = useState(false)
  const focusAfter = useRef<string | null>(null)
  const close = () => setModal(null)
  const hiddenCount = envelopesOf(data).filter((e) => e.hidden).length
  const underfunded = toneOf(data.underfunded.amount) === 'inflow'

  // Po zmianie kolejności fokus wraca na ten sam przycisk (wiersz zmienił miejsce).
  const refocus = () => {
    const target = focusAfter.current
    focusAfter.current = null
    if (target) {
      requestAnimationFrame(() => {
        const button = document.querySelector<HTMLButtonElement>(`[data-reorder="${target}"]`)
        const fallback = document.querySelector<HTMLButtonElement>(`[data-reorder="${target.replace(/:(up|down)$/, (_, d) => (d === 'up' ? ':down' : ':up'))}"]`)
        ;(button && !button.disabled ? button : fallback)?.focus()
      })
    }
  }

  const applyOrder = async (patches: { id: string; position: number }[], kind: 'group' | 'envelope', focus: string) => {
    if (patches.length === 0 || reordering) {
      return
    }
    setReordering(true)
    focusAfter.current = focus
    try {
      for (const patch of patches) {
        if (kind === 'group') {
          await updateGroup.mutateAsync({ groupId: patch.id, body: { position: patch.position } })
        } else {
          await updateEnvelope.mutateAsync({ envelopeId: patch.id, body: { position: patch.position } })
        }
      }
    } catch (err) {
      toast.show({ message: t('toolbar.reorderFailed', { error: errorMessage(err) }), kind: 'error' })
    } finally {
      setReordering(false)
      refocus()
    }
  }

  const actions: GridActions = {
    onDetails: (envelope) => setModal({ kind: 'details', envelopeId: envelope.id }),
    onAddEnvelope: (group) => setModal({ kind: 'envelope', groupId: group.id }),
    onEditGroup: (group) => setModal({ kind: 'group', group }),
    onDeleteGroup: (group) => setModal({ kind: 'deleteGroup', group }),
    onMoveGroup: (index, delta) => {
      const group = data.groups[index]!
      void applyOrder(reorderPatches(data.groups, index, index + delta), 'group', `${group.id}:${delta < 0 ? 'up' : 'down'}`)
    },
    onMoveEnvelope: (group, index, delta) => {
      const envelope = group.envelopes[index]!
      void applyOrder(
        reorderPatches(group.envelopes, index, index + delta),
        'envelope',
        `${envelope.id}:${delta < 0 ? 'up' : 'down'}`,
      )
    },
  }

  const runAuto = () =>
    auto.mutate(data.month, {
      onSuccess: (result) => setModal({ kind: 'auto', result }),
      onError: (err) => toast.show({ message: t('toolbar.autoFailed', { error: errorMessage(err) }), kind: 'error' }),
    })

  const details = modal?.kind === 'details' ? envelopesOf(data).find((e) => e.id === modal.envelopeId) : undefined
  const hasEnvelopes = data.groups.some((g) => g.envelopes.length > 0)

  return (
    <div className={styles.page}>
      <BudgetSummary month={data} />

      {canWrite && data.groups.length > 0 && (
        <div className={styles.toolbar} role="toolbar" aria-label={t('toolbar.label')}>
          <Button icon="transfer" onClick={() => setModal({ kind: 'move' })} disabled={!hasEnvelopes}>
            {t('toolbar.move')}
          </Button>
          <Button icon="sparkle" onClick={runAuto} busy={auto.isPending} disabled={!hasEnvelopes} aria-describedby="underfunded-note">
            {t('toolbar.autoAssign')}
          </Button>
          <span id="underfunded-note" className={styles.underfunded}>
            {underfunded ? (
              <Trans
                t={t}
                i18nKey="toolbar.underfunded"
                components={{ money: <Money amount={data.underfunded.amount} currency={data.currency} size="sm" /> }}
              />
            ) : (
              t('toolbar.covered')
            )}
          </span>
          <div className={styles.toolbarEnd}>
            {hiddenCount > 0 && (
              <Checkbox
                label={t('toolbar.showHidden', { n: hiddenCount })}
                checked={showHidden}
                onChange={(event) => {
                  setShowHidden(event.target.checked)
                  writeStored(hiddenKey, event.target.checked ? '1' : null)
                }}
              />
            )}
            <Button icon="list" aria-pressed={arranging} onClick={() => setArranging((value) => !value)}>
              {arranging ? t('toolbar.arrangeDone') : t('toolbar.arrange')}
            </Button>
            <Button icon="plus" onClick={() => setModal({ kind: 'group' })}>
              {t('toolbar.newGroup')}
            </Button>
          </div>
        </div>
      )}

      {data.groups.length === 0 ? (
        <EmptyState
          icon="budget"
          title={t('empty.title')}
          action={
            canWrite && (
              <Button variant="primary" icon="plus" onClick={() => setModal({ kind: 'group' })}>
                {t('empty.action')}
              </Button>
            )
          }
        >
          <p>{t('empty.body')}</p>
        </EmptyState>
      ) : (
        <BudgetGrid
          month={data}
          decimals={decimals}
          canWrite={canWrite}
          arranging={arranging}
          showHidden={showHidden || arranging}
          actions={actions}
        />
      )}

      <BudgetFlows month={data} />

      <MoveDialog
        open={modal?.kind === 'move'}
        onClose={close}
        month={data}
        decimals={decimals}
        preset={modal?.kind === 'move' ? modal.preset : undefined}
      />
      <GroupDialog
        open={modal?.kind === 'group'}
        onClose={close}
        group={modal?.kind === 'group' ? modal.group : undefined}
        position={data.groups.length}
      />
      <EnvelopeDialog
        open={modal?.kind === 'envelope'}
        onClose={close}
        month={data}
        decimals={decimals}
        envelope={modal?.kind === 'envelope' ? modal.envelope : undefined}
        groupId={modal?.kind === 'envelope' ? modal.groupId : undefined}
      />
      {modal?.kind === 'deleteGroup' && <DeleteGroupDialog group={modal.group} onDone={close} />}
      {modal?.kind === 'deleteEnvelope' && <DeleteEnvelopeDialog envelope={modal.envelope} onDone={close} />}
      {details && (
        <EnvelopeDetails
          envelope={details}
          month={data}
          canWrite={canWrite}
          onClose={close}
          onEdit={() => setModal({ kind: 'envelope', envelope: details })}
          onDelete={() => setModal({ kind: 'deleteEnvelope', envelope: details })}
          onMove={() =>
            setModal({
              kind: 'move',
              preset:
                toneOf(details.available.amount) === 'outflow'
                  ? { to: details.id, amount: amountForInput(absDecimal(details.available.amount)) }
                  : { from: details.id },
            })
          }
        />
      )}
      {modal?.kind === 'auto' && <AutoAssignSummary result={modal.result} onClose={close} />}
    </div>
  )
}

/** Wynik auto-przydziału: co i ile uzupełniono, ile zostało. */
function AutoAssignSummary({ result, onClose }: { result: AutoAssignResult; onClose: () => void }) {
  const { t } = useTranslation('budget')
  const names = new Map(envelopesOf(result.month).map((e) => [e.id, e.name]))
  const month = result.month
  return (
    <Dialog
      open
      title={t('auto.title')}
      onClose={onClose}
      size="sm"
      footer={
        <Button variant="primary" onClick={onClose} data-autofocus>
          {t('auto.done')}
        </Button>
      }
    >
      {result.assigned.length === 0 ? (
        <p>{toneOf(month.underfunded.amount) === 'inflow' ? t('auto.nothingNoMoney') : t('auto.nothingCovered')}</p>
      ) : (
        <>
          <p>{t('auto.intro')}</p>
          <ul className={styles.resultList} aria-label={t('auto.listLabel')}>
            {result.assigned.map((item) => (
              <li key={item.envelope_id}>
                <span>{names.get(item.envelope_id) ?? t('auto.envelopeFallback')}</span>
                <Money amount={item.amount.amount} currency={item.amount.currency} tone="auto" />
              </li>
            ))}
          </ul>
        </>
      )}
      <p className={styles.muted}>
        <Trans
          t={t}
          i18nKey={toneOf(month.underfunded.amount) === 'inflow' ? 'auto.leftNeeds' : 'auto.left'}
          components={{
            rta: <Money amount={month.ready_to_assign.amount} currency={month.currency} />,
            needed: <Money amount={month.underfunded.amount} currency={month.currency} />,
          }}
        />
      </p>
    </Dialog>
  )
}
