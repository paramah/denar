import { useState } from 'react'
import { fieldErrors } from '../../api/errors.ts'
import { useCreateChannel, useUpdateChannel } from '../../api/hooks/notifications.ts'
import { useSessionUser } from '../../api/hooks/session.ts'
import type { Channel, ChannelType, ChannelTypeInfo } from '../../api/types.ts'
import { Button } from '../../components/Button/Button.tsx'
import { Dialog } from '../../components/Dialog/Dialog.tsx'
import { Checkbox, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { channelTypeLabels, unavailableReasonLabels } from '../../lib/labels.ts'
import {
  canRemoveSecrets,
  channelGuides,
  channelTypeDescriptions,
  createBody,
  describeField,
  emptyForm,
  formFromChannel,
  secretsAvailable,
  updateBody,
  validateForm,
  type ChannelFormState,
  type SecretMode,
} from './channelForm.ts'
import styles from './notifications.module.css'

type Props = {
  open: boolean
  /** Edytowany kanał; bez niego — nowy. */
  channel: Channel | null
  types: ChannelTypeInfo[]
  onClose: () => void
}

export function ChannelDialog({ open, channel, types, onClose }: Props) {
  return (
    <Dialog
      open={open}
      title={channel ? `Edycja kanału ${channel.name}` : 'Nowy kanał powiadomień'}
      onClose={onClose}
      size="lg"
    >
      <ChannelForm channel={channel} types={types} onClose={onClose} />
    </Dialog>
  )
}

function firstAvailable(types: ChannelTypeInfo[]): ChannelType {
  return (types.find((t) => t.available) ?? types[0])?.type ?? 'webhook'
}

function ChannelForm({ channel, types, onClose }: Omit<Props, 'open'>) {
  const { user } = useSessionUser()
  const toast = useToast()
  const create = useCreateChannel()
  const update = useUpdateChannel()
  const mutation = channel ? update : create
  const [form, setForm] = useState<ChannelFormState>(() =>
    channel ? formFromChannel(channel) : emptyForm(firstAvailable(types)),
  )
  const [errors, setErrors] = useState<Record<string, string>>({})
  const info = types.find((t) => t.type === form.type)
  const server = fieldErrors(mutation.error)
  const canSecrets = secretsAvailable(types)

  if (!info) {
    return <p>Ten serwer nie obsługuje kanału typu {form.type}.</p>
  }

  const set = (patch: Partial<ChannelFormState>) => setForm((f) => ({ ...f, ...patch }))
  const error = (key: string) => errors[key] ?? server[key]
  const showSecretInputs = !channel || form.secretMode === 'change'
  const shownFields = [
    'name',
    'type',
    'enabled',
    ...info.config_fields.map((f) => `config.${f.name}`),
    ...info.secret_fields.map((f) => `secrets.${f.name}`),
  ]

  const submit = async () => {
    const found = validateForm(form, info, channel)
    setErrors(found)
    if (Object.keys(found).length > 0) return
    try {
      if (channel) {
        const body = updateBody(form, info, channel)
        if (Object.keys(body).length === 0) {
          onClose()
          return
        }
        const saved = await update.mutateAsync({ id: channel.id, body })
        toast.show({ message: `Zapisano kanał ${saved.name}.` })
      } else {
        const saved = await create.mutateAsync(createBody(form, info))
        toast.show({ message: `Dodano kanał ${saved.name}. Wyślij test, żeby sprawdzić, czy działa.` })
      }
      onClose()
    } catch {
      // Błąd pokazuje formularz.
    }
  }

  return (
    <form
      noValidate
      aria-label={channel ? 'Edycja kanału' : 'Nowy kanał'}
      className={styles.form}
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <FormAlert error={mutation.error} shownFields={shownFields} />
      {channel ? (
        <p className={styles.formMeta}>
          Typ: <strong>{channelTypeLabels[channel.type]}</strong> — typu kanału nie da się zmienić.
        </p>
      ) : (
        <fieldset className={styles.fieldset}>
          <legend>Typ kanału</legend>
          <div className={styles.typeGrid}>
            {types.map((t) => (
              <label
                key={t.type}
                htmlFor={`channel-type-${t.type}`}
                className={styles.typeOption}
                data-disabled={!t.available || undefined}
              >
                <input
                  id={`channel-type-${t.type}`}
                  type="radio"
                  name="channel-type"
                  value={t.type}
                  checked={form.type === t.type}
                  disabled={!t.available}
                  onChange={() => {
                    setErrors({})
                    setForm((f) => ({ ...emptyForm(t.type), name: f.name, enabled: f.enabled }))
                  }}
                />
                <span className={styles.typeName}>
                  {channelTypeLabels[t.type] ?? t.type}
                  <small>
                    {t.available
                      ? (channelTypeDescriptions[t.type] ?? '')
                      : `Niedostępny: ${unavailableReasonLabels[t.unavailable_reason ?? ''] ?? 'serwer go nie obsługuje.'}`}
                  </small>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
      )}

      <TextField
        label="Nazwa"
        value={form.name}
        onChange={(event) => set({ name: event.target.value })}
        error={error('name')}
        hint="Np. „Telefon (ntfy)” — tak kanał nazywa się w preferencjach."
        maxLength={100}
        autoComplete="off"
        data-autofocus
        required
      />

      {info.config_fields.map((field) => {
        const meta = describeField(form.type, field.name)
        const hint = form.type === 'email' && field.name === 'address' ? `Puste — ${user.email}.` : meta.hint
        return (
          <TextField
            key={`config.${field.name}`}
            label={field.required ? meta.label : `${meta.label} (opcjonalnie)`}
            type={meta.inputType ?? 'text'}
            value={form.config[field.name] ?? ''}
            placeholder={meta.placeholder}
            hint={hint}
            error={error(`config.${field.name}`)}
            onChange={(event) => set({ config: { ...form.config, [field.name]: event.target.value } })}
            autoComplete="off"
            spellCheck={false}
            required={field.required}
          />
        )
      })}

      {info.secret_fields.length > 0 && (
        <SecretFields
          info={info}
          form={form}
          channel={channel}
          canSecrets={canSecrets}
          showInputs={showSecretInputs}
          error={error}
          onMode={(secretMode) => set({ secretMode, secrets: {} })}
          onSecret={(name, value) => set({ secrets: { ...form.secrets, [name]: value } })}
        />
      )}

      <Checkbox
        label="Włączony"
        hint="Wyłączony kanał nie dostaje powiadomień, ale zostaje w preferencjach."
        checked={form.enabled}
        onChange={(event) => set({ enabled: event.target.checked })}
      />

      <Callout title={`Jak ustawić: ${channelTypeLabels[form.type]}`}>
        {channelGuides[form.type]?.map((text) => (
          <p key={text}>{text}</p>
        ))}
      </Callout>

      <div className={styles.footer}>
        <Button onClick={onClose}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={mutation.isPending}>
          {channel ? 'Zapisz zmiany' : 'Dodaj kanał'}
        </Button>
      </div>
    </form>
  )
}

type SecretProps = {
  info: ChannelTypeInfo
  form: ChannelFormState
  channel: Channel | null
  canSecrets: boolean
  showInputs: boolean
  error: (key: string) => string | undefined
  onMode: (mode: SecretMode) => void
  onSecret: (name: string, value: string) => void
}

/**
 * Sekrety są tylko na wejściu: serwer mówi jedynie, czy kanał je ma. Przy
 * edycji: zostaw bez zmian, zmień albo usuń (gdy żaden nie jest wymagany).
 */
function SecretFields({ info, form, channel, canSecrets, showInputs, error, onMode, onSecret }: SecretProps) {
  const required = info.secret_fields.some((f) => f.required)
  const modes: { value: SecretMode; label: string; hint: string }[] = [
    { value: 'keep', label: 'Zostaw bez zmian', hint: 'Zapisany sekret działa dalej.' },
    { value: 'change', label: 'Zmień', hint: 'Wpisz nową wartość — zastąpi zapisaną.' },
    ...(canRemoveSecrets(info)
      ? [{ value: 'remove' as const, label: 'Usuń', hint: 'Kanał będzie działał bez sekretu.' }]
      : []),
  ]
  return (
    <fieldset className={styles.fieldset}>
      <legend>{required ? 'Sekret' : 'Sekret (opcjonalnie)'}</legend>
      {!canSecrets && (
        <Callout tone="warn">
          Serwer nie ma klucza DENAR_MASTER_KEY, więc nie zapisze sekretu. Poproś administratora o ustawienie klucza.
        </Callout>
      )}
      {channel?.has_secret && (
        <div className={styles.secretModes} role="radiogroup" aria-label="Zapisany sekret">
          {modes.map((mode) => (
            <label key={mode.value} className={styles.radio}>
              <input
                type="radio"
                name="secret-mode"
                checked={form.secretMode === mode.value}
                onChange={() => onMode(mode.value)}
                disabled={mode.value === 'change' && !canSecrets}
              />
              <span>
                {mode.label}
                <small>{mode.hint}</small>
              </span>
            </label>
          ))}
        </div>
      )}
      {showInputs &&
        info.secret_fields.map((field) => {
          const meta = describeField(form.type, field.name)
          return (
            <TextField
              key={`secrets.${field.name}`}
              label={field.required ? meta.label : `${meta.label} (opcjonalnie)`}
              type="password"
              value={form.secrets[field.name] ?? ''}
              hint={meta.hint}
              placeholder={meta.placeholder}
              error={error(`secrets.${field.name}`)}
              onChange={(event) => onSecret(field.name, event.target.value)}
              autoComplete="new-password"
              spellCheck={false}
              disabled={!canSecrets}
              required={field.required}
            />
          )
        })}
      <p className={styles.formMeta}>Sekret szyfrujemy na serwerze i nigdy go nie pokazujemy — nawet Tobie.</p>
    </fieldset>
  )
}
