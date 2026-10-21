import type { ActorKind, Role } from '../api/types.ts'

export const roleLabels: Record<Role, string> = {
  owner: 'Właściciel',
  admin: 'Administrator',
  member: 'Domownik',
  viewer: 'Podgląd',
}

export const roleDescriptions: Record<Role, string> = {
  owner: 'Pełna kontrola, w tym usunięcie gospodarstwa',
  admin: 'Zarządza członkami, zaproszeniami i botami',
  member: 'Prowadzi finanse i tworzy własne boty',
  viewer: 'Tylko przegląda dane',
}

export const roleOrder: Role[] = ['owner', 'admin', 'member', 'viewer']

export const actorKindLabels: Record<ActorKind, string> = {
  user: 'Człowiek',
  bot: 'Bot',
  system: 'System',
}

const actionLabels: Record<string, string> = {
  'household.created': 'Założenie gospodarstwa',
  'household.updated': 'Zmiana ustawień gospodarstwa',
  'membership.joined': 'Dołączenie do gospodarstwa',
  'membership.role_changed': 'Zmiana roli',
  'membership.removed': 'Usunięcie członka',
  'invitation.created': 'Wysłanie zaproszenia',
  'invitation.revoked': 'Odwołanie zaproszenia',
  'bot.created': 'Utworzenie bota',
  'bot.updated': 'Zmiana bota',
  'bot.disabled': 'Wyłączenie bota',
  'bot.token_rotated': 'Nowy token bota',
  'bot.deleted': 'Usunięcie bota',
}

/** Czytelna nazwa akcji audytu; nieznana (z kolejnych etapów) — surowy kod. */
export function actionLabel(action: string): string {
  return actionLabels[action] ?? action
}

export const resourceTypeLabels: Record<string, string> = {
  household: 'Gospodarstwo',
  membership: 'Członkostwo',
  invitation: 'Zaproszenie',
  bot: 'Bot',
}

export function resourceTypeLabel(type: string): string {
  return resourceTypeLabels[type] ?? type
}

const fieldLabels: Record<string, string> = {
  name: 'nazwa',
  role: 'rola',
  timezone: 'strefa czasowa',
  month_start_day: 'dzień startu miesiąca',
  base_currency: 'waluta',
  scopes: 'uprawnienia',
  wallet_ids: 'portfele',
  expires_at: 'wygaśnięcie',
  disabled: 'wyłączony',
  email: 'e-mail',
}

export function fieldLabel(field: string): string {
  return fieldLabels[field] ?? field
}
