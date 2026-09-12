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
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'

import {
  disablePlatformRedemption,
  getHostingPlans,
  getPlatformRedemptions,
  getRedemptionUses,
} from '../api'
import type { PlatformRedemption, RedemptionUse } from '../types'
import { CreateCodesDialog } from './create-codes-dialog'
import { PlatformTable } from './table'

export default function PlatformRedemptions() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const [create, setCreate] = useState(false)
  const [disable, setDisable] = useState<PlatformRedemption | null>(null)
  const [history, setHistory] = useState<PlatformRedemption | null>(null)
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: disablePlatformRedemption,
    onSuccess: () => {
      setDisable(null)
      void queryClient.invalidateQueries({
        queryKey: ['platform', 'admin', 'redemptions'],
      })
      toast.success(t('Redemption code disabled'))
    },
  })
  const columns: ColumnDef<PlatformRedemption>[] = [
    { accessorKey: 'id', header: t('ID') },
    {
      accessorKey: 'code_hint',
      header: t('Code suffix'),
      cell: ({ row }) => <code>…{row.original.code_hint}</code>,
    },
    {
      id: 'plan',
      header: t('Hosting plan'),
      cell: ({ row }) =>
        plans.data?.find((plan) => plan.id === row.original.plan_id)?.name,
    },
    { accessorKey: 'duration_months', header: t('Months') },
    {
      id: 'uses',
      header: t('Uses'),
      cell: ({ row }) => `${row.original.used} / ${row.original.max_uses}`,
    },
    {
      id: 'expiry',
      header: t('Expires at'),
      cell: ({ row }) =>
        row.original.expires_at
          ? new Date(row.original.expires_at).toLocaleString()
          : t('No expiry'),
    },
    { accessorKey: 'created_by', header: t('Created by') },
    {
      id: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        if (row.original.status === 'disabled') return t('Disabled')
        if (row.original.used >= row.original.max_uses) return t('Exhausted')
        if (
          row.original.expires_at &&
          new Date(row.original.expires_at).getTime() <= Date.now()
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
            name: row.original.code_hint,
          })}
        >
          <DropdownMenuItem onClick={() => setHistory(row.original)}>
            {t('Usage history')}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={row.original.status === 'disabled'}
            onClick={() => {
              mutation.reset()
              setDisable(row.original)
            }}
          >
            {t('Disable code')}
          </DropdownMenuItem>
        </DataTableRowActionMenu>
      ),
    },
  ]
  const useColumns: ColumnDef<RedemptionUse>[] = [
    { accessorKey: 'tenant_id', header: t('Workspace ID') },
    { accessorKey: 'platform_user_id', header: t('User ID') },
    { accessorKey: 'assignment_id', header: t('Assignment ID') },
    {
      id: 'created_at',
      header: t('Redeemed at'),
      cell: ({ row }) => new Date(row.original.created_at).toLocaleString(),
    },
  ]
  return (
    <>
      <PlatformTable
        resource='redemptions'
        columns={columns}
        load={async (params) => {
          const data = await getPlatformRedemptions(params)
          return { ...data, items: data.redemptions }
        }}
        searchPlaceholder={t('Search by code suffix')}
        statuses={[
          { value: 'active', label: t('Active') },
          { value: 'disabled', label: t('Disabled') },
          { value: 'exhausted', label: t('Exhausted') },
          { value: 'expired', label: t('Expired') },
        ]}
        actions={
          <Button disabled={!plans.data} onClick={() => setCreate(true)}>
            {t('Generate platform codes')}
          </Button>
        }
      />
      {create && (
        <CreateCodesDialog
          plans={plans.data ?? []}
          onClose={() => setCreate(false)}
        />
      )}
      {disable && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setDisable(null)
          }}
          title={t('Disable code')}
          desc={
            <>
              <p>…{disable.code_hint}</p>
              <p>
                {t(
                  'This code will no longer activate plans. Existing activations remain valid.'
                )}
              </p>
            </>
          }
          destructive
          handleConfirm={() => mutation.mutate(disable.id)}
          isLoading={mutation.isPending}
        >
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
        </ConfirmDialog>
      )}
      {history && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setHistory(null)
          }}
          title={t('Usage history')}
          description={`…${history.code_hint}`}
          contentClassName='sm:max-w-4xl'
        >
          <PlatformTable
            key={history.id}
            resource={`redemption-uses-${history.id}`}
            searchable={false}
            columns={useColumns}
            load={async (params) => {
              const data = await getRedemptionUses(history.id, params)
              return { ...data, items: data.uses }
            }}
          />
        </Dialog>
      )}
    </>
  )
}
