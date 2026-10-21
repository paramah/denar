import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation, useNavigate } from 'react-router'
import { useLogout, useSessionUser } from '../api/hooks/session.ts'
import type { Household } from '../api/types.ts'
import { IconButton } from '../components/Button/Button.tsx'
import { CoinMark, Icon } from '../components/Icon/Icon.tsx'
import { SkipLink } from '../components/SkipLink/SkipLink.tsx'
import { NotificationBell } from '../features/notifications/NotificationBell.tsx'
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
  const { t } = useTranslation()

  return (
    <div className={styles.shell}>
      <SkipLink />
      <aside className={styles.rail} aria-label={t('shell.appMenu')}>
        <RailContent bell />
      </aside>
      <header className={styles.topbar}>
        <IconButton
          icon="menu"
          label={t('shell.openMenu')}
          className={styles.menuButton}
          aria-expanded={drawerOpen}
          onClick={() => setDrawerOpen(true)}
        />
        <Link to={householdPath(household.id)} className={styles.topBrand}>
          <CoinMark size={28} />
          <span className={styles.topName}>{household.name}</span>
        </Link>
        <div className={styles.topBell}>
          <NotificationBell placement="topbar" />
        </div>
      </header>
      {drawerOpen && <Drawer onClose={() => setDrawerOpen(false)} />}
      <main id="main" className={styles.main} tabIndex={-1}>
        <div className={styles.content}>{children}</div>
      </main>
    </div>
  )
}

function Drawer({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
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
    <dialog ref={ref} className={styles.drawer} aria-label={t('shell.appMenu')}>
      <div className={styles.drawerInner}>
        <div className={styles.drawerClose}>
          <IconButton icon="close" label={t('shell.closeMenu')} onClick={onClose} className={styles.railButton} />
        </div>
        <RailContent />
      </div>
    </dialog>
  )
}

/** Treść paska; dzwonek tylko w stałym pasku (na telefonie jest w górnej belce). */
function RailContent({ bell = false }: { bell?: boolean }) {
  const { household, role } = useHousehold()
  const { t } = useTranslation()
  return (
    <div className={styles.railContent}>
      <div className={styles.brandRow}>
        <Link to={householdPath(household.id)} className={styles.brand}>
          <CoinMark size={40} />
          <span className={styles.wordmark}>{t('appName')}</span>
        </Link>
        {bell && <NotificationBell placement="rail" />}
      </div>
      <HouseholdSwitcher current={household} roleLabel={roleLabels[role]} />
      <nav aria-label={t('nav.label')} className={styles.nav}>
        <ul>
          {navItems.map((item) => (
            <li key={item.path}>
              <NavLink
                to={householdPath(household.id, item.path)}
                end={item.path === ''}
                className={({ isActive }) => (isActive ? `${styles.navLink} ${styles.active}` : styles.navLink)}
              >
                <Icon name={item.icon} />
                {t(`nav.${item.label}`)}
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
  const { t } = useTranslation()

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
          <span className={styles.switcherLabel}>{t('shell.household')}</span>
          <span className={styles.householdName}>{current.name}</span>
          <span className={styles.switcherMeta}>
            {roleLabel}, {current.base_currency}
          </span>
        </span>
        <Icon name="chevronDown" size={18} />
        <span className="visually-hidden">{t('shell.switchHousehold')}</span>
      </button>
      {menu.open && (
        <div id={menuId} className={styles.popover}>
          {others.length > 0 && (
            <ul aria-label={t('shell.yourHouseholds')} className={styles.popoverList}>
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
            <span>{t('shell.newHousehold')}</span>
          </Link>
        </div>
      )}
    </div>
  )
}

const themeOptions: { value: ThemeChoice; icon: 'monitor' | 'sun' | 'moon' }[] = [
  { value: 'system', icon: 'monitor' },
  { value: 'light', icon: 'sun' },
  { value: 'dark', icon: 'moon' },
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
  const { t } = useTranslation()
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
        <span className="visually-hidden">{t('userMenu.open')}</span>
      </button>
      {menu.open && (
        <div id={menuId} className={`${styles.popover} ${styles[placement]}`}>
          <div className={styles.userInfo}>
            <p className={styles.userInfoName}>{user.name}</p>
            <p className={styles.popoverMeta}>{user.email}</p>
          </div>
          <fieldset className={styles.themes}>
            <legend>{t('theme.legend')}</legend>
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
                  {t(`theme.${option.value}`)}
                </button>
              ))}
            </div>
          </fieldset>
          <Link to="/profile" className={styles.popoverItem} onClick={() => menu.close()}>
            <Icon name="user" size={18} />
            <span>{t('userMenu.profile')}</span>
          </Link>
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
            <span>{t('userMenu.logout')}</span>
          </button>
        </div>
      )}
    </div>
  )
}

/** Układ stron bez gospodarstwa (powitanie, nowe gospodarstwo). */
export function PlainLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className={styles.plain}>
      <SkipLink />
      <header className={styles.plainBar}>
        <Link to="/" className={styles.plainBrand}>
          <CoinMark size={32} />
          <span className={styles.wordmark}>{t('appName')}</span>
        </Link>
        <UserMenu placement="down" />
      </header>
      <main id="main" className="plain-main" tabIndex={-1}>
        {children}
      </main>
    </div>
  )
}
