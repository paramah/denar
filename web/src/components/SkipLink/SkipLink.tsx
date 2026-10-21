import styles from './SkipLink.module.css'
import { tk } from '../../i18n/translate.ts'

/** Pierwszy element strony: pomija nawigację klawiaturą. */
export function SkipLink() {
  return (
    <a className={styles.skip} href="#main">
      {tk('common:components.skipToContent')}
    </a>
  )
}
