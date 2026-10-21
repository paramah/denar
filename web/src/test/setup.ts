import './polish.ts'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import '../i18n/index.ts'
import { setLocale } from '../i18n/locale.ts'
import { installDialogShim } from './dialogShim.ts'
import { server, takeContractViolations } from './msw/server.ts'

installDialogShim()

window.scrollTo = () => undefined

// Żądanie bez handlera to błąd testu, nie wywołanie do wykonania.
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))

afterEach(async () => {
  cleanup()
  server.resetHandlers()
  window.localStorage.clear()
  window.sessionStorage.clear()
  // Test zmieniający język nie zostawia go następnym.
  setLocale('pl')
  const violations = await takeContractViolations()
  if (violations.length > 0) {
    throw new Error(`Mocked responses do not match api/openapi.yaml:\n${violations.join('\n')}`)
  }
})

afterAll(() => server.close())
