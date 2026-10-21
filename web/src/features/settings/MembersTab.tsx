import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { ApiError, errorMessage } from '../../api/errors.ts'
import { forgetHousehold } from '../../api/hooks/households.ts'
import { useMembers, useRemoveMember, useUpdateMemberRole } from '../../api/hooks/members.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import type { Member, Role } from '../../api/types.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { selectClass } from '../../components/Field/Field.tsx'
import { Callout } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { tk } from '../../i18n/translate.ts'
import { formatDate } from '../../lib/format.ts'
import { roleLabels, roleOrder } from '../../lib/labels.ts'
import { storageKeys, writeStored } from '../../lib/storage.ts'
import styles from './settings.module.css'

/** Role, które `actor` może nadać członkowi `target` (jak CheckRoleChange na serwerze). */
export function assignableRoles(actor: Role, target: Member): Role[] {
  if (actor === 'owner') {
    return roleOrder
  }
  if (actor === 'admin' && target.role !== 'owner') {
    return roleOrder.filter((role) => role !== 'owner')
  }
  return []
}

function conflictMessage(error: unknown): string {
  if (error instanceof ApiError && error.reason === 'last-owner') {
    return tk('settings:members.lastOwner')
  }
  return errorMessage(error)
}

export function MembersTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('members.title'))
  const { household, role } = useHousehold()
  const members = useMembers(household.id)
  const { user } = useSessionUser()
  const [removing, setRemoving] = useState<Member | null>(null)

  return (
    <section className={styles.section} aria-labelledby="members-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="members-heading">{t('members.title')}</h2>
          <p>
            {canManage(role) ? t('members.leadManage') : t('members.lead')}
          </p>
        </div>
      </div>
      <QueryState query={members} errorTitle={t('members.loadFailed')}>
        {(items) => (
          <DataList
            label={t('members.listLabel')}
            items={items}
            getKey={(member) => member.user_id}
            renderItem={(member) => (
              <MemberRow
                member={member}
                isSelf={member.user_id === user.id}
                actorRole={role}
                onRemove={() => setRemoving(member)}
              />
            )}
          />
        )}
      </QueryState>
      {removing && (
        <RemoveMemberDialog member={removing} isSelf={removing.user_id === user.id} onDone={() => setRemoving(null)} />
      )}
    </section>
  )
}

function MemberRow({
  member,
  isSelf,
  actorRole,
  onRemove,
}: {
  member: Member
  isSelf: boolean
  actorRole: Role
  onRemove: () => void
}) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const update = useUpdateMemberRole(household.id)
  const toast = useToast()
  const roles = assignableRoles(actorRole, member)
  const canRemove = isSelf || (canManage(actorRole) && (actorRole === 'owner' || member.role !== 'owner'))
  const selectId = `role-${member.user_id}`

  return (
    <div className={styles.row}>
      <div className={styles.rowMain}>
        <span className={styles.rowTitle}>
          {member.name}
          {isSelf && <Badge tone="brand">{t('members.you')}</Badge>}
        </span>
        <span className={styles.rowMeta}>
          {t('members.meta', { email: member.email, since: formatDate(member.joined_at) })}
        </span>
        {update.error ? (
          <span role="alert" className={styles.rowMeta}>
            {conflictMessage(update.error)}
          </span>
        ) : null}
      </div>
      <div className={styles.rowActions}>
        {roles.length > 0 ? (
          <>
            <label htmlFor={selectId} className="visually-hidden">
              {t('members.roleLabel', { name: member.name })}
            </label>
            <select
              id={selectId}
              className={selectClass}
              value={member.role}
              disabled={update.isPending}
              onChange={(event) =>
                update.mutate(
                  { userId: member.user_id, role: event.target.value as Role },
                  {
                    onSuccess: (changed) =>
                      toast.show({
                        message: t('members.roleChanged', {
                          name: changed.name,
                          role: roleLabels[changed.role].toLowerCase(),
                        }),
                      }),
                  },
                )
              }
            >
              {roles.map((r) => (
                <option key={r} value={r}>
                  {roleLabels[r]}
                </option>
              ))}
            </select>
          </>
        ) : (
          <Badge>{roleLabels[member.role]}</Badge>
        )}
        {canRemove && (
          <Button
            size="sm"
            variant={isSelf ? 'secondary' : 'danger'}
            onClick={onRemove}
            aria-label={isSelf ? t('members.leaveHousehold') : t('members.removeLabel', { name: member.name })}
          >
            {isSelf ? t('members.leave') : t('members.remove')}
          </Button>
        )}
      </div>
    </div>
  )
}

function RemoveMemberDialog({ member, isSelf, onDone }: { member: Member; isSelf: boolean; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const remove = useRemoveMember(household.id)
  const client = useQueryClient()
  const navigate = useNavigate()
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={isSelf ? t('members.leaveDialog.title', { name: household.name }) : t('members.removeDialog.title', { name: member.name })}
      confirmLabel={isSelf ? t('members.leaveHousehold') : t('members.removeDialog.confirm')}
      busy={remove.isPending}
      error={
        remove.error ? (
          <Callout tone="warn">
            <span role="alert">{conflictMessage(remove.error)}</span>
          </Callout>
        ) : null
      }
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(member.user_id, {
          onSuccess: () => {
            onDone()
            if (isSelf) {
              writeStored(storageKeys.household, null)
              forgetHousehold(client, household.id)
              toast.show({ message: t('members.leaveDialog.done', { name: household.name }) })
              void navigate('/', { replace: true })
            } else {
              toast.show({ message: t('members.removeDialog.done', { name: member.name }) })
            }
          },
        })
      }
    >
      <p>
        {isSelf ? t('members.leaveDialog.body') : t('members.removeDialog.body', { name: member.name })}
      </p>
      <p>{isSelf ? t('members.leaveDialog.bots') : t('members.removeDialog.bots')}</p>
    </ConfirmDialog>
  )
}
