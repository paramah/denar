import { Link } from 'react-router'
import { useMarkNotificationRead } from '../../api/hooks/notifications.ts'
import type { Notification } from '../../api/types.ts'
import { Icon } from '../../components/Icon/Icon.tsx'
import { intlLocale } from '../../i18n/locale.ts'
import { formatRelative } from '../../lib/format.ts'
import { eventTypeLabels } from '../../lib/labels.ts'
import { formatTime, notificationIcon, notificationLink, notificationTone } from './model.ts'
import styles from './notifications.module.css'

type Props = {
  notification: Notification
  /** Nazwa gospodarstwa, gdy użytkownik ma kilka. */
  householdName?: string
  /** Panel dzwonka: czas względny, bez przycisku „przeczytane”. */
  compact?: boolean
  /** Po kliknięciu w powiadomienie (np. zamknięcie panelu). */
  onOpen?: () => void
}

/**
 * Powiadomienie: cała karta prowadzi do linku w aplikacji i oznacza je jako
 * przeczytane; na liście jest też osobny przycisk „Oznacz jako przeczytane”.
 */
export function NotificationItem({ notification: n, householdName, compact, onOpen }: Props) {
  const markRead = useMarkNotificationRead()
  const unread = n.read_at === null
  const typeLabel = eventTypeLabels[n.type] ?? n.type
  return (
    <article
      className={[styles.item, unread ? styles.unread : '', compact ? styles.compact : ''].filter(Boolean).join(' ')}
      aria-label={n.title}
    >
      <span className={`${styles.icon} ${styles[notificationTone(n.type)]}`}>
        <Icon name={notificationIcon(n.type)} size={compact ? 18 : 20} />
      </span>
      <div className={styles.itemBody}>
        <Link
          to={notificationLink(n)}
          className={styles.itemLink}
          onClick={() => {
            if (unread) {
              markRead.mutate(n)
            }
            onOpen?.()
          }}
        >
          {unread && <span className="visually-hidden">Nieprzeczytane:</span>}
          {unread && ' '}
          {n.title}
        </Link>
        {n.body && <p className={styles.itemText}>{n.body}</p>}
        <p className={styles.itemMeta}>
          <time dateTime={n.created_at} title={new Date(n.created_at).toLocaleString(intlLocale())}>
            {compact ? formatRelative(n.created_at) : formatTime(n.created_at)}
          </time>
          {' · '}
          {typeLabel}
          {householdName && ` · ${householdName}`}
        </p>
      </div>
      {unread && <span className={styles.dot} aria-hidden="true" />}
      {unread && !compact && (
        <button
          type="button"
          className={styles.readButton}
          onClick={() => markRead.mutate(n)}
          disabled={markRead.isPending}
          aria-label={`Oznacz jako przeczytane: ${n.title}`}
          title="Oznacz jako przeczytane"
        >
          <Icon name="check" size={18} />
        </button>
      )}
    </article>
  )
}
