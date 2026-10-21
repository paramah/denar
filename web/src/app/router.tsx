import { createBrowserRouter, Navigate, type RouteObject } from 'react-router'
import { BudgetPage } from '../features/budget/BudgetPage.tsx'
import { InvitePage } from '../features/auth/InvitePage.tsx'
import { LoginPage } from '../features/auth/LoginPage.tsx'
import { ImportBatchPage } from '../features/import/ImportBatchPage.tsx'
import { ImportHistoryPage } from '../features/import/ImportHistoryPage.tsx'
import { ImportLayout } from '../features/import/ImportLayout.tsx'
import { ImportWizardPage } from '../features/import/ImportWizardPage.tsx'
import { CreateHouseholdPage } from '../features/households/CreateHouseholdPage.tsx'
import { NotificationsPage } from '../features/notifications/NotificationsPage.tsx'
import { ChargesQueuePage } from '../features/subscriptions/ChargesQueuePage.tsx'
import { ListPage as SubscriptionListPage } from '../features/subscriptions/ListPage.tsx'
import { OverviewPage as SubscriptionOverviewPage } from '../features/subscriptions/OverviewPage.tsx'
import { SubscriptionPage } from '../features/subscriptions/SubscriptionPage.tsx'
import { SubscriptionsLayout } from '../features/subscriptions/SubscriptionsLayout.tsx'
import { DashboardPage } from '../features/dashboard/DashboardPage.tsx'
import { AuditTab } from '../features/settings/AuditTab.tsx'
import { BotsTab } from '../features/settings/BotsTab.tsx'
import { ConnectionsTab } from '../features/settings/ConnectionsTab.tsx'
import { GeneralTab } from '../features/settings/GeneralTab.tsx'
import { InvitationsTab } from '../features/settings/InvitationsTab.tsx'
import { MembersTab } from '../features/settings/MembersTab.tsx'
import { NotificationsTab } from '../features/settings/NotificationsTab.tsx'
import { PayeesTab } from '../features/settings/PayeesTab.tsx'
import { RatesTab } from '../features/settings/RatesTab.tsx'
import { RequireTab, SettingsLayout } from '../features/settings/SettingsLayout.tsx'
import { TransactionsPage } from '../features/transactions/TransactionsPage.tsx'
import { WalletPage } from '../features/wallets/WalletPage.tsx'
import { WalletsPage } from '../features/wallets/WalletsPage.tsx'
import { HomePage } from './HomePage.tsx'
import { HouseholdRoute } from './household.tsx'
import { HouseholdNotFoundPage, NotFoundPage } from './NotFoundPage.tsx'
import { RequireSession } from './RequireSession.tsx'

/**
 * Wszystkie trasy aplikacji. Nowa strona gospodarstwa (części B/C) to
 * dziecko `h/:householdId`, np. { path: 'wallets', element: <WalletsPage/> }.
 */
export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  { path: '/invite', element: <InvitePage /> },
  {
    path: '/',
    element: <RequireSession />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'households/new', element: <CreateHouseholdPage /> },
      {
        path: 'h/:householdId',
        element: <HouseholdRoute />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: 'wallets', element: <WalletsPage /> },
          { path: 'wallets/:walletId', element: <WalletPage /> },
          { path: 'transactions', element: <TransactionsPage /> },
          { path: 'budget', element: <BudgetPage /> },
          { path: 'budget/:month', element: <BudgetPage /> },
          {
            path: 'subscriptions',
            element: <SubscriptionsLayout />,
            children: [
              { index: true, element: <SubscriptionOverviewPage /> },
              { path: 'list', element: <SubscriptionListPage /> },
              { path: 'charges', element: <ChargesQueuePage /> },
            ],
          },
          { path: 'subscriptions/:subscriptionId', element: <SubscriptionPage /> },
          {
            path: 'import',
            element: <ImportLayout />,
            children: [
              { index: true, element: <ImportWizardPage /> },
              { path: 'history', element: <ImportHistoryPage /> },
            ],
          },
          { path: 'import/:importId', element: <ImportBatchPage /> },
          { path: 'notifications', element: <NotificationsPage /> },
          {
            path: 'settings',
            element: <SettingsLayout />,
            children: [
              { index: true, element: <Navigate to="general" replace /> },
              { path: 'general', element: <GeneralTab /> },
              { path: 'members', element: <MembersTab /> },
              {
                path: 'invitations',
                element: (
                  <RequireTab path="invitations">
                    <InvitationsTab />
                  </RequireTab>
                ),
              },
              {
                path: 'bots',
                element: (
                  <RequireTab path="bots">
                    <BotsTab />
                  </RequireTab>
                ),
              },
              { path: 'payees', element: <PayeesTab /> },
              { path: 'rates', element: <RatesTab /> },
              { path: 'notifications', element: <NotificationsTab /> },
              {
                path: 'connections',
                element: (
                  <RequireTab path="connections">
                    <ConnectionsTab />
                  </RequireTab>
                ),
              },
              {
                path: 'audit',
                element: (
                  <RequireTab path="audit">
                    <AuditTab />
                  </RequireTab>
                ),
              },
            ],
          },
          // Nieznany adres w gospodarstwie — 404 w powłoce aplikacji.
          { path: '*', element: <HouseholdNotFoundPage /> },
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
]

export function createAppRouter() {
  return createBrowserRouter(routes)
}
