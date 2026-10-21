// Testy idą po polsku (FR-1309): jsdom zgłasza przeglądarkę `en-US`, więc
// zanim zainicjuje się i18n, przeglądarka „mówi” po polsku. Test, który
// sprawdza inny język, ustawia go sam (setLocale albo własne navigator.languages).

Object.defineProperty(window.navigator, 'languages', { value: ['pl-PL', 'pl'], configurable: true })
Object.defineProperty(window.navigator, 'language', { value: 'pl-PL', configurable: true })

export {}
