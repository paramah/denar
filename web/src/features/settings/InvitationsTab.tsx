import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { fieldErrors } from '../../api/errors.ts'
import { translateDeliveryError } from '../../api/messages.ts'
import { useCreateInvitation, useInvitations, useRevokeInvitation } from '../../api/hooks/invitations.ts'
import { useChannelTypes } from '../../api/hooks/notifications.ts'
import type { Invitation, InvitationCreated, Role } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { CopyField } from '../../components/CopyField/CopyField.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Checkbox, SelectField, TextField } from '../../components/Field/Field.tsx'
import { Callout, Form, FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDate, formatDateTime } from '../../lib/format.ts'
import { roleDescriptions, roleLabels, roleOrder } from '../../lib/labels.ts'
import styles from './settings.module.css'

const stateTones: Record<Invitation['state'], 'brand' | 'positive' | 'neutral' | 'warn'> = {
  pending: 'brand',
  accepted: 'positive',
  revoked: 'neutral',
  expired: 'warn',
}

export function InvitationsTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('invitations.documentTitle'))
  const { household } = useHousehold()
  const invitations = useInvitations(household.id)
  const [created, setCreated] = useState<InvitationCreated | null>(null)
  const [revoking, setRevoking] = useState<Invitation | null>(null)
  // E-mail z linkiem tylko, gdy serwer ma SMTP (typ kanału „email” dostępny).
  const channelTypes = useChannelTypes()
  const canEmail = channelTypes.data?.some((c) => c.type === 'email' && c.available) ?? false

  return (
    <>
      <section className={styles.section} aria-labelledby="invite-heading">
        <div className={styles.sectionHead}>
          <div>
            <h2 id="invite-heading">{t('invitations.inviteHeading')}</h2>
            <p>
              {canEmail ? t('invitations.leadEmail') : t('invitations.leadNoEmail')}
            </p>
          </div>
        </div>
        <div className={styles.card}>
          <InviteForm onCreated={setCreated} canEmail={canEmail} />
        </div>
      </section>

      <section className={styles.section} aria-labelledby="invitations-heading">
        <h2 id="invitations-heading">{t('invitations.sentHeading')}</h2>
        <QueryState query={invitations} errorTitle={t('invitations.loadFailed')}>
          {(items) => (
            <DataList
              label={t('invitations.sentHeading')}
              items={items}
              getKey={(invitation) => invitation.id}
              empty={
                <EmptyState icon="mail" title={t('invitations.empty.title')} headingLevel={3}>
                  <p>{t('invitations.empty.body')}</p>
                </EmptyState>
              }
              renderItem={(invitation) => (
                <div className={styles.row}>
                  <div className={styles.rowMain}>
                    <span className={styles.rowTitle}>
                      {invitation.email}
                      <Badge tone={stateTones[invitation.state]}>{t(`invitations.state.${invitation.state}`)}</Badge>
                    </span>
                    <span className={styles.rowMeta}>
                      {invitation.state === 'pending'
                        ? t('invitations.metaPending', {
                            role: roleLabels[invitation.role],
                            sent: formatDate(invitation.created_at),
                            expires: formatDateTime(invitation.expires_at),
                          })
                        : t('invitations.meta', { role: roleLabels[invitation.role], sent: formatDate(invitation.created_at) })}
                    </span>
                  </div>
                  {invitation.state === 'pending' && (
                    <div className={styles.rowActions}>
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={() => setRevoking(invitation)}
                        aria-label={t('invitations.revokeLabel', { email: invitation.email })}
                      >
                        {t('invitations.revoke')}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            />
          )}
        </QueryState>
      </section>

      <Dialog open={!!created} title={t('invitations.link')} onClose={() => setCreated(null)} footer={
        <Button variant="primary" onClick={() => setCreated(null)}>
          {t('invitations.done')}
        </Button>
      }>
        {created && (
          <>
            <p>
              <Trans
                t={t}
                i18nKey="invitations.sendLink"
                values={{ email: created.invitation.email, date: formatDateTime(created.invitation.expires_at) }}
                components={{ strong: <strong /> }}
              />
            </p>
            {created.email_sent && (
              <Callout>{t('invitations.emailSent', { email: created.invitation.email })}</Callout>
            )}
            {created.email_error && (
              <Callout tone="warn" title={t('invitations.emailFailedTitle')}>
                {t('invitations.emailFailedBody', { error: translateDeliveryError(created.email_error) })}
              </Callout>
            )}
            <CopyField label={t('invitations.link')} value={created.url} initialFocus />
            <Callout tone="warn">{t('invitations.linkOnce')}</Callout>
          </>
        )}
      </Dialog>

      {revoking && <RevokeDialog invitation={revoking} onDone={() => setRevoking(null)} />}
    </>
  )
}

function InviteForm({ onCreated, canEmail }: { onCreated: (created: InvitationCreated) => void; canEmail: boolean }) {
  const { household, role } = useHousehold()
  const { t } = useTranslation('settings')
  const create = useCreateInvitation(household.id)
  const [email, setEmail] = useState('')
  const [inviteRole, setInviteRole] = useState<Role>('member')
  const [emailError, setEmailError] = useState<string>()
  const [sendEmail, setSendEmail] = useState(true)
  const server = fieldErrors(create.error)
  const roles = roleOrder.filter((r) => role === 'owner' || r !== 'owner')

  const submit = () => {
    if (!/^\S+@\S+\.\S+$/.test(email.trim())) {
      setEmailError(t('invitations.form.emailInvalid'))
      return
    }
    setEmailError(undefined)
    create.mutate(
      { email: email.trim(), role: inviteRole, ...(canEmail && sendEmail ? { send_email: true } : {}) },
      {
        onSuccess: (result) => {
          setEmail('')
          onCreated(result)
        },
      },
    )
  }

  return (
    <Form onSubmit={submit} label={t('invitations.form.label')}>
      <FormAlert error={create.error} shownFields={['email', 'role']} />
      <div className={styles.inlineForm}>
        <TextField
          label={t('invitations.form.email')}
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          autoComplete="off"
          error={emailError ?? server.email}
          required
        />
        <SelectField
          label={t('invitations.form.role')}
          value={inviteRole}
          onChange={(event) => setInviteRole(event.target.value as Role)}
          options={roles.map((r) => ({ value: r, label: roleLabels[r] }))}
          error={server.role}
          hint={roleDescriptions[inviteRole]}
        />
        <Button type="submit" variant="primary" icon="link" busy={create.isPending}>
          {canEmail && sendEmail ? t('invitations.form.createAndSend') : t('invitations.form.create')}
        </Button>
      </div>
      {canEmail && (
        <Checkbox
          label={t('invitations.form.sendEmail')}
          hint={t('invitations.form.sendEmailHint')}
          checked={sendEmail}
          onChange={(event) => setSendEmail(event.target.checked)}
        />
      )}
    </Form>
  )
}

function RevokeDialog({ invitation, onDone }: { invitation: Invitation; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const revoke = useRevokeInvitation(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={t('invitations.revokeDialog.title')}
      confirmLabel={t('invitations.revokeDialog.confirm')}
      busy={revoke.isPending}
      error={<FormAlert error={revoke.error} />}
      onCancel={onDone}
      onConfirm={() =>
        revoke.mutate(invitation.id, {
          onSuccess: () => {
            toast.show({ message: t('invitations.revokeDialog.done', { email: invitation.email }) })
            onDone()
          },
        })
      }
    >
      <p>{t('invitations.revokeDialog.body', { email: invitation.email })}</p>
    </ConfirmDialog>
  )
}
