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
  type RouterHistory,
} from '@tanstack/react-router'

import { LoadingState } from '@/components/loading-state'

import { platformSessionQuery } from './api'
import { PlatformLayout } from './layout'
import { PlatformAccessDenied } from './navigation'

const root = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: PlatformLayout,
})
const home = createRoute({
  getParentRoute: () => root,
  path: '/',
  component: lazyRouteComponent(() => import('./dashboard')),
})
const platform = createRoute({
  getParentRoute: () => root,
  path: '/platform',
  component: lazyRouteComponent(() => import('./dashboard')),
})
const admin = createRoute({
  getParentRoute: () => root,
  path: '/platform/admin',
  beforeLoad: async ({ context }) => {
    // Resolve platform authentication before the page can mount. The root
    // layout renders sign-in, required password changes or access denied and
    // withholds Outlet until the account is authorized. Keeping those states
    // out of router errors also lets sign-in resume a protected deep link.
    await context.queryClient.ensureQueryData(platformSessionQuery)
  },
  errorComponent: PlatformAccessDenied,
  component: lazyRouteComponent(() => import('./admin/layout')),
})
const adminHome = createRoute({
  getParentRoute: () => admin,
  path: '/',
  component: lazyRouteComponent(() => import('./admin/users')),
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
const routeTree = root.addChildren([
  home,
  platform,
  admin.addChildren([adminHome, users, workspaces, plans, redemptions, audits]),
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
