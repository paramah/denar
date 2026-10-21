import styles from './Skeleton.module.css'

type Props = {
  /** Liczba wierszy. */
  lines?: number
  /** Opis dla czytników ekranu. */
  label?: string
}

/** Zastępczy kształt ładowanej treści; kontener ogłasza zajętość. */
export function Skeleton({ lines = 3, label = 'Wczytywanie…' }: Props) {
  return (
    <div className={styles.skeleton} aria-busy="true" role="status">
      <span className="visually-hidden">{label}</span>
      {Array.from({ length: lines }, (_, i) => (
        <span key={i} className={styles.line} aria-hidden="true" data-short={i === lines - 1 && lines > 1} />
      ))}
    </div>
  )
}
