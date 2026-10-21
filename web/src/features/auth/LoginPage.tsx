import { useState } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { classify, errorMessage } from '../../api/errors.ts'
import { useAuthConfig, useLogin, useMe } from '../../api/hooks/session.ts'
import { Button, buttonClassName } from '../../components/Button/Button.tsx'
import { TextField } from '../../components/Field/Field.tsx'
import { Callout, Form } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { safeReturnTo } from '../../lib/returnTo.ts'
import { AuthLayout } from './AuthLayout.tsx'
import styles from './auth.module.css'

const oidcErrors: Record<string, string> = {
  oidc: 'Logowanie przez dostawcę tożsamości nie powiodło się. Spróbuj ponownie.',
}

export function LoginPage() {
  useDocumentTitle('Logowanie')
  const [params] = useSearchParams()
  const returnTo = safeReturnTo(params.get('return_to'))
  const config = useAuthConfig()
  const me = useMe()

  if (me.data?.user) {
    return <Navigate to={returnTo} replace />
  }

  return (
    <AuthLayout>
      <div className={styles.heading}>
        <h1>Zaloguj się</h1>
        <p>Do swojego budżetu domowego.</p>
      </div>
      {params.get('error') && (
        <Callout tone="warn">{oidcErrors[params.get('error')!] ?? oidcErrors.oidc}</Callout>
      )}
      {config.isPending ? (
        <Skeleton lines={4} />
      ) : (
        <>
          {config.data?.local_login !== false && <LocalLoginForm returnTo={returnTo} />}
          {config.data?.oidc_enabled && (
            <>
              {config.data.local_login && <p className={styles.divider}>albo</p>}
              <a
                className={buttonClassName('secondary', 'md', styles.wide)}
                href={`/api/auth/login?return_to=${encodeURIComponent(returnTo)}`}
              >
                Zaloguj się przez SSO
              </a>
            </>
          )}
          {config.data && !config.data.local_login && !config.data.oidc_enabled && (
            <Callout tone="warn">Na tej instancji nie włączono żadnej metody logowania.</Callout>
          )}
        </>
      )}
    </AuthLayout>
  )
}

function LocalLoginForm({ returnTo }: { returnTo: string }) {
  const login = useLogin()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [missing, setMissing] = useState<{ email?: string; password?: string }>({})

  const submit = () => {
    const next = {
      email: email.trim() ? undefined : 'Podaj adres e-mail.',
      password: password ? undefined : 'Podaj hasło.',
    }
    setMissing(next)
    if (next.email || next.password) {
      return
    }
    login.mutate(
      { email: email.trim(), password },
      {
        onSuccess: () => {
          void navigate(returnTo, { replace: true })
        },
      },
    )
  }

  const failure = login.error
    ? classify(login.error) === 'unauthenticated' || classify(login.error) === 'validation'
      ? 'Nieprawidłowy e-mail lub hasło.'
      : errorMessage(login.error)
    : null

  return (
    <Form onSubmit={submit}>
      {failure && (
        <div role="alert">
          <Callout tone="warn">{failure}</Callout>
        </div>
      )}
      <TextField
        label="E-mail"
        type="email"
        name="email"
        autoComplete="username"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        error={missing.email}
        required
      />
      <TextField
        label="Hasło"
        type="password"
        name="password"
        autoComplete="current-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        error={missing.password}
        required
      />
      <Button type="submit" variant="primary" busy={login.isPending} className={styles.wide}>
        Zaloguj się
      </Button>
    </Form>
  )
}
