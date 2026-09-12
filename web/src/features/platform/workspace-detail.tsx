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
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  Field,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { AssignPlanDialog } from './admin/assign-plan-dialog'
import {
  getHostingPlans,
  platformSessionQuery,
  setWorkspaceStatus,
  transferWorkspace,
  updateWorkspace,
} from './api'
import { PlatformLink, PlatformNotFound } from './navigation'
import { RedeemPlanDialog } from './redeem-plan-dialog'
import type { PlatformRouter } from './router'
import { getWorkspaceDetail } from './workspace-api'
import { WorkspaceStatus, WorkspaceUsageMeter } from './workspace-status'

export default function PlatformWorkspaceDetail() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const params = useParams({ strict: false }) as { workspaceId: string }
  const id = Number(params.workspaceId)
  const admin = useRouterState({
    select: (state) => state.location.pathname.startsWith('/platform/admin/'),
  })
  const session = useQuery(platformSessionQuery)
  const detail = useQuery({
    queryKey: ['platform', 'workspace', id, admin],
    queryFn: () => getWorkspaceDetail(id, admin),
    enabled: Number.isSafeInteger(id) && id > 0,
  })
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const [redeemOpen, setRedeemOpen] = useState(false)
  const [assignOpen, setAssignOpen] = useState(false)
  const [statusOpen, setStatusOpen] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [transferOpen, setTransferOpen] = useState(false)
  const [ownerEmail, setOwnerEmail] = useState('')
  const [tab, setTab] = useState('overview')
  useEffect(() => {
    if (!detail.data) return
    setName(detail.data.tenant.name)
    setSlug(detail.data.tenant.slug)
  }, [detail.data])
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: () =>
      setWorkspaceStatus(
        id,
        detail.data?.tenant.status === 'active' ? 'suspended' : 'active'
      ),
    onSuccess: async () => {
      setStatusOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const rename = useMutation({
    meta: { errorToast: false },
    mutationFn: () => updateWorkspace(id, { name, slug }, admin),
    onSuccess: async () => {
      toast.success(t('Workspace updated'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const transfer = useMutation({
    meta: { errorToast: false },
    mutationFn: () => transferWorkspace(id, ownerEmail),
    onSuccess: async () => {
      setTransferOpen(false)
      setOwnerEmail('')
      toast.success(t('Workspace transferred'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  if (!Number.isSafeInteger(id) || id <= 0) return <PlatformNotFound />
  if (detail.isPending || plans.isPending) return <LoadingState />
  if (detail.isError || plans.isError) {
    return (
      <ErrorState
        title={t('Workspace unavailable')}
        description={detail.error?.message ?? plans.error?.message}
        onRetry={() => {
          void detail.refetch()
          void plans.refetch()
        }}
      />
    )
  }
  const data = detail.data
  const workspace = data.tenant
  const owner = workspace.owner_platform_user_id === session.data?.user.id
  const suspended = workspace.status === 'suspended'
  const expired =
    !!workspace.plan_expires_at &&
    new Date(workspace.plan_expires_at).getTime() <= Date.now()
  const remaining =
    data.plan.limits.requests === 0
      ? null
      : Math.max(0, data.plan.limits.requests - data.usage.requests)
  return (
    <section className='space-y-6'>
      <PlatformLink
        to={admin ? '/platform/admin/workspaces' : '/platform'}
        className='text-muted-foreground text-sm hover:underline'
      >
        {admin ? t('All workspaces') : t('My workspaces')}
      </PlatformLink>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div className='min-w-0 space-y-2'>
          <div className='flex flex-wrap items-center gap-3'>
            <h1 className='text-2xl font-semibold break-words'>
              {workspace.name}
            </h1>
            <WorkspaceStatus workspace={workspace} />
          </div>
          <p className='text-muted-foreground text-sm break-all'>
            /t/{workspace.slug}
          </p>
          {expired && !suspended && (
            <p className='text-muted-foreground max-w-2xl text-sm'>
              {t(
                'This workspace now uses Lite. Existing users can keep working. Renew the plan to create tokens, channels and more users.'
              )}
            </p>
          )}
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            render={<a href={`/t/${workspace.slug}/`} />}
            nativeButton={false}
            role='link'
            disabled={suspended}
          >
            {t('Enter workspace')}
          </Button>
          {!data.setup_complete && (
            <Button
              variant='outline'
              render={<a href={`/t/${workspace.slug}/setup`} />}
              nativeButton={false}
              role='link'
              disabled={suspended}
            >
              {t('Finish setup')}
            </Button>
          )}
        </div>
      </header>
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList variant='line'>
          <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
          <TabsTrigger value='settings'>{t('Settings')}</TabsTrigger>
          <TabsTrigger value='usage'>{t('Usage')}</TabsTrigger>
        </TabsList>
        <TabsContent value='overview' className='space-y-4 pt-4'>
      <div className='grid gap-4 lg:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle>{t('Hosting plan')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-5'>
            <p className='text-2xl font-semibold'>{data.plan.name}</p>
            <dl className='grid grid-cols-2 gap-3 text-sm'>
              <dt className='text-muted-foreground'>{t('Owner')}</dt>
              <dd className='break-all'>{data.owner_name}</dd>
              <dt className='text-muted-foreground'>{t('Plan expires')}</dt>
              <dd>
                {workspace.plan_expires_at
                  ? new Date(workspace.plan_expires_at).toLocaleString()
                  : t('No expiry')}
              </dd>
              <dt className='text-muted-foreground'>{t('Users')}</dt>
              <dd>
                {data.plan.limits.users === 0
                  ? t('Unlimited')
                  : data.plan.limits.users.toLocaleString()}
              </dd>
              <dt className='text-muted-foreground'>{t('Monthly requests')}</dt>
              <dd>
                {data.plan.limits.requests === 0
                  ? t('Unlimited')
                  : data.plan.limits.requests.toLocaleString()}
              </dd>
            </dl>
            <p className='text-muted-foreground text-sm'>
              {data.plan.capabilities.custom_branding
                ? t('Custom branding')
                : t('Basic branding')}{' '}
              ·{' '}
              {data.plan.capabilities.remove_platform_footer
                ? t('Custom platform footer')
                : t('Platform footer required')}
            </p>
            <div className='flex flex-wrap gap-2'>
              {owner && (
                <Button
                  variant='outline'
                  disabled={workspace.status === 'suspended'}
                  onClick={() => setRedeemOpen(true)}
                >
                  {t('Redeem hosting plan')}
                </Button>
              )}
              {admin && (
                <Button
                  variant='outline'
                  disabled={!plans.data}
                  onClick={() => setAssignOpen(true)}
                >
                  {t('Activate plan manually')}
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>
              {t('Monthly requests')} · {data.usage.month} UTC
            </CardTitle>
          </CardHeader>
          <CardContent className='space-y-5'>
            <WorkspaceUsageMeter
              requests={data.usage.requests}
              limit={data.plan.limits.requests}
              name={workspace.name}
            />
            <p className='text-sm'>
              {t('Remaining requests')}:{' '}
              <strong className='tabular-nums'>
                {remaining === null ? t('Unlimited') : remaining.toLocaleString()}
              </strong>
            </p>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Usage is counted from gateway requests in the UTC calendar month.'
              )}
            </p>
          </CardContent>
        </Card>
      </div>
        </TabsContent>
        <TabsContent value='settings' className='space-y-4 pt-4'>
        <Card className='max-w-xl'>
          <CardHeader>
            <CardTitle>{t('Workspace')}</CardTitle>
          </CardHeader>
          <CardContent>
            <form
              className='space-y-4'
              onSubmit={(event) => {
                event.preventDefault()
                rename.mutate()
              }}
            >
              <Field>
                <FieldLabel htmlFor='workspace-display-name'>
                  {t('Name')}
                </FieldLabel>
                <Input
                  id='workspace-display-name'
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={rename.isPending}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='workspace-slug'>
                  {t('Workspace address')}
                </FieldLabel>
                <Input
                  id='workspace-slug'
                  value={slug}
                  onChange={(event) => setSlug(event.target.value)}
                  disabled={rename.isPending}
                />
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Changing the address updates the /t/ path. Existing bookmarks need the new URL.'
                  )}
                </p>
                <FieldError>{rename.error?.message}</FieldError>
              </Field>
              <Button
                type='submit'
                disabled={rename.isPending || !name.trim() || !slug.trim()}
              >
                {t('Save changes')}
              </Button>
            </form>
            <Button
              className='mt-6'
              variant='outline'
              render={
                <Link<
                  PlatformRouter,
                  string,
                  | '/platform/workspaces/$workspaceId/administrators'
                  | '/platform/admin/workspaces/$workspaceId/administrators'
                >
                  to={
                    admin
                      ? '/platform/admin/workspaces/$workspaceId/administrators'
                      : '/platform/workspaces/$workspaceId/administrators'
                  }
                  params={{ workspaceId: String(id) }}
                />
              }
              nativeButton={false}
            >
              {t('Workspace administrators')}
            </Button>
          </CardContent>
        </Card>
        </TabsContent>
        <TabsContent value='usage' className='space-y-6 pt-4'>
      <section className='space-y-3'>
        <h2 className='text-base font-semibold'>
          {t('Monthly request history')}
        </h2>
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
            {
              id: 'emails',
              header: t('Emails'),
              cell: (row) => (row.emails ?? 0).toLocaleString(),
            },
          ]}
        />
      </section>
      <section className='space-y-3'>
        <h2 className='text-base font-semibold'>{t('Plan history')}</h2>
        <StaticDataTable
          data={data.assignments ?? []}
          getRowKey={(row) => row.id}
          emptyContent={t('No plan changes yet')}
          columns={[
            {
              id: 'plan',
              header: t('Hosting plan'),
              cell: (row) =>
                plans.data?.find((plan) => plan.id === row.plan_id)?.name ??
                String(row.plan_id),
            },
            {
              id: 'source',
              header: t('Source'),
              cell: (row) =>
                row.source === 'redeem'
                  ? t('Redemption code')
                  : t('Manual activation'),
            },
            {
              id: 'created',
              header: t('Created at'),
              cell: (row) => new Date(row.created_at).toLocaleString(),
            },
            {
              id: 'expires',
              header: t('Plan expires'),
              cell: (row) => new Date(row.expires_at).toLocaleString(),
            },
          ]}
        />
      </section>
        </TabsContent>
      </Tabs>
      {admin && (
        <div className='flex flex-wrap justify-end gap-2 border-t pt-5'>
          <Button variant='outline' onClick={() => setTransferOpen(true)}>
            {t('Transfer ownership')}
          </Button>
          <Button
            variant='outline'
            onClick={() => {
              mutation.reset()
              setStatusOpen(true)
            }}
          >
            {workspace.status === 'active'
              ? t('Suspend workspace')
              : t('Enable workspace')}
          </Button>
        </div>
      )}
      {redeemOpen && (
        <RedeemPlanDialog
          workspaces={[workspace]}
          onClose={() => setRedeemOpen(false)}
        />
      )}
      {assignOpen && (
        <AssignPlanDialog
          workspace={workspace}
          plans={plans.data ?? []}
          onClose={() => setAssignOpen(false)}
        />
      )}
      {statusOpen && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setStatusOpen(false)
          }}
          title={
            workspace.status === 'active'
              ? t('Suspend workspace')
              : t('Enable workspace')
          }
          desc={workspace.name}
          destructive={workspace.status === 'active'}
          handleConfirm={() => mutation.mutate()}
          isLoading={mutation.isPending}
        >
          {mutation.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {mutation.error.message}
            </p>
          )}
        </ConfirmDialog>
      )}
      {transferOpen && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !transfer.isPending) setTransferOpen(false)
          }}
          title={t('Transfer ownership')}
          desc={t(
            'The new owner must already have a platform account and unused workspace capacity.'
          )}
          confirmText={t('Transfer ownership')}
          handleConfirm={() => transfer.mutate()}
          isLoading={transfer.isPending}
        >
          <Field>
            <FieldLabel htmlFor='workspace-new-owner'>
              {t('New owner email')}
            </FieldLabel>
            <Input
              id='workspace-new-owner'
              type='email'
              value={ownerEmail}
              autoComplete='off'
              onChange={(event) => setOwnerEmail(event.target.value)}
              disabled={transfer.isPending}
            />
          </Field>
          {transfer.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {transfer.error.message}
            </p>
          )}
        </ConfirmDialog>
      )}
    </section>
  )
}
