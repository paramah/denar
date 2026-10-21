/**
 * Komunikaty serwera w języku interfejsu. Serwer zwraca błędy walidacji pól
 * (`errors[].message`), opisy problemów i błędy dostarczania powiadomień po
 * angielsku; reguły niżej zamieniają je na klucz `errors:…` z parametrami
 * (i18n/{pl,en}/errors.json). Reguły pokrywają komunikaty z `internal/domain`
 * i `internal/usecase`; nieznany angielski komunikat zamienia się w ogólny, a
 * tekst już polski (np. ze starszego serwera) przechodzi bez zmian.
 */

import { hasKey, tk } from '../i18n/translate.ts'

export type Message = { key: string; params?: Record<string, string | number> }

type Out = string | ((...groups: string[]) => Message)
type Rule = [RegExp, Out]

const count = (key: string) => (_: string, n: string) => ({ key, params: { count: Number(n) } })

const rules: Rule[] = [
  // Import CSV (etap 008): plik, opcje parsera i błędy wierszy.
  [/^is larger than 5 MiB$/, 'fileTooLarge'],
  [/^is empty$/, 'emptyValue'],
  [/^has no transactions$/, 'noTransactions'],
  [/^has more than (\d+) rows$/, (_, n) => ({ key: 'tooManyRows', params: { n: n! } })],
  [/^must be multipart\/form-data/, 'badUpload'],
  [/^must be a JSON object of CsvImportOptions$/, 'badImportOptions'],
  [/^is not a known preset$/, 'unknownPreset'],
  [/^must be ;, , , \| or tab$/, 'delimiter'],
  [/^must be utf-8 or windows-1250$/, 'encoding'],
  [/^may hold only YYYY, YY, MM, M, DD, D, HH, mm, ss and separators$/, 'dateFormatTokens'],
  [/^needs a year, a month and a day/, 'dateFormatParts'],
  [/^must be , or \.$/, 'decimalSeparator'],
  [/^must be (\d+) to (\d+)$/, (_, a, b) => ({ key: 'range', params: { min: a!, max: b! } })],
  [/^is required, or debit and credit$/, 'amountColumns'],
  [/^no header row has the date column "(.*)"$/, (_, name) => ({ key: 'noHeaderRow', params: { name: name! } })],
  [/^is not a column of the file, nor are debit and credit$/, 'noAmountColumns'],
  [/^is not a column of the file$/, 'noSuchColumn'],
  [/^is "(.*)"; only (.+) rows are imported$/, (_, state, accepted) =>
    ({ key: 'rowState', params: { state: state!, accepted: accepted! } })],
  [/^is not a date: "(.*)"$/, (_, value) => ({ key: 'notADate', params: { value: value! } })],
  [/^is empty: neither debit nor credit is given$/, 'noAmount'],
  [/^is not a number: "(.*)"$/, (_, value) => ({ key: 'notANumber', params: { value: value! } })],
  [/^must not be zero$/, 'zeroAmount'],
  [/^is not an amount$/, 'notAnAmount'],
  [/^is (\w{3}), not the wallet's (\w{3})$/, (_, c, w) => ({ key: 'rowCurrency', params: { currency: c!, wallet: w! } })],
  [/^is not valid CSV$/, 'invalidCsv'],
  [/^is not a valid transaction$/, 'invalidRow'],
  [/^is required: no exchange rate from (\w+) to (\w+) is known for the due date$/, (_, from, to) =>
    ({ key: 'noRateForDueDate', params: { from: from!, to: to! } })],
  [/^is required: the subscription (?:names no|has no) wallet$/, 'subscriptionWallet'],
  [/^is required between wallets of different currencies$/, 'transferAmount'],
  [/^is required for a target_by_date goal$/, 'goalDate'],
  [/^is required$/, 'required'],
  [/^must not be (?:empty|blank)$/, 'notEmpty'],
  [/^must be 1 to (\d+) characters$/, count('lengthRange')],
  [/^must be at most (\d+) characters$/, count('lengthMax')],
  [/^must be at least (\d+) characters$/, count('lengthMin')],
  [/^must each be 1 to (\d+) characters$/, count('eachLengthRange')],
  [/^must each be at most (\d+) characters$/, count('eachLengthMax')],
  [/^must not contain control characters$/, 'controlCharacters'],
  [/^must be between (-?\d+) and (-?\d+)$/, (_, a, b) => ({ key: 'range', params: { min: a!, max: b! } })],
  [/^must be greater than zero and at most 10\^13$/, 'amountPositiveLimited'],
  [/^must be greater than zero, without a sign$/, 'amountPositiveUnsigned'],
  [/^must be greater than zero: what was paid$/, 'amountPaid'],
  [/^must be greater than zero$/, 'positive'],
  [/^must be a decimal amount with at most (\d+) decimal places$/, (_, n) =>
    n === '0' ? { key: 'noDecimals' } : { key: 'decimalPlaces', params: { count: Number(n) } }],
  [/^must be a positive decimal number with at most 12 decimal places$/, 'rate'],
  [/^is out of range in the base currency$/, 'baseOutOfRange'],
  [/^is out of range$/, 'outOfRange'],
  [/^makes the assignment too large$/, 'assignmentTooLarge'],
  [/^must be (\w{3}), the currency of the wallet$/, (_, c) => ({ key: 'walletCurrency', params: { currency: c! } })],
  [/^must be (\w{3}), the base currency of the household$/, (_, c) => ({ key: 'baseCurrency', params: { currency: c! } })],
  [/^has an unsupported currency$|^is not supported$|^must be a supported ISO 4217 currency code$/, 'unsupportedCurrency'],
  [/^is the base currency, whose rate is always 1$/, 'baseCurrencyRate'],
  [/^must be a date written YYYY-MM-DD$|^must be a date$/, 'date'],
  [/^must be a month written as YYYY-MM/, 'month'],
  [/^must not be before from$/, 'endBeforeStart'],
  [/^the period may be at most 366 days long$/, 'periodTooLong'],
  [/^must be in the future$/, 'future'],
  [/^must be an email address$/, 'email'],
  [/^must be an IANA time zone/, 'timeZone'],
  [/^must be an http or https URL/, 'urlLimited'],
  [/^must be one of the known vendors$/, 'vendor'],
  [/^must name at least one scope$/, 'scopeRequired'],
  [/^contains an unknown scope/, 'unknownScope'],
  [/^contains a wallet that is not one of yours/, 'foreignWallet'],
  [/^contains someone who is not a member/, 'nonMember'],
  [/^must hold at most (\d+) (?:wallets|values)$/, (_, n) => ({ key: 'maxItems', params: { n: n! } })],
  [/^may (?:name|have) at most (\d+) (\w+)$/, (_, n) => ({ key: 'maxItems', params: { n: n! } })],
  [/^must hold days between 0 and 60$/, 'reminderDays'],
  [/^must differ from from_wallet_id$/, 'sameWallet'],
  [/^must differ from from_envelope_id$/, 'sameEnvelope'],
  [/^must equal amount between wallets of one currency$/, 'sameCurrencyAmounts'],
  [/^must add up to the amount of the transaction$/, 'splitSum'],
  [/^only an expense can be split$/, 'splitExpenseOnly'],
  [/^a split transaction takes its envelopes from its lines$/, 'splitEnvelopes'],
  [/^a transfer has no payee$|^a transfer is made with the transfer operation$/, 'transferSeparate'],
  [/^only a transfer between an on-budget and an off-budget wallet has an envelope$/, 'transferEnvelope'],
  [/^cannot change on a transfer leg/, 'transferLeg'],
  [/^give payee_id or payee_name, not both$/, 'payeeTwice'],
  [/^is not a payee of this household$/, 'unknownPayee'],
  [/^is not an envelope (?:id|of this household)$/, 'unknownEnvelope'],
  [/^is not an envelope group of this household$/, 'unknownEnvelopeGroup'],
  [/^is not a transaction you can see$/, 'unknownTransaction'],
  [/^must be an expense, not a transfer or an income$/, 'expenseOnly'],
  [/^is only for a target_by_date goal$/, 'targetDateOnly'],
  [/^is set by cancelling the subscription$/, 'endsByCancel'],
  [/^is (?:not a valid cursor|not a cursor) of this query$/, 'staleCursor'],
  [/^the household already has the (\d+) bots it may have$/, (_, n) => ({ key: 'botLimit', params: { n: n! } })],
  [/^a viewer cannot create bots$/, 'viewerBots'],
  [/^is ambiguous/, 'ambiguous'],
  [/^has the wrong (?:type|format)$|^must be valid JSON$|^must be given once$/, 'format'],
  // Powiadomienia (etap 005): kanały i preferencje.
  [/^must have 1 to (\d+) characters$/, count('lengthRange')],
  [/^must be an e-mail address$/, 'email'],
  [/^must be an http\(s\) URL$/, 'url'],
  [/^must have 1 to 64 letters, digits, - or _$/, 'topic'],
  [/^must be a chat id such as/, 'chatId'],
  [/^must be a room id such as/, 'roomId'],
  [/^is not a setting of an? \w+ channel$/, 'channelSetting'],
  [/^is not a secret of an? \w+ channel$/, 'channelSecret'],
  [/^is too long$/, 'tooLong'],
  [/^is not one of your channels/, 'unknownChannel'],
  [/^is not a known event type/, 'unknownEventType'],
  [/^is given twice/, 'eventTypeTwice'],
  [/^has too many channels$/, 'tooManyChannels'],
  [/^is too large$/, 'tooLarge'],
  // Profil (013): PATCH /me.
  [/^must be pl or en$/, 'locale'],
  [/^must be (?:one of [a-z_]+(?:, [a-z_]+)+|[a-z_]+(?:, [a-z_]+)* or [a-z_]+)$/, 'oneOf'],
]

const english = /^(?:must|is|has|may|contains|cannot|give|a|an|the|only|makes|needs|no)\b/

function match(rulesList: Rule[], message: string, prefix: string): Message | null {
  for (const [pattern, out] of rulesList) {
    const found = pattern.exec(message)
    if (found) {
      const msg = typeof out === 'string' ? { key: out } : out(...found)
      return { ...msg, key: `${prefix}${msg.key}` }
    }
  }
  return null
}

/** Klucz i parametry dla komunikatu pola; null — komunikat nie po angielsku (pokazujemy go bez zmian). */
export function fieldMessage(message: string): Message | null {
  if (deliveryRules[0]![0].test(message)) {
    return { key: 'errors:delivery.targetForbidden' }
  }
  return match(rules, message, 'errors:field.') ?? (english.test(message) ? { key: 'errors:field.invalid' } : null)
}

/** Komunikat błędu pola w języku interfejsu. */
export function translateFieldMessage(message: string): string {
  const msg = fieldMessage(message)
  return msg ? tk(msg.key, msg.params) : message
}

/** Opis konfliktu (409) lub odmowy (403) po ostatnim segmencie `type`; undefined dla nieznanego. */
export function reasonMessage(reason: string): string | undefined {
  // Serwer pisze przyczyny z myślnikiem (`human-only`); podkreślnik traktujemy tak samo.
  const key = `errors:reason.${reason.replaceAll('_', '-')}`
  return reason && hasKey(key) ? tk(key) : undefined
}

// Błędy dostarczania powiadomień (`last_error` kanału, wynik testu kanału,
// `email_error` zaproszenia). Serwer zapisuje je po angielsku (013, lista
// uzgodniona z backendem); po polsku pokazujemy dawne brzmienie. Polskie
// teksty starszego serwera (i zapisane wcześniej w bazie) przechodzą bez
// zmian, bo żadna reguła ich nie łapie; tak samo odpowiedzi serwerów
// (`HTTP 500`, `Telegram: <opis>`, `Matrix: HTTP 4xx M_*`, odpowiedzi SMTP).
const deliveryRules: Rule[] = [
  [/^target not allowed \(private network\): see DENAR_NOTIFY_ALLOW_PRIVATE$/, 'targetForbidden'],
  [/^channel is disabled$/, 'channelDisabled'],
  [/^DENAR_MASTER_KEY is not set: cannot read the channel secrets$/, 'noMasterKey'],
  [/^cannot read the channel secrets \(DENAR_MASTER_KEY changed\?\): set them again$/, 'secretsUnreadable'],
  [/^SMTP is not configured on the server$/, 'smtpNotConfigured'],
  [/^connection timed out$/, 'timeout'],
  [/^connection failed: (.*)$/, (_, cause) => ({ key: 'connectionFailedWith', params: { cause: cause! } })],
  [/^connection failed$/, 'connectionFailed'],
  [/^invalid Bot API URL$/, 'telegramBadUrl'],
  [/^Telegram: response without ok$/, 'telegramNotOk'],
  [/^invalid Matrix homeserver URL$/, 'matrixBadUrl'],
  [/^invalid recipient address$/, 'invalidRecipient'],
  [/^SMTP: connection failed: (.*)$/, (_, cause) => ({ key: 'smtpConnect', params: { cause: cause! } })],
  [/^SMTP: server does not support STARTTLS$/, 'smtpNoStarttls'],
  [/^SMTP: server does not support AUTH$/, 'smtpNoAuth'],
  [/^SMTP: authentication failed: (.*)$/, (_, cause) => ({ key: 'smtpLogin', params: { cause: cause! } })],
]

/** Błąd dostarczania w języku interfejsu; nieznany tekst (np. opis z Telegrama) bez zmian. */
export function translateDeliveryError(message: string | null | undefined): string {
  if (!message) {
    return tk('errors:delivery.unknown')
  }
  const msg = match(deliveryRules, message, 'errors:delivery.')
  if (msg) {
    return tk(msg.key, msg.params)
  }
  // Telegram i Matrix przekazują błędy połączenia z prefiksem („Telegram: connection failed: …”).
  const prefixed = /^(Telegram|Matrix): (.+)$/.exec(message)
  if (prefixed) {
    const inner = match(deliveryRules, prefixed[2]!, 'errors:delivery.')
    if (inner) {
      return tk('errors:delivery.prefixed', { prefix: prefixed[1]!, message: tk(inner.key, inner.params) })
    }
  }
  return message
}
