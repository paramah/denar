import styles from './SkipLink.module.css'

/** Pierwszy element strony: pomija nawigację klawiaturą. */
export function SkipLink() {
  return (
    <a className={styles.skip} href="#main">
      Przejdź do treści
    </a>
  )
}
