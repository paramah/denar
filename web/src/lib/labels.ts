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
  'household.deleted': 'Usunięcie gospodarstwa',
  'envelope_group.created': 'Utworzenie grupy kopert',
  'envelope_group.updated': 'Zmiana grupy kopert',
  'envelope_group.deleted': 'Usunięcie grupy kopert',
  'envelope.created': 'Utworzenie koperty',
  'envelope.updated': 'Zmiana koperty',
  'envelope.deleted': 'Usunięcie koperty',
  'assignment.set': 'Przydział do koperty',
  'subscription.created': 'Dodanie subskrypcji',
  'subscription.updated': 'Zmiana subskrypcji',
  'subscription.deleted': 'Usunięcie subskrypcji',
  'subscription.price_changed': 'Zmiana ceny subskrypcji',
  'charge.matched': 'Dopasowanie płatności subskrypcji',
  'charge.unmatched': 'Cofnięcie dopasowania płatności',
  'charge.skipped': 'Pominięcie płatności subskrypcji',
  'charge.posted': 'Zaksięgowanie płatności subskrypcji',
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
  envelope_group: 'Grupa kopert',
  envelope: 'Koperta',
  assignment: 'Przydział',
  subscription: 'Subskrypcja',
  charge: 'Płatność subskrypcji',
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
  base_amount: 'kwota bazowa',
  count: 'liczba',
  created_by: 'autor',
  cycle: 'cykl',
  fx_rate: 'kurs',
  goal: 'cel',
  'goal.type': 'rodzaj celu',
  'goal.amount': 'kwota celu',
  'goal.amount.amount': 'kwota celu',
  'goal.target_date': 'data celu',
  group_id: 'grupa',
  hidden: 'ukryta',
  invitation_id: 'zaproszenie',
  month: 'miesiąc',
  position: 'kolejność',
  price: 'cena',
  reminder_offsets: 'przypomnienia',
  score: 'trafność',
  subscription_id: 'subskrypcja',
  vendor: 'dostawca',
  via: 'sposób',
  gone: 'usunięto',
  trial_ends_at: 'koniec okresu próbnego',
  ends_at: 'koniec',
  manage_url: 'adres zarządzania',
  'match_rule.payee_pattern': 'wzorzec kontrahenta',
  'match_rule.amount_tolerance_pct': 'tolerancja kwoty',
  'match_rule.date_window_days': 'okno dat',
  'cycle.unit': 'jednostka cyklu',
  'cycle.every': 'co ile',
  'cycle.anchor': 'dzień płatności',
  default_envelope_id: 'domyślna koperta',
  from_envelope_id: 'koperta źródłowa',
  to_envelope_id: 'koperta docelowa',
  transaction_id: 'transakcja',
  effective_from: 'od dnia',
  'amount.amount': 'kwota',
  'to_amount.amount': 'kwota docelowa',
  'opening_balance.amount': 'saldo otwarcia',
  'price.amount': 'cena',
  password: 'hasło',
}

export function fieldLabel(field: string): string {
  const known = fieldLabels[field]
  if (known) {
    return known
  }
  // splits[2].amount → „podział 3: kwota”
  const split = /^splits\[(\d+)\]\.(.+)$/.exec(field)
  if (split?.[1] && split[2]) {
    return `podział ${Number(split[1]) + 1}: ${fieldLabel(split[2])}`
  }
  return field
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
