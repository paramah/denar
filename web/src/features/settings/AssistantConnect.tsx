import type { Scope } from '../../api/types.ts'
import { CopyBlock, CopyField } from '../../components/CopyField/CopyField.tsx'
import { Callout } from '../../components/Form/Form.tsx'
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
    ...(has('subscriptions:read') ? ['Co odnawia się w tym tygodniu i ile to kosztuje?'] : []),
    ...(has('budget:read') ? ['Ile zostało mi w kopercie „Jedzenie” w tym miesiącu?'] : []),
    ...(has('reports:read') ? ['Na co wydałem najwięcej w zeszłym miesiącu?'] : []),
    ...(has('wallets:read') ? ['Jakie mam salda na portfelach?'] : []),
    ...(has('transactions:write') ? ['Wydałem 42,50 zł w Biedronce z karty, na jedzenie.'] : []),
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
  const url = mcpUrl(origin)
  const prompts = examplePrompts(scopes)
  return (
    <section className={styles.connect} aria-labelledby="assistant-connect">
      <h3 id="assistant-connect">Podłącz do asystenta AI</h3>
      <p className={styles.connectLead}>
        Denar ma serwer MCP — asystent (np. Claude) czyta przez niego dane i dopisuje wydatki z uprawnieniami tego bota.
        Polecenia poniżej zawierają token, więc też skopiuj je teraz.
      </p>
      {isInsecure(origin) && (
        <Callout tone="warn" title="Połączenie bez HTTPS">
          Token pójdzie przez sieć niezaszyfrowany. Udostępnij Denara przez HTTPS (np. za reverse proxy), zanim podłączysz
          asystenta spoza tego komputera.
        </Callout>
      )}
      <CopyField label="Adres serwera MCP" value={url} hint="Transport: Streamable HTTP." />
      <CopyField label="Nagłówek uwierzytelnienia" value={`Authorization: Bearer ${token}`} />
      <CopyBlock label="Claude Code — polecenie w terminalu" value={claudeCodeCommand(url, token)} />
      <details className={styles.connectMore}>
        <summary>Inne klienty: konfiguracja JSON</summary>
        <div className={styles.connectBody}>
          <CopyBlock
            label="Klient ze zdalnym MCP (HTTP z nagłówkami)"
            value={remoteHttpConfig(url, token)}
            hint="Np. plik .mcp.json Claude Code albo konfiguracja Cursora."
          />
          <CopyBlock
            label="Klient bez nagłówków — most mcp-remote"
            value={mcpRemoteConfig(url, token)}
            hint="Np. claude_desktop_config.json w Claude Desktop. Wymaga Node.js (npx)."
          />
        </div>
      </details>
      {prompts.length > 0 && (
        <div>
          <p className={styles.connectLead}>Potem zapytaj na przykład:</p>
          <ul className={styles.prompts}>
            {prompts.map((prompt) => (
              <li key={prompt}>„{prompt}”</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}
