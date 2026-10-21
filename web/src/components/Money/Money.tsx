import { formatMoney, toneOf } from '../../lib/money.ts'
import styles from './Money.module.css'

type Props = {
  /** Dziesiętny string z API, np. "-12.34". Nigdy liczba. */
  amount: string
  /** Kod ISO 4217. */
  currency: string
  /**
   * `auto` — wpływ zielony z „+”, wydatek czerwony z „−”; `neutral` — bez
   * koloru (salda, sumy, limity).
   */
  tone?: 'auto' | 'neutral'
  /** Pokazuj „+” przy dodatnich (domyślnie przy tone=auto). */
  signed?: boolean
  size?: 'sm' | 'md' | 'lg' | 'xl'
  className?: string
}

/**
 * Kwota w pl-PL cyframi tabelarycznymi. Znak jest zawsze widoczny, więc
 * kolor nie jest jedynym nośnikiem informacji o wpływie/wydatku.
 */
export function Money({ amount, currency, tone = 'neutral', signed, size = 'md', className }: Props) {
  const showSign = signed ?? tone === 'auto'
  const text = formatMoney(amount, currency, { signed: showSign })
  const kind = tone === 'auto' ? toneOf(amount) : 'neutral'
  return (
    <span
      className={[styles.money, styles[size], styles[kind], className].filter(Boolean).join(' ')}
      data-tone={kind}
      title={text === null ? `${amount} ${currency}` : undefined}
    >
      {text ?? '—'}
    </span>
  )
}
