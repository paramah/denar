import type {
  ActorKind,
  ChannelType,
  EventType,
  FxSource,
  Role,
  TransactionKind,
  TransactionSource,
  TransactionStatus,
  WalletType,
} from '../api/types.ts'
import { tk } from '../i18n/translate.ts'

// Nazwy wartości z API w aktywnym języku (i18n/{pl,en}/labels.json). Mapy
// zachowują dawny kształt `Record<klucz, string>`: każda właściwość to getter
// czytający tłumaczenie w chwili odczytu, więc `roleLabels[role]` po zmianie
// języka daje nowy tekst. Kropki w kodach (np. `household.created`) są w JSON
// zapisane jako `/`, bo kropka rozdziela poziomy kluczy i18next.

function labelMap<K extends string>(group: string, keys: readonly K[]): Record<K, string> {
  const map = {} as Record<K, string>
  for (const key of keys) {
    Object.defineProperty(map, key, {
      enumerable: true,
      get: () => tk(`labels:${group}.${key.replaceAll('.', '/')}`),
    })
  }
  return map
}

export const roleLabels: Record<Role, string> = labelMap('role', ['owner', 'admin', 'member', 'viewer'])

export const roleDescriptions: Record<Role, string> = labelMap('roleDescription', ['owner', 'admin', 'member', 'viewer'])

export const roleOrder: Role[] = ['owner', 'admin', 'member', 'viewer']

export const actorKindLabels: Record<ActorKind, string> = labelMap('actorKind', ['user', 'bot', 'system'])

const actionLabels: Record<string, string> = labelMap('action', [
  'household.created',
  'household.updated',
  'membership.joined',
  'membership.role_changed',
  'membership.removed',
  'invitation.created',
  'invitation.revoked',
  'bot.created',
  'bot.updated',
  'bot.disabled',
  'bot.token_rotated',
  'bot.deleted',
  'wallet.created',
  'wallet.updated',
  'transaction.created',
  'transaction.updated',
  'transaction.deleted',
  'transaction.rate_filled',
  'payee.created',
  'payee.updated',
  'payee.deleted',
  'fx_rate.set',
  'household.deleted',
  'envelope_group.created',
  'envelope_group.updated',
  'envelope_group.deleted',
  'envelope.created',
  'envelope.updated',
  'envelope.deleted',
  'assignment.set',
  'subscription.created',
  'subscription.updated',
  'subscription.deleted',
  'subscription.price_changed',
  'charge.matched',
  'charge.unmatched',
  'charge.skipped',
  'charge.posted',
  'import.created',
  'import.reverted',
  'connection.created',
  'connection.account_mapped',
  'connection.synced',
])

/** Czytelna nazwa akcji audytu; nieznana (z kolejnych etapów) — surowy kod. */
export function actionLabel(action: string): string {
  return Object.hasOwn(actionLabels, action) ? actionLabels[action]! : action
}

export const resourceTypeLabels: Record<string, string> = labelMap('resourceType', [
  'household',
  'membership',
  'invitation',
  'bot',
  'wallet',
  'transaction',
  'payee',
  'fx_rate',
  'envelope_group',
  'envelope',
  'assignment',
  'subscription',
  'charge',
  'import',
  'connection',
])

export function resourceTypeLabel(type: string): string {
  return Object.hasOwn(resourceTypeLabels, type) ? resourceTypeLabels[type]! : type
}

const fieldLabels: Record<string, string> = labelMap('field', [
  'name',
  'enabled',
  'config.address',
  'config.url',
  'config.server',
  'config.topic',
  'config.chat_id',
  'config.homeserver',
  'config.room_id',
  'secrets.secret',
  'secrets.token',
  'secrets.bot_token',
  'secrets.access_token',
  'items.type',
  'items.channel_ids',
  'send_email',
  'role',
  'timezone',
  'month_start_day',
  'base_currency',
  'scopes',
  'wallet_ids',
  'expires_at',
  'disabled',
  'email',
  'type',
  'currency',
  'on_budget',
  'visibility',
  'member_ids',
  'opening_balance',
  'opened_on',
  'archived',
  'amount',
  'to_amount',
  'date',
  'payee_id',
  'payee_name',
  'envelope_id',
  'kind',
  'status',
  'tags',
  'note',
  'external_id',
  'splits',
  'wallet_id',
  'from_wallet_id',
  'to_wallet_id',
  'patterns',
  'rate',
  'base_amount',
  'count',
  'created_by',
  'cycle',
  'fx_rate',
  'goal',
  'goal.type',
  'goal.amount',
  'goal.amount.amount',
  'goal.target_date',
  'group_id',
  'hidden',
  'invitation_id',
  'month',
  'position',
  'price',
  'reminder_offsets',
  'score',
  'subscription_id',
  'vendor',
  'via',
  'gone',
  'trial_ends_at',
  'ends_at',
  'manage_url',
  'match_rule.payee_pattern',
  'match_rule.amount_tolerance_pct',
  'match_rule.date_window_days',
  'cycle.unit',
  'cycle.every',
  'cycle.anchor',
  'default_envelope_id',
  'from_envelope_id',
  'to_envelope_id',
  'transaction_id',
  'effective_from',
  'amount.amount',
  'to_amount.amount',
  'opening_balance.amount',
  'price.amount',
  'password',
  'file',
  'options',
  'body',
  'preview_limit',
  'options.wallet_id',
  'options.preset',
  'options.delimiter',
  'options.encoding',
  'options.date_format',
  'options.decimal_separator',
  'options.skip_rows',
  'options.accept_states',
  'options.columns.date',
  'options.columns.amount',
  'options.columns.debit',
  'options.columns.credit',
  'options.columns.payee',
  'options.columns.memo',
  'options.columns.external_id',
  'options.columns.currency',
  'options.columns.fee',
  'options.columns.state',
  'source',
  'source_ref',
  'preset',
  'rows',
  'created',
  'duplicates',
  'failed',
  'matched',
  'reverted',
  'import_id',
  'provider',
  'display_name',
  'connection_id',
  'account',
])

export function fieldLabel(field: string): string {
  if (Object.hasOwn(fieldLabels, field)) {
    return fieldLabels[field]!
  }
  // splits[2].amount → „podział 3: kwota”
  const split = /^splits\[(\d+)\]\.(.+)$/.exec(field)
  if (split?.[1] && split[2]) {
    return tk('labels:splitField', { line: Number(split[1]) + 1, field: fieldLabel(split[2]) })
  }
  return field
}

export const walletTypeLabels: Record<WalletType, string> = labelMap('walletType', ['cash', 'checking', 'card', 'savings', 'investment', 'loan'])

export const walletTypeOrder: WalletType[] = ['checking', 'cash', 'card', 'savings', 'investment', 'loan']

export const kindLabels: Record<TransactionKind, string> = labelMap('kind', ['expense', 'income', 'transfer'])

export const statusLabels: Record<TransactionStatus, string> = labelMap('status', ['pending', 'cleared', 'reconciled'])

export const statusDescriptions: Record<TransactionStatus, string> = labelMap('statusDescription', ['pending', 'cleared', 'reconciled'])

export const sourceLabels: Record<TransactionSource, string> = labelMap('source', ['manual', 'import', 'subscription', 'bot'])

export const fxSourceLabels: Record<FxSource, string> = labelMap('fxSource', ['identity', 'manual', 'nbp', 'unavailable'])

/** Typy zdarzeń powiadomień w kolejności serwera (`event.Types`). */
export const eventTypeOrder: EventType[] = [
  'subscription.renewal_upcoming',
  'subscription.trial_ending',
  'subscription.price_changed',
  'subscription.charge_missed',
  'envelope.overspent',
  'envelope.goal_due',
]

export const eventTypeLabels: Record<EventType, string> = labelMap('eventType', [
  'subscription.renewal_upcoming',
  'subscription.trial_ending',
  'subscription.price_changed',
  'subscription.charge_missed',
  'envelope.overspent',
  'envelope.goal_due',
])

export const eventTypeDescriptions: Record<EventType, string> = labelMap('eventTypeDescription', [
  'subscription.renewal_upcoming',
  'subscription.trial_ending',
  'subscription.price_changed',
  'subscription.charge_missed',
  'envelope.overspent',
  'envelope.goal_due',
])

export const channelTypeOrder: ChannelType[] = ['email', 'webhook', 'ntfy', 'telegram', 'matrix']

export const channelTypeLabels: Record<ChannelType, string> = labelMap('channelType', ['email', 'webhook', 'ntfy', 'telegram', 'matrix'])

export const unavailableReasonLabels: Record<string, string> = labelMap('unavailableReason', ['smtp_not_configured', 'secrets_unavailable'])

export const connectionKindLabels: Record<'bank' | 'billing', string> = labelMap('connectionKind', ['bank', 'billing'])

export const connectionStatusLabels: Record<'active' | 'error' | 'disabled', string> = labelMap('connectionStatus', ['active', 'error', 'disabled'])
