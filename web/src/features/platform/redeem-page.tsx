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
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { PlatformTable } from './admin/table'
import { getWorkspaces } from './api'
import { WorkspaceLink } from './navigation'
import { RedeemPlanDialog } from './redeem-plan-dialog'
import type { Workspace, WorkspaceUsage } from './types'
import { WorkspaceStatus } from './workspace-status'

export default function PlatformRedeemPage() {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<Workspace | null>(null)
  const columns: ColumnDef<WorkspaceUsage>[] = [
    {
      id: 'name',
      header: t('Workspace'),
      cell: ({ row }) => (
        <WorkspaceLink id={row.original.tenant.id}>
          {row.original.tenant.name}
        </WorkspaceLink>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: ({ row }) => <WorkspaceStatus workspace={row.original.tenant} />,
    },
    {
      id: 'expires',
      header: t('Plan expires'),
      cell: ({ row }) =>
        row.original.tenant.plan_expires_at
          ? new Date(row.original.tenant.plan_expires_at).toLocaleDateString()
          : t('No expiry'),
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => (
        <Button
          variant='outline'
          size='sm'
          disabled={row.original.tenant.status === 'suspended'}
          onClick={() => setSelected(row.original.tenant)}
        >
          {t('Redeem hosting plan')}
        </Button>
      ),
    },
  ]
  return (
    <section className='space-y-6'>
      <header>
        <h1 className='text-2xl font-semibold'>{t('Redeem hosting plan')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Select a workspace to activate or renew its hosting plan.')}
        </p>
      </header>
      <PlatformTable
        scope='user'
        resource='redeem-workspaces'
        columns={columns}
        load={async (params) => {
          const data = await getWorkspaces(false, params)
          return { ...data, items: data.tenants }
        }}
      />
      {selected && (
        <RedeemPlanDialog
          workspace={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </section>
  )
}
