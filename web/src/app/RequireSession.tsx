import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, Outlet, useLocation } from 'react-router'
import { classify } from '../api/errors.ts'
import { SessionContext, useMe } from '../api/hooks/session.ts'
import { SectionError } from '../components/QueryState/QueryState.tsx'
import { Skeleton } from '../components/Skeleton/Skeleton.tsx'
import { useProfileLocale } from '../i18n/useProfileLocale.ts'
import { loginUrl } from '../lib/returnTo.ts'

/**
 * Rodzic tras wymagających zalogowanego człowieka. Czeka na GET /me; bez
 * sesji idzie na logowanie z zapamiętaniem adresu. Strony pod nim mogą
 * polegać na useSessionUser().
 */
export function RequireSession() {
  const me = useMe()
  const location = useLocation()
  const user = me.data?.user
  const households = me.data?.households
  const session = useMemo(() => (user && households ? { user, households } : null), [user, households])
  const { t } = useTranslation()
  useProfileLocale(user?.locale)

  if (session) {
    return (
      <SessionContext.Provider value={session}>
        <Outlet />
      </SessionContext.Provider>
    )
  }
  if (me.isPending) {
    return (
      <main id="main" className="plain-main">
        <Skeleton lines={4} label={t('session.checking')} />
      </main>
    )
  }
  if (me.isError && classify(me.error) !== 'unauthenticated') {
    return (
      <main id="main" className="plain-main">
        <SectionError title={t('session.checkFailed')} error={me.error} onRetry={() => void me.refetch()} />
      </main>
    )
  }
  // Brak sesji albo token bota w przeglądarce (bez konta człowieka).
  return <Navigate to={loginUrl(location)} replace />
}
