import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Link, type LinkProps } from 'react-router'
import { Icon, type IconName } from '../Icon/Icon.tsx'
import styles from './Button.module.css'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

type Common = {
  variant?: Variant
  size?: Size
  icon?: IconName
  children?: ReactNode
}

/** Klasy przycisku dla zwykłego <a> (np. link do API, którego router nie obsługuje). */
export function buttonClassName(variant: Variant = 'secondary', size: Size = 'md', extra?: string) {
  return classes(variant, size, extra)
}

function classes(variant: Variant, size: Size, extra?: string) {
  return [styles.button, styles[variant], styles[size], extra].filter(Boolean).join(' ')
}

type ButtonProps = Common &
  ButtonHTMLAttributes<HTMLButtonElement> & {
    /** Trwa operacja: przycisk blokuje się i ogłasza zajętość. */
    busy?: boolean
  }

export function Button({
  variant = 'secondary',
  size = 'md',
  icon,
  busy = false,
  children,
  className,
  disabled,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={classes(variant, size, className)}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      {...rest}
    >
      {busy ? <span className={styles.spinner} aria-hidden="true" /> : icon && <Icon name={icon} size={18} />}
      {children}
    </button>
  )
}

type LinkButtonProps = Common & LinkProps

/** Link wyglądający jak przycisk (nawigacja, nie akcja). */
export function LinkButton({ variant = 'secondary', size = 'md', icon, children, className, ...rest }: LinkButtonProps) {
  return (
    <Link className={classes(variant, size, className)} {...rest}>
      {icon && <Icon name={icon} size={18} />}
      {children}
    </Link>
  )
}

type IconButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> & {
  icon: IconName
  /** Dostępna nazwa; widoczna też jako podpowiedź. */
  label: string
  variant?: Variant
}

export function IconButton({ icon, label, variant = 'ghost', className, type = 'button', ...rest }: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      className={[styles.button, styles[variant], styles.iconOnly, className].filter(Boolean).join(' ')}
      {...rest}
    >
      <Icon name={icon} size={20} />
    </button>
  )
}
