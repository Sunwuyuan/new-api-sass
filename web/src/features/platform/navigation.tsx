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
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { buttonVariants } from '@/components/ui/button'

import type { PlatformRouter } from './router'

export type PlatformPath =
  | '/platform'
  | '/platform/sign-in'
  | '/platform/sign-up'
  | '/platform/security'
  | '/platform/plans'
  | '/platform/usage'
  | '/platform/redeem'
  | '/platform/workspaces/new'
  | '/platform/admin/users'
  | '/platform/admin/workspaces'
  | '/platform/admin/plans'
  | '/platform/admin/redemptions'
  | '/platform/admin/audits'
  | '/platform/admin/usage'
  | '/platform/admin/settings'

export function PlatformLink(props: {
  to: PlatformPath
  children: ReactNode
  className?: string
  search?: { redirect?: string }
}) {
  return (
    <Link<PlatformRouter, string, PlatformPath>
      to={props.to}
      search={props.search}
      className={props.className ?? buttonVariants({ variant: 'ghost' })}
    >
      {props.children}
    </Link>
  )
}

export function WorkspaceLink(props: {
  id: number
  admin?: boolean
  children: ReactNode
  className?: string
}) {
  return (
    <Link<
      PlatformRouter,
      string,
      | '/platform/workspaces/$workspaceId'
      | '/platform/admin/workspaces/$workspaceId'
    >
      to={
        props.admin
          ? '/platform/admin/workspaces/$workspaceId'
          : '/platform/workspaces/$workspaceId'
      }
      params={{ workspaceId: String(props.id) }}
      className={
        props.className ?? 'font-medium underline-offset-4 hover:underline'
      }
    >
      {props.children}
    </Link>
  )
}

export function PlatformAccessDenied() {
  const { t } = useTranslation()
  return (
    <ErrorState
      title={t('Platform administrator access required')}
      description={t('You can only manage your own workspaces.')}
      action={<PlatformLink to='/platform'>{t('My workspaces')}</PlatformLink>}
    />
  )
}

export function PlatformNotFound() {
  const { t } = useTranslation()
  return (
    <ErrorState
      title={t('Page not found')}
      action={<PlatformLink to='/platform'>{t('My workspaces')}</PlatformLink>}
    />
  )
}
