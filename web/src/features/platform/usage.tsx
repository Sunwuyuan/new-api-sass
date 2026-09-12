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
import { useRouterState } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { PlatformTable } from './admin/table'
import { getHostingPlans, getWorkspaces } from './api'
import { WorkspaceLink } from './navigation'
import type { WorkspaceUsage } from './types'
import { platformUsageQuery } from './workspace-api'
import { WorkspaceStatus, WorkspaceUsageMeter } from './workspace-status'

export default function PlatformUsage() {
  const { t } = useTranslation()
  const admin = useRouterState({
    select: (state) => state.location.pathname.startsWith('/platform/admin/'),
  })
  const usage = useQuery(platformUsageQuery(admin))
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const columns: ColumnDef<WorkspaceUsage>[] = [
    {
      id: 'name',
      header: t('Workspace'),
      cell: ({ row }) => (
        <WorkspaceLink id={row.original.tenant.id} admin={admin}>
          {row.original.tenant.name}
        </WorkspaceLink>
      ),
    },
    {
      id: 'plan',
      header: t('Hosting plan'),
      cell: ({ row }) =>
        plans.data?.find((plan) => plan.id === row.original.tenant.plan_id)
          ?.name ?? '—',
    },
    {
      id: 'usage',
      header: t('Monthly requests'),
      cell: ({ row }) => (
        <WorkspaceUsageMeter
          requests={row.original.usage.requests}
          limit={
            plans.data?.find((plan) => plan.id === row.original.tenant.plan_id)
              ?.limits.requests ?? 0
          }
          name={row.original.tenant.name}
        />
      ),
    },
    {
      id: 'remaining',
      header: t('Remaining requests'),
      cell: ({ row }) =>
        Math.max(
          0,
          (plans.data?.find((plan) => plan.id === row.original.tenant.plan_id)
            ?.limits.requests ?? 0) - row.original.usage.requests
        ).toLocaleString(),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: ({ row }) => <WorkspaceStatus workspace={row.original.tenant} />,
    },
    {
      id: 'expiry',
      header: t('Plan expires'),
      cell: ({ row }) =>
        row.original.tenant.plan_expires_at
          ? new Date(row.original.tenant.plan_expires_at).toLocaleDateString()
          : t('No expiry'),
    },
  ]
  if (usage.isPending || plans.isPending) return <LoadingState />
  if (usage.isError || plans.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void usage.refetch()
          void plans.refetch()
        }}
      />
    )
  }
  const data = usage.data
  const stats = [
    { label: t('Monthly requests'), value: data.summary.requests },
    { label: t('Active workspaces'), value: data.summary.active },
    { label: t('Expiring within 7 days'), value: data.summary.expiring_soon },
    { label: t('Monthly limit reached'), value: data.summary.exhausted },
  ]
  return (
    <section className='space-y-6'>
      <header>
        <h1 className='text-2xl font-semibold'>
          {admin ? t('Platform overview') : t('Usage analytics')}
        </h1>
        <p className='text-muted-foreground mt-1 text-sm'>
          {data.month} UTC ·{' '}
          {t(
            'Usage is counted from gateway requests in the UTC calendar month.'
          )}
        </p>
      </header>
      <dl className='grid grid-cols-2 gap-4 xl:grid-cols-4'>
        {stats.map((stat) => (
          <div key={stat.label} className='rounded-lg border p-4'>
            <dt className='text-muted-foreground text-sm'>{stat.label}</dt>
            <dd className='mt-2 text-2xl font-semibold tabular-nums'>
              {stat.value.toLocaleString()}
            </dd>
          </div>
        ))}
      </dl>
      {admin && (
        <p className='text-muted-foreground text-sm'>
          {t('Workspaces')}: {data.summary.workspaces.toLocaleString()} ·{' '}
          {t('Suspended')}: {data.summary.suspended.toLocaleString()} ·{' '}
          {t('Expired')}: {data.summary.expired.toLocaleString()}
        </p>
      )}
      <section className='space-y-3'>
        <h2 className='text-base font-semibold'>{t('Usage by workspace')}</h2>
        <PlatformTable
          scope={admin ? 'admin' : 'user'}
          resource='usage-workspaces'
          columns={columns}
          statuses={[
            { value: 'active', label: t('Active') },
            { value: 'suspended', label: t('Suspended') },
            { value: 'expired', label: t('Expired') },
          ]}
          load={async (params) => {
            const response = await getWorkspaces(admin, params)
            return { ...response, items: response.tenants }
          }}
        />
      </section>
      <div className='grid gap-4 lg:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle>{t('Plan distribution')}</CardTitle>
          </CardHeader>
          <CardContent>
            <StaticDataTable
              data={data.plans}
              getRowKey={(row) => row.plan_id}
              columns={[
                {
                  id: 'plan',
                  header: t('Hosting plan'),
                  cell: (row) => row.name,
                },
                {
                  id: 'workspaces',
                  header: t('Workspaces'),
                  cell: (row) => row.workspaces.toLocaleString(),
                },
                {
                  id: 'requests',
                  header: t('Requests'),
                  cell: (row) => row.requests.toLocaleString(),
                },
              ]}
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t('Monthly request history')}</CardTitle>
          </CardHeader>
          <CardContent>
            <StaticDataTable
              data={data.history}
              getRowKey={(row) => row.month}
              emptyContent={t('No recorded usage yet')}
              columns={[
                { id: 'month', header: t('Month'), cell: (row) => row.month },
                {
                  id: 'requests',
                  header: t('Requests'),
                  cell: (row) => row.requests.toLocaleString(),
                },
              ]}
            />
          </CardContent>
        </Card>
      </div>
    </section>
  )
}
