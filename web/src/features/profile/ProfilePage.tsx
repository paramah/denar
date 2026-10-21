import { useTranslation } from 'react-i18next'
import { useSessionUser, useUpdateMe } from '../../api/hooks/session.ts'
import { PlainLayout } from '../../app/AppShell.tsx'
import { LinkButton } from '../../components/Button/Button.tsx'
import { SelectField } from '../../components/Field/Field.tsx'
import { FormAlert } from '../../components/Form/Form.tsx'
import { PageHeader } from '../../components/PageHeader/PageHeader.tsx'
import { useToast } from '../../components/Toast/Toast.tsx'
import { applyBrowserLocale, isLocale, locales, setLocale } from '../../i18n/locale.ts'
import { tk } from '../../i18n/translate.ts'
import styles from './profile.module.css'

const auto = 'auto'

/**
 * Profil zalogowanego człowieka (013): dane konta i język interfejsu oraz
 * powiadomień, zapisywany na serwerze (PATCH /me) i stosowany od razu.
 */
export function ProfilePage() {
  const { user } = useSessionUser()
  const { t } = useTranslation(['profile', 'common'])
  const update = useUpdateMe()
  const toast = useToast()
  const value = user.locale ?? auto

  const change = (next: string) => {
    const locale = isLocale(next) ? next : null
    // mutateAsync: obietnica kończy się także wtedy, gdy zmiana języka zamontuje stronę od nowa.
    update
      .mutateAsync({ locale })
      .then(() => {
        if (locale) {
          setLocale(locale, { persist: true })
        } else {
          applyBrowserLocale()
        }
        toast.show({ message: tk('profile:language.saved') })
      })
      .catch(() => undefined) // Błąd pokazuje FormAlert.
  }

  return (
    <PlainLayout>
      <PageHeader
        title={t('title')}
        description={t('description')}
        actions={
          <LinkButton to="/" variant="ghost" icon="chevronLeft">
            {t('back')}
          </LinkButton>
        }
      />
      <div className={styles.sections}>
        <section className={styles.card} aria-labelledby="profile-account">
          <h2 id="profile-account" className={styles.heading}>
            {t('account.heading')}
          </h2>
          <dl className={styles.facts}>
            <div>
              <dt>{t('account.name')}</dt>
              <dd>{user.name}</dd>
            </div>
            <div>
              <dt>{t('account.email')}</dt>
              <dd>{user.email}</dd>
            </div>
          </dl>
        </section>
        <section className={styles.card} aria-labelledby="profile-language">
          <h2 id="profile-language" className={styles.heading}>
            {t('language.heading')}
          </h2>
          <FormAlert error={update.error} />
          <SelectField
            label={t('language.label')}
            hint={t('language.hint')}
            data-focus-key="profile-locale"
            value={value}
            disabled={update.isPending}
            onChange={(event) => change(event.target.value)}
            options={[
              { value: auto, label: t('common:language.auto') },
              ...locales.map((locale) => ({ value: locale, label: t(`common:language.${locale}`) })),
            ]}
          />
        </section>
      </div>
    </PlainLayout>
  )
}
