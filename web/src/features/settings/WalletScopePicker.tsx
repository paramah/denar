import { useId } from 'react'
import type { Wallet } from '../../api/types.ts'
import { Checkbox } from '../../components/Field/Field.tsx'
import styles from './settings.module.css'

export type WalletScope = { restricted: boolean; ids: string[] }

/** Lista portfeli bota dla API: pusta = wszystkie widoczne dla twórcy. */
export function walletIdsOf(scope: WalletScope): string[] {
  return scope.restricted ? scope.ids : []
}

export function walletScopeError(scope: WalletScope): string | undefined {
  return scope.restricted && scope.ids.length === 0 ? 'Wybierz co najmniej jeden portfel albo pozwól na wszystkie.' : undefined
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
  const name = useId()
  const errorId = `${name}-error`
  const known = new Set(wallets.map((w) => w.id))
  const hidden = value.ids.filter((id) => !known.has(id))
  return (
    <fieldset className={styles.fieldset} aria-describedby={error ? errorId : undefined}>
      <legend>Portfele</legend>
      <Checkbox
        label="Tylko wybrane portfele"
        hint="Bez ograniczenia bot widzi wszystkie portfele, które widzi jego twórca (także te dodane później)."
        checked={value.restricted}
        onChange={(event) => onChange({ ...value, restricted: event.target.checked })}
      />
      {value.restricted && (
        <ul className={styles.walletPicker} aria-label="Portfele bota">
          {wallets.map((wallet) => (
            <li key={wallet.id}>
              <Checkbox
                label={`${wallet.name} (${wallet.currency})${wallet.archived_at ? ' — zarchiwizowany' : ''}`}
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
          {wallets.length === 0 && <li className={styles.rowMeta}>Nie widzisz jeszcze żadnych portfeli.</li>}
          {hidden.length > 0 && (
            <li className={styles.rowMeta}>
              Oraz portfele niewidoczne dla Ciebie: {hidden.length} (zostaną bez zmian).
            </li>
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
