/* global window, document */
// Ustawia zapisany motyw przed pierwszym malowaniem, żeby strona nie mignęła
// w innym. Zewnętrzny, blokujący skrypt: CSP nie pozwala na skrypty inline.
// Klucz jest ten sam co w src/lib/storage.ts.
;(function () {
  try {
    var theme = window.localStorage.getItem('denar:theme')
    if (theme === 'dark' || theme === 'light') {
      document.documentElement.setAttribute('data-theme', theme)
    }
  } catch {
    // Pamięć zablokowana: decyduje preferencja systemu.
  }
})()
