/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
  type RouterHistory,
} from '@tanstack/react-router'

import { LoadingState } from '@/components/loading-state'

import { platformSessionQuery } from './api'
import { PlatformLayout, PlatformRoot } from './layout'
import { platformAuthSearch, safePlatformRedirect } from './lib/auth-redirect'
import { PlatformAccessDenied, PlatformNotFound } from './navigation'

const root = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: PlatformRoot,
  notFoundComponent: PlatformNotFound,
})
const home = createRoute({
  getParentRoute: () => root,
  path: '/',
  beforeLoad: () => {
    throw redirect({ href: '/platform', replace: true })
  },
})
const auth = createRoute({
  getParentRoute: () => root,
  id: 'auth',
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.fetchQuery(platformSessionQuery)
    if (session) throw redirect({ href: '/platform', replace: true })
  },
})
const signIn = createRoute({
  getParentRoute: () => auth,
  path: '/platform/sign-in',
  validateSearch: platformAuthSearch,
  component: lazyRouteComponent(() => import('./auth-page'), 'PlatformSignIn'),
})
const signUp = createRoute({
  getParentRoute: () => auth,
  path: '/platform/sign-up',
  validateSearch: platformAuthSearch,
  component: lazyRouteComponent(() => import('./auth-page'), 'PlatformSignUp'),
})
const oauth = createRoute({
  getParentRoute: () => root,
  path: '/platform/oauth/$provider',
  component: lazyRouteComponent(() => import('./oauth-callback')),
})
const authenticated = createRoute({
  getParentRoute: () => root,
  id: 'authenticated',
  component: PlatformLayout,
  beforeLoad: async ({ context, location }) => {
    const session = await context.queryClient.fetchQuery({
      ...platformSessionQuery,
      staleTime: 0,
    })
    if (!session) {
      throw redirect({
        href: `/platform/sign-in?redirect=${encodeURIComponent(safePlatformRedirect(location.href))}`,
        replace: true,
      })
    }
    return { platformSession: session }
  },
})
const platform = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform',
  component: lazyRouteComponent(() => import('./dashboard')),
})
const security = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/security',
  component: lazyRouteComponent(() => import('./security')),
})
const hostingPlans = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/plans',
  component: lazyRouteComponent(() => import('./plans-page')),
})
const createWorkspace = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/workspaces/new',
  component: lazyRouteComponent(() => import('./create-page')),
})
const workspace = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/workspaces/$workspaceId',
  component: lazyRouteComponent(() => import('./workspace-detail')),
})
const workspaceAdministrators = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/workspaces/$workspaceId/administrators',
  component: lazyRouteComponent(() => import('./workspace-administrators')),
})
const usage = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/usage',
  component: lazyRouteComponent(() => import('./usage')),
})
const redeem = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/redeem',
  beforeLoad: () => {
    throw redirect({ href: '/platform/plans', replace: true })
  },
})
const admin = createRoute({
  getParentRoute: () => authenticated,
  path: '/platform/admin',
  beforeLoad: ({ context }) => {
    if (!['admin', 'root'].includes(context.platformSession.user.role)) {
      throw new Error('platform_admin_required')
    }
  },
  errorComponent: PlatformAccessDenied,
  component: lazyRouteComponent(() => import('./admin/layout')),
})
const adminHome = createRoute({
  getParentRoute: () => admin,
  path: '/',
  beforeLoad: () => {
    throw redirect({ href: '/platform/admin/usage', replace: true })
  },
})
const users = createRoute({
  getParentRoute: () => admin,
  path: 'users',
  component: lazyRouteComponent(() => import('./admin/users')),
})
const workspaces = createRoute({
  getParentRoute: () => admin,
  path: 'workspaces',
  component: lazyRouteComponent(() => import('./admin/workspaces')),
})
const adminWorkspace = createRoute({
  getParentRoute: () => admin,
  path: 'workspaces/$workspaceId',
  component: lazyRouteComponent(() => import('./workspace-detail')),
})
const adminWorkspaceAdministrators = createRoute({
  getParentRoute: () => admin,
  path: 'workspaces/$workspaceId/administrators',
  component: lazyRouteComponent(() => import('./workspace-administrators')),
})
const plans = createRoute({
  getParentRoute: () => admin,
  path: 'plans',
  component: lazyRouteComponent(() => import('./admin/plans')),
})
const redemptions = createRoute({
  getParentRoute: () => admin,
  path: 'redemptions',
  component: lazyRouteComponent(() => import('./admin/redemptions')),
})
const audits = createRoute({
  getParentRoute: () => admin,
  path: 'audits',
  component: lazyRouteComponent(() => import('./admin/audits')),
})
const adminSettings = createRoute({
  getParentRoute: () => admin,
  path: 'settings',
  component: lazyRouteComponent(() => import('./admin/settings')),
})
const adminUsage = createRoute({
  getParentRoute: () => admin,
  path: 'usage',
  component: lazyRouteComponent(() => import('./usage')),
})
const routeTree = root.addChildren([
  home,
  auth.addChildren([signIn, signUp]),
  oauth,
  authenticated.addChildren([
    platform,
    security,
    hostingPlans,
    createWorkspace,
    workspace,
    workspaceAdministrators,
    usage,
    redeem,
    admin.addChildren([
      adminHome,
      users,
      workspaces,
      adminWorkspace,
      adminWorkspaceAdministrators,
      plans,
      redemptions,
      audits,
      adminSettings,
      adminUsage,
    ]),
  ]),
])

export function createPlatformRouter(
  queryClient: QueryClient,
  history?: RouterHistory
) {
  return createRouter({
    routeTree,
    history,
    context: { queryClient },
    defaultPreload: 'intent',
    defaultPendingComponent: LoadingState,
  })
}
export type PlatformRouter = ReturnType<typeof createPlatformRouter>
