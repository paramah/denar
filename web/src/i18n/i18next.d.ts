import 'i18next'
import type { pl } from './resources.ts'

// Klucze t() są sprawdzane względem polskich plików (wzorzec); angielskie
// muszą mieć te same klucze — pilnuje tego test i18n.test.ts.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'common'
    resources: typeof pl
  }
}
