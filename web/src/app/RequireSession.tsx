import { Navigate, Outlet, useLocation } from 'react-router'
import { classify } from '../api/errors.ts'
import { useMe } from '../api/hooks/session.ts'
import { SectionError } from '../components/QueryState/QueryState.tsx'
import { Skeleton } from '../components/Skeleton/Skeleton.tsx'
import { loginUrl } from '../lib/returnTo.ts'

/**
 * Rodzic tras wymagających zalogowanego człowieka. Czeka na GET /me; bez
 * sesji idzie na logowanie z zapamiętaniem adresu. Strony pod nim mogą
 * polegać na useSessionUser().
 */
export function RequireSession() {
  const me = useMe()
  const location = useLocation()

  if (me.data?.user) {
    return <Outlet />
  }
  if (me.isPending) {
    return (
      <main id="main" className="plain-main">
        <Skeleton lines={4} label="Sprawdzanie sesji…" />
      </main>
    )
  }
  if (me.isError && classify(me.error) !== 'unauthenticated') {
    return (
      <main id="main" className="plain-main">
        <SectionError title="Nie udało się sprawdzić sesji" error={me.error} onRetry={() => void me.refetch()} />
      </main>
    )
  }
  // Brak sesji albo token bota w przeglądarce (bez konta człowieka).
  return <Navigate to={loginUrl(location)} replace />
}
