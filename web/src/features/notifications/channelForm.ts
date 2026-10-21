import type { Channel, ChannelCreate, ChannelType, ChannelTypeInfo, ChannelUpdate } from '../../api/types.ts'

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

const fieldInfo: Record<string, FieldInfo> = {
  'email.address': {
    label: 'Adres e-mail',
    hint: 'Puste — adres Twojego konta.',
    inputType: 'email',
  },
  'webhook.url': {
    label: 'Adres URL',
    placeholder: 'https://example.org/denar',
    hint: 'Denar wyśle tu POST z JSON-em każdego powiadomienia.',
    inputType: 'url',
  },
  'webhook.secret': {
    label: 'Sekret podpisu',
    hint: 'Opcjonalny. Z nim każde żądanie ma nagłówek X-Denar-Signature (HMAC-SHA256).',
  },
  'ntfy.server': {
    label: 'Serwer ntfy',
    placeholder: 'https://ntfy.sh',
    hint: 'Puste — publiczny https://ntfy.sh.',
    inputType: 'url',
  },
  'ntfy.topic': {
    label: 'Temat',
    placeholder: 'denar-dom-7f3k',
    hint: 'Litery, cyfry, „-” i „_”. Na publicznym serwerze wybierz trudny do zgadnięcia.',
  },
  'ntfy.token': {
    label: 'Token dostępu',
    hint: 'Opcjonalny — dla tematów chronionych (tk_…).',
  },
  'telegram.chat_id': {
    label: 'chat_id',
    placeholder: '123456789',
    hint: 'Numer czatu (grupy mają minus na początku) albo @nazwa_kanału.',
  },
  'telegram.bot_token': {
    label: 'Token bota',
    placeholder: '123456789:AA…',
    hint: 'Od @BotFather po utworzeniu bota (/newbot).',
  },
  'matrix.homeserver': {
    label: 'Serwer domowy',
    placeholder: 'https://matrix.org',
    inputType: 'url',
  },
  'matrix.room_id': {
    label: 'ID pokoju',
    placeholder: '!abc123:matrix.org',
    hint: 'Wewnętrzne ID pokoju (zaczyna się od „!”), nie alias #pokój.',
  },
  'matrix.access_token': {
    label: 'Token dostępu',
    hint: 'Token konta, które pisze do pokoju (najlepiej osobnego konta-bota).',
  },
}

export function describeField(type: ChannelType, name: string): FieldInfo {
  return fieldInfo[`${type}.${name}`] ?? { label: name }
}

export const channelTypeDescriptions: Record<ChannelType, string> = {
  email: 'Wiadomość na Twoją skrzynkę',
  webhook: 'POST z JSON-em do Twojej usługi (n8n, Home Assistant, własny serwer)',
  ntfy: 'Powiadomienia push na telefon przez ntfy',
  telegram: 'Wiadomość od Twojego bota na Telegramie',
  matrix: 'Wiadomość w pokoju Matrix (Element)',
}

/** Jak skonfigurować kanał — akapity pomocy pod formularzem. */
export const channelGuides: Record<ChannelType, string[]> = {
  email: ['Wiadomości wysyła serwer Denara przez skonfigurowany SMTP. Sprawdź folder spamu po pierwszym teście.'],
  webhook: [
    'Każde powiadomienie to POST z treścią JSON: id, type, title, body, url, household {id, name}, occurred_at i data.',
    'Z sekretem żądanie ma nagłówek X-Denar-Signature: sha256=<hex>, gdzie <hex> to HMAC-SHA256 surowej treści żądania liczony sekretem. Porównaj go po swojej stronie, zanim zaufasz treści.',
    'X-Denar-Event to typ zdarzenia, a X-Denar-Delivery — identyfikator dostawy (ponowienia mają ten sam, więc łatwo odrzucić duplikat).',
  ],
  ntfy: [
    'Zainstaluj aplikację ntfy i zasubskrybuj ten sam temat na tym samym serwerze.',
    'Na własnym serwerze ntfy z kontrolą dostępu podaj token użytkownika, który może publikować w temacie.',
  ],
  telegram: [
    'Utwórz bota u @BotFather i skopiuj jego token.',
    'Napisz cokolwiek do swojego bota (albo dodaj go do grupy), a potem otwórz https://api.telegram.org/bot<token>/getUpdates — chat_id to pole message.chat.id.',
  ],
  matrix: [
    'Zaproś konto, którego token podajesz, do pokoju i przyjmij zaproszenie.',
    'ID pokoju znajdziesz w Element: Ustawienia pokoju → Zaawansowane → Wewnętrzne ID pokoju.',
  ],
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
  if (!name) errors.name = 'Podaj nazwę kanału.'
  else if (name.length > 100) errors.name = 'Najwyżej 100 znaków.'
  for (const field of info.config_fields) {
    if (field.required && !form.config[field.name]?.trim()) {
      errors[`config.${field.name}`] = 'To pole jest wymagane.'
    }
  }
  // Wymagany sekret: przy tworzeniu i przy „Zmień” (albo gdy kanał go nie ma).
  if (!editing || form.secretMode === 'change') {
    for (const field of info.secret_fields) {
      if (field.required && !form.secrets[field.name]?.trim()) {
        errors[`secrets.${field.name}`] = 'To pole jest wymagane.'
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
      return c.chat_id ? `chat ${c.chat_id}` : ''
    case 'matrix':
      return [c.room_id, c.homeserver].filter(Boolean).join(' na ')
    default:
      return ''
  }
}

/** Serwer bez klucza głównego nie zapisze żadnego sekretu (wnioskowane z typów z wymaganym sekretem). */
export function secretsAvailable(types: ChannelTypeInfo[]): boolean {
  return !types.some((t) => t.unavailable_reason === 'secrets_unavailable')
}
