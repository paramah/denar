import type { Channel, ChannelCreate, ChannelType, ChannelTypeInfo, ChannelUpdate } from '../../api/types.ts'
import { hasKey, tk } from '../../i18n/translate.ts'

/**
 * Formularz kanału budowany z `GET /me/channel-types` (`config_fields`,
 * `secret_fields`). Tu są czyste funkcje: opisy pól, stan i treść żądań.
 */

export type FieldInfo = {
  label: string
  hint?: string
  placeholder?: string
  inputType?: 'text' | 'email' | 'url'
}

/** Kształt pól: typ, przykład niezależny od języka i które teksty są w tłumaczeniach (field.<typ>.<pole>). */
const fieldShape: Record<string, Pick<FieldInfo, 'placeholder' | 'inputType'> & { translatedPlaceholder?: boolean; hint?: boolean }> = {
  'email.address': { inputType: 'email', hint: true },
  'webhook.url': { placeholder: 'https://example.org/denar', inputType: 'url', hint: true },
  'webhook.secret': { hint: true },
  'ntfy.server': { placeholder: 'https://ntfy.sh', inputType: 'url', hint: true },
  'ntfy.topic': { translatedPlaceholder: true, hint: true },
  'ntfy.token': { hint: true },
  'telegram.chat_id': { placeholder: '123456789', hint: true },
  'telegram.bot_token': { placeholder: '123456789:AA…', hint: true },
  'matrix.homeserver': { placeholder: 'https://matrix.org', inputType: 'url' },
  'matrix.room_id': { placeholder: '!abc123:matrix.org', hint: true },
  'matrix.access_token': { hint: true },
}

export function describeField(type: ChannelType, name: string): FieldInfo {
  const shape = fieldShape[`${type}.${name}`]
  if (!shape) return { label: name }
  const base = `notifications:field.${type}.${name}`
  const info: FieldInfo = { label: tk(`${base}.label`) }
  if (shape.hint) info.hint = tk(`${base}.hint`)
  if (shape.translatedPlaceholder) info.placeholder = tk(`${base}.placeholder`)
  else if (shape.placeholder) info.placeholder = shape.placeholder
  if (shape.inputType) info.inputType = shape.inputType
  return info
}

/** Krótki opis typu kanału w aktywnym języku. */
export function channelTypeDescription(type: ChannelType): string {
  return hasKey(`notifications:typeDescription.${type}`) ? tk(`notifications:typeDescription.${type}`) : ''
}

const guideParagraphs: Record<ChannelType, number> = { email: 1, webhook: 3, ntfy: 2, telegram: 2, matrix: 2 }

/** Jak skonfigurować kanał — akapity pomocy pod formularzem. */
export function channelGuide(type: ChannelType): string[] {
  const count = guideParagraphs[type] ?? 0
  return Array.from({ length: count }, (_, i) => tk(`notifications:guide.${type}.p${i + 1}`))
}

/** Co zrobić z zapisanymi sekretami przy edycji. */
export type SecretMode = 'keep' | 'change' | 'remove'

export type ChannelFormState = {
  type: ChannelType
  name: string
  enabled: boolean
  config: Record<string, string>
  secrets: Record<string, string>
  secretMode: SecretMode
}

export function emptyForm(type: ChannelType): ChannelFormState {
  return { type, name: '', enabled: true, config: {}, secrets: {}, secretMode: 'change' }
}

export function formFromChannel(channel: Channel): ChannelFormState {
  return {
    type: channel.type,
    name: channel.name,
    enabled: channel.enabled,
    config: { ...channel.config },
    secrets: {},
    // Bez zapisanych sekretów pola są puste i opcjonalne: wpisane — zapisujemy.
    secretMode: channel.has_secret ? 'keep' : 'change',
  }
}

function nonEmpty(values: Record<string, string>, names: string[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const name of names) {
    const value = values[name]?.trim()
    if (value) out[name] = value
  }
  return out
}

/** Sekrety można usunąć tylko, gdy żaden nie jest wymagany. */
export function canRemoveSecrets(info: ChannelTypeInfo): boolean {
  return info.secret_fields.length > 0 && info.secret_fields.every((f) => !f.required)
}

/** Błędy formularza po stronie przeglądarki (klucze jak w problem+json: `config.url`). */
export function validateForm(
  form: ChannelFormState,
  info: ChannelTypeInfo,
  editing: Channel | null,
): Record<string, string> {
  const errors: Record<string, string> = {}
  const name = form.name.trim()
  if (!name) errors.name = tk('notifications:validation.nameRequired')
  else if (name.length > 100) errors.name = tk('notifications:validation.nameMax')
  for (const field of info.config_fields) {
    if (field.required && !form.config[field.name]?.trim()) {
      errors[`config.${field.name}`] = tk('notifications:validation.required')
    }
  }
  // Wymagany sekret: przy tworzeniu i przy „Zmień” (albo gdy kanał go nie ma).
  if (!editing || form.secretMode === 'change') {
    for (const field of info.secret_fields) {
      if (field.required && !form.secrets[field.name]?.trim()) {
        errors[`secrets.${field.name}`] = tk('notifications:validation.required')
      }
    }
  }
  return errors
}

export function createBody(form: ChannelFormState, info: ChannelTypeInfo): ChannelCreate {
  const config = nonEmpty(form.config, info.config_fields.map((f) => f.name))
  const secrets = nonEmpty(form.secrets, info.secret_fields.map((f) => f.name))
  return {
    type: form.type,
    name: form.name.trim(),
    config,
    ...(Object.keys(secrets).length > 0 ? { secrets } : {}),
    enabled: form.enabled,
  }
}

function sameValues(a: Record<string, string>, b: Record<string, string>): boolean {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  return [...keys].every((k) => (a[k] ?? '') === (b[k] ?? ''))
}

/**
 * PATCH tylko ze zmianami. `config` zastępuje całą konfigurację (puste pola
 * wracają do domyślnych); `secrets`: brak pola — bez zmian, obiekt — nowe,
 * `null` — usunięcie.
 */
export function updateBody(form: ChannelFormState, info: ChannelTypeInfo, channel: Channel): ChannelUpdate {
  const body: ChannelUpdate = {}
  const name = form.name.trim()
  if (name !== channel.name) body.name = name
  if (form.enabled !== channel.enabled) body.enabled = form.enabled
  const config = nonEmpty(form.config, info.config_fields.map((f) => f.name))
  if (!sameValues(config, channel.config)) body.config = config
  if (form.secretMode === 'remove') {
    body.secrets = null
  } else if (form.secretMode === 'change') {
    const secrets = nonEmpty(form.secrets, info.secret_fields.map((f) => f.name))
    if (Object.keys(secrets).length > 0) body.secrets = secrets
  }
  return body
}

/** Krótki opis konfiguracji na liście kanałów (bez sekretów — serwer ich nie zwraca). */
export function channelSummary(channel: Channel): string {
  const c = channel.config
  switch (channel.type) {
    case 'email':
      return c.address ?? ''
    case 'webhook':
      return c.url ?? ''
    case 'ntfy':
      return [c.server ?? 'https://ntfy.sh', c.topic].filter(Boolean).join('/')
    case 'telegram':
      return c.chat_id ? tk('notifications:channels.summaryChat', { id: c.chat_id }) : ''
    case 'matrix':
      if (c.room_id && c.homeserver) return tk('notifications:channels.summaryMatrix', { room: c.room_id, server: c.homeserver })
      return c.room_id ?? c.homeserver ?? ''
    default:
      return ''
  }
}

/** Serwer bez klucza głównego nie zapisze żadnego sekretu (wnioskowane z typów z wymaganym sekretem). */
export function secretsAvailable(types: ChannelTypeInfo[]): boolean {
  return !types.some((t) => t.unavailable_reason === 'secrets_unavailable')
}
