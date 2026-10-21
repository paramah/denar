import { mkdtempSync } from 'node:fs'
import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// Wspólne dla konfiguracji i testów. Katalog, port i hasło wybiera pierwszy
// proces, który wczyta konfigurację; pozostałe dostają je przez środowisko.

const dirKey = 'DENAR_E2E_DIR'
const passwordKey = 'DENAR_E2E_ADMIN_PASSWORD'
const portKey = 'DENAR_E2E_PORT'

async function freePort(): Promise<number> {
  const server = createServer()
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address() as AddressInfo
  await new Promise<void>((resolve) => server.close(() => resolve()))
  return port
}

process.env[portKey] ??= String(await freePort())
export const listenAddress = `127.0.0.1:${process.env[portKey]}`
/** 127.0.0.1, nie localhost: kontrola originu API porównuje hosty. */
export const baseURL = `http://${listenAddress}`

export const adminEmail = 'admin-e2e@example.test'
export const adminName = 'Ada Admin'

/** Katalog tymczasowy tego przebiegu: baza danych. */
export function runDir(): string {
  process.env[dirKey] ??= mkdtempSync(join(tmpdir(), 'denar-e2e-'))
  return process.env[dirKey]
}

/** Z otoczenia, gdy ustawione; inaczej losowe dla przebiegu. */
export function adminPassword(): string {
  process.env[passwordKey] ??= `e2e-${crypto.randomUUID()}`
  return process.env[passwordKey]
}
