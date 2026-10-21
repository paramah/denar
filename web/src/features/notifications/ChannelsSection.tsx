import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { errorMessage } from '../../api/errors.ts'
import { translateDeliveryError } from '../../api/messages.ts'
import {
  useChannels,
  useChannelTypes,
  useDeleteChannel,
  useTestChannel,
  useUpdateChannel,
} from '../../api/hooks/notifications.ts'
import type { Channel, ChannelTestResult, ChannelTypeInfo } from '../../api/types.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { ConfirmDialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { formatDateTime } from '../../lib/format.ts'
import { channelTypeLabels, unavailableReasonLabels } from '../../lib/labels.ts'
import settings from '../settings/settings.module.css'
import { ChannelDialog } from './ChannelDialog.tsx'
import { channelSummary } from './channelForm.ts'
import styles from './notifications.module.css'

export const maxChannels = 20

/** Kanały osoby: e-mail, webhook, ntfy, Telegram, Matrix — wspólne dla wszystkich gospodarstw. */
export function ChannelsSection() {
  const { t } = useTranslation('notifications')
  const channels = useChannels()
  const types = useChannelTypes()
  const [editing, setEditing] = useState<Channel | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Channel | null>(null)
  const items = channels.data ?? []
  const unavailable = (types.data ?? []).filter((ct) => !ct.available)

  return (
    <section className={settings.section} aria-labelledby="channels-heading">
      <div className={settings.sectionHead}>
        <div>
          <h2 id="channels-heading">{t('channels.heading')}</h2>
          <p>{t('channels.intro')}</p>
        </div>
        <Button
          variant="primary"
          icon="plus"
          disabled={!types.data || items.length >= maxChannels}
          onClick={() => setEditing('new')}
        >
          {t('channels.add')}
        </Button>
      </div>
      {unavailable.length > 0 && (
        <ul className={styles.unavailable} aria-label={t('channels.unavailableList')}>
          {unavailable.map((ct) => (
            <li key={ct.type}>
              <Trans
                t={t}
                i18nKey="channels.unavailable"
                values={{
                  type: channelTypeLabels[ct.type],
                  reason: unavailableReasonLabels[ct.unavailable_reason ?? ''] ?? t('channels.unsupported'),
                }}
              />
            </li>
          ))}
        </ul>
      )}
      {channels.isPending || types.isPending ? (
        <Skeleton lines={3} label={t('channels.loading')} />
      ) : channels.isError || types.isError ? (
        <SectionError
          title={t('channels.loadFailed')}
          error={channels.error ?? types.error}
          onRetry={() => {
            void channels.refetch()
            void types.refetch()
          }}
        />
      ) : items.length === 0 ? (
        <EmptyState
          icon="send"
          title={t('channels.emptyTitle')}
          headingLevel={3}
          action={
            <Button icon="plus" onClick={() => setEditing('new')}>
              {t('channels.addFirst')}
            </Button>
          }
        >
          <p>{t('channels.emptyBody')}</p>
        </EmptyState>
      ) : (
        <ul className={styles.channels} aria-label={t('channels.listLabel')}>
          {items.map((channel) => (
            <li key={channel.id}>
              <ChannelRow
                channel={channel}
                info={types.data.find((ct) => ct.type === channel.type)}
                onEdit={() => setEditing(channel)}
                onDelete={() => setDeleting(channel)}
              />
            </li>
          ))}
        </ul>
      )}
      {types.data && (
        <ChannelDialog
          open={editing !== null}
          channel={editing === 'new' ? null : editing}
          types={types.data}
          onClose={() => setEditing(null)}
        />
      )}
      {deleting && <DeleteChannelDialog channel={deleting} onDone={() => setDeleting(null)} />}
    </section>
  )
}

function ChannelRow({
  channel,
  info,
  onEdit,
  onDelete,
}: {
  channel: Channel
  info: ChannelTypeInfo | undefined
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation('notifications')
  const test = useTestChannel()
  const update = useUpdateChannel()
  const toast = useToast()
  const [result, setResult] = useState<ChannelTestResult | null>(null)
  const summary = channelSummary(channel)
  const headingId = `channel-${channel.id}`

  return (
    <article className={styles.channel} aria-labelledby={headingId}>
      <div className={styles.channelMain}>
        <h3 id={headingId} className={styles.channelName}>
          {channel.name}
        </h3>
        <p className={styles.channelMeta}>
          {channelTypeLabels[channel.type] ?? channel.type}
          {summary && <> · <span className={styles.channelTarget}>{summary}</span></>}
          {channel.has_secret && t('channels.withSecret')}
        </p>
        <div className={styles.channelBadges}>
          {channel.enabled ? <Badge tone="positive">{t('channels.enabled')}</Badge> : <Badge>{t('channels.disabled')}</Badge>}
          {channel.verified_at ? (
            <Badge tone="brand" title={t('channels.verifiedTitle', { date: formatDateTime(channel.verified_at) })}>
              {t('channels.verified', { date: formatDateTime(channel.verified_at) })}
            </Badge>
          ) : (
            <Badge tone="warn">{t('channels.unverified')}</Badge>
          )}
          {info && !info.available && (
            <Badge tone="negative">{t('channels.unavailableOnServer')}</Badge>
          )}
        </div>
        {channel.last_error && (
          <p className={styles.channelError}>
            {channel.last_error_at
              ? t('channels.lastErrorAt', {
                  date: formatDateTime(channel.last_error_at),
                  error: translateDeliveryError(channel.last_error),
                })
              : t('channels.lastError', { error: translateDeliveryError(channel.last_error) })}
          </p>
        )}
        <div role="status" className={styles.testResult}>
          {result &&
            (result.ok ? (
              <span className={styles.testOk}>{t('channels.testOk')}</span>
            ) : (
              <span className={styles.testFail}>{t('channels.testFailed', { error: translateDeliveryError(result.error) })}</span>
            ))}
        </div>
      </div>
      <div className={styles.channelActions}>
        <Button
          size="sm"
          icon="send"
          busy={test.isPending}
          aria-label={t('channels.testNamed', { name: channel.name })}
          onClick={() =>
            test.mutate(channel.id, {
              onSuccess: setResult,
              onError: (error) => toast.show({ message: errorMessage(error), kind: 'error' }),
            })
          }
        >
          {t('channels.test')}
        </Button>
        <Button
          size="sm"
          icon={channel.enabled ? 'pause' : 'play'}
          busy={update.isPending}
          aria-label={
            channel.enabled
              ? t('channels.disableNamed', { name: channel.name })
              : t('channels.enableNamed', { name: channel.name })
          }
          onClick={() =>
            update.mutate(
              { id: channel.id, body: { enabled: !channel.enabled } },
              {
                onSuccess: (saved) =>
                  toast.show({
                    message: saved.enabled
                      ? t('channels.enabledToast', { name: saved.name })
                      : t('channels.disabledToast', { name: saved.name }),
                  }),
                onError: (error) => toast.show({ message: errorMessage(error), kind: 'error' }),
              },
            )
          }
        >
          {channel.enabled ? t('channels.disable') : t('channels.enable')}
        </Button>
        <Button size="sm" icon="edit" aria-label={t('channels.editNamed', { name: channel.name })} onClick={onEdit}>
          {t('channels.edit')}
        </Button>
        <Button
          size="sm"
          variant="danger"
          icon="trash"
          aria-label={t('channels.deleteNamed', { name: channel.name })}
          onClick={onDelete}
        >
          {t('channels.delete')}
        </Button>
      </div>
    </article>
  )
}

function DeleteChannelDialog({ channel, onDone }: { channel: Channel; onDone: () => void }) {
  const { t } = useTranslation('notifications')
  const remove = useDeleteChannel()
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={t('channels.deleteTitle')}
      confirmLabel={t('channels.deleteConfirm')}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(channel.id, {
          onSuccess: () => {
            toast.show({ message: t('channels.deletedToast', { name: channel.name }) })
            onDone()
          },
        })
      }
    >
      <p>{t('channels.deleteBody', { name: channel.name })}</p>
    </ConfirmDialog>
  )
}
