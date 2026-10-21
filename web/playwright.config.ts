import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig, devices } from '@playwright/test'
import { adminEmail, adminName, adminPassword, baseURL, listenAddress, nbpPort, nbpURL, runDir } from './e2e/support/state.ts'

// Testy działają na zbudowanym binarium (`task build`, robi to task web:e2e),
// na bazie w katalogu tymczasowym, z kontem startowym z otoczenia.

const binary = process.env.DENAR_E2E_BINARY ?? fileURLToPath(new URL('../denar', import.meta.url))
if (!existsSync(binary)) {
  throw new Error(`No binary at ${binary}: run \`task build\` first (task web:e2e does).`)
}

const dir = runDir()

// Wygląd (motyw, szerokość telefonu) sprawdzają testy dostępności i układu;
// przepływ biznesowy idzie raz, w ciemnym motywie na komputerze.
const looks = /(a11y|mobile)\.spec\.ts$/

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  outputDir: './node_modules/.e2e/results',
  // Testy dzielą jedną bazę.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: './node_modules/.e2e/report' }]],
  globalTeardown: './e2e/global-teardown.ts',
  use: {
    baseURL,
    locale: 'pl-PL',
    timezoneId: 'Europe/Warsaw',
    trace: 'retain-on-failure',
    // Bez animacji wejścia: axe nie łapie dialogu w połowie przejścia.
    contextOptions: { reducedMotion: 'reduce' },
  },
  webServer: [
    {
      // Kursy walut bez sieci: serwer pyta atrapę zamiast api.nbp.pl.
      command: `node ${JSON.stringify(fileURLToPath(new URL('./e2e/support/nbp-stub.mjs', import.meta.url)))} ${nbpPort}`,
      url: `${nbpURL}/healthz`,
      reuseExistingServer: false,
      timeout: 10_000,
    },
    {
      command: `${JSON.stringify(binary)} serve`,
      url: `${baseURL}/healthz`,
      reuseExistingServer: false,
      timeout: 30_000,
      env: {
        ...(process.env as Record<string, string>),
        DENAR_ADDR: listenAddress,
        DENAR_DB: `${dir}/denar.db`,
        DENAR_AUTO_MIGRATE: 'true',
        DENAR_ADMIN_EMAIL: adminEmail,
        DENAR_ADMIN_PASSWORD: adminPassword(),
        DENAR_ADMIN_PASSWORD_FILE: '',
        DENAR_ADMIN_NAME: adminName,
        DENAR_INSTANCE_NAME: 'Denar E2E',
        DENAR_BASE_URL: baseURL,
        DENAR_LOCAL_LOGIN: 'true',
        DENAR_OIDC_ISSUER: '',
        DENAR_OIDC_CLIENT_ID: '',
        DENAR_OIDC_CLIENT_SECRET: '',
        DENAR_SESSION_TTL: '168h',
        DENAR_NBP_URL: nbpURL,
        DENAR_FX_INTERVAL: '1m',
      },
    },
  ],
  projects: [
    { name: 'chromium-dark', use: { ...devices['Desktop Chrome'], colorScheme: 'dark' } },
    { name: 'chromium-light', testMatch: looks, use: { ...devices['Desktop Chrome'], colorScheme: 'light' } },
    {
      name: 'chromium-mobile',
      testMatch: looks,
      use: { ...devices['Desktop Chrome'], colorScheme: 'light', viewport: { width: 390, height: 844 }, hasTouch: true },
    },
  ],
})
