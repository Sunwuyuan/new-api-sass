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

type PlatformPath =
  | '/platform'
  | '/platform/admin/users'
  | '/platform/admin/workspaces'
  | '/platform/admin/plans'
  | '/platform/admin/redemptions'
  | '/platform/admin/audits'

export function PlatformLink(props: { to: PlatformPath; children: ReactNode }) {
  return (
    <Link<PlatformRouter, string, PlatformPath>
      to={props.to}
      className={buttonVariants({ variant: 'ghost' })}
      activeProps={{ className: 'bg-accent text-accent-foreground' }}
      activeOptions={{ exact: true }}
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
