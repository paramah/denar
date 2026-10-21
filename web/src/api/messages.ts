/**
 * Komunikaty serwera po polsku. Serwer zwraca błędy walidacji pól
 * (`errors[].message`) i opisy problemów po angielsku; interfejs pokazuje je
 * po polsku. Reguły pokrywają komunikaty z `internal/domain` i
 * `internal/usecase`; nieznany angielski komunikat zamienia się w ogólny,
 * a tekst już polski (np. z testów) przechodzi bez zmian.
 */

type Rule = [RegExp, string | ((...groups: string[]) => string)]

const charactersRange = (n: string) => `${n} ${plural(Number(n), 'znak', 'znaki', 'znaków')}`

function plural(n: number, one: string, few: string, many: string): string {
  if (n === 1) return one
  const tens = n % 100
  const units = n % 10
  return units >= 2 && units <= 4 && (tens < 12 || tens > 14) ? few : many
}

const rules: Rule[] = [
  // Import CSV (etap 008): plik, opcje parsera i błędy wierszy.
  [/^is larger than 5 MiB$/, 'Plik jest większy niż 5 MiB — wyeksportuj krótszy okres.'],
  [/^is empty$/, 'Pusta wartość.'],
  [/^has no transactions$/, 'W pliku nie ma żadnych operacji.'],
  [/^has more than (\d+) rows$/, (_, n) => `Plik ma więcej niż ${n} wierszy — podziel go na części.`],
  [/^must be multipart\/form-data/, 'Nieprawidłowe żądanie wysyłki pliku. Odśwież stronę i spróbuj ponownie.'],
  [/^must be a JSON object of CsvImportOptions$/, 'Nieprawidłowe opcje importu. Odśwież stronę i spróbuj ponownie.'],
  [/^is not a known preset$/, 'Nieznany preset banku.'],
  [/^must be ;, , , \| or tab$/, 'Wybierz separator: średnik, przecinek, kreskę pionową albo tabulator.'],
  [/^must be utf-8 or windows-1250$/, 'Wybierz kodowanie UTF-8 albo Windows-1250.'],
  [/^may hold only YYYY, YY, MM, M, DD, D, HH, mm, ss and separators$/,
    'Format daty: tylko YYYY, YY, MM, M, DD, D, HH, mm, ss i separatory (np. DD.MM.YYYY).'],
  [/^needs a year, a month and a day/, 'Format daty musi mieć rok, miesiąc i dzień (YYYY, MM, DD).'],
  [/^must be , or \.$/, 'Wybierz przecinek albo kropkę.'],
  [/^must be (\d+) to (\d+)$/, (_, a, b) => `Podaj wartość od ${a} do ${b}.`],
  [/^is required, or debit and credit$/, 'Wskaż kolumnę kwoty albo kolumny obciążeń i uznań.'],
  [/^no header row has the date column "(.*)"$/, (_, name) =>
    `Nie znaleziono wiersza nagłówka z kolumną daty (${name}). Wskaż kolumnę daty, zmień separator albo kodowanie.`],
  [/^is not a column of the file, nor are debit and credit$/,
    'Nie ma w pliku kolumny kwoty ani kolumn obciążeń i uznań — wskaż je.'],
  [/^is not a column of the file$/, 'Nie ma takiej kolumny w pliku — wybierz inną.'],
  [/^is "(.*)"; only (.+) rows are imported$/, (_, state, accepted) =>
    `Stan „${state}” — importowane są tylko: ${accepted}.`],
  [/^is not a date: "(.*)"$/, (_, value) => `„${value}” to nie data w tym formacie.`],
  [/^is empty: neither debit nor credit is given$/, 'Brak kwoty: puste obciążenie i uznanie.'],
  [/^is not a number: "(.*)"$/, (_, value) => `„${value}” to nie liczba.`],
  [/^must not be zero$/, 'Kwota nie może być zerem.'],
  [/^is not an amount$/, 'To nie jest kwota.'],
  [/^is (\w{3}), not the wallet's (\w{3})$/, (_, c, w) => `Waluta ${c} — portfel jest w ${w}.`],
  [/^is not valid CSV$/, 'Wiersz nie jest poprawnym CSV (np. niedomknięty cudzysłów).'],
  [/^is not a valid transaction$/, 'Z tego wiersza nie powstanie poprawna transakcja.'],
  [/^is required: no exchange rate from (\w+) to (\w+) is known for the due date$/, (_, from, to) =>
    `Podaj kwotę — nie znamy kursu ${from} → ${to} z dnia płatności.`],
  [/^is required: the subscription (?:names no|has no) wallet$/, 'Wybierz portfel — subskrypcja nie ma portfela płatności.'],
  [/^is required between wallets of different currencies$/, 'Podaj kwotę — portfele mają różne waluty.'],
  [/^is required for a target_by_date goal$/, 'Podaj datę celu.'],
  [/^is required$/, 'To pole jest wymagane.'],
  [/^must not be (?:empty|blank)$/, 'To pole nie może być puste.'],
  [/^must be 1 to (\d+) characters$/, (_, n) => `Wpisz od 1 do ${charactersRange(n)}.`],
  [/^must be at most (\d+) characters$/, (_, n) => `Najwyżej ${charactersRange(n)}.`],
  [/^must be at least (\d+) characters$/, (_, n) => `Co najmniej ${charactersRange(n)}.`],
  [/^must each be 1 to (\d+) characters$/, (_, n) => `Każda pozycja: od 1 do ${charactersRange(n)}.`],
  [/^must each be at most (\d+) characters$/, (_, n) => `Każda pozycja: najwyżej ${charactersRange(n)}.`],
  [/^must not contain control characters$/, 'Usuń znaki sterujące (np. tabulatory).'],
  [/^must be between (-?\d+) and (-?\d+)$/, (_, a, b) => `Podaj wartość od ${a} do ${b}.`],
  [/^must be greater than zero and at most 10\^13$/, 'Kwota musi być większa od zera i nie za duża.'],
  [/^must be greater than zero, without a sign$/, 'Podaj kwotę większą od zera, bez znaku.'],
  [/^must be greater than zero: what was paid$/, 'Podaj zapłaconą kwotę, większą od zera.'],
  [/^must be greater than zero$/, 'Wartość musi być większa od zera.'],
  [/^must be a decimal amount with at most (\d+) decimal places$/, (_, n) =>
    n === '0' ? 'Podaj kwotę bez części ułamkowej.' : `Podaj kwotę z najwyżej ${n} ${plural(Number(n), 'miejscem', 'miejscami', 'miejscami')} po przecinku.`],
  [/^must be a positive decimal number with at most 12 decimal places$/, 'Podaj liczbę dodatnią (najwyżej 12 miejsc po przecinku).'],
  [/^is out of range in the base currency$/, 'Kwota w walucie bazowej jest poza zakresem.'],
  [/^is out of range$/, 'Wartość jest poza zakresem.'],
  [/^makes the assignment too large$/, 'Przydział byłby za duży.'],
  [/^must be (\w{3}), the currency of the wallet$/, (_, c) => `Waluta musi być ${c} — waluta portfela.`],
  [/^must be (\w{3}), the base currency of the household$/, (_, c) => `Waluta musi być ${c} — waluta bazowa gospodarstwa.`],
  [/^has an unsupported currency$|^is not supported$|^must be a supported ISO 4217 currency code$/, 'Nieobsługiwana waluta.'],
  [/^is the base currency, whose rate is always 1$/, 'To waluta bazowa — jej kurs to zawsze 1.'],
  [/^must be a date written YYYY-MM-DD$|^must be a date$/, 'Podaj poprawną datę.'],
  [/^must be a month written as YYYY-MM/, 'Podaj miesiąc w postaci RRRR-MM (2000–2100).'],
  [/^must not be before from$/, 'Data końcowa nie może być wcześniejsza niż początkowa.'],
  [/^the period may be at most 366 days long$/, 'Okres może mieć najwyżej 366 dni.'],
  [/^must be in the future$/, 'Wybierz datę w przyszłości.'],
  [/^must be an email address$/, 'Podaj poprawny adres e-mail.'],
  [/^must be an IANA time zone/, 'Wybierz strefę czasową z listy.'],
  [/^must be an http or https URL/, 'Podaj adres zaczynający się od http:// lub https:// (najwyżej 500 znaków).'],
  [/^must be one of the known vendors$/, 'Wybierz dostawcę z listy.'],
  [/^must name at least one scope$/, 'Wybierz co najmniej jedno uprawnienie.'],
  [/^contains an unknown scope/, 'Lista zawiera nieznane uprawnienie.'],
  [/^contains a wallet that is not one of yours/, 'Lista zawiera portfel, którego nie widzisz.'],
  [/^contains someone who is not a member/, 'Lista zawiera osobę spoza gospodarstwa.'],
  [/^must hold at most (\d+) (?:wallets|values)$/, (_, n) => `Najwyżej ${n} pozycji.`],
  [/^may (?:name|have) at most (\d+) (\w+)$/, (_, n) => `Najwyżej ${n} pozycji.`],
  [/^must hold days between 0 and 60$/, 'Przypomnienia: od 0 do 60 dni przed płatnością.'],
  [/^must differ from from_wallet_id$/, 'Wybierz inny portfel niż źródłowy.'],
  [/^must differ from from_envelope_id$/, 'Wybierz inną kopertę niż źródłowa.'],
  [/^must equal amount between wallets of one currency$/, 'Przy tej samej walucie kwoty muszą być równe.'],
  [/^must add up to the amount of the transaction$/, 'Suma podziału musi być równa kwocie transakcji.'],
  [/^only an expense can be split$/, 'Na koperty można dzielić tylko wydatki.'],
  [/^a split transaction takes its envelopes from its lines$/, 'Przy podziale koperty wybiera się w liniach podziału.'],
  [/^a transfer has no payee$|^a transfer is made with the transfer operation$/, 'Przelew zapisuje się osobno („Przelew”).'],
  [/^only a transfer between an on-budget and an off-budget wallet has an envelope$/,
    'Kopertę ma tylko przelew między portfelem budżetowym a portfelem poza budżetem.'],
  [/^cannot change on a transfer leg/, 'Tego nie zmienisz w przelewie — usuń go i zapisz ponownie.'],
  [/^give payee_id or payee_name, not both$/, 'Podaj kontrahenta tylko raz.'],
  [/^is not a payee of this household$/, 'Nie ma takiego kontrahenta.'],
  [/^is not an envelope (?:id|of this household)$/, 'Nie ma takiej koperty.'],
  [/^is not an envelope group of this household$/, 'Nie ma takiej grupy kopert.'],
  [/^is not a transaction you can see$/, 'Nie ma takiej transakcji.'],
  [/^must be an expense, not a transfer or an income$/, 'Powiązać można tylko wydatek.'],
  [/^is only for a target_by_date goal$/, 'Data dotyczy tylko celu „kwota do daty”.'],
  [/^is set by cancelling the subscription$/, 'Datę końca ustawia anulowanie subskrypcji.'],
  [/^is (?:not a valid cursor|not a cursor) of this query$/, 'Lista się zmieniła — odśwież stronę.'],
  [/^the household already has the (\d+) bots it may have$/, (_, n) => `Gospodarstwo ma już ${n} botów — więcej się nie da.`],
  [/^a viewer cannot create bots$/, 'Osoba z rolą „Podgląd” nie tworzy botów.'],
  [/^is ambiguous/, 'Nazwa pasuje do kilku pozycji — podaj pełną nazwę.'],
  [/^has the wrong (?:type|format)$|^must be valid JSON$|^must be given once$/, 'Nieprawidłowy format wartości.'],
  // Powiadomienia (etap 005): kanały i preferencje.
  [/^must have 1 to (\d+) characters$/, (_, n) => `Wpisz od 1 do ${charactersRange(n)}.`],
  [/^must be an e-mail address$/, 'Podaj poprawny adres e-mail.'],
  [/^must be an http\(s\) URL$/, 'Podaj adres zaczynający się od http:// lub https://.'],
  [/^must have 1 to 64 letters, digits, - or _$/, 'Temat: od 1 do 64 liter, cyfr, „-” lub „_”, bez spacji.'],
  [/^must be a chat id such as/, 'Podaj chat_id: liczbę (np. 123456789, dla grup z minusem) albo @nazwa_kanału.'],
  [/^must be a room id such as/, 'Podaj identyfikator pokoju zaczynający się od „!”, np. !abc:example.org.'],
  [/^is not a setting of an? \w+ channel$/, 'Tego ustawienia nie ma w tym typie kanału.'],
  [/^is not a secret of an? \w+ channel$/, 'Tego sekretu nie ma w tym typie kanału.'],
  [/^is too long$/, 'Za długa wartość.'],
  [/^is not one of your channels/, 'Ten kanał nie istnieje — odśwież stronę.'],
  [/^is not a known event type/, 'Nieznany typ zdarzenia — odśwież stronę.'],
  [/^is given twice/, 'Typ zdarzenia podany dwa razy.'],
  [/^has too many channels$/, 'Za dużo kanałów dla jednego zdarzenia.'],
  [/^is too large$/, 'Za dużo danych.'],
  [/^must be (?:one of [a-z_]+(?:, [a-z_]+)+|[a-z_]+(?:, [a-z_]+)* or [a-z_]+)$/, 'Wybierz wartość z listy.'],
]

const english = /^(?:must|is|has|may|contains|cannot|give|a|an|the|only|makes|needs|no)\b/

/** Komunikat błędu pola po polsku. */
export function translateFieldMessage(message: string): string {
  for (const [pattern, out] of rules) {
    const match = pattern.exec(message)
    if (match) {
      return typeof out === 'string' ? out : out(...match)
    }
  }
  return english.test(message) ? 'Nieprawidłowa wartość.' : message
}

/** Opisy konfliktów (409) i odmów (403) po ostatnim segmencie `type`. */
export const reasonMessages: Record<string, string> = {
  'duplicate-name': 'Ta nazwa jest już zajęta. Wybierz inną.',
  'duplicate-email': 'Ten adres e-mail jest już używany.',
  'duplicate-external-id': 'Ta transakcja jest już zapisana.',
  'duplicate-charge': 'Ta płatność już istnieje.',
  'last-owner': 'Gospodarstwo musi mieć co najmniej jednego właściciela.',
  'already-member': 'Ta osoba już należy do gospodarstwa.',
  'invitation-used': 'To zaproszenie zostało już wykorzystane.',
  'invitation-expired': 'To zaproszenie wygasło.',
  'wallet-archived': 'Portfel jest zarchiwizowany — przywróć go, żeby coś w nim zmienić.',
  'group-not-empty': 'W grupie są koperty — przenieś je albo ukryj.',
  'envelope-in-use': 'Koperta ma transakcje — możesz ją ukryć zamiast usuwać.',
  'charge-state': 'Ta płatność została już rozliczona albo pominięta. Odśwież stronę.',
  'transaction-linked': 'Ta transakcja opłaca już inną płatność subskrypcji.',
  'secrets-unavailable':
    'Serwer nie ma klucza DENAR_MASTER_KEY, więc nie zapisze tokenów ani sekretów. Poproś administratora o jego ustawienie.',
  'smtp-not-configured': 'Serwer nie ma skonfigurowanej poczty (SMTP) — e-maili nie da się wysłać.',
  'too-many-channels': 'Masz już 20 kanałów — usuń któryś, żeby dodać nowy.',
  'human-only': 'Tę operację wykonuje tylko człowiek, nie bot.',
  'import-reverted': 'Ten import został już cofnięty.',
  'provider-unavailable': 'Dostawca tego połączenia nie jest dostępny na tym serwerze.',
  conflict: 'Dane się zmieniły w międzyczasie. Odśwież stronę i spróbuj ponownie.',
  validation: 'Popraw dane i spróbuj ponownie.',
  'bad-request': 'Nieprawidłowe żądanie. Odśwież stronę i spróbuj ponownie.',
}
