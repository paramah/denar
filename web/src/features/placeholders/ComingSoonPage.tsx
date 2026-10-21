import { Badge } from '../../components/Badge/Badge.tsx'
import { EmptyState } from '../../components/EmptyState/EmptyState.tsx'
import type { IconName } from '../../components/Icon/Icon.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import styles from './placeholders.module.css'

export type Section = 'budget' | 'subscriptions' | 'notifications'

const sections: Record<Section, { title: string; icon: IconName; lead: string; empty: string; points: string[] }> = {
  budget: {
    title: 'Budżet',
    icon: 'budget',
    lead: 'Koperty na każdy miesiąc: ile przydzielono, ile wydano i ile jeszcze zostało.',
    empty: 'Budżet kopertowy jest w przygotowaniu',
    points: [
      'Kwota „do przydzielenia” z wpływów na portfele budżetowe',
      'Przenoszenie pieniędzy między kopertami',
      'Cele: odkładanie na konkretną kwotę do wybranej daty',
    ],
  },
  subscriptions: {
    title: 'Subskrypcje',
    icon: 'repeat',
    lead: 'Koszty cykliczne z datami odnowień, historią cen i przypomnieniami.',
    empty: 'Śledzenie subskrypcji jest w przygotowaniu',
    points: [
      'Kalendarz odnowień i miesięczny koszt wszystkich subskrypcji',
      'Historia zmian cen każdej usługi',
      'Dopasowanie obciążeń z konta do oczekiwanych płatności',
    ],
  },
  notifications: {
    title: 'Powiadomienia',
    icon: 'bell',
    lead: 'Przypomnienia o odnowieniach, końcach okresów próbnych i przekroczonych kopertach.',
    empty: 'Powiadomienia są w przygotowaniu',
    points: [
      'Skrzynka powiadomień w aplikacji',
      'Wysyłka e-mailem, przez webhook, ntfy, Telegram lub Matrix',
      'Wybór, o czym i którym kanałem chcesz być powiadamiany',
    ],
  },
}

/** Dopracowany pusty stan sekcji, którą wypełni część C etapu 007. */
export function ComingSoonPage({ section }: { section: Section }) {
  const info = sections[section]
  return (
    <>
      <PageHeader title={info.title} description={info.lead} />
      <EmptyState icon={info.icon} title={info.empty} badge={<Badge tone="brand">Wkrótce</Badge>}>
        <p>Ten widok pojawi się w jednej z najbliższych wersji. Będzie w nim:</p>
        <ul className={styles.points}>
          {info.points.map((point) => (
            <li key={point}>{point}</li>
          ))}
        </ul>
      </EmptyState>
    </>
  )
}
