import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { useRecentNotifications } from '../../api/hooks/notifications.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { NotificationItem } from './NotificationItem.tsx'
import styles from './notifications.module.css'

export const dashboardNotifications = 3

/** Pulpit: ostatnie powiadomienia z tego gospodarstwa; bez powiadomień — nic. */
export function RecentNotificationsTile() {
  const { t } = useTranslation('notifications')
  const { household } = useHousehold()
  const recent = useRecentNotifications(dashboardNotifications, { householdId: household.id })
  if (!recent.data || recent.data.length === 0) {
    return null
  }
  return (
    <section className={styles.tile} aria-labelledby="dash-notifications">
      <div className={styles.tileHead}>
        <h2 id="dash-notifications">{t('tile.heading')}</h2>
        <Link to={householdPath(household.id, 'notifications')}>{t('tile.all')}</Link>
      </div>
      <ul className={styles.panelList} aria-label={t('tile.listLabel')}>
        {recent.data.map((n) => (
          <li key={n.id}>
            <NotificationItem notification={n} compact />
          </li>
        ))}
      </ul>
    </section>
  )
}
