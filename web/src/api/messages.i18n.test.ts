import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '../i18n/locale.ts'
import { ApiError, errorMessage } from './errors.ts'
import { reasonMessage, translateDeliveryError, translateFieldMessage } from './messages.ts'

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
  // Serwer (013) zapisuje błędy dostarczania po angielsku; po polsku UI pokazuje dawny tekst,
  // po angielsku — tekst serwera.
  it.each([
    [
      'target not allowed (private network): see DENAR_NOTIFY_ALLOW_PRIVATE',
      'adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE',
    ],
    ['channel is disabled', 'kanał jest wyłączony'],
    ['DENAR_MASTER_KEY is not set: cannot read the channel secrets', 'brak klucza DENAR_MASTER_KEY do odczytu sekretów kanału'],
    [
      'cannot read the channel secrets (DENAR_MASTER_KEY changed?): set them again',
      'nie można odczytać sekretów kanału (zmieniony DENAR_MASTER_KEY?) — ustaw je ponownie',
    ],
    ['SMTP is not configured on the server', 'serwer nie ma skonfigurowanego SMTP'],
    ['connection timed out', 'przekroczony czas połączenia'],
    ['connection failed: dial tcp: connection refused', 'błąd połączenia: dial tcp: connection refused'],
    ['connection failed', 'błąd połączenia'],
    ['invalid Bot API URL', 'niepoprawny adres Bot API'],
    ['Telegram: response without ok', 'Telegram: odpowiedź bez ok'],
    ['invalid Matrix homeserver URL', 'niepoprawny adres serwera Matrix'],
    ['invalid recipient address', 'niepoprawny adres odbiorcy'],
    ['SMTP: connection failed: i/o timeout', 'SMTP: połączenie nieudane: i/o timeout'],
    ['SMTP: server does not support STARTTLS', 'SMTP: serwer nie obsługuje STARTTLS'],
    ['SMTP: server does not support AUTH', 'SMTP: serwer nie obsługuje AUTH'],
    ['SMTP: authentication failed: 535 bad credentials', 'SMTP: logowanie nieudane: 535 bad credentials'],
    ['Telegram: connection failed: EOF', 'Telegram: błąd połączenia: EOF'],
    ['Matrix: connection timed out', 'Matrix: przekroczony czas połączenia'],
  ])('%s', (server, inPolish) => {
    expect(translateDeliveryError(server)).toBe(inPolish)
    setLocale('en')
    expect(translateDeliveryError(server)).toBe(server)
  })

  it('passes server replies, Polish text from an older server and unknown text through', () => {
    for (const verbatim of ['HTTP 500', 'Telegram: Bad Request: chat not found', 'Matrix: HTTP 403 M_FORBIDDEN', 'błąd połączenia: dial tcp']) {
      expect(translateDeliveryError(verbatim)).toBe(verbatim)
    }
    expect(translateDeliveryError(null)).toBe('nieznany błąd')
    setLocale('en')
    expect(translateDeliveryError('HTTP 500')).toBe('HTTP 500')
    expect(translateDeliveryError(null)).toBe('unknown error')
  })

  it('translates the private-network refusal and the locale error as field messages', () => {
    expect(translateFieldMessage('target not allowed (private network): see DENAR_NOTIFY_ALLOW_PRIVATE')).toBe(
      'adres docelowy niedozwolony (sieć prywatna) — zobacz DENAR_NOTIFY_ALLOW_PRIVATE',
    )
    expect(translateFieldMessage('must be pl or en')).toBe('Wybierz polski albo angielski.')
  })

  it('knows the human-only reason in either spelling', () => {
    const bot = new ApiError({ status: 403, code: 'forbidden', type: 'urn:denar:problem:human_only', title: 'Forbidden' })
    expect(bot.reason).toBe('human_only')
    expect(reasonMessage('human_only')).toBe('Tę operację wykonuje tylko człowiek, nie bot.')
    expect(reasonMessage('human-only')).toBe('Tę operację wykonuje tylko człowiek, nie bot.')
  })
})
