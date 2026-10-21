import { useEffect, useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { ApiError, classify, fieldErrors } from '../../api/errors.ts'
import { useAcceptInvitation, useInvitationPreview, useRegisterWithInvitation } from '../../api/hooks/invitations.ts'
import { useMe } from '../../api/hooks/session.ts'
import type { InvitationPreview } from '../../api/types.ts'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { TextField } from '../../components/Field/Field.tsx'
import { Callout, Form, FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { roleDescriptions, roleLabels } from '../../lib/labels.ts'
import { readStored, storageKeys, writeStored } from '../../lib/storage.ts'
import { AuthLayout } from './AuthLayout.tsx'
import styles from './auth.module.css'

/**
 * Token z fragmentu adresu (#token) — nie trafia do serwera w adresie.
 * Zapamiętany w sessionStorage, żeby przetrwał logowanie (return_to=/invite),
 * a fragment znika z paska adresu.
 */
function useInviteToken(): string | null {
  const [token] = useState(() => {
    const fromHash = window.location.hash.replace(/^#/, '').trim()
    if (fromHash) {
      writeStored(storageKeys.invite, fromHash, 'session')
      return fromHash
    }
    return readStored(storageKeys.invite, 'session')
  })
  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search)
    }
  }, [])
  return token
}

function forgetToken() {
  writeStored(storageKeys.invite, null, 'session')
}

export function InvitePage() {
  const { t } = useTranslation('auth')
  useDocumentTitle(t('invite.documentTitle'))
  const token = useInviteToken()
  const preview = useInvitationPreview(token)
  const me = useMe()

  return (
    <AuthLayout>
      {!token ? (
        <InvalidInvitation title={t('invite.missingTitle')}>{t('invite.missingBody')}</InvalidInvitation>
      ) : preview.isPending || me.isPending ? (
        <Skeleton lines={5} label={t('invite.checking')} />
      ) : preview.isError ? (
        <PreviewError error={preview.error} onRetry={() => void preview.refetch()} />
      ) : (
        <Invitation token={token} preview={preview.data} signedInAs={me.data?.user?.email} />
      )}
    </AuthLayout>
  )
}

function InvalidInvitation({ title, children }: { title: string; children: React.ReactNode }) {
  const { t } = useTranslation('auth')
  return (
    <>
      <div className={styles.heading}>
        <h1>{title}</h1>
        <p>{children}</p>
      </div>
      <LinkButton to="/" variant="secondary">
        {t('invite.toApp')}
      </LinkButton>
    </>
  )
}

function PreviewError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation('auth')
  const reason = error instanceof ApiError ? error.reason : ''
  if (classify(error) === 'notFound') {
    return (
      <InvalidInvitation title={t('invite.inactiveTitle')}>{t('invite.inactiveBody')}</InvalidInvitation>
    )
  }
  if (classify(error) === 'conflict') {
    return reason === 'invitation-expired' ? (
      <InvalidInvitation title={t('invite.expiredTitle')}>{t('invite.expiredBody')}</InvalidInvitation>
    ) : (
      <InvalidInvitation title={t('invite.usedTitle')}>{t('invite.usedBody')}</InvalidInvitation>
    )
  }
  return <SectionError title={t('invite.checkFailed')} error={error} onRetry={onRetry} />
}

function Invitation({
  token,
  preview,
  signedInAs,
}: {
  token: string
  preview: InvitationPreview
  signedInAs?: string
}) {
  const { t } = useTranslation('auth')
  return (
    <>
      <div className={styles.heading}>
        <h1>{t('invite.title', { name: preview.household_name })}</h1>
        <p>{roleDescriptions[preview.role]}.</p>
      </div>
      <dl className={styles.facts}>
        <div>
          <dt>{t('invite.email')}</dt>
          <dd>{preview.email}</dd>
        </div>
        <div>
          <dt>{t('invite.role')}</dt>
          <dd>{roleLabels[preview.role]}</dd>
        </div>
        <div>
          <dt>{t('invite.expires')}</dt>
          <dd>{formatDateTime(preview.expires_at)}</dd>
        </div>
      </dl>
      {signedInAs ? (
        <AcceptAsSignedIn token={token} email={signedInAs} />
      ) : preview.account_exists ? (
        <>
          <Callout>{t('invite.accountExists')}</Callout>
          <LinkButton to="/login?return_to=%2Finvite" variant="primary" className={styles.wide}>
            {t('invite.signInToJoin')}
          </LinkButton>
        </>
      ) : (
        <RegisterForm token={token} email={preview.email} />
      )}
    </>
  )
}

function AcceptAsSignedIn({ token, email }: { token: string; email: string }) {
  const { t } = useTranslation('auth')
  const accept = useAcceptInvitation()
  const navigate = useNavigate()
  const alreadyMember = accept.error instanceof ApiError && accept.error.reason === 'already-member'
  return (
    <>
      <p>
        <Trans t={t} i18nKey="invite.signedInAs" values={{ email }} components={{ strong: <strong /> }} />
      </p>
      {alreadyMember ? (
        <Callout>{t('invite.alreadyMember')}</Callout>
      ) : (
        <FormAlert error={accept.error} />
      )}
      <Button
        variant="primary"
        className={styles.wide}
        busy={accept.isPending}
        onClick={() =>
          accept.mutate(token, {
            onSuccess: (household) => {
              forgetToken()
              void navigate(householdPath(household.id), { replace: true })
            },
          })
        }
      >
        {t('invite.accept')}
      </Button>
    </>
  )
}

function RegisterForm({ token, email }: { token: string; email: string }) {
  const { t } = useTranslation('auth')
  const register = useRegisterWithInvitation()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [repeat, setRepeat] = useState('')
  const [local, setLocal] = useState<Record<string, string | undefined>>({})
  const server = fieldErrors(register.error)
  const duplicate = register.error instanceof ApiError && register.error.reason === 'duplicate-email'

  const submit = () => {
    const errors = {
      name: name.trim() ? undefined : t('invite.nameMissing'),
      password:
        password.length < 12
          ? t('invite.passwordShort')
          : password.length > 128
            ? t('invite.passwordLong')
            : undefined,
      repeat: repeat === password ? undefined : t('invite.passwordMismatch'),
    }
    setLocal(errors)
    if (errors.name || errors.password || errors.repeat) {
      return
    }
    // mutateAsync, nie callback mutate: po rejestracji /me ma już użytkownika
    // i strona przełącza się na widok zalogowanego, odmontowując ten formularz.
    register
      .mutateAsync({ token, name: name.trim(), password })
      .then((result) => {
        forgetToken()
        void navigate(householdPath(result.household.id), { replace: true })
      })
      .catch(() => undefined)
  }

  return (
    <Form onSubmit={submit} label={t('invite.register')}>
      <h2>{t('invite.register')}</h2>
      {duplicate ? (
        <Callout tone="warn">{t('invite.accountExists')}</Callout>
      ) : (
        <FormAlert error={register.error} shownFields={['name', 'password']} />
      )}
      <TextField label={t('login.email')} value={email} readOnly autoComplete="username" />
      <TextField
        label={t('invite.name')}
        value={name}
        onChange={(event) => setName(event.target.value)}
        autoComplete="name"
        error={local.name ?? server.name}
        required
      />
      <TextField
        label={t('login.password')}
        type="password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        autoComplete="new-password"
        hint={t('invite.passwordHint')}
        error={local.password ?? server.password}
        required
      />
      <TextField
        label={t('invite.repeat')}
        type="password"
        value={repeat}
        onChange={(event) => setRepeat(event.target.value)}
        autoComplete="new-password"
        error={local.repeat}
        required
      />
      <Button type="submit" variant="primary" busy={register.isPending} className={styles.wide}>
        {t('invite.submit')}
      </Button>
    </Form>
  )
}
