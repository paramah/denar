import { useCurrencies } from '../../api/hooks/households.ts'
import { decimalsOf } from '../../lib/money.ts'

/** Liczba miejsc dziesiętnych waluty według /currencies (z zapasem w Intl). */
export function useDecimals(): (currency: string) => number {
  const currencies = useCurrencies()
  return (currency) => decimalsOf(currency, currencies.data)
}

/** Pierwszy błąd serwera spośród podanych pól (np. amount i amount.amount). */
export function pickError(errors: Record<string, string>, ...fields: string[]): string | undefined {
  for (const field of fields) {
    if (errors[field]) {
      return errors[field]
    }
  }
  return undefined
}

export const lastWalletKey = (hid: string) => `denar:wallet:${hid}`
