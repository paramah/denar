import { useEffect, useState } from 'react'
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
  useDocumentTitle('Zaproszenie')
  const token = useInviteToken()
  const preview = useInvitationPreview(token)
  const me = useMe()

  return (
    <AuthLayout>
      {!token ? (
        <InvalidInvitation title="Brak zaproszenia w linku">
          Otwórz pełny link z zaproszenia — zawiera on kod po znaku #.
        </InvalidInvitation>
      ) : preview.isPending || me.isPending ? (
        <Skeleton lines={5} label="Sprawdzanie zaproszenia…" />
      ) : preview.isError ? (
        <PreviewError error={preview.error} onRetry={() => void preview.refetch()} />
      ) : (
        <Invitation token={token} preview={preview.data} signedInAs={me.data?.user?.email} />
      )}
    </AuthLayout>
  )
}

function InvalidInvitation({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <>
      <div className={styles.heading}>
        <h1>{title}</h1>
        <p>{children}</p>
      </div>
      <LinkButton to="/" variant="secondary">
        Przejdź do aplikacji
      </LinkButton>
    </>
  )
}

function PreviewError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const reason = error instanceof ApiError ? error.reason : ''
  if (classify(error) === 'notFound') {
    return (
      <InvalidInvitation title="Zaproszenie nie jest aktywne">
        Link jest nieprawidłowy albo zaproszenie zostało odwołane. Poproś o nowe.
      </InvalidInvitation>
    )
  }
  if (classify(error) === 'conflict') {
    return reason === 'invitation-expired' ? (
      <InvalidInvitation title="Zaproszenie wygasło">
        Linki z zaproszeniem są ważne przez ograniczony czas. Poproś o nowe.
      </InvalidInvitation>
    ) : (
      <InvalidInvitation title="Zaproszenie zostało już użyte">
        Każdy link działa raz. Jeśli to Ty z niego skorzystałeś, zaloguj się.
      </InvalidInvitation>
    )
  }
  return <SectionError title="Nie udało się sprawdzić zaproszenia" error={error} onRetry={onRetry} />
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
  return (
    <>
      <div className={styles.heading}>
        <h1>Dołącz do gospodarstwa {preview.household_name}</h1>
        <p>{roleDescriptions[preview.role]}.</p>
      </div>
      <dl className={styles.facts}>
        <div>
          <dt>Zaproszony adres</dt>
          <dd>{preview.email}</dd>
        </div>
        <div>
          <dt>Rola</dt>
          <dd>{roleLabels[preview.role]}</dd>
        </div>
        <div>
          <dt>Ważne do</dt>
          <dd>{formatDateTime(preview.expires_at)}</dd>
        </div>
      </dl>
      {signedInAs ? (
        <AcceptAsSignedIn token={token} email={signedInAs} />
      ) : preview.account_exists ? (
        <>
          <Callout>Ten adres ma już konto. Zaloguj się, a potem przyjmij zaproszenie.</Callout>
          <LinkButton to="/login?return_to=%2Finvite" variant="primary" className={styles.wide}>
            Zaloguj się, aby dołączyć
          </LinkButton>
        </>
      ) : (
        <RegisterForm token={token} email={preview.email} />
      )}
    </>
  )
}

function AcceptAsSignedIn({ token, email }: { token: string; email: string }) {
  const accept = useAcceptInvitation()
  const navigate = useNavigate()
  const alreadyMember = accept.error instanceof ApiError && accept.error.reason === 'already-member'
  return (
    <>
      <p>
        Jesteś zalogowany jako <strong>{email}</strong>.
      </p>
      {alreadyMember ? (
        <Callout>Już należysz do tego gospodarstwa.</Callout>
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
        Przyjmij zaproszenie
      </Button>
    </>
  )
}

function RegisterForm({ token, email }: { token: string; email: string }) {
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
      name: name.trim() ? undefined : 'Podaj imię lub nazwę.',
      password:
        password.length < 12
          ? 'Hasło musi mieć co najmniej 12 znaków.'
          : password.length > 128
            ? 'Hasło może mieć najwyżej 128 znaków.'
            : undefined,
      repeat: repeat === password ? undefined : 'Hasła się różnią.',
    }
    setLocal(errors)
    if (errors.name || errors.password || errors.repeat) {
      return
    }
    register.mutate(
      { token, name: name.trim(), password },
      {
        onSuccess: (result) => {
          forgetToken()
          void navigate(householdPath(result.household.id), { replace: true })
        },
      },
    )
  }

  return (
    <Form onSubmit={submit} label="Załóż konto">
      <h2>Załóż konto</h2>
      {duplicate ? (
        <Callout tone="warn">Ten adres ma już konto. Zaloguj się, a potem przyjmij zaproszenie.</Callout>
      ) : (
        <FormAlert error={register.error} shownFields={['name', 'password']} />
      )}
      <TextField label="E-mail" value={email} readOnly autoComplete="username" />
      <TextField
        label="Imię lub nazwa"
        value={name}
        onChange={(event) => setName(event.target.value)}
        autoComplete="name"
        error={local.name ?? server.name}
        required
      />
      <TextField
        label="Hasło"
        type="password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        autoComplete="new-password"
        hint="Co najmniej 12 znaków. Najłatwiej zapamiętać kilka słów."
        error={local.password ?? server.password}
        required
      />
      <TextField
        label="Powtórz hasło"
        type="password"
        value={repeat}
        onChange={(event) => setRepeat(event.target.value)}
        autoComplete="new-password"
        error={local.repeat}
        required
      />
      <Button type="submit" variant="primary" busy={register.isPending} className={styles.wide}>
        Załóż konto i dołącz
      </Button>
    </Form>
  )
}
