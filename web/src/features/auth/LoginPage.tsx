import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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

export function LoginPage() {
  const { t } = useTranslation('auth')
  useDocumentTitle(t('login.documentTitle'))
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
        <h1>{t('login.title')}</h1>
        <p>{t('login.subtitle')}</p>
      </div>
      {params.get('error') && (
        // Jedyny znany kod błędu OIDC to `oidc`; każdy inny opisujemy tak samo.
        <Callout tone="warn">{t('login.oidcFailed')}</Callout>
      )}
      {config.isPending ? (
        <Skeleton lines={4} />
      ) : (
        <>
          {config.data?.local_login !== false && <LocalLoginForm returnTo={returnTo} />}
          {config.data?.oidc_enabled && (
            <>
              {config.data.local_login && <p className={styles.divider}>{t('login.or')}</p>}
              <a
                className={buttonClassName('secondary', 'md', styles.wide)}
                href={`/api/auth/login?return_to=${encodeURIComponent(returnTo)}`}
              >
                {t('login.sso')}
              </a>
            </>
          )}
          {config.data && !config.data.local_login && !config.data.oidc_enabled && (
            <Callout tone="warn">{t('login.noMethod')}</Callout>
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
  const { t } = useTranslation('auth')

  const submit = () => {
    const next = {
      email: email.trim() ? undefined : t('login.emailMissing'),
      password: password ? undefined : t('login.passwordMissing'),
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
      ? t('login.badCredentials')
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
        label={t('login.email')}
        type="email"
        name="email"
        autoComplete="username"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        error={missing.email}
        required
      />
      <TextField
        label={t('login.password')}
        type="password"
        name="password"
        autoComplete="current-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        error={missing.password}
        required
      />
      <Button type="submit" variant="primary" busy={login.isPending} className={styles.wide}>
        {t('login.submit')}
      </Button>
    </Form>
  )
}
