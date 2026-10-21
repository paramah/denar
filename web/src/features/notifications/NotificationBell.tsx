import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef } from 'react'
import { Link } from 'react-router'
import { useMarkAllNotificationsRead, useRecentNotifications, useUnreadCount } from '../../api/hooks/notifications.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import { keys } from '../../api/keys.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Icon } from '../../components/Icon/Icon.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { errorMessage } from '../../api/errors.ts'
import { useDisclosure } from '../../lib/useDisclosure.ts'
import { NotificationItem } from './NotificationItem.tsx'
import styles from './notifications.module.css'

export const bellPanelSize = 8

/** „3 nieprzeczytane”, „1 nieprzeczytane”, „5 nieprzeczytanych”. */
export function unreadLabel(count: number): string {
  if (count === 0) return 'brak nieprzeczytanych'
  const tens = count % 100
  const units = count % 10
  const few = units >= 2 && units <= 4 && (tens < 12 || tens > 14)
  return `${count} ${count === 1 || few ? 'nieprzeczytane' : 'nieprzeczytanych'}`
}

/** Mapa id gospodarstwa → nazwa, gdy użytkownik ma ich kilka (inaczej nazwa jest zbędna). */
export function useHouseholdNames(): Map<string, string> | null {
  const { households } = useSessionUser()
  return households.length > 1 ? new Map(households.map((h) => [h.id, h.name])) : null
}

/**
 * Dzwonek z licznikiem nieprzeczytanych (odpytywanie co minutę i przy powrocie
 * do okna) i panelem ostatnich powiadomień.
 */
export function NotificationBell({ placement }: { placement: 'rail' | 'topbar' }) {
  const { household } = useHousehold()
  const client = useQueryClient()
  const unread = useUnreadCount()
  const count = unread.data ?? 0
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const panel = useDisclosure(containerRef, buttonRef)
  const panelId = useId()
  const headingId = useId()
  const names = useHouseholdNames()
  const recent = useRecentNotifications(bellPanelSize, { enabled: panel.open })
  const markAll = useMarkAllNotificationsRead()

  // Nowe powiadomienie (licznik się zmienił): odśwież otwarte listy.
  const previous = useRef<number | undefined>(undefined)
  useEffect(() => {
    if (unread.data === undefined) return
    if (previous.current !== undefined && previous.current !== unread.data) {
      void client.invalidateQueries({ queryKey: [...keys.notifications(), 'list'] })
    }
    previous.current = unread.data
  }, [unread.data, client])

  const badge = count > 99 ? '99+' : String(count)

  return (
    <div className={`${styles.bell} ${styles[placement]}`} ref={containerRef}>
      <button
        ref={buttonRef}
        type="button"
        className={styles.bellButton}
        aria-expanded={panel.open}
        aria-controls={panelId}
        onClick={panel.toggle}
        title="Powiadomienia"
      >
        <Icon name="bell" size={22} />
        {count > 0 && (
          <span className={styles.count} aria-hidden="true">
            {badge}
          </span>
        )}
        <span className="visually-hidden">Powiadomienia, {unreadLabel(count)}</span>
      </button>
      {panel.open && (
        <section id={panelId} className={styles.panel} aria-labelledby={headingId}>
          <div className={styles.panelHead}>
            <h2 id={headingId}>Powiadomienia</h2>
            <button
              type="button"
              className={styles.textButton}
              disabled={count === 0 || markAll.isPending}
              onClick={() => markAll.mutate(undefined)}
            >
              Oznacz wszystkie jako przeczytane
            </button>
          </div>
          {markAll.isError && (
            <p className={styles.panelError} role="alert">
              {errorMessage(markAll.error)}
            </p>
          )}
          <div className={styles.panelBody}>
            {recent.isPending ? (
              <Skeleton lines={3} />
            ) : recent.isError ? (
              <p className={styles.panelEmpty}>{errorMessage(recent.error)}</p>
            ) : recent.data.length === 0 ? (
              <p className={styles.panelEmpty}>Nie masz powiadomień. Damy znać o odnowieniach, końcach okresów próbnych i przekroczonych kopertach.</p>
            ) : (
              <ul className={styles.panelList} aria-label="Ostatnie powiadomienia">
                {recent.data.map((n) => (
                  <li key={n.id}>
                    <NotificationItem
                      notification={n}
                      compact
                      householdName={names?.get(n.household_id)}
                      onOpen={() => panel.close()}
                    />
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className={styles.panelFoot}>
            <Link to={householdPath(household.id, 'notifications')} onClick={() => panel.close()}>
              Wszystkie powiadomienia
            </Link>
            <Link to={householdPath(household.id, 'settings/notifications')} onClick={() => panel.close()}>
              Ustawienia
            </Link>
          </div>
        </section>
      )}
    </div>
  )
}
