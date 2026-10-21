import type {
  ActorKind,
  FxSource,
  Role,
  TransactionKind,
  TransactionSource,
  TransactionStatus,
  WalletType,
} from '../api/types.ts'

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
  'wallet.created': 'Utworzenie portfela',
  'wallet.updated': 'Zmiana portfela',
  'transaction.created': 'Dodanie transakcji',
  'transaction.updated': 'Zmiana transakcji',
  'transaction.deleted': 'Usunięcie transakcji',
  'transaction.rate_filled': 'Uzupełnienie kursu transakcji',
  'payee.created': 'Dodanie kontrahenta',
  'payee.updated': 'Zmiana kontrahenta',
  'payee.deleted': 'Usunięcie kontrahenta',
  'fx_rate.set': 'Ręczny kurs waluty',
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
  wallet: 'Portfel',
  transaction: 'Transakcja',
  payee: 'Kontrahent',
  fx_rate: 'Kurs waluty',
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
  type: 'typ',
  currency: 'waluta',
  on_budget: 'w budżecie',
  visibility: 'widoczność',
  member_ids: 'członkowie',
  opening_balance: 'saldo otwarcia',
  opened_on: 'data otwarcia',
  archived: 'archiwizacja',
  amount: 'kwota',
  to_amount: 'kwota docelowa',
  date: 'data',
  payee_id: 'kontrahent',
  payee_name: 'kontrahent',
  envelope_id: 'koperta',
  kind: 'rodzaj',
  status: 'status',
  tags: 'tagi',
  note: 'notatka',
  external_id: 'identyfikator zewnętrzny',
  splits: 'podział',
  wallet_id: 'portfel',
  from_wallet_id: 'portfel źródłowy',
  to_wallet_id: 'portfel docelowy',
  patterns: 'wzorce',
  rate: 'kurs',
}

export function fieldLabel(field: string): string {
  return fieldLabels[field] ?? field
}

export const walletTypeLabels: Record<WalletType, string> = {
  cash: 'Gotówka',
  checking: 'Konto bieżące',
  card: 'Karta',
  savings: 'Oszczędności',
  investment: 'Inwestycje',
  loan: 'Kredyt',
}

export const walletTypeOrder: WalletType[] = ['checking', 'cash', 'card', 'savings', 'investment', 'loan']

export const kindLabels: Record<TransactionKind, string> = {
  expense: 'Wydatek',
  income: 'Wpływ',
  transfer: 'Przelew',
}

export const statusLabels: Record<TransactionStatus, string> = {
  pending: 'Oczekująca',
  cleared: 'Rozliczona',
  reconciled: 'Uzgodniona',
}

export const statusDescriptions: Record<TransactionStatus, string> = {
  pending: 'Jeszcze nie ma jej na wyciągu',
  cleared: 'Widać ją na wyciągu',
  reconciled: 'Sprawdzona z wyciągiem i zamknięta',
}

export const sourceLabels: Record<TransactionSource, string> = {
  manual: 'Ręcznie',
  import: 'Import',
  subscription: 'Subskrypcja',
  bot: 'Bot',
}

export const fxSourceLabels: Record<FxSource, string> = {
  identity: 'waluta bazowa',
  manual: 'kurs ręczny',
  nbp: 'NBP',
  unavailable: 'brak kursu',
}
