import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
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
import { tk } from '../../i18n/translate.ts'
import { addDays, endOfLocalDay, formatDate, formatRelative, toDateInput } from '../../lib/format.ts'
import { allScopes, assistantScopes, readScopes, roleAllowsScope, scopeGroups, scopeLabel, toggleScope } from '../../lib/scopes.ts'
import { AssistantConnect } from './AssistantConnect.tsx'
import styles from './settings.module.css'
import { walletIdsOf, walletScopeError, WalletScopePicker, type WalletScope } from './WalletScopePicker.tsx'

/** Domyślne wygaśnięcie nowego tokenu. */
export const defaultExpiryDays = 90

type Status = { label: string; tone: 'positive' | 'neutral' | 'warn' }

export function botStatus(bot: Bot, now = new Date()): Status {
  if (bot.disabled) {
    return { label: tk('settings:bots.status.disabled'), tone: 'neutral' }
  }
  if (bot.expires_at && new Date(bot.expires_at) <= now) {
    return { label: tk('settings:bots.status.expired'), tone: 'warn' }
  }
  if (bot.scopes.length > 0 && bot.effective_scopes.length === 0) {
    return { label: tk('settings:bots.status.noScopes'), tone: 'warn' }
  }
  return { label: tk('settings:bots.status.active'), tone: 'positive' }
}

export function BotsTab() {
  const { t } = useTranslation('settings')
  useDocumentTitle(t('bots.documentTitle'))
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
          <h2 id="bots-heading">{t('bots.heading')}</h2>
          <p>{t('bots.lead')}</p>
        </div>
        <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
          {t('bots.new')}
        </Button>
      </div>
      <QueryState query={bots} errorTitle={t('bots.loadFailed')}>
        {(items) => (
          <DataList
            label={t('bots.listLabel')}
            items={items}
            variant="cards"
            getKey={(bot) => bot.id}
            empty={
              <EmptyState
                icon="bot"
                title={t('bots.empty.title')}
                headingLevel={3}
                action={
                  <Button variant="primary" icon="plus" onClick={() => setCreating(true)}>
                    {t('bots.empty.action')}
                  </Button>
                }
              >
                <p>{t('bots.empty.body')}</p>
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
      <Dialog open={!!scoping} title={scoping ? t('bots.walletsOf', { name: scoping.name }) : ''} onClose={() => setScoping(null)}>
        {scoping && <BotWalletsForm bot={scoping} wallets={wallets.data ?? []} onDone={() => setScoping(null)} />}
      </Dialog>
    </section>
  )
}

/** „wszystkie widoczne dla twórcy” albo nazwy portfeli bota. */
export function botWalletsLabel(bot: Bot, wallets: Wallet[]): string {
  if (bot.wallet_ids.length === 0) {
    return tk('settings:bots.walletsAll')
  }
  const names = new Map(wallets.map((w) => [w.id, w.name]))
  const known = bot.wallet_ids.flatMap((id) => (names.has(id) ? [names.get(id)!] : []))
  const hidden = bot.wallet_ids.length - known.length
  return [...known, ...(hidden > 0 ? [tk('settings:bots.walletsHidden', { count: hidden })] : [])].join(', ')
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
  const { t } = useTranslation('settings')
  const update = useUpdateBot(household.id)
  const toast = useToast()
  const status = botStatus(bot)
  const effective = new Set(bot.effective_scopes)

  return (
    <article className={styles.row} aria-label={t('bots.row.label', { name: bot.name })}>
      <div className={styles.rowMain}>
        <h3 className={styles.rowTitle}>
          {bot.name}
          <Badge tone={status.tone}>{status.label}</Badge>
        </h3>
        <span className={styles.rowMeta}>
          <Trans
            t={t}
            i18nKey="bots.row.meta"
            values={{
              prefix: bot.token_prefix,
              created: formatDate(bot.created_at),
              used: bot.last_used_at
                ? t('bots.row.usedAt', { when: formatRelative(bot.last_used_at) })
                : t('bots.row.neverUsed'),
              expires: bot.expires_at ? t('bots.row.expiresAt', { date: formatDate(bot.expires_at) }) : t('bots.row.noExpiry'),
            }}
            components={{ code: <code /> }}
          />
        </span>
        <span className={styles.rowMeta}>{t('bots.row.wallets', { wallets: botWalletsLabel(bot, wallets) })}</span>
        <ul className={styles.chips} aria-label={t('bots.scopes')}>
          {bot.scopes.map((scope) => (
            <li key={scope}>
              <Badge
                struck={!effective.has(scope)}
                title={effective.has(scope) ? undefined : t('bots.row.inactiveTitle')}
              >
                {scopeLabel(scope)}
                {!effective.has(scope) && <span className="visually-hidden">{t('bots.row.inactive')}</span>}
              </Badge>
            </li>
          ))}
        </ul>
        {update.error ? (
          <span role="alert" className={styles.rowMeta}>
            {t('bots.row.updateFailed')}
          </span>
        ) : null}
      </div>
      <div className={styles.rowActions}>
        <Button
          size="sm"
          icon={bot.disabled ? 'play' : 'pause'}
          busy={update.isPending}
          aria-label={t(bot.disabled ? 'bots.row.enableLabel' : 'bots.row.disableLabel', { name: bot.name })}
          onClick={() =>
            update.mutate(
              { botId: bot.id, body: { disabled: !bot.disabled } },
              {
                onSuccess: (changed) =>
                  toast.show({
                    message: t(changed.disabled ? 'bots.row.disabled' : 'bots.row.enabled', { name: changed.name }),
                  }),
              },
            )
          }
        >
          {bot.disabled ? t('bots.row.enable') : t('bots.row.disable')}
        </Button>
        <Button size="sm" icon="wallet" onClick={onWallets} aria-label={t('bots.walletsOf', { name: bot.name })}>
          {t('bots.row.walletsButton')}
        </Button>
        <Button size="sm" icon="key" onClick={onRotate} aria-label={t('bots.row.rotateLabel', { name: bot.name })}>
          {t('bots.row.rotate')}
        </Button>
        <Button size="sm" variant="danger" icon="trash" onClick={onDelete} aria-label={t('bots.row.deleteLabel', { name: bot.name })}>
          {t('bots.row.delete')}
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
  const { t } = useTranslation('settings')
  return (
    <Dialog open={open} title={t('bots.new')} onClose={onClose} size="lg">
      <CreateBotForm onCancel={onClose} onCreated={onCreated} />
    </Dialog>
  )
}

function CreateBotForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: (result: BotWithToken) => void }) {
  const { household, role } = useHousehold()
  const { t } = useTranslation('settings')
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
      name: name.trim() ? undefined : t('bots.form.nameRequired'),
      scopes: scopes.length > 0 ? undefined : t('bots.form.scopesRequired'),
      expires_at:
        expires && (!expiresAt || new Date(expiresAt) <= new Date()) ? t('bots.form.futureDate') : undefined,
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
      aria-label={t('bots.new')}
      className={styles.section}
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      {duplicate ? (
        <Callout tone="warn">{t('bots.form.duplicate')}</Callout>
      ) : (
        <FormAlert error={create.error} shownFields={['name', 'scopes', 'expires_at', 'wallet_ids']} />
      )}
      <TextField
        label={t('bots.form.name')}
        value={name}
        onChange={(event) => setName(event.target.value)}
        hint={t('bots.form.nameHint')}
        error={errors.name ?? server.name}
        maxLength={100}
        autoComplete="off"
        required
      />

      <fieldset className={styles.fieldset} aria-describedby={errors.scopes ? 'scopes-error' : undefined}>
        <legend>{t('bots.scopes')}</legend>
        <div className={styles.presets}>
          <span>{t('bots.form.presets')}</span>
          <Button size="sm" onClick={() => setScopes(readScopes)}>
            {t('bots.form.presetRead')}
          </Button>
          <Button size="sm" onClick={() => setScopes(assistantScopes.filter((s) => roleAllowsScope(role, s)))}>
            {t('bots.form.presetAssistant')}
          </Button>
          <Button size="sm" onClick={() => setScopes(allScopes.filter((s) => roleAllowsScope(role, s)))}>
            {t('bots.form.presetAll')}
          </Button>
        </div>
        <table className={styles.scopeTable}>
          <thead>
            <tr>
              <th scope="col">{t('bots.form.area')}</th>
              <th scope="col">{t('bots.form.read')}</th>
              <th scope="col">{t('bots.form.write')}</th>
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
                    <span aria-label={t('bots.form.noWrite')}>—</span>
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
        <legend>{t('bots.form.validity')}</legend>
        <div className={styles.expiry}>
          <Checkbox
            label={t('bots.form.expires')}
            checked={expires}
            onChange={(event) => setExpires(event.target.checked)}
            hint={t('bots.form.expiresHint', { count: defaultExpiryDays })}
          />
          {expires && (
            <TextField
              label={t('bots.form.expiryDate')}
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
        <Button onClick={onCancel}>{t('bots.cancel')}</Button>
        <Button type="submit" variant="primary" busy={create.isPending}>
          {t('bots.form.submit')}
        </Button>
      </div>
    </form>
  )
}

function TokenDialog({ result, onClose }: { result: BotWithToken | null; onClose: () => void }) {
  const { t } = useTranslation('settings')
  return (
    <Dialog
      open={!!result}
      title={result ? t('bots.token.title', { name: result.bot.name }) : ''}
      onClose={onClose}
      size="lg"
      footer={
        <Button variant="primary" onClick={onClose}>
          {t('bots.token.done')}
        </Button>
      }
    >
      {result && (
        <>
          <Callout tone="warn" title={t('bots.token.onceTitle')}>
            {t('bots.token.onceBody')}
          </Callout>
          <CopyField
            label={t('bots.token.label')}
            value={result.token}
            initialFocus
            hint={t('bots.token.hint')}
          />
          <AssistantConnect token={result.token} scopes={result.bot.effective_scopes} />
        </>
      )}
    </Dialog>
  )
}

function RotateDialog({ bot, onDone }: { bot: Bot; onDone: (result: BotWithToken | null) => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const rotate = useRotateBotToken(household.id)
  return (
    <ConfirmDialog
      open
      title={t('bots.rotate.title', { name: bot.name })}
      confirmLabel={t('bots.rotate.confirm')}
      danger={false}
      busy={rotate.isPending}
      error={<FormAlert error={rotate.error} />}
      onCancel={() => onDone(null)}
      onConfirm={() => rotate.mutate(bot.id, { onSuccess: (result) => onDone(result) })}
    >
      <p>{t('bots.rotate.body')}</p>
    </ConfirmDialog>
  )
}

function DeleteDialog({ bot, onDone }: { bot: Bot; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const remove = useDeleteBot(household.id)
  const toast = useToast()
  return (
    <ConfirmDialog
      open
      title={t('bots.delete.title', { name: bot.name })}
      confirmLabel={t('bots.delete.confirm')}
      busy={remove.isPending}
      error={<FormAlert error={remove.error} />}
      onCancel={onDone}
      onConfirm={() =>
        remove.mutate(bot.id, {
          onSuccess: () => {
            toast.show({ message: t('bots.delete.done', { name: bot.name }) })
            onDone()
          },
        })
      }
    >
      <p>{t('bots.delete.body')}</p>
    </ConfirmDialog>
  )
}

function BotWalletsForm({ bot, wallets, onDone }: { bot: Bot; wallets: Wallet[]; onDone: () => void }) {
  const { household } = useHousehold()
  const { t } = useTranslation('settings')
  const update = useUpdateBot(household.id)
  const toast = useToast()
  const [scope, setScope] = useState<WalletScope>({ restricted: bot.wallet_ids.length > 0, ids: bot.wallet_ids })
  const [error, setError] = useState<string>()
  const server = fieldErrors(update.error)
  return (
    <form
      noValidate
      aria-label={t('bots.walletsOf', { name: bot.name })}
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
              toast.show({ message: t('bots.walletsSaved', { name: bot.name }) })
              onDone()
            },
          },
        )
      }}
    >
      <FormAlert error={update.error} shownFields={['wallet_ids']} />
      <WalletScopePicker value={scope} onChange={setScope} wallets={wallets} error={error ?? server.wallet_ids} />
      <div className={styles.rowActions}>
        <Button onClick={onDone}>{t('bots.cancel')}</Button>
        <Button type="submit" variant="primary" busy={update.isPending}>
          {t('bots.save')}
        </Button>
      </div>
    </form>
  )
}
