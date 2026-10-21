import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { useMarkAllNotificationsRead, useNotifications, useUnreadCount } from '../../api/hooks/notifications.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import { useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Button, LinkButton } from '../../components/Button/Button.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Checkbox, SelectField } from '../../components/Field/Field.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { errorMessage } from '../../api/errors.ts'
import { groupByDay } from './model.ts'
import { unreadLabel, useHouseholdNames } from './NotificationBell.tsx'
import { NotificationItem } from './NotificationItem.tsx'
import styles from './notifications.module.css'

/**
 * Skrzynka powiadomień: od najnowszych, pogrupowane po dniu, z filtrem
 * nieprzeczytanych i (przy kilku gospodarstwach) gospodarstwa.
 */
export function NotificationsPage() {
  const { t } = useTranslation('notifications')
  const { household } = useHousehold()
  const { households } = useSessionUser()
  const [params, setParams] = useSearchParams()
  const unreadOnly = params.get('unread') === '1'
  const householdFilter = params.get('household') ?? ''
  const names = useHouseholdNames()
  const list = useNotifications({ householdId: householdFilter || undefined, unread: unreadOnly })
  const unread = useUnreadCount()
  const markAll = useMarkAllNotificationsRead()
  const toast = useToast()
  const items = list.data?.pages.flatMap((page) => page.items) ?? []
  const groups = groupByDay(items)
  const anyUnread = (unread.data ?? 0) > 0 || items.some((n) => n.read_at === null)

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  return (
    <>
      <PageHeader
        title={t('page.title')}
        description={
          unread.data !== undefined
            ? t('page.descriptionUnread', { unread: unreadLabel(unread.data) })
            : t('page.description')
        }
        actions={
          <>
            <Button
              icon="check"
              disabled={!anyUnread}
              busy={markAll.isPending}
              onClick={() =>
                markAll.mutate(householdFilter || undefined, {
                  onSuccess: (updated) =>
                    toast.show({ message: updated > 0 ? t('page.markedRead') : t('page.allWereRead') }),
                  onError: (error) => toast.show({ message: errorMessage(error), kind: 'error' }),
                })
              }
            >
              {t('page.markAll')}
            </Button>
            <LinkButton icon="settings" to={householdPath(household.id, 'settings/notifications')}>
              {t('page.settings')}
            </LinkButton>
          </>
        }
      />
      <div className={styles.filters} role="group" aria-label={t('page.filters')}>
        <Checkbox
          label={t('page.unreadOnly')}
          checked={unreadOnly}
          onChange={(event) => setParam('unread', event.target.checked ? '1' : null)}
        />
        {households.length > 1 && (
          <SelectField
            label={t('page.household')}
            value={householdFilter}
            onChange={(event) => setParam('household', event.target.value || null)}
            options={[{ value: '', label: t('page.allHouseholds') }, ...households.map((h) => ({ value: h.id, label: h.name }))]}
          />
        )}
      </div>
      {list.isPending ? (
        <Skeleton lines={6} label={t('page.loading')} />
      ) : list.isError && items.length === 0 ? (
        <SectionError title={t('page.loadFailed')} error={list.error} onRetry={() => void list.refetch()} />
      ) : items.length === 0 ? (
        unreadOnly ? (
          <EmptyState icon="checkCircle" title={t('page.allReadTitle')}>
            <p>{t('page.allReadBody')}</p>
          </EmptyState>
        ) : (
          <EmptyState
            icon="bell"
            title={t('page.emptyTitle')}
            action={
              <LinkButton variant="primary" icon="settings" to={householdPath(household.id, 'settings/notifications')}>
                {t('page.emptyAction')}
              </LinkButton>
            }
          >
            <p>{t('page.emptyBody')}</p>
          </EmptyState>
        )
      ) : (
        <div className={styles.days}>
          {groups.map((group) => (
            <section key={group.day} className={styles.day} aria-labelledby={`day-${group.day}`}>
              <h2 id={`day-${group.day}`} className={styles.dayHeading}>
                {group.label}
              </h2>
              <ul className={styles.dayList} aria-label={t('page.dayList', { day: group.label })}>
                {group.items.map((n) => (
                  <li key={n.id}>
                    <NotificationItem notification={n} householdName={names?.get(n.household_id)} />
                  </li>
                ))}
              </ul>
            </section>
          ))}
          {list.hasNextPage && (
            <div className={styles.more}>
              <Button busy={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>
                {t('page.older')}
              </Button>
            </div>
          )}
        </div>
      )}
    </>
  )
}
