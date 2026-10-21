import type { ReactNode, SVGProps } from 'react'

// Ikony liniowe 24×24, rysowane kreską w kolorze tekstu. Zawsze aria-hidden:
// znaczenie niesie tekst obok albo etykieta przycisku.
const paths: Record<string, ReactNode> = {
  dashboard: (
    <>
      <path d="M4 13h6V4H4zM14 20h6v-9h-6zM4 20h6v-3H4zM14 7h6V4h-6z" />
    </>
  ),
  wallet: (
    <>
      <path d="M4 7.5A2.5 2.5 0 0 1 6.5 5H18v3" />
      <path d="M4 7.5V17a2 2 0 0 0 2 2h14V8H6.5A2.5 2.5 0 0 1 4 7.5Z" />
      <circle cx="16" cy="13.5" r="1.2" />
    </>
  ),
  budget: (
    <>
      <path d="M3 7h18v12H3z" />
      <path d="m3 7 9 6 9-6" />
    </>
  ),
  repeat: (
    <>
      <path d="M17 3l3 3-3 3" />
      <path d="M4 12V9a3 3 0 0 1 3-3h13" />
      <path d="M7 21l-3-3 3-3" />
      <path d="M20 12v3a3 3 0 0 1-3 3H4" />
    </>
  ),
  bell: (
    <>
      <path d="M6 16V11a6 6 0 1 1 12 0v5l1.5 2h-15z" />
      <path d="M10 20.5a2 2 0 0 0 4 0" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2.5v3M12 18.5v3M4.2 6.2l2.1 2.1M17.7 15.7l2.1 2.1M2.5 12h3M18.5 12h3M4.2 17.8l2.1-2.1M17.7 8.3l2.1-2.1" />
    </>
  ),
  menu: <path d="M4 7h16M4 12h16M4 17h16" />,
  close: <path d="M6 6l12 12M18 6 6 18" />,
  copy: (
    <>
      <rect x="9" y="9" width="11" height="11" rx="1.5" />
      <path d="M15 9V5.5A1.5 1.5 0 0 0 13.5 4h-8A1.5 1.5 0 0 0 4 5.5v8A1.5 1.5 0 0 0 5.5 15H9" />
    </>
  ),
  check: <path d="m5 12.5 4.5 4.5L19 7.5" />,
  alert: (
    <>
      <path d="M12 3.5 2.5 20h19z" />
      <path d="M12 10v4.5M12 17.2v.1" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5.5M12 7.8v.1" />
    </>
  ),
  logout: (
    <>
      <path d="M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4" />
      <path d="M10 16l-4-4 4-4M6 12h10" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
  user: (
    <>
      <circle cx="12" cy="8.5" r="3.5" />
      <path d="M5 20a7 7 0 0 1 14 0" />
    </>
  ),
  users: (
    <>
      <circle cx="9" cy="9" r="3.2" />
      <path d="M3 19.5a6 6 0 0 1 12 0" />
      <path d="M15.5 6a3 3 0 0 1 0 6M17.5 14.5a5.5 5.5 0 0 1 3.5 5" />
    </>
  ),
  bot: (
    <>
      <rect x="4.5" y="8" width="15" height="11" rx="2" />
      <path d="M12 4.5V8M9 13v1M15 13v1M2.5 12.5v3M21.5 12.5v3" />
      <circle cx="12" cy="4" r="1" />
    </>
  ),
  mail: (
    <>
      <rect x="3" y="5.5" width="18" height="13" rx="1.5" />
      <path d="m3.5 6.5 8.5 6.5 8.5-6.5" />
    </>
  ),
  history: (
    <>
      <path d="M3.5 12a8.5 8.5 0 1 0 2.5-6" />
      <path d="M3 4v4h4M12 7.5V12l3 2" />
    </>
  ),
  home: (
    <>
      <path d="M4 10.5 12 4l8 6.5V20H4z" />
      <path d="M10 20v-5h4v5" />
    </>
  ),
  chevronDown: <path d="m6 9 6 6 6-6" />,
  chevronRight: <path d="m9 6 6 6-6 6" />,
  key: (
    <>
      <circle cx="8" cy="15" r="4" />
      <path d="M11 12l8.5-8.5M16 7l2.5 2.5M14 9l2 2" />
    </>
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" />
    </>
  ),
  moon: <path d="M19.5 14.5A8 8 0 0 1 9.5 4.5a8 8 0 1 0 10 10Z" />,
  monitor: (
    <>
      <rect x="3" y="4.5" width="18" height="12" rx="1.5" />
      <path d="M9 20h6M12 16.5V20" />
    </>
  ),
  trash: (
    <>
      <path d="M4.5 7h15M9.5 7V4.5h5V7M6.5 7l1 13h9l1-13" />
    </>
  ),
  refresh: (
    <>
      <path d="M20 11a8 8 0 0 0-14.5-4.5L4 8" />
      <path d="M4 4v4h4M4 13a8 8 0 0 0 14.5 4.5L20 16" />
      <path d="M20 20v-4h-4" />
    </>
  ),
  pause: <path d="M9 5v14M15 5v14" />,
  play: <path d="M7 5l12 7-12 7z" />,
  link: (
    <>
      <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" />
      <path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
    </>
  ),
  list: (
    <>
      <path d="M9 6h11M9 12h11M9 18h11" />
      <path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01" />
    </>
  ),
  transfer: (
    <>
      <path d="M4 8h14l-3.5-3.5" />
      <path d="M20 16H6l3.5 3.5" />
    </>
  ),
  edit: (
    <>
      <path d="M4 20h4L19 9l-4-4L4 16z" />
      <path d="M13.5 6.5l4 4" />
    </>
  ),
  archive: (
    <>
      <path d="M3.5 4.5h17v4h-17z" />
      <path d="M5 8.5V19.5h14V8.5M10 12.5h4" />
    </>
  ),
  lock: (
    <>
      <rect x="5" y="10.5" width="14" height="9.5" rx="1.5" />
      <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4.5 4.5" />
    </>
  ),
  tag: (
    <>
      <path d="M3.5 12.5V4.5h8l9 9-8 8z" />
      <circle cx="8" cy="9" r="1.3" />
    </>
  ),
  rate: (
    <>
      <path d="M4 18 9.5 12l3.5 3.5L20 8" />
      <path d="M15 8h5v5" />
    </>
  ),
  circle: <circle cx="12" cy="12" r="7.5" />,
  checkCircle: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="m8.5 12.2 2.4 2.4 4.6-4.9" />
    </>
  ),
}

export type IconName = keyof typeof paths

type Props = SVGProps<SVGSVGElement> & { name: IconName; size?: number }

export function Icon({ name, size = 20, ...rest }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {paths[name]}
    </svg>
  )
}

/** Duży karbowany rant monety — dekoracja ekranu logowania (aria-hidden). */
export function MilledRing({ className }: { className?: string }) {
  const notches = Array.from({ length: 120 }, (_, i) => {
    const angle = (i / 120) * Math.PI * 2
    return (
      <line
        key={i}
        x1={100 + Math.cos(angle) * 90}
        y1={100 + Math.sin(angle) * 90}
        x2={100 + Math.cos(angle) * 98}
        y2={100 + Math.sin(angle) * 98}
      />
    )
  })
  return (
    <svg viewBox="0 0 200 200" className={className} aria-hidden="true" focusable="false">
      <g stroke="currentColor" strokeWidth="1.2" strokeLinecap="round">
        {notches}
      </g>
      <circle cx="100" cy="100" r="84" fill="none" stroke="currentColor" strokeWidth="0.6" />
      <circle cx="100" cy="100" r="70" fill="none" stroke="currentColor" strokeWidth="0.4" strokeDasharray="1 3" />
    </svg>
  )
}

/**
 * Znak marki: moneta z karbowanym rantem i literą D. Rant to jedyny
 * „ozdobnik” aplikacji — nawiązanie do bitej srebrnej monety.
 */
export function CoinMark({ size = 36, title }: { size?: number; title?: string }) {
  const notches = Array.from({ length: 40 }, (_, i) => {
    const angle = (i / 40) * Math.PI * 2
    const x1 = 20 + Math.cos(angle) * 17.2
    const y1 = 20 + Math.sin(angle) * 17.2
    const x2 = 20 + Math.cos(angle) * 19.2
    const y2 = 20 + Math.sin(angle) * 19.2
    return <line key={i} x1={x1} y1={y1} x2={x2} y2={y2} />
  })
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 40 40"
      role={title ? 'img' : undefined}
      aria-hidden={title ? undefined : 'true'}
      aria-label={title}
      focusable="false"
    >
      <g stroke="var(--silver)" strokeWidth="1.1" strokeLinecap="round">
        {notches}
      </g>
      <circle cx="20" cy="20" r="16" fill="var(--patina)" />
      <circle cx="20" cy="20" r="13.3" fill="none" stroke="var(--on-patina)" strokeOpacity="0.45" strokeWidth="0.8" />
      <path
        d="M15 12.5h4.6c4.6 0 7.6 3 7.6 7.5s-3 7.5-7.6 7.5H15z"
        fill="none"
        stroke="var(--on-patina)"
        strokeWidth="2.4"
        strokeLinejoin="round"
      />
    </svg>
  )
}
