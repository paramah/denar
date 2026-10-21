import { createBrowserRouter, Navigate, type RouteObject } from 'react-router'
import { InvitePage } from '../features/auth/InvitePage.tsx'
import { LoginPage } from '../features/auth/LoginPage.tsx'
import { CreateHouseholdPage } from '../features/households/CreateHouseholdPage.tsx'
import { ComingSoonPage } from '../features/placeholders/ComingSoonPage.tsx'
import { DashboardPage } from '../features/dashboard/DashboardPage.tsx'
import { AuditTab } from '../features/settings/AuditTab.tsx'
import { BotsTab } from '../features/settings/BotsTab.tsx'
import { GeneralTab } from '../features/settings/GeneralTab.tsx'
import { InvitationsTab } from '../features/settings/InvitationsTab.tsx'
import { MembersTab } from '../features/settings/MembersTab.tsx'
import { PayeesTab } from '../features/settings/PayeesTab.tsx'
import { RatesTab } from '../features/settings/RatesTab.tsx'
import { RequireTab, SettingsLayout } from '../features/settings/SettingsLayout.tsx'
import { TransactionsPage } from '../features/transactions/TransactionsPage.tsx'
import { WalletPage } from '../features/wallets/WalletPage.tsx'
import { WalletsPage } from '../features/wallets/WalletsPage.tsx'
import { HomePage } from './HomePage.tsx'
import { HouseholdRoute } from './household.tsx'
import { NotFoundPage } from './NotFoundPage.tsx'
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
          { path: 'budget', element: <ComingSoonPage section="budget" /> },
          { path: 'subscriptions', element: <ComingSoonPage section="subscriptions" /> },
          { path: 'notifications', element: <ComingSoonPage section="notifications" /> },
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
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
]

export function createAppRouter() {
  return createBrowserRouter(routes)
}
