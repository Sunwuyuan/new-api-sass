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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableRowActionMenu } from '@/components/data-table'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'

import { getPlatformUsers, updatePlatformUser } from '../api'
import type { PlatformUser, UserAction } from '../types'
import { PlatformTable } from './table'

export default function PlatformUsers() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<{
    user: PlatformUser
    action: UserAction
    label: string
  } | null>(null)
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (selection: NonNullable<typeof selected>) =>
      updatePlatformUser(selection.user.id, selection.action),
    onSuccess: () => {
      setSelected(null)
      void queryClient.invalidateQueries({ queryKey: ['platform'] })
      toast.success(t('Platform user updated'))
    },
  })
  const columns: ColumnDef<PlatformUser>[] = [
    { accessorKey: 'id', header: t('ID') },
    {
      accessorKey: 'email',
      header: t('Email'),
      cell: ({ row }) => (
        <span className='break-all'>{row.original.email}</span>
      ),
    },
    {
      accessorKey: 'role',
      header: t('Role'),
      cell: ({ row }) => {
        if (row.original.role === 'root') return t('Root administrator')
        return row.original.role === 'user' ? t('User') : t('Administrator')
      },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) =>
        row.original.status === 'active' ? t('Active') : t('Disabled'),
    },
    { accessorKey: 'tenant_count', header: t('Workspaces') },
    {
      accessorKey: 'must_change_password',
      header: t('Password change required'),
      cell: ({ row }) =>
        row.original.must_change_password ? t('Yes') : t('No'),
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => {
        const user = row.original
        const actions: { action: UserAction; label: string }[] = [
          {
            action: user.status === 'active' ? 'disable' : 'enable',
            label:
              user.status === 'active' ? t('Disable user') : t('Enable user'),
          },
          {
            action: user.role === 'user' ? 'promote' : 'demote',
            label:
              user.role === 'user'
                ? t('Make platform administrator')
                : t('Remove administrator role'),
          },
          {
            action: 'require_password_change',
            label: t('Require password change'),
          },
          { action: 'revoke_sessions', label: t('Revoke platform sessions') },
        ]
        return (
          <DataTableRowActionMenu
            ariaLabel={t('Actions for {{name}}', { name: user.email })}
          >
            {actions.map((action) => (
              <DropdownMenuItem
                key={action.action}
                onClick={() => {
                  mutation.reset()
                  setSelected({ user, ...action })
                }}
              >
                {action.label}
              </DropdownMenuItem>
            ))}
          </DataTableRowActionMenu>
        )
      },
    },
  ]
  return (
    <>
      <PlatformTable
        resource='users'
        columns={columns}
        load={async (params) => {
          const data = await getPlatformUsers(params)
          return { ...data, items: data.users }
        }}
        searchPlaceholder={t('Search by email')}
        statuses={[
          { value: 'active', label: t('Active') },
          { value: 'disabled', label: t('Disabled') },
        ]}
        roles={[
          { value: 'user', label: t('User') },
          { value: 'admin', label: t('Administrator') },
          { value: 'root', label: t('Root administrator') },
        ]}
      />
      {selected && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setSelected(null)
          }}
          title={selected.label}
          desc={
            <>
              <p className='font-medium break-all'>{selected.user.email}</p>
              <p>
                {t(
                  'This change revokes all platform sessions for this account. At least one enabled administrator must remain.'
                )}
              </p>
            </>
          }
          destructive
          handleConfirm={() => mutation.mutate(selected)}
          isLoading={mutation.isPending}
        >
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
        </ConfirmDialog>
      )}
    </>
  )
}
