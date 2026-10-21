import { describe, expect, it } from 'vitest'
import { ApiError, errorMessage, fieldErrors } from './errors.ts'
import { translateFieldMessage } from './messages.ts'

describe('translateFieldMessage', () => {
  it.each([
    ['is required', 'To pole jest wymagane.'],
    ['must be 1 to 100 characters', 'Wpisz od 1 do 100 znaków.'],
    ['must be at least 12 characters', 'Co najmniej 12 znaków.'],
    ['must be between 1 and 28', 'Podaj wartość od 1 do 28.'],
    ['must be a decimal amount with at most 2 decimal places', 'Podaj kwotę z najwyżej 2 miejscami po przecinku.'],
    ['must be a decimal amount with at most 0 decimal places', 'Podaj kwotę bez części ułamkowej.'],
    ['must be USD, the currency of the wallet', 'Waluta musi być USD — waluta portfela.'],
    ['must be one of cash, checking, card, savings, investment, loan', 'Wybierz wartość z listy.'],
    ['must be owner, admin, member or viewer', 'Wybierz wartość z listy.'],
    ['must be in the future', 'Wybierz datę w przyszłości.'],
    ['is required: no exchange rate from USD to EUR is known for the due date', 'Podaj kwotę — nie znamy kursu USD → EUR z dnia płatności.'],
    ['must add up to the amount of the transaction', 'Suma podziału musi być równa kwocie transakcji.'],
  ])('%s', (message, want) => {
    expect(translateFieldMessage(message)).toBe(want)
  })

  it('hides an unknown English message and keeps a Polish one', () => {
    expect(translateFieldMessage('must be something new')).toBe('Nieprawidłowa wartość.')
    expect(translateFieldMessage('za długa')).toBe('za długa')
  })
})

describe('errorMessage', () => {
  it('never shows the English detail of a problem', () => {
    const conflict = new ApiError({ status: 409, code: 'conflict', type: 'urn:denar:problem:conflict', title: 'Conflict', detail: 'The request conflicts with the current state.' })
    expect(errorMessage(conflict)).toBe('Dane się zmieniły w międzyczasie. Odśwież stronę i spróbuj ponownie.')
    const archived = new ApiError({ status: 409, code: 'conflict', type: 'urn:denar:problem:wallet-archived', title: 'Conflict' })
    expect(errorMessage(archived)).toMatch(/zarchiwizowany/)
  })

  it('uses the first field error of a 422 and translates field errors', () => {
    const invalid = new ApiError({
      status: 422,
      code: 'validation',
      type: 'urn:denar:problem:validation',
      title: 'Validation failed',
      errors: [{ field: 'amount.amount', message: 'makes the assignment too large' }],
    })
    expect(errorMessage(invalid)).toBe('Przydział byłby za duży.')
    expect(fieldErrors(invalid)).toEqual({ 'amount.amount': 'Przydział byłby za duży.' })
  })
})
