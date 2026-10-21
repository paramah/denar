import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router'
import { useLogout, useSessionUser } from '../api/hooks/session.ts'
import type { Household } from '../api/types.ts'
import { IconButton } from '../components/Button/Button.tsx'
import { CoinMark, Icon } from '../components/Icon/Icon.tsx'
import { SkipLink } from '../components/SkipLink/SkipLink.tsx'
import { roleLabels } from '../lib/labels.ts'
import { storageKeys, writeStored } from '../lib/storage.ts'
import { useDisclosure } from '../lib/useDisclosure.ts'
import styles from './AppShell.module.css'
import { useHousehold } from './household.tsx'
import { householdPath, navItems, switchHouseholdPath } from './nav.ts'
import { useTheme, type ThemeChoice } from './ThemeProvider.tsx'

/**
 * Układ gospodarstwa: na komputerze stały pasek po lewej, na telefonie
 * górna belka z przyciskiem menu i nawigacją w wysuwanym panelu.
 */
export function AppShell({ children }: { children: ReactNode }) {
  const { household } = useHousehold()
  const location = useLocation()
  // Panel zamyka się sam po przejściu na inną stronę: jest otwarty tylko na ścieżce, na której go otwarto.
  const [drawerAt, setDrawerAt] = useState<string | null>(null)
  const drawerOpen = drawerAt === location.pathname
  const setDrawerOpen = (open: boolean) => setDrawerAt(open ? location.pathname : null)

  return (
    <div className={styles.shell}>
      <SkipLink />
      <aside className={styles.rail} aria-label="Menu aplikacji">
        <RailContent />
      </aside>
      <header className={styles.topbar}>
        <IconButton
          icon="menu"
          label="Otwórz menu"
          className={styles.menuButton}
          aria-expanded={drawerOpen}
          onClick={() => setDrawerOpen(true)}
        />
        <Link to={householdPath(household.id)} className={styles.topBrand}>
          <CoinMark size={28} />
          <span className={styles.topName}>{household.name}</span>
        </Link>
      </header>
      {drawerOpen && <Drawer onClose={() => setDrawerOpen(false)} />}
      <main id="main" className={styles.main} tabIndex={-1}>
        <div className={styles.content}>{children}</div>
      </main>
    </div>
  )
}

function Drawer({ onClose }: { onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const dialog = ref.current!
    const opener = document.activeElement
    dialog.showModal()
    const onCancel = (event: Event) => {
      event.preventDefault()
      onClose()
    }
    const onClick = (event: MouseEvent) => {
      if (event.target === dialog) {
        onClose()
      }
    }
    dialog.addEventListener('cancel', onCancel)
    dialog.addEventListener('click', onClick)
    return () => {
      dialog.removeEventListener('cancel', onCancel)
      dialog.removeEventListener('click', onClick)
      dialog.close()
      if (opener instanceof HTMLElement && opener.isConnected) {
        opener.focus()
      }
    }
  }, [onClose])
  return (
    <dialog ref={ref} className={styles.drawer} aria-label="Menu aplikacji">
      <div className={styles.drawerInner}>
        <div className={styles.drawerClose}>
          <IconButton icon="close" label="Zamknij menu" onClick={onClose} className={styles.railButton} />
        </div>
        <RailContent />
      </div>
    </dialog>
  )
}

function RailContent() {
  const { household, role } = useHousehold()
  return (
    <div className={styles.railContent}>
      <Link to={householdPath(household.id)} className={styles.brand}>
        <CoinMark size={40} />
        <span className={styles.wordmark}>Denar</span>
      </Link>
      <HouseholdSwitcher current={household} roleLabel={roleLabels[role]} />
      <nav aria-label="Główna" className={styles.nav}>
        <ul>
          {navItems.map((item) => (
            <li key={item.path}>
              <NavLink
                to={householdPath(household.id, item.path)}
                end={item.path === ''}
                className={({ isActive }) => (isActive ? `${styles.navLink} ${styles.active}` : styles.navLink)}
              >
                <Icon name={item.icon} />
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <div className={styles.railFooter}>
        <UserMenu />
      </div>
    </div>
  )
}

function HouseholdSwitcher({ current, roleLabel }: { current: Household; roleLabel: string }) {
  const { households } = useSessionUser()
  const location = useLocation()
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const menu = useDisclosure(containerRef, buttonRef)
  const menuId = useId()
  const others = households.filter((h) => h.id !== current.id)

  return (
    <div className={styles.switcher} ref={containerRef}>
      <button
        ref={buttonRef}
        type="button"
        className={styles.switcherButton}
        aria-expanded={menu.open}
        aria-controls={menuId}
        onClick={menu.toggle}
      >
        <span className={styles.switcherText}>
          <span className={styles.switcherLabel}>Gospodarstwo</span>
          <span className={styles.householdName}>{current.name}</span>
          <span className={styles.switcherMeta}>
            {roleLabel}, {current.base_currency}
          </span>
        </span>
        <Icon name="chevronDown" size={18} />
        <span className="visually-hidden">— zmień gospodarstwo</span>
      </button>
      {menu.open && (
        <div id={menuId} className={styles.popover}>
          {others.length > 0 && (
            <ul aria-label="Twoje gospodarstwa" className={styles.popoverList}>
              {others.map((h) => (
                <li key={h.id}>
                  <Link
                    to={switchHouseholdPath(location.pathname, h.id)}
                    className={styles.popoverItem}
                    onClick={() => {
                      writeStored(storageKeys.household, h.id)
                      menu.close()
                    }}
                  >
                    <span>{h.name}</span>
                    <span className={styles.popoverMeta}>{h.base_currency}</span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
          <Link to="/households/new" className={styles.popoverItem} onClick={() => menu.close()}>
            <Icon name="plus" size={18} />
            <span>Nowe gospodarstwo</span>
          </Link>
        </div>
      )}
    </div>
  )
}

const themeOptions: { value: ThemeChoice; label: string; icon: 'monitor' | 'sun' | 'moon' }[] = [
  { value: 'system', label: 'Systemowy', icon: 'monitor' },
  { value: 'light', label: 'Jasny', icon: 'sun' },
  { value: 'dark', label: 'Ciemny', icon: 'moon' },
]

export function UserMenu({ placement = 'up' }: { placement?: 'up' | 'down' }) {
  const { user } = useSessionUser()
  const { theme, setTheme } = useTheme()
  const logout = useLogout()
  const navigate = useNavigate()
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const menu = useDisclosure(containerRef, buttonRef)
  const menuId = useId()
  const initials = user.name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('')

  return (
    <div className={styles.userMenu} ref={containerRef}>
      <button
        ref={buttonRef}
        type="button"
        className={styles.userButton}
        aria-expanded={menu.open}
        aria-controls={menuId}
        onClick={menu.toggle}
      >
        <span className={styles.avatar} aria-hidden="true">
          {initials || '?'}
        </span>
        <span className={styles.userName}>{user.name}</span>
        <span className="visually-hidden">— menu konta</span>
      </button>
      {menu.open && (
        <div id={menuId} className={`${styles.popover} ${styles[placement]}`}>
          <div className={styles.userInfo}>
            <p className={styles.userInfoName}>{user.name}</p>
            <p className={styles.popoverMeta}>{user.email}</p>
          </div>
          <fieldset className={styles.themes}>
            <legend>Motyw</legend>
            <div className={styles.themeButtons}>
              {themeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  aria-pressed={theme === option.value}
                  className={styles.themeButton}
                  onClick={() => setTheme(option.value)}
                >
                  <Icon name={option.icon} size={16} />
                  {option.label}
                </button>
              ))}
            </div>
          </fieldset>
          <button
            type="button"
            className={styles.popoverItem}
            onClick={() =>
              logout.mutate(undefined, {
                onSettled: () => {
                  void navigate('/login', { replace: true })
                },
              })
            }
          >
            <Icon name="logout" size={18} />
            <span>Wyloguj się</span>
          </button>
        </div>
      )}
    </div>
  )
}

/** Układ stron bez gospodarstwa (powitanie, nowe gospodarstwo). */
export function PlainLayout({ children }: { children: ReactNode }) {
  return (
    <div className={styles.plain}>
      <SkipLink />
      <header className={styles.plainBar}>
        <Link to="/" className={styles.plainBrand}>
          <CoinMark size={32} />
          <span className={styles.wordmark}>Denar</span>
        </Link>
        <UserMenu placement="down" />
      </header>
      <main id="main" className="plain-main" tabIndex={-1}>
        {children}
      </main>
    </div>
  )
}
