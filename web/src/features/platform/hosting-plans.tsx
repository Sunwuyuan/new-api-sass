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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'

import { getHostingPlans } from './api'
import type { HostingPlan } from './types'

export function HostingPlans() {
  const { t } = useTranslation()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  if (plans.isPending) return <LoadingState />
  if (plans.isError) return <ErrorState onRetry={() => void plans.refetch()} />
  const rows = [
    {
      label: t('Monthly requests'),
      value: (plan: HostingPlan) => plan.limits.requests.toLocaleString(),
    },
    {
      label: t('Users'),
      value: (plan: HostingPlan) => plan.limits.users.toLocaleString(),
    },
    {
      label: t('Tokens'),
      value: (plan: HostingPlan) => plan.limits.tokens.toLocaleString(),
    },
    {
      label: t('Channels'),
      value: (plan: HostingPlan) => plan.limits.channels.toLocaleString(),
    },
    {
      label: t('Workspaces per account'),
      value: (plan: HostingPlan) =>
        plan.capabilities.max_workspaces.toLocaleString(),
    },
    {
      label: t('Branding'),
      value: (plan: HostingPlan) =>
        plan.capabilities.custom_branding
          ? t('Custom branding')
          : t('Basic branding'),
    },
    {
      label: t('Platform footer'),
      value: (plan: HostingPlan) =>
        plan.capabilities.remove_platform_footer
          ? t('Custom platform footer')
          : t('Platform footer required'),
    },
  ]
  const columns: StaticDataTableColumn<(typeof rows)[number]>[] = [
    {
      id: 'capability',
      header: t('Included capabilities'),
      cell: (row) => row.label,
    },
    ...[...plans.data]
      .sort((a, b) => a.limits.requests - b.limits.requests)
      .map((plan) => ({
        id: String(plan.id),
        header: (
          <div className='py-3'>
            <p className='text-foreground font-semibold'>{plan.name}</p>
            <p className='text-muted-foreground mt-1 text-xs font-normal'>
              {t(plan.price)}
            </p>
          </div>
        ),
        cell: (row: (typeof rows)[number]) => row.value(plan),
      })),
  ]
  return (
    <section aria-label={t('Hosting plans')}>
      <StaticDataTable
        data={rows}
        columns={columns}
        getRowKey={(row) => row.label}
      />
    </section>
  )
}
