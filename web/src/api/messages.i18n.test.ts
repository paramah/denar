import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '../i18n/locale.ts'
import { ApiError, errorMessage } from './errors.ts'
import { translateDeliveryError, translateFieldMessage } from './messages.ts'

afterEach(() => setLocale('pl'))

describe('field messages in English', () => {
  it.each([
    ['is required', 'This field is required.'],
    ['must be 1 to 100 characters', 'Enter 1 to 100 characters.'],
    ['must be at most 1 characters', 'At most 1 character.'],
    ['must be a decimal amount with at most 1 decimal places', 'Enter an amount with at most 1 decimal place.'],
    ['must be USD, the currency of the wallet', 'The currency must be USD — the currency of the wallet.'],
    ['is not a date: "2026-10-32"', '“2026-10-32” is not a date in this format.'],
    ['must be something new', 'Invalid value.'],
  ])('%s', (message, want) => {
    setLocale('en')
    expect(translateFieldMessage(message)).toBe(want)
  })

  it('keeps a Polish message from an older server as it is', () => {
    setLocale('en')
    expect(translateFieldMessage('za długa')).toBe('za długa')
  })

  it('uses Polish plural forms', () => {
    expect(translateFieldMessage('must be at most 1 characters')).toBe('Najwyżej 1 znak.')
    expect(translateFieldMessage('must be at most 3 characters')).toBe('Najwyżej 3 znaki.')
    expect(translateFieldMessage('must be at most 12 characters')).toBe('Najwyżej 12 znaków.')
    expect(translateFieldMessage('must be at most 22 characters')).toBe('Najwyżej 22 znaki.')
    expect(translateFieldMessage('must be a decimal amount with at most 1 decimal places')).toBe(
      'Podaj kwotę z najwyżej 1 miejscem po przecinku.',
    )
  })

  it('describes problems in English', () => {
    setLocale('en')
    const archived = new ApiError({ status: 409, code: 'conflict', type: 'urn:denar:problem:wallet-archived', title: 'Conflict' })
    expect(errorMessage(archived)).toBe('The wallet is archived — restore it to change anything in it.')
    expect(errorMessage(new ApiError({ status: 409, code: 'conflict', type: 'urn:denar:problem:new-thing', title: 'Conflict' }))).toBe(
      'Something went wrong. Try again.',
    )
    expect(errorMessage(new ApiError({ status: 429, code: 'rate_limited', title: 'Too many', retryAfter: 30 }))).toBe(
      'Too many attempts. Try again in 30 s.',
    )
  })
})

describe('translateDeliveryError', () => {
  // Serwer (013) zapisuje błędy dostarczania po angielsku; po polsku UI pokazuje dawny tekst.
  it.each([
    ['HTTP 503', 'HTTP 503', 'HTTP 503'],
    ['connection timed out', 'przekroczony czas połączenia', 'connection timed out'],
    ['connection failed: dial tcp: connection refused', 'błąd połączenia: dial tcp: connection refused', 'connection failed: dial tcp: connection refused'],
    ['connection failed', 'błąd połączenia', 'connection failed'],
    [
      'target address not allowed (private network) — see DENAR_NOTIFY_ALLOW_PRIVATE',
      'adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE',
      'target address not allowed (private network) — see DENAR_NOTIFY_ALLOW_PRIVATE',
    ],
    ['invalid recipient address', 'niepoprawny adres odbiorcy', 'invalid recipient address'],
    ['SMTP: connection failed: i/o timeout', 'SMTP: połączenie nieudane: i/o timeout', 'SMTP: connection failed: i/o timeout'],
    ['SMTP: server does not support STARTTLS', 'SMTP: serwer nie obsługuje STARTTLS', 'SMTP: the server does not support STARTTLS'],
    ['SMTP: server does not support AUTH', 'SMTP: serwer nie obsługuje AUTH', 'SMTP: the server does not support AUTH'],
    ['SMTP: login failed: 535 bad credentials', 'SMTP: logowanie nieudane: 535 bad credentials', 'SMTP: login failed: 535 bad credentials'],
    ['invalid Bot API address', 'niepoprawny adres Bot API', 'invalid Bot API address'],
    ['Telegram: response without ok', 'Telegram: odpowiedź bez ok', 'Telegram: response without ok'],
    ['invalid Matrix server address', 'niepoprawny adres serwera Matrix', 'invalid Matrix server address'],
    ['channel is disabled', 'kanał jest wyłączony', 'the channel is disabled'],
    [
      'DENAR_MASTER_KEY is missing, so the channel secrets cannot be read',
      'brak klucza DENAR_MASTER_KEY do odczytu sekretów kanału',
      'DENAR_MASTER_KEY is missing, so the channel secrets cannot be read',
    ],
    [
      'channel secrets cannot be read (was DENAR_MASTER_KEY changed?) — set them again',
      'nie można odczytać sekretów kanału (zmieniony DENAR_MASTER_KEY?) — ustaw je ponownie',
      'the channel secrets cannot be read (was DENAR_MASTER_KEY changed?) — set them again',
    ],
  ])('%s', (server, inPolish, inEnglish) => {
    expect(translateDeliveryError(server)).toBe(inPolish)
    setLocale('en')
    expect(translateDeliveryError(server)).toBe(inEnglish)
  })

  it('passes Polish text from an older server and unknown text through', () => {
    expect(translateDeliveryError('błąd połączenia: dial tcp')).toBe('błąd połączenia: dial tcp')
    setLocale('en')
    expect(translateDeliveryError('Telegram: Bad Request: chat not found')).toBe('Telegram: Bad Request: chat not found')
    expect(translateDeliveryError(null)).toBe('unknown error')
  })
})
