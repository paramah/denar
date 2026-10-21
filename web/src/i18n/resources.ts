// Tłumaczenia w paczce aplikacji: jeden plik JSON na obszar i język. Nowy
// obszar = para plików pl/<obszar>.json i en/<obszar>.json + wpis niżej.

import enAuth from './en/auth.json'
import enBudget from './en/budget.json'
import enCommon from './en/common.json'
import enDashboard from './en/dashboard.json'
import enErrors from './en/errors.json'
import enHouseholds from './en/households.json'
import enImport from './en/import.json'
import enLabels from './en/labels.json'
import enNotifications from './en/notifications.json'
import enProfile from './en/profile.json'
import enSettings from './en/settings.json'
import enSubscriptions from './en/subscriptions.json'
import enTransactions from './en/transactions.json'
import enWallets from './en/wallets.json'
import plAuth from './pl/auth.json'
import plBudget from './pl/budget.json'
import plCommon from './pl/common.json'
import plDashboard from './pl/dashboard.json'
import plErrors from './pl/errors.json'
import plHouseholds from './pl/households.json'
import plImport from './pl/import.json'
import plLabels from './pl/labels.json'
import plNotifications from './pl/notifications.json'
import plProfile from './pl/profile.json'
import plSettings from './pl/settings.json'
import plSubscriptions from './pl/subscriptions.json'
import plTransactions from './pl/transactions.json'
import plWallets from './pl/wallets.json'

export const pl = {
  common: plCommon,
  auth: plAuth,
  dashboard: plDashboard,
  wallets: plWallets,
  transactions: plTransactions,
  budget: plBudget,
  subscriptions: plSubscriptions,
  notifications: plNotifications,
  import: plImport,
  settings: plSettings,
  households: plHouseholds,
  profile: plProfile,
  labels: plLabels,
  errors: plErrors,
}

export type Namespace = keyof typeof pl

// Angielskie pliki mają te same klucze (test i18n.test.ts); typ pilnuje,
// że każdy obszar ma swój plik.
export const en: Record<Namespace, object> = {
  common: enCommon,
  auth: enAuth,
  dashboard: enDashboard,
  wallets: enWallets,
  transactions: enTransactions,
  budget: enBudget,
  subscriptions: enSubscriptions,
  notifications: enNotifications,
  import: enImport,
  settings: enSettings,
  households: enHouseholds,
  profile: enProfile,
  labels: enLabels,
  errors: enErrors,
}

export const namespaces = Object.keys(pl) as Namespace[]

export const resources = { pl, en }
