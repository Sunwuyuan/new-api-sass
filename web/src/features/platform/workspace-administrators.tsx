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
import { Link, useParams, useRouterState } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { updateWorkspaceAdministrator } from './api'
import { PlatformNotFound } from './navigation'
import type { PlatformRouter } from './router'
import {
  getWorkspaceDetail,
  type WorkspaceAdministrator,
} from './workspace-api'

export default function WorkspaceAdministratorsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const params = useParams({ strict: false }) as { workspaceId: string }
  const id = Number(params.workspaceId)
  const admin = useRouterState({
    select: (state) => state.location.pathname.startsWith('/platform/admin/'),
  })
  const detail = useQuery({
    queryKey: ['platform', 'workspace', id, admin],
    queryFn: () => getWorkspaceDetail(id, admin),
    enabled: Number.isSafeInteger(id) && id > 0,
  })
  const [editing, setEditing] = useState<WorkspaceAdministrator | null>(null)
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  useEffect(() => {
    if (!editing) return
    setUsername(editing.username)
    setDisplayName(editing.display_name ?? '')
    setEmail(editing.email ?? '')
    setPassword('')
  }, [editing])
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: () =>
      updateWorkspaceAdministrator(
        id,
        {
          id: editing?.id,
          username: username || undefined,
          display_name: displayName || undefined,
          email: email || undefined,
          password: password || undefined,
        },
        admin
      ),
    onSuccess: async () => {
      setEditing(null)
      setPassword('')
      toast.success(t('Workspace administrator updated'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  if (!Number.isSafeInteger(id) || id <= 0) return <PlatformNotFound />
  if (detail.isPending) return <LoadingState />
  if (detail.isError) {
    return (
      <ErrorState
        title={t('Workspace unavailable')}
        description={detail.error.message}
        onRetry={() => void detail.refetch()}
      />
    )
  }
  const workspace = detail.data.tenant
  const administrators = detail.data.administrators ?? []
  return (
    <section className='space-y-6'>
      <Link<
        PlatformRouter,
        string,
        | '/platform/workspaces/$workspaceId'
        | '/platform/admin/workspaces/$workspaceId'
      >
        to={
          admin
            ? '/platform/admin/workspaces/$workspaceId'
            : '/platform/workspaces/$workspaceId'
        }
        params={{ workspaceId: String(id) }}
        className='text-muted-foreground text-sm hover:underline'
      >
        {workspace.name}
      </Link>
      <header className='space-y-1'>
        <h1 className='text-2xl font-semibold'>
          {t('Workspace administrators')}
        </h1>
        <p className='text-muted-foreground text-sm'>
          {t('Edit the administrator accounts for this workspace.')}
        </p>
      </header>
      <StaticDataTable
        data={administrators}
        getRowKey={(row) => row.id}
        emptyContent={t('No administrators yet')}
        columns={[
          { id: 'username', header: t('Username'), cell: (row) => row.username },
          {
            id: 'display',
            header: t('Display Name'),
            cell: (row) => row.display_name || '—',
          },
          {
            id: 'email',
            header: t('Email'),
            cell: (row) => row.email || '—',
          },
          {
            id: 'role',
            header: t('Role'),
            cell: (row) =>
              row.role === 'root' ? t('Root administrator') : t('Administrator'),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (row) =>
              row.status === 1 ? t('Enabled') : t('Disabled'),
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (row) => (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => setEditing(row)}
              >
                {t('Edit')}
              </Button>
            ),
          },
        ]}
      />
      {editing && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setEditing(null)
          }}
          title={t('Edit administrator')}
          desc={editing.username}
          confirmText={t('Save changes')}
          handleConfirm={() => mutation.mutate()}
          isLoading={mutation.isPending}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='workspace-admin-username'>
                {t('Username')}
              </FieldLabel>
              <Input
                id='workspace-admin-username'
                value={username}
                autoComplete='off'
                onChange={(event) => setUsername(event.target.value)}
                disabled={mutation.isPending}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='workspace-admin-display'>
                {t('Display Name')}
              </FieldLabel>
              <Input
                id='workspace-admin-display'
                value={displayName}
                autoComplete='off'
                onChange={(event) => setDisplayName(event.target.value)}
                disabled={mutation.isPending}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='workspace-admin-email'>
                {t('Email')}
              </FieldLabel>
              <Input
                id='workspace-admin-email'
                type='email'
                value={email}
                autoComplete='off'
                onChange={(event) => setEmail(event.target.value)}
                disabled={mutation.isPending}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='workspace-admin-password'>
                {t('New password')}
              </FieldLabel>
              <PasswordInput
                id='workspace-admin-password'
                autoComplete='new-password'
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={mutation.isPending}
              />
            </Field>
            {mutation.isError && (
              <p role='alert' className='text-destructive text-sm'>
                {mutation.error.message}
              </p>
            )}
          </FieldGroup>
        </ConfirmDialog>
      )}
    </section>
  )
}
