import { useState } from 'react'
import { errorMessage } from '../../api/errors.ts'
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
  const channels = useChannels()
  const types = useChannelTypes()
  const [editing, setEditing] = useState<Channel | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Channel | null>(null)
  const items = channels.data ?? []
  const unavailable = (types.data ?? []).filter((t) => !t.available)

  return (
    <section className={settings.section} aria-labelledby="channels-heading">
      <div className={settings.sectionHead}>
        <div>
          <h2 id="channels-heading">Kanały</h2>
          <p>
            Gdzie poza aplikacją wysyłać powiadomienia. Kanały są Twoje — działają we wszystkich Twoich gospodarstwach,
            a to, co na nie trafia, wybierasz niżej.
          </p>
        </div>
        <Button
          variant="primary"
          icon="plus"
          disabled={!types.data || items.length >= maxChannels}
          onClick={() => setEditing('new')}
        >
          Dodaj kanał
        </Button>
      </div>
      {unavailable.length > 0 && (
        <ul className={styles.unavailable} aria-label="Niedostępne typy kanałów">
          {unavailable.map((t) => (
            <li key={t.type}>
              <strong>{channelTypeLabels[t.type]}</strong> niedostępny —{' '}
              {unavailableReasonLabels[t.unavailable_reason ?? ''] ?? 'serwer go nie obsługuje.'}
            </li>
          ))}
        </ul>
      )}
      {channels.isPending || types.isPending ? (
        <Skeleton lines={3} label="Wczytywanie kanałów…" />
      ) : channels.isError || types.isError ? (
        <SectionError
          title="Nie udało się wczytać kanałów"
          error={channels.error ?? types.error}
          onRetry={() => {
            void channels.refetch()
            void types.refetch()
          }}
        />
      ) : items.length === 0 ? (
        <EmptyState
          icon="send"
          title="Nie masz jeszcze kanałów"
          headingLevel={3}
          action={
            <Button icon="plus" onClick={() => setEditing('new')}>
              Dodaj pierwszy kanał
            </Button>
          }
        >
          <p>Powiadomienia i tak trafiają do dzwonka w aplikacji. Kanał doda e-mail, push z ntfy, Telegram, Matrix albo webhook.</p>
        </EmptyState>
      ) : (
        <ul className={styles.channels} aria-label="Twoje kanały">
          {items.map((channel) => (
            <li key={channel.id}>
              <ChannelRow
                channel={channel}
                info={types.data.find((t) => t.type === channel.type)}
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
          {channel.has_secret && ' · z sekretem'}
        </p>
        <div className={styles.channelBadges}>
          {channel.enabled ? <Badge tone="positive">Włączony</Badge> : <Badge>Wyłączony</Badge>}
          {channel.verified_at ? (
            <Badge tone="brand" title={`Ostatnia udana wiadomość: ${formatDateTime(channel.verified_at)}`}>
              Zweryfikowany {formatDateTime(channel.verified_at)}
            </Badge>
          ) : (
            <Badge tone="warn">Niezweryfikowany</Badge>
          )}
          {info && !info.available && (
            <Badge tone="negative">Niedostępny na serwerze</Badge>
          )}
        </div>
        {channel.last_error && (
          <p className={styles.channelError}>
            Ostatni błąd{channel.last_error_at && ` (${formatDateTime(channel.last_error_at)})`}: {channel.last_error}
          </p>
        )}
        <div role="status" className={styles.testResult}>
          {result &&
            (result.ok ? (
              <span className={styles.testOk}>Test wysłany — sprawdź, czy wiadomość dotarła.</span>
            ) : (
              <span className={styles.testFail}>Test nieudany: {result.error ?? 'nieznany błąd'}</span>
            ))}
        </div>
      </div>
      <div className={styles.channelActions}>
        <Button
          size="sm"
          icon="send"
          busy={test.isPending}
          aria-label={`Wyślij test: ${channel.name}`}
          onClick={() =>
            test.mutate(channel.id, {
              onSuccess: setResult,
              onError: (error) => toast.show({ message: errorMessage(error), kind: 'error' }),
            })
          }
        >
          Wyślij test
        </Button>
        <Button
          size="sm"
          icon={channel.enabled ? 'pause' : 'play'}
          busy={update.isPending}
          aria-label={`${channel.enabled ? 'Wyłącz' : 'Włącz'}: ${channel.name}`}
          onClick={() =>
            update.mutate(
              { id: channel.id, body: { enabled: !channel.enabled } },
              {
                onSuccess: (saved) =>
                  toast.show({ message: saved.enabled ? `Włączono kanał ${saved.name}.` : `Wyłączono kanał ${saved.name}.` }),
                onError: (error) => toast.show({ message: errorMessage(error), kind: 'error' }),
              },
            )
          }
        >
          {channel.enabled ? 'Wyłącz' : 'Włącz'}
        </Button>
        <Button size="sm" icon="edit" aria-label={`Edytuj: ${channel.name}`} onClick={onEdit}>
          Edytuj
        </Button>
        <Button size="sm" variant="danger" icon="trash" aria-label={`Usuń: ${channel.name}`} onClick={onDelete}>
          Usuń
        </Button>
      </div>
    </article>
  )
}

function DeleteChannelDialog({ channel, onDone }: { channel: Channel; onDone: () => void }) {
  const remove = useDeleteChannel()
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title="Usunąć kanał?"
      confirmLabel="Usuń kanał"
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(channel.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto kanał ${channel.name}.` })
            onDone()
          },
        })
      }
    >
      <p>
        Kanał {channel.name} zniknie ze wszystkich preferencji, a niewysłane jeszcze powiadomienia na niego przepadną.
      </p>
    </ConfirmDialog>
  )
}
