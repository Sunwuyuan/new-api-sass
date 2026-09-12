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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableRowActionMenu } from '@/components/data-table'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'

import { getHostingPlans, getWorkspaces, setWorkspaceStatus } from '../api'
import { WorkspaceLink } from '../navigation'
import type { Workspace, WorkspaceUsage } from '../types'
import { AssignPlanDialog } from './assign-plan-dialog'
import { PlatformTable } from './table'

export default function PlatformWorkspaces() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const [assign, setAssign] = useState<Workspace | null>(null)
  const [status, setStatus] = useState<Workspace | null>(null)
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (workspace: Workspace) =>
      setWorkspaceStatus(
        workspace.id,
        workspace.status === 'active' ? 'suspended' : 'active'
      ),
    onSuccess: () => {
      setStatus(null)
      void queryClient.invalidateQueries({ queryKey: ['platform'] })
      toast.success(t('Workspace updated'))
    },
  })
  const columns: ColumnDef<WorkspaceUsage>[] = [
    { id: 'id', header: t('ID'), accessorFn: (item) => item.tenant.id },
    {
      id: 'name',
      header: t('Workspace'),
      cell: ({ row }) => (
        <div className='break-words'>
          <WorkspaceLink id={row.original.tenant.id} admin>
            {row.original.tenant.name}
          </WorkspaceLink>
          <p className='text-muted-foreground'>/t/{row.original.tenant.slug}</p>
        </div>
      ),
    },
    {
      accessorKey: 'owner_email',
      header: t('Owner'),
      cell: ({ row }) => (
        <span className='break-all'>
          {row.original.owner_email} (#
          {row.original.tenant.owner_platform_user_id})
        </span>
      ),
    },
    {
      id: 'plan',
      header: t('Hosting plan'),
      cell: ({ row }) =>
        plans.data?.find((plan) => plan.id === row.original.tenant.plan_id)
          ?.name,
    },
    {
      id: 'usage',
      header: t('Monthly requests'),
      cell: ({ row }) => {
        const limit =
          plans.data?.find((plan) => plan.id === row.original.tenant.plan_id)
            ?.limits.requests
        return `${row.original.usage.requests.toLocaleString()} / ${
          limit === 0 ? t('Unlimited') : (limit?.toLocaleString() ?? '—')
        }`
      },
    },
    {
      id: 'expiry',
      header: t('Plan expires'),
      cell: ({ row }) =>
        row.original.tenant.plan_expires_at
          ? new Date(row.original.tenant.plan_expires_at).toLocaleString()
          : t('No expiry'),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        if (row.original.tenant.status === 'suspended') return t('Suspended')
        if (
          row.original.tenant.plan_expires_at &&
          new Date(row.original.tenant.plan_expires_at).getTime() <= Date.now()
        ) {
          return t('Expired')
        }
        return t('Active')
      },
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => (
        <DataTableRowActionMenu
          ariaLabel={t('Actions for {{name}}', {
            name: row.original.tenant.name,
          })}
        >
          <DropdownMenuItem
            onClick={() => {
              window.location.assign(`/t/${row.original.tenant.slug}/`)
            }}
          >
            {t('Enter workspace')}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!plans.data}
            onClick={() => setAssign(row.original.tenant)}
          >
            {t('Activate plan manually')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              mutation.reset()
              setStatus(row.original.tenant)
            }}
          >
            {row.original.tenant.status === 'active'
              ? t('Suspend workspace')
              : t('Enable workspace')}
          </DropdownMenuItem>
        </DataTableRowActionMenu>
      ),
    },
  ]
  return (
    <>
      <PlatformTable
        resource='workspaces'
        columns={columns}
        load={async (params) => {
          const data = await getWorkspaces(true, params)
          return { ...data, items: data.tenants }
        }}
        searchPlaceholder={t('Search workspaces')}
        statuses={[
          { value: 'active', label: t('Active') },
          { value: 'suspended', label: t('Suspended') },
          { value: 'expired', label: t('Expired') },
        ]}
      />
      {assign && (
        <AssignPlanDialog
          workspace={assign}
          plans={plans.data ?? []}
          onClose={() => setAssign(null)}
        />
      )}
      {status && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setStatus(null)
          }}
          title={
            status.status === 'active'
              ? t('Suspend workspace')
              : t('Enable workspace')
          }
          desc={
            <>
              <p className='font-medium break-words'>{status.name}</p>
              <p>
                {t(
                  'Suspended workspaces cannot access the gateway or their dashboard.'
                )}
              </p>
            </>
          }
          destructive={status.status === 'active'}
          handleConfirm={() => mutation.mutate(status)}
          isLoading={mutation.isPending}
        >
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
        </ConfirmDialog>
      )}
    </>
  )
}
