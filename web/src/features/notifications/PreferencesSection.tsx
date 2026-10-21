import { useState } from 'react'
import { useChannels, useNotificationPreferences, useSetNotificationPreferences } from '../../api/hooks/notifications.ts'
import type { Channel, EventType, NotificationPreference } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { SectionError } from '../../components/QueryState/QueryState.tsx'
import { Skeleton } from '../../components/Skeleton/Skeleton.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { eventTypeDescriptions, eventTypeLabels, eventTypeOrder } from '../../lib/labels.ts'
import settings from '../settings/settings.module.css'
import styles from './notifications.module.css'

/** Pełna lista preferencji w stałej kolejności typów; brakujące — domyślne (w aplikacji, bez kanałów). */
export function completePreferences(items: NotificationPreference[]): NotificationPreference[] {
  const byType = new Map(items.map((p) => [p.type, p]))
  const known = eventTypeOrder.map((type) => byType.get(type) ?? { type, in_app: true, channel_ids: [] })
  // Typy z nowszego serwera zostają, żeby PUT ich nie skasował.
  const extra = items.filter((p) => !eventTypeOrder.includes(p.type))
  return [...known, ...extra]
}

export function toggleChannel(prefs: NotificationPreference[], type: EventType, channelId: string, on: boolean) {
  return prefs.map((p) =>
    p.type !== type
      ? p
      : {
          ...p,
          channel_ids: on ? [...p.channel_ids.filter((id) => id !== channelId), channelId] : p.channel_ids.filter((id) => id !== channelId),
        },
  )
}

export function toggleInApp(prefs: NotificationPreference[], type: EventType, on: boolean) {
  return prefs.map((p) => (p.type === type ? { ...p, in_app: on } : p))
}

/** Preferencje w bieżącym gospodarstwie: macierz typ zdarzenia × (w aplikacji, kanały). */
export function PreferencesSection() {
  const { household } = useHousehold()
  const prefs = useNotificationPreferences(household.id)
  const channels = useChannels()

  return (
    <section className={settings.section} aria-labelledby="prefs-heading">
      <div className={settings.sectionHead}>
        <div>
          <h2 id="prefs-heading">O czym i gdzie powiadamiać</h2>
          <p>
            Dla gospodarstwa <strong>{household.name}</strong>. W innych gospodarstwach ustawiasz to osobno. Powiadomienia
            o subskrypcjach z prywatnych portfeli dostają tylko osoby, które te portfele widzą.
          </p>
        </div>
      </div>
      {prefs.isPending || channels.isPending ? (
        <Skeleton lines={6} label="Wczytywanie preferencji…" />
      ) : prefs.isError || channels.isError ? (
        <SectionError
          title="Nie udało się wczytać preferencji"
          error={prefs.error ?? channels.error}
          onRetry={() => {
            void prefs.refetch()
            void channels.refetch()
          }}
        />
      ) : (
        // Nowy stan po zapisie albo zmianie kanałów: formularz startuje od danych serwera.
        <PreferencesForm
          key={JSON.stringify(prefs.data) + channels.data.map((c) => c.id).join()}
          initial={completePreferences(prefs.data)}
          channels={channels.data}
        />
      )}
    </section>
  )
}

function PreferencesForm({ initial, channels }: { initial: NotificationPreference[]; channels: Channel[] }) {
  const { household } = useHousehold()
  const save = useSetNotificationPreferences(household.id)
  const toast = useToast()
  const [prefs, setPrefs] = useState(initial)
  const dirty = JSON.stringify(prefs) !== JSON.stringify(initial)
  const known = new Set(channels.map((c) => c.id))

  return (
    <form
      noValidate
      aria-label="Preferencje powiadomień"
      className={styles.prefsForm}
      onSubmit={(event) => {
        event.preventDefault()
        // Kanały usunięte w międzyczasie nie idą do serwera (422).
        const items = prefs.map((p) => ({ ...p, channel_ids: p.channel_ids.filter((id) => known.has(id)) }))
        save.mutate(items, { onSuccess: () => toast.show({ message: 'Zapisano preferencje powiadomień.' }) })
      }}
    >
      <FormAlert error={save.error} />
      {/* Przewijany poziomo przy wielu kanałach; checkboxy są osiągalne klawiaturą. */}
      <div className={styles.matrixWrap}>
        <table className={styles.matrix}>
          <caption className="visually-hidden">Typ zdarzenia i miejsca, do których trafia powiadomienie</caption>
          <thead>
            <tr>
              <th scope="col">Zdarzenie</th>
              <th scope="col">W aplikacji</th>
              {channels.map((channel) => (
                <th scope="col" key={channel.id}>
                  {channel.name}
                  {!channel.enabled && <span className={styles.matrixNote}> (wyłączony)</span>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {prefs
              .filter((p) => eventTypeOrder.includes(p.type))
              .map((p) => (
                <tr key={p.type}>
                  <th scope="row">
                    <span className={styles.matrixType}>{eventTypeLabels[p.type]}</span>
                    <span className={styles.matrixNote}>{eventTypeDescriptions[p.type]}</span>
                  </th>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={`${eventTypeLabels[p.type]}: w aplikacji`}
                      checked={p.in_app}
                      onChange={(event) => setPrefs(toggleInApp(prefs, p.type, event.target.checked))}
                    />
                  </td>
                  {channels.map((channel) => (
                    <td key={channel.id}>
                      <input
                        type="checkbox"
                        aria-label={`${eventTypeLabels[p.type]}: ${channel.name}`}
                        checked={p.channel_ids.includes(channel.id)}
                        onChange={(event) => setPrefs(toggleChannel(prefs, p.type, channel.id, event.target.checked))}
                      />
                    </td>
                  ))}
                </tr>
              ))}
          </tbody>
        </table>
      </div>
      {channels.length === 0 && (
        <p className={styles.formMeta}>Dodaj kanał powyżej, żeby wybrać, które powiadomienia mają trafiać poza aplikację.</p>
      )}
      <div className={styles.prefsActions}>
        <Button type="submit" variant="primary" disabled={!dirty} busy={save.isPending}>
          Zapisz preferencje
        </Button>
        {dirty && (
          <Button onClick={() => setPrefs(initial)} disabled={save.isPending}>
            Cofnij zmiany
          </Button>
        )}
      </div>
    </form>
  )
}
