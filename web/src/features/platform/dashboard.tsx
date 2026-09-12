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
import {
  getCoreRowModel,
  useReactTable,
  type PaginationState,
} from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePagination } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'

import { getHostingPlans, getWorkspaces, platformSessionQuery } from './api'
import { CreateWorkspace } from './create-workspace'
import { HostingPlans } from './hosting-plans'
import { WorkspaceCard } from './workspace-card'

export default function PlatformDashboard() {
  const { t } = useTranslation()
  const session = useQuery(platformSessionQuery)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const workspaces = useQuery({
    queryKey: ['platform', 'tenants', session.data?.user.id, pagination],
    queryFn: () =>
      getWorkspaces(false, {
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }),
    enabled: !!session.data && !session.data.user.must_change_password,
  })
  const table = useReactTable({
    data: workspaces.data?.tenants ?? [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    rowCount: workspaces.data?.pagination.total ?? 0,
    state: { pagination },
    onPaginationChange: setPagination,
  })
  return (
    <>
      <HostingPlans />
      <section className='space-y-4' aria-label={t('My workspaces')}>
        <h2 className='text-xl font-semibold'>{t('My workspaces')}</h2>
        {workspaces.data && (
          <p className='text-muted-foreground'>
            {t('Workspace capacity: {{used}} / {{limit}}', {
              used: workspaces.data.workspace_count,
              limit: workspaces.data.max_workspaces,
            })}
          </p>
        )}
        {workspaces.data && (
          <CreateWorkspace
            key={session.data?.user.id}
            disabled={
              workspaces.data.workspace_count >= workspaces.data.max_workspaces
            }
          />
        )}
        {workspaces.isPending && <LoadingState />}
        {workspaces.isError && (
          <ErrorState onRetry={() => void workspaces.refetch()} />
        )}
        {workspaces.data?.tenants.length === 0 && (
          <EmptyState title={t('No workspaces yet')} />
        )}
        <div className='grid gap-4 md:grid-cols-2'>
          {workspaces.data?.tenants.map((item) => (
            <WorkspaceCard
              key={item.tenant.id}
              item={item}
              plans={plans.data ?? []}
            />
          ))}
        </div>
        {workspaces.data && <DataTablePagination table={table} compact />}
      </section>
    </>
  )
}
