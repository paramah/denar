import type { Wallet } from '../../api/types.ts'
import { sumAmounts } from '../../lib/money.ts'

export type WalletTotals = {
  /** Suma sald portfeli budżetowych w walucie bazowej. */
  onBudget: string
  offBudget: string
  all: string
  /** Któraś kwota bazowa czeka na kurs — sumy są częściowe. */
  partial: boolean
  count: number
}

/**
 * Sumy sald aktywnych portfeli w walucie bazowej. Liczone na bigint z
 * dziesiętnych stringów (`amount`), nie z `minor`: liczba JS traci
 * precyzję powyżej 2^53.
 */
export function walletTotals(wallets: Wallet[], decimals: number): WalletTotals {
  const active = wallets.filter((w) => !w.archived_at)
  const sum = (list: Wallet[]) => sumAmounts(list.map((w) => w.base_balance.amount), decimals) ?? '0'
  return {
    onBudget: sum(active.filter((w) => w.on_budget)),
    offBudget: sum(active.filter((w) => !w.on_budget)),
    all: sum(active),
    partial: active.some((w) => w.rate_pending),
    count: active.length,
  }
}
