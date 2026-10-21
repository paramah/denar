import { useState } from 'react'
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

const stateBadges: Record<Invitation['state'], { label: string; tone: 'brand' | 'positive' | 'neutral' | 'warn' }> = {
  pending: { label: 'Oczekuje', tone: 'brand' },
  accepted: { label: 'Przyjęte', tone: 'positive' },
  revoked: { label: 'Odwołane', tone: 'neutral' },
  expired: { label: 'Wygasło', tone: 'warn' },
}

export function InvitationsTab() {
  useDocumentTitle('Zaproszenia')
  const { household } = useHousehold()
  const invitations = useInvitations(household.id)
  const [created, setCreated] = useState<InvitationCreated | null>(null)
  const [revoking, setRevoking] = useState<Invitation | null>(null)
  // E-mail z linkiem tylko, gdy serwer ma SMTP (typ kanału „email” dostępny).
  const channelTypes = useChannelTypes()
  const canEmail = channelTypes.data?.some((t) => t.type === 'email' && t.available) ?? false

  return (
    <>
      <section className={styles.section} aria-labelledby="invite-heading">
        <div className={styles.sectionHead}>
          <div>
            <h2 id="invite-heading">Zaproś do gospodarstwa</h2>
            <p>
              {canEmail
                ? 'Dostaniesz jednorazowy link. Denar może też wysłać go e-mailem do zaproszonej osoby.'
                : 'Dostaniesz jednorazowy link. Przekaż go zaproszonej osobie — ten serwer nie ma skonfigurowanej poczty.'}
            </p>
          </div>
        </div>
        <div className={styles.card}>
          <InviteForm onCreated={setCreated} canEmail={canEmail} />
        </div>
      </section>

      <section className={styles.section} aria-labelledby="invitations-heading">
        <h2 id="invitations-heading">Wysłane zaproszenia</h2>
        <QueryState query={invitations} errorTitle="Nie udało się wczytać zaproszeń">
          {(items) => (
            <DataList
              label="Wysłane zaproszenia"
              items={items}
              getKey={(invitation) => invitation.id}
              empty={
                <EmptyState icon="mail" title="Nie ma jeszcze zaproszeń" headingLevel={3}>
                  <p>Zaproszone osoby pojawią się tutaj razem ze stanem zaproszenia.</p>
                </EmptyState>
              }
              renderItem={(invitation) => (
                <div className={styles.row}>
                  <div className={styles.rowMain}>
                    <span className={styles.rowTitle}>
                      {invitation.email}
                      <Badge tone={stateBadges[invitation.state].tone}>{stateBadges[invitation.state].label}</Badge>
                    </span>
                    <span className={styles.rowMeta}>
                      {roleLabels[invitation.role]}, wysłane {formatDate(invitation.created_at)}
                      {invitation.state === 'pending' && `, ważne do ${formatDateTime(invitation.expires_at)}`}
                    </span>
                  </div>
                  {invitation.state === 'pending' && (
                    <div className={styles.rowActions}>
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={() => setRevoking(invitation)}
                        aria-label={`Odwołaj zaproszenie dla ${invitation.email}`}
                      >
                        Odwołaj
                      </Button>
                    </div>
                  )}
                </div>
              )}
            />
          )}
        </QueryState>
      </section>

      <Dialog open={!!created} title="Link z zaproszeniem" onClose={() => setCreated(null)} footer={
        <Button variant="primary" onClick={() => setCreated(null)}>
          Gotowe
        </Button>
      }>
        {created && (
          <>
            <p>
              Wyślij ten link do <strong>{created.invitation.email}</strong>. Działa raz, do{' '}
              {formatDateTime(created.invitation.expires_at)}.
            </p>
            {created.email_sent && (
              <Callout>Wysłaliśmy link e-mailem na adres {created.invitation.email}.</Callout>
            )}
            {created.email_error && (
              <Callout tone="warn" title="Nie udało się wysłać e-maila">
                {translateDeliveryError(created.email_error)}. Zaproszenie powstało — przekaż link samodzielnie.
              </Callout>
            )}
            <CopyField label="Link z zaproszeniem" value={created.url} initialFocus />
            <Callout tone="warn">Link pokazujemy tylko teraz. Po zamknięciu okna możesz wygenerować nowy.</Callout>
          </>
        )}
      </Dialog>

      {revoking && <RevokeDialog invitation={revoking} onDone={() => setRevoking(null)} />}
    </>
  )
}

function InviteForm({ onCreated, canEmail }: { onCreated: (created: InvitationCreated) => void; canEmail: boolean }) {
  const { household, role } = useHousehold()
  const create = useCreateInvitation(household.id)
  const [email, setEmail] = useState('')
  const [inviteRole, setInviteRole] = useState<Role>('member')
  const [emailError, setEmailError] = useState<string>()
  const [sendEmail, setSendEmail] = useState(true)
  const server = fieldErrors(create.error)
  const roles = roleOrder.filter((r) => role === 'owner' || r !== 'owner')

  const submit = () => {
    if (!/^\S+@\S+\.\S+$/.test(email.trim())) {
      setEmailError('Podaj poprawny adres e-mail.')
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
    <Form onSubmit={submit} label="Nowe zaproszenie">
      <FormAlert error={create.error} shownFields={['email', 'role']} />
      <div className={styles.inlineForm}>
        <TextField
          label="E-mail"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          autoComplete="off"
          error={emailError ?? server.email}
          required
        />
        <SelectField
          label="Rola"
          value={inviteRole}
          onChange={(event) => setInviteRole(event.target.value as Role)}
          options={roles.map((r) => ({ value: r, label: roleLabels[r] }))}
          error={server.role}
          hint={roleDescriptions[inviteRole]}
        />
        <Button type="submit" variant="primary" icon="link" busy={create.isPending}>
          {canEmail && sendEmail ? 'Utwórz i wyślij' : 'Utwórz link'}
        </Button>
      </div>
      {canEmail && (
        <Checkbox
          label="Wyślij link e-mailem"
          hint="Link i tak zobaczysz po utworzeniu — możesz go też przekazać sam."
          checked={sendEmail}
          onChange={(event) => setSendEmail(event.target.checked)}
        />
      )}
    </Form>
  )
}

function RevokeDialog({ invitation, onDone }: { invitation: Invitation; onDone: () => void }) {
  const { household } = useHousehold()
  const revoke = useRevokeInvitation(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title="Odwołać zaproszenie?"
      confirmLabel="Odwołaj zaproszenie"
      busy={revoke.isPending}
      error={<FormAlert error={revoke.error} />}
      onCancel={onDone}
      onConfirm={() =>
        revoke.mutate(invitation.id, {
          onSuccess: () => {
            toast.show({ message: `Odwołano zaproszenie dla ${invitation.email}.` })
            onDone()
          },
        })
      }
    >
      <p>Link wysłany do {invitation.email} przestanie działać.</p>
    </ConfirmDialog>
  )
}
