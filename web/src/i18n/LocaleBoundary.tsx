import { Fragment, useEffect, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { takePendingFocus } from './locale.ts'

/**
 * Po zmianie języka montuje drzewo od nowa (klucz = język). Dzięki temu nowy
 * tekst dostają też miejsca bez useTranslation — etykiety z lib/labels.ts,
 * formattery kwot i dat, komunikaty z api/messages.ts — bez przeładowania
 * strony. Dane zostają w kliencie zapytań (nad granicą), adres w routerze;
 * znika tylko stan lokalny komponentów (np. niewysłany formularz). Fokus
 * wraca na element o tym samym `id`, z którego zmieniono język.
 */
export function LocaleBoundary({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation()
  const language = i18n.resolvedLanguage ?? i18n.language
  useEffect(() => {
    const id = takePendingFocus()
    if (id) {
      document.getElementById(id)?.focus()
    }
  }, [language])
  return <Fragment key={language}>{children}</Fragment>
}
