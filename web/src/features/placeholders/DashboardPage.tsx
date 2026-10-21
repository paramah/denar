import { Link } from 'react-router'
import { useSessionUser } from '../../api/hooks/session.ts'
import { canManage, useHousehold } from '../../app/household.tsx'
import { householdPath } from '../../app/nav.ts'
import { Badge } from '../../components/Badge/Badge.tsx'
import { Icon, type IconName } from '../../components/Icon/Icon.tsx'
import { Money } from '../../components/Money/Money.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { roleLabels } from '../../lib/labels.ts'
import styles from './placeholders.module.css'

function greeting(now = new Date()): string {
  const hour = now.getHours()
  if (hour < 5) return 'Dobry wieczór'
  if (hour < 18) return 'Dzień dobry'
  return 'Dobry wieczór'
}

type Step = { to: string; icon: IconName; title: string; text: string }

/**
 * Pulpit części A: gospodarstwo i następne kroki. Salda, „do przydzielenia”
 * i odnowienia dojdą w części C.
 */
export function DashboardPage() {
  const { household, role } = useHousehold()
  const { user } = useSessionUser()
  const firstName = user.name.split(/\s+/)[0] ?? user.name

  const steps: Step[] = [
    ...(canManage(role)
      ? [
          {
            to: householdPath(household.id, 'settings/invitations'),
            icon: 'users' as const,
            title: 'Zaproś domowników',
            text: 'Wyślij link z zaproszeniem i wybierz, co każdy może robić.',
          },
        ]
      : []),
    ...(role !== 'viewer'
      ? [
          {
            to: householdPath(household.id, 'settings/bots'),
            icon: 'bot' as const,
            title: 'Podłącz asystenta AI',
            text: 'Utwórz bota z tokenem i uprawnieniami tylko do tego, czego potrzebuje.',
          },
        ]
      : []),
    {
      to: householdPath(household.id, 'settings'),
      icon: 'settings',
      title: 'Sprawdź ustawienia',
      text: 'Strefa czasowa i dzień, od którego liczysz miesiąc budżetowy.',
    },
  ]

  return (
    <>
      <PageHeader title={`${greeting()}, ${firstName}`} documentTitle="Pulpit" />
      <section className={styles.ledger} aria-labelledby="household-facts">
        <h2 id="household-facts" className="visually-hidden">
          Gospodarstwo
        </h2>
        <div className={styles.ledgerMain}>
          <p className={styles.ledgerLabel}>Do przydzielenia w tym miesiącu</p>
          <Money amount="0.00" currency={household.base_currency} size="xl" />
          <p className={styles.ledgerNote}>
            Kwota pojawi się, gdy dodasz portfele i pierwsze wpływy. <Badge tone="brand">Wkrótce</Badge>
          </p>
        </div>
        <dl className={styles.facts}>
          <div>
            <dt>Twoja rola</dt>
            <dd>{roleLabels[role]}</dd>
          </div>
          <div>
            <dt>Waluta bazowa</dt>
            <dd>{household.base_currency}</dd>
          </div>
          <div>
            <dt>Miesiąc budżetowy od</dt>
            <dd>{household.month_start_day}. dnia</dd>
          </div>
          <div>
            <dt>Strefa czasowa</dt>
            <dd>{household.timezone}</dd>
          </div>
        </dl>
      </section>

      <section className={styles.steps} aria-labelledby="next-steps">
        <h2 id="next-steps">Następne kroki</h2>
        <ul>
          {steps.map((step) => (
            <li key={step.to}>
              <Link to={step.to} className={styles.step}>
                <Icon name={step.icon} size={22} />
                <span>
                  <span className={styles.stepTitle}>{step.title}</span>
                  <span className={styles.stepText}>{step.text}</span>
                </span>
                <Icon name="chevronRight" size={18} />
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </>
  )
}
