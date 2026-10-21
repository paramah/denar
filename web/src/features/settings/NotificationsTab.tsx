import { useDocumentTitle } from '../../components/PageHeader/PageHeader.tsx'
import { ChannelsSection } from '../notifications/ChannelsSection.tsx'
import { PreferencesSection } from '../notifications/PreferencesSection.tsx'

/** Ustawienia osobiste (każda rola): kanały i preferencje powiadomień w tym gospodarstwie. */
export function NotificationsTab() {
  useDocumentTitle('Powiadomienia — ustawienia')
  return (
    <>
      <ChannelsSection />
      <PreferencesSection />
    </>
  )
}
