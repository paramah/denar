import { useTranslation } from 'react-i18next'
import type { Scope } from '../../api/types.ts'
import { CopyBlock, CopyField } from '../../components/CopyField/CopyField.tsx'
import { Callout } from '../../components/Form/Form.tsx'
import { tk } from '../../i18n/translate.ts'
import styles from './settings.module.css'

/** Adres serwera MCP tej instancji (ten sam origin co Web UI, ścieżka `/mcp`). */
export function mcpUrl(origin = window.location.origin): string {
  return `${origin.replace(/\/+$/, '')}/mcp`
}

/** Polecenie Claude Code dodające serwer Denara. */
export function claudeCodeCommand(url: string, token: string): string {
  return `claude mcp add --transport http denar ${url} --header "Authorization: Bearer ${token}"`
}

/** Konfiguracja klienta z obsługą zdalnego MCP przez HTTP i nagłówki. */
export function remoteHttpConfig(url: string, token: string): string {
  return JSON.stringify(
    { mcpServers: { denar: { type: 'http', url, headers: { Authorization: `Bearer ${token}` } } } },
    null,
    2,
  )
}

/** Most `mcp-remote` dla klientów, które uruchamiają tylko lokalne serwery (np. claude_desktop_config.json). */
export function mcpRemoteConfig(url: string, token: string): string {
  return JSON.stringify(
    {
      mcpServers: {
        denar: { command: 'npx', args: ['-y', 'mcp-remote', url, '--header', `Authorization: Bearer ${token}`] },
      },
    },
    null,
    2,
  )
}

/** Przykładowe pytania dopasowane do uprawnień bota. */
export function examplePrompts(scopes: readonly Scope[]): string[] {
  const has = (scope: Scope) => scopes.includes(scope)
  return [
    ...(has('subscriptions:read') ? [tk('settings:assistant.prompts.renewals')] : []),
    ...(has('budget:read') ? [tk('settings:assistant.prompts.envelope')] : []),
    ...(has('reports:read') ? [tk('settings:assistant.prompts.spending')] : []),
    ...(has('wallets:read') ? [tk('settings:assistant.prompts.balances')] : []),
    ...(has('transactions:write') ? [tk('settings:assistant.prompts.expense')] : []),
  ]
}

function isInsecure(origin: string): boolean {
  try {
    const url = new URL(origin)
    return url.protocol === 'http:' && !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
  } catch {
    return false
  }
}

/**
 * Instrukcja podłączenia asystenta AI (MCP) z gotowymi do skopiowania
 * poleceniami; pokazywana razem z jednorazowym tokenem bota.
 */
export function AssistantConnect({ token, scopes, origin = window.location.origin }: { token: string; scopes: readonly Scope[]; origin?: string }) {
  const { t } = useTranslation('settings')
  const url = mcpUrl(origin)
  const prompts = examplePrompts(scopes)
  return (
    <section className={styles.connect} aria-labelledby="assistant-connect">
      <h3 id="assistant-connect">{t('assistant.title')}</h3>
      <p className={styles.connectLead}>{t('assistant.lead')}</p>
      {isInsecure(origin) && (
        <Callout tone="warn" title={t('assistant.insecureTitle')}>
          {t('assistant.insecureBody')}
        </Callout>
      )}
      <CopyField label={t('assistant.url')} value={url} hint={t('assistant.urlHint')} />
      <CopyField label={t('assistant.header')} value={`Authorization: Bearer ${token}`} />
      <CopyBlock label={t('assistant.claudeCode')} value={claudeCodeCommand(url, token)} />
      <details className={styles.connectMore}>
        <summary>{t('assistant.otherClients')}</summary>
        <div className={styles.connectBody}>
          <CopyBlock
            label={t('assistant.remoteHttp')}
            value={remoteHttpConfig(url, token)}
            hint={t('assistant.remoteHttpHint')}
          />
          <CopyBlock
            label={t('assistant.mcpRemote')}
            value={mcpRemoteConfig(url, token)}
            hint={t('assistant.mcpRemoteHint')}
          />
        </div>
      </details>
      {prompts.length > 0 && (
        <div>
          <p className={styles.connectLead}>{t('assistant.thenAsk')}</p>
          <ul className={styles.prompts}>
            {prompts.map((prompt) => (
              <li key={prompt}>{t('assistant.quoted', { prompt })}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}
