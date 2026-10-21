import { useState } from 'react'
import { ApiError, fieldErrors } from '../../api/errors.ts'
import { useBots, useCreateBot, useDeleteBot, useRotateBotToken, useUpdateBot } from '../../api/hooks/bots.ts'
import { useWallets } from '../../api/hooks/wallets.ts'
import type { Bot, BotWithToken, Scope, Wallet } from '../../api/types.ts'
import { useHousehold } from '../../app/household.tsx'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Button } from '../../components/Button/Button.tsx'
import { CopyField } from '../../components/CopyField/CopyField.tsx'
import { DataList } from '../../components/DataList/DataList.tsx'
import { ConfirmDialog, Dialog } from '../../components/Dialog/Dialog.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import { Checkbox, TextField } from '../../components/Field/Field.tsx'
import { Callout, FormAlert } from '../../components/Form/Form.tsx'
import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { QueryState } from '../../components/QueryState/QueryState.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { addDays, endOfLocalDay, formatDate, formatRelative, toDateInput } from '../../lib/format.ts'
import { allScopes, readScopes, roleAllowsScope, scopeGroups, scopeLabel, toggleScope } from '../../lib/scopes.ts'
import styles from './settings.module.css'
import { walletIdsOf, walletScopeError, WalletScopePicker, type WalletScope } from './WalletScopePicker.tsx'

/** Domyślne wygaśnięcie nowego tokenu. */
export const defaultExpiryDays = 90

type Status = { label: string; tone: 'positive' | 'neutral' | 'warn' }

export function botStatus(bot: Bot, now = new Date()): Status {
  if (bot.disabled) {
    return { label: 'Wyłączony', tone: 'neutral' }
  }
  if (bot.expires_at && new Date(bot.expires_at) <= now) {
    return { label: 'Wygasł', tone: 'warn' }
  }
  if (bot.scopes.length > 0 && bot.effective_scopes.length === 0) {
    return { label: 'Bez uprawnień', tone: 'warn' }
  }
  return { label: 'Aktywny', tone: 'positive' }
}

export function BotsTab() {
  useDocumentTitle('Boty')
  const { household } = useHousehold()
  const bots = useBots(household.id)
  const [creating, setCreating] = useState(false)
  const [token, setToken] = useState<BotWithToken | null>(null)
  const [rotating, setRotating] = useState<Bot | null>(null)
  const [deleting, setDeleting] = useState<Bot | null>(null)
  const [scoping, setScoping] = useState<Bot | null>(null)
  const wallets = useWallets(household.id, { includeArchived: true })

  return (
    <section className={styles.section} aria-labelledby="bots-heading">
      <div className={styles.sectionHead}>
        <div>
          <h2 id="bots-heading">Boty i asystenci AI</h2>
          <p>
            Bot dostaje token do API i MCP. Widzi tylko to, na co pozwalają jego uprawnienia — i nigdy więcej niż
            osoba, która go utworzyła.
          </p>
        </div>
        <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
          Nowy bot
        </Button>
      </div>
      <QueryState query={bots} errorTitle="Nie udało się wczytać botów">
        {(items) => (
          <DataList
            label="Boty gospodarstwa"
            items={items}
            variant="cards"
            getKey={(bot) => bot.id}
            empty={
              <EmptyState
                icon="bot"
                title="Nie ma jeszcze botów"
                headingLevel={3}
                action={
                  <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                    Utwórz pierwszego bota
                  </Button>
                }
              >
                <p>
                  Bot pozwala asystentowi AI (np. Claude przez MCP) albo własnemu skryptowi czytać analizy i dopisywać
                  wydatki w Twoim imieniu.
                </p>
              </EmptyState>
            }
            renderItem={(bot) => (
              <BotRow
                bot={bot}
                wallets={wallets.data ?? []}
                onWallets={() => setScoping(bot)}
                onRotate={() => setRotating(bot)}
                onDelete={() => setDeleting(bot)}
              />
            )}
          />
        )}
      </QueryState>

      <CreateBotDialog
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={(result) => {
          setCreating(false)
          setToken(result)
        }}
      />
      <TokenDialog result={token} onClose={() => setToken(null)} />
      {rotating && (
        <RotateDialog
          bot={rotating}
          onDone={(result) => {
            setRotating(null)
            if (result) {
              setToken(result)
            }
          }}
        />
      )}
      {deleting && <DeleteDialog bot={deleting} onDone={() => setDeleting(null)} />}
      <Dialog open={!!scoping} title={scoping ? `Portfele bota ${scoping.name}` : ''} onClose={() => setScoping(null)}>
        {scoping && <BotWalletsForm bot={scoping} wallets={wallets.data ?? []} onDone={() => setScoping(null)} />}
      </Dialog>
    </section>
  )
}

/** „wszystkie widoczne dla twórcy” albo nazwy portfeli bota. */
export function botWalletsLabel(bot: Bot, wallets: Wallet[]): string {
  if (bot.wallet_ids.length === 0) {
    return 'wszystkie widoczne dla twórcy'
  }
  const names = new Map(wallets.map((w) => [w.id, w.name]))
  const known = bot.wallet_ids.flatMap((id) => (names.has(id) ? [names.get(id)!] : []))
  const hidden = bot.wallet_ids.length - known.length
  return [...known, ...(hidden > 0 ? [`niewidoczne dla Ciebie: ${hidden}`] : [])].join(', ')
}

function BotRow({
  bot,
  wallets,
  onWallets,
  onRotate,
  onDelete,
}: {
  bot: Bot
  wallets: Wallet[]
  onWallets: () => void
  onRotate: () => void
  onDelete: () => void
}) {
  const { household } = useHousehold()
  const update = useUpdateBot(household.id)
  const toast = useToast()
  const status = botStatus(bot)
  const effective = new Set(bot.effective_scopes)

  return (
    <article className={styles.row} aria-label={`Bot ${bot.name}`}>
      <div className={styles.rowMain}>
        <h3 className={styles.rowTitle}>
          {bot.name}
          <Badge tone={status.tone}>{status.label}</Badge>
        </h3>
        <span className={styles.rowMeta}>
          Token <code>{bot.token_prefix}…</code>, utworzony {formatDate(bot.created_at)},{' '}
          {bot.last_used_at ? `ostatnio użyty ${formatRelative(bot.last_used_at)}` : 'jeszcze nieużyty'},{' '}
          {bot.expires_at ? `wygasa ${formatDate(bot.expires_at)}` : 'bez daty wygaśnięcia'}
        </span>
        <span className={styles.rowMeta}>Portfele: {botWalletsLabel(bot, wallets)}</span>
        <ul className={styles.chips} aria-label="Uprawnienia">
          {bot.scopes.map((scope) => (
            <li key={scope}>
              <Badge
                struck={!effective.has(scope)}
                title={effective.has(scope) ? undefined : 'Obecna rola twórcy nie daje tego uprawnienia'}
              >
                {scopeLabel(scope)}
                {!effective.has(scope) && <span className="visually-hidden"> (nieaktywne)</span>}
              </Badge>
            </li>
          ))}
        </ul>
        {update.error ? (
          <span role="alert" className={styles.rowMeta}>
            Nie udało się zmienić bota. Spróbuj ponownie.
          </span>
        ) : null}
      </div>
      <div className={styles.rowActions}>
        <Button
          size="sm"
          icon={bot.disabled ? 'play' : 'pause'}
          busy={update.isPending}
          aria-label={`${bot.disabled ? 'Włącz' : 'Wyłącz'} ${bot.name}`}
          onClick={() =>
            update.mutate(
              { botId: bot.id, body: { disabled: !bot.disabled } },
              {
                onSuccess: (changed) =>
                  toast.show({ message: changed.disabled ? `Wyłączono bota ${changed.name}.` : `Włączono bota ${changed.name}.` }),
              },
            )
          }
        >
          {bot.disabled ? 'Włącz' : 'Wyłącz'}
        </Button>
        <Button size="sm" icon="wallet" onClick={onWallets} aria-label={`Portfele bota ${bot.name}`}>
          Portfele
        </Button>
        <Button size="sm" icon="key" onClick={onRotate} aria-label={`Nowy token dla ${bot.name}`}>
          Nowy token
        </Button>
        <Button size="sm" variant="danger" icon="trash" onClick={onDelete} aria-label={`Usuń ${bot.name}`}>
          Usuń
        </Button>
      </div>
    </article>
  )
}

function CreateBotDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (result: BotWithToken) => void
}) {
  return (
    <Dialog open={open} title="Nowy bot" onClose={onClose} size="lg">
      <CreateBotForm onCancel={onClose} onCreated={onCreated} />
    </Dialog>
  )
}

function CreateBotForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: (result: BotWithToken) => void }) {
  const { household, role } = useHousehold()
  const create = useCreateBot(household.id)
  const wallets = useWallets(household.id)
  const [walletScope, setWalletScope] = useState<WalletScope>({ restricted: false, ids: [] })
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<Scope[]>(readScopes)
  const [expires, setExpires] = useState(true)
  const [expiryDate, setExpiryDate] = useState(() => toDateInput(addDays(new Date(), defaultExpiryDays)))
  const [errors, setErrors] = useState<Record<string, string | undefined>>({})
  const server = fieldErrors(create.error)
  const duplicate = create.error instanceof ApiError && create.error.reason === 'duplicate-name'
  const tomorrow = toDateInput(addDays(new Date(), 1))

  const submit = () => {
    const expiresAt = expires ? endOfLocalDay(expiryDate) : null
    const next = {
      name: name.trim() ? undefined : 'Podaj nazwę bota.',
      scopes: scopes.length > 0 ? undefined : 'Wybierz co najmniej jedno uprawnienie.',
      expires_at:
        expires && (!expiresAt || new Date(expiresAt) <= new Date()) ? 'Wybierz datę w przyszłości.' : undefined,
      wallet_ids: walletScopeError(walletScope),
    }
    setErrors(next)
    if (next.name || next.scopes || next.expires_at || next.wallet_ids) {
      return
    }
    const walletIds = walletIdsOf(walletScope)
    create.mutate(
      { name: name.trim(), scopes, expires_at: expiresAt, ...(walletIds.length > 0 ? { wallet_ids: walletIds } : {}) },
      { onSuccess: onCreated },
    )
  }

  return (
    <form
      noValidate
      aria-label="Nowy bot"
      className={styles.section}
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      {duplicate ? (
        <Callout tone="warn">Bot o tej nazwie już istnieje. Wybierz inną.</Callout>
      ) : (
        <FormAlert error={create.error} shownFields={['name', 'scopes', 'expires_at', 'wallet_ids']} />
      )}
      <TextField
        label="Nazwa"
        value={name}
        onChange={(event) => setName(event.target.value)}
        hint="Po nazwie rozpoznasz bota w dzienniku zmian, np. „Claude”, „Import z banku”."
        error={errors.name ?? server.name}
        maxLength={100}
        autoComplete="off"
        required
      />

      <fieldset className={styles.fieldset} aria-describedby={errors.scopes ? 'scopes-error' : undefined}>
        <legend>Uprawnienia</legend>
        <div className={styles.presets}>
          <span>Gotowe zestawy:</span>
          <Button size="sm" onClick={() => setScopes(readScopes)}>
            Tylko odczyt
          </Button>
          <Button size="sm" onClick={() => setScopes(allScopes.filter((s) => roleAllowsScope(role, s)))}>
            Odczyt i zapis
          </Button>
        </div>
        <table className={styles.scopeTable}>
          <thead>
            <tr>
              <th scope="col">Obszar</th>
              <th scope="col">Odczyt</th>
              <th scope="col">Zapis</th>
            </tr>
          </thead>
          <tbody>
            {scopeGroups.map((group) => (
              <tr key={group.id}>
                <th scope="row" className={styles.scopeArea}>
                  {group.label}
                  <span>{group.description}</span>
                </th>
                <td>
                  <input
                    type="checkbox"
                    aria-label={scopeLabel(group.read)}
                    checked={scopes.includes(group.read)}
                    onChange={(event) => setScopes(toggleScope(scopes, group.read, event.target.checked))}
                  />
                </td>
                <td>
                  {group.write ? (
                    <input
                      type="checkbox"
                      aria-label={scopeLabel(group.write)}
                      checked={scopes.includes(group.write)}
                      disabled={!roleAllowsScope(role, group.write)}
                      onChange={(event) => setScopes(toggleScope(scopes, group.write!, event.target.checked))}
                    />
                  ) : (
                    <span aria-label="Brak zapisu">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(errors.scopes ?? server.scopes) && (
          <p id="scopes-error" className={styles.rowMeta} role="alert">
            {errors.scopes ?? server.scopes}
          </p>
        )}
      </fieldset>

      <WalletScopePicker
        value={walletScope}
        onChange={setWalletScope}
        wallets={(wallets.data ?? []).filter((w) => !w.archived_at)}
        error={errors.wallet_ids ?? server.wallet_ids}
      />

      <fieldset className={styles.fieldset}>
        <legend>Ważność tokenu</legend>
        <div className={styles.expiry}>
          <Checkbox
            label="Token wygasa"
            checked={expires}
            onChange={(event) => setExpires(event.target.checked)}
            hint={`Zalecane. Domyślnie po ${defaultExpiryDays} dniach.`}
          />
          {expires && (
            <TextField
              label="Wygasa po dniu"
              type="date"
              value={expiryDate}
              min={tomorrow}
              onChange={(event) => setExpiryDate(event.target.value)}
              error={errors.expires_at ?? server.expires_at}
            />
          )}
        </div>
      </fieldset>

      <div className={styles.rowActions}>
        <Button onClick={onCancel}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={create.isPending}>
          Utwórz bota
        </Button>
      </div>
    </form>
  )
}

function TokenDialog({ result, onClose }: { result: BotWithToken | null; onClose: () => void }) {
  return (
    <Dialog
      open={!!result}
      title={result ? `Token bota ${result.bot.name}` : ''}
      onClose={onClose}
      footer={
        <Button variant="primary" onClick={onClose}>
          Zapisałem token
        </Button>
      }
    >
      {result && (
        <>
          <Callout tone="warn" title="Token pokazujemy tylko raz">
            Skopiuj go teraz i zapisz w bezpiecznym miejscu (np. menedżerze haseł). Po zamknięciu okna odzyskać go się
            nie da — można jedynie wygenerować nowy.
          </Callout>
          <CopyField
            label="Token"
            value={result.token}
            initialFocus
            hint="Bot przedstawia się nagłówkiem Authorization: Bearer <token>."
          />
        </>
      )}
    </Dialog>
  )
}

function RotateDialog({ bot, onDone }: { bot: Bot; onDone: (result: BotWithToken | null) => void }) {
  const { household } = useHousehold()
  const rotate = useRotateBotToken(household.id)
  return (
    <ConfirmDialog
      open
      title={`Nowy token dla ${bot.name}?`}
      confirmLabel="Wygeneruj nowy token"
      danger={false}
      busy={rotate.isPending}
      error={<FormAlert error={rotate.error} />}
      onCancel={() => onDone(null)}
      onConfirm={() => rotate.mutate(bot.id, { onSuccess: (result) => onDone(result) })}
    >
      <p>Obecny token przestanie działać od razu. Wpisz nowy wszędzie, gdzie bot jest używany.</p>
    </ConfirmDialog>
  )
}

function DeleteDialog({ bot, onDone }: { bot: Bot; onDone: () => void }) {
  const { household } = useHousehold()
  const remove = useDeleteBot(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={`Usunąć bota ${bot.name}?`}
      confirmLabel="Usuń bota"
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(bot.id, {
          onSuccess: () => {
            toast.show({ message: `Usunięto bota ${bot.name}.` })
            onDone()
          },
        })
      }
    >
      <p>Token przestanie działać. Wpisy bota w dzienniku zmian zostaną.</p>
    </ConfirmDialog>
  )
}

function BotWalletsForm({ bot, wallets, onDone }: { bot: Bot; wallets: Wallet[]; onDone: () => void }) {
  const { household } = useHousehold()
  const update = useUpdateBot(household.id)
  const toast = useToast()
  const [scope, setScope] = useState<WalletScope>({ restricted: bot.wallet_ids.length > 0, ids: bot.wallet_ids })
  const [error, setError] = useState<string>()
  const server = fieldErrors(update.error)
  return (
    <form
      noValidate
      aria-label={`Portfele bota ${bot.name}`}
      className={styles.section}
      onSubmit={(event) => {
        event.preventDefault()
        const problem = walletScopeError(scope)
        setError(problem)
        if (problem) {
          return
        }
        update.mutate(
          { botId: bot.id, body: { wallet_ids: walletIdsOf(scope) } },
          {
            onSuccess: () => {
              toast.show({ message: `Zapisano portfele bota ${bot.name}.` })
              onDone()
            },
          },
        )
      }}
    >
      <FormAlert error={update.error} shownFields={['wallet_ids']} />
      <WalletScopePicker value={scope} onChange={setScope} wallets={wallets} error={error ?? server.wallet_ids} />
      <div className={styles.rowActions}>
        <Button onClick={onDone}>Anuluj</Button>
        <Button type="submit" variant="primary" busy={update.isPending}>
          Zapisz
        </Button>
      </div>
    </form>
  )
}
