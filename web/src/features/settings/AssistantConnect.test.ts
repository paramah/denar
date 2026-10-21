import { describe, expect, it } from 'vitest'
import { claudeCodeCommand, examplePrompts, mcpRemoteConfig, mcpUrl, remoteHttpConfig } from './AssistantConnect.tsx'

describe('assistant connection snippets', () => {
  const url = mcpUrl('https://denar.example.com/')

  it('builds the MCP address from the origin', () => {
    expect(url).toBe('https://denar.example.com/mcp')
  })

  it('builds the Claude Code command and both JSON configurations', () => {
    expect(claudeCodeCommand(url, 'dnr_x')).toBe(
      'claude mcp add --transport http denar https://denar.example.com/mcp --header "Authorization: Bearer dnr_x"',
    )
    expect(JSON.parse(remoteHttpConfig(url, 'dnr_x'))).toEqual({
      mcpServers: { denar: { type: 'http', url, headers: { Authorization: 'Bearer dnr_x' } } },
    })
    expect(JSON.parse(mcpRemoteConfig(url, 'dnr_x'))).toEqual({
      mcpServers: { denar: { command: 'npx', args: ['-y', 'mcp-remote', url, '--header', 'Authorization: Bearer dnr_x'] } },
    })
  })

  it('suggests only prompts the bot can answer', () => {
    expect(examplePrompts(['wallets:read'])).toEqual(['Jakie mam salda na portfelach?'])
    expect(examplePrompts([])).toEqual([])
    expect(examplePrompts(['transactions:write']).join()).toMatch(/Biedronce/)
  })
})
