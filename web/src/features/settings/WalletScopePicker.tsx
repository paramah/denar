import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { Wallet } from '../../api/types.ts'
import { Checkbox } from '../../components/Field/Field.tsx'
import { tk } from '../../i18n/translate.ts'
import styles from './settings.module.css'

export type WalletScope = { restricted: boolean; ids: string[] }

/** Lista portfeli bota dla API: pusta = wszystkie widoczne dla twórcy. */
export function walletIdsOf(scope: WalletScope): string[] {
  return scope.restricted ? scope.ids : []
}

export function walletScopeError(scope: WalletScope): string | undefined {
  return scope.restricted && scope.ids.length === 0 ? tk('settings:walletScope.required') : undefined
}

/** Ograniczenie bota do portfeli: wszystkie widoczne dla twórcy albo wybrane. */
export function WalletScopePicker({
  value,
  onChange,
  wallets,
  error,
}: {
  value: WalletScope
  onChange: (scope: WalletScope) => void
  wallets: Wallet[]
  error?: string
}) {
  const { t } = useTranslation('settings')
  const fieldId = useId()
  const errorId = `${fieldId}-error`
  const known = new Set(wallets.map((w) => w.id))
  const hidden = value.ids.filter((id) => !known.has(id))
  return (
    <fieldset className={styles.fieldset} aria-describedby={error ? errorId : undefined}>
      <legend>{t('walletScope.legend')}</legend>
      <Checkbox
        label={t('walletScope.restricted')}
        hint={t('walletScope.restrictedHint')}
        checked={value.restricted}
        onChange={(event) => onChange({ ...value, restricted: event.target.checked })}
      />
      {value.restricted && (
        <ul className={styles.walletPicker} aria-label={t('walletScope.listLabel')}>
          {wallets.map((wallet) => (
            <li key={wallet.id}>
              <Checkbox
                label={t(wallet.archived_at ? 'walletScope.walletArchived' : 'walletScope.wallet', {
                  name: wallet.name,
                  currency: wallet.currency,
                })}
                checked={value.ids.includes(wallet.id)}
                onChange={(event) =>
                  onChange({
                    ...value,
                    ids: event.target.checked ? [...value.ids, wallet.id] : value.ids.filter((id) => id !== wallet.id),
                  })
                }
              />
            </li>
          ))}
          {wallets.length === 0 && <li className={styles.rowMeta}>{t('walletScope.none')}</li>}
          {hidden.length > 0 && (
            <li className={styles.rowMeta}>{t('walletScope.hidden', { count: hidden.length })}</li>
          )}
        </ul>
      )}
      {error && (
        <p id={errorId} role="alert" className={styles.rowMeta}>
          {error}
        </p>
      )}
    </fieldset>
  )
}
