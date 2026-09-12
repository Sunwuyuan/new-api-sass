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
import {
  ChartHistogramIcon,
  CreditCardIcon,
  CubeIcon,
  File01Icon,
  Shield01Icon,
  Ticket01Icon,
  UserGroupIcon,
  Wrench01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar'

import { PlatformBrand } from './brand'
import type { PlatformPath } from './navigation'
import type { PlatformRouter } from './router'
import type { PlatformUser } from './types'

type NavigationItem = { to: PlatformPath; label: string; icon: typeof CubeIcon }

export function PlatformSidebar(props: { user: PlatformUser }) {
  const { t } = useTranslation()
  const path = useRouterState({ select: (state) => state.location.pathname })
  const sidebar = useSidebar()
  const workspaceItems: NavigationItem[] = [
    { to: '/platform', label: t('My workspaces'), icon: CubeIcon },
    {
      to: '/platform/usage',
      label: t('Usage analytics'),
      icon: ChartHistogramIcon,
    },
    { to: '/platform/plans', label: t('Plans'), icon: CreditCardIcon },
    {
      to: '/platform/security',
      label: t('Account'),
      icon: Shield01Icon,
    },
  ]
  const adminItems: NavigationItem[] = [
    {
      to: '/platform/admin/usage',
      label: t('Platform overview'),
      icon: ChartHistogramIcon,
    },
    {
      to: '/platform/admin/users',
      label: t('Platform users'),
      icon: UserGroupIcon,
    },
    {
      to: '/platform/admin/workspaces',
      label: t('All workspaces'),
      icon: CubeIcon,
    },
    {
      to: '/platform/admin/plans',
      label: t('Hosting plans'),
      icon: CreditCardIcon,
    },
    {
      to: '/platform/admin/redemptions',
      label: t('Platform redemption codes'),
      icon: Ticket01Icon,
    },
    {
      to: '/platform/admin/audits',
      label: t('Platform audit'),
      icon: File01Icon,
    },
    {
      to: '/platform/admin/settings',
      label: t('Platform settings'),
      icon: Wrench01Icon,
    },
  ]
  const groups = [{ title: t('Workspace'), items: workspaceItems }]
  if (
    ['admin', 'root'].includes(props.user.role) &&
    !props.user.must_change_password
  ) {
    groups.push({ title: t('Administration'), items: adminItems })
  }
  return (
    <Sidebar collapsible='offcanvas'>
      <SidebarHeader className='px-3 py-4'>
        <PlatformBrand />
      </SidebarHeader>
      <SidebarContent>
        {groups.map((group) => (
          <SidebarGroup key={group.title}>
            <SidebarGroupLabel>{group.title}</SidebarGroupLabel>
            <SidebarMenu>
              {group.items.map((item) => {
                const active =
                  path === item.to ||
                  (item.to === '/platform' &&
                    path.startsWith('/platform/workspaces/')) ||
                  (item.to === '/platform/admin/workspaces' &&
                    path.startsWith(`${item.to}/`))
                return (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton
                      isActive={active}
                      render={
                        <Link<PlatformRouter, string, PlatformPath>
                          to={item.to}
                        />
                      }
                      onClick={() => sidebar.setOpenMobile(false)}
                    >
                      <HugeiconsIcon icon={item.icon} aria-hidden='true' />
                      <span>{item.label}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarFooter className='gap-1 border-t p-4'>
        <p className='truncate text-sm font-medium'>
          {props.user.email || props.user.display_name}
        </p>
        <p className='text-muted-foreground text-xs'>
          {['admin', 'root'].includes(props.user.role)
            ? t('Platform administrator')
            : t('Platform account')}
        </p>
      </SidebarFooter>
    </Sidebar>
  )
}
