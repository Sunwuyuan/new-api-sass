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
import { useParams, useRouterState } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { AssignPlanDialog } from './admin/assign-plan-dialog'
import {
  getHostingPlans,
  platformSessionQuery,
  setWorkspaceStatus,
} from './api'
import { PlatformLink, PlatformNotFound } from './navigation'
import { RedeemPlanDialog } from './redeem-plan-dialog'
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
  const inactive =
    workspace.status === 'suspended' ||
    (!!workspace.plan_expires_at &&
      new Date(workspace.plan_expires_at).getTime() <= Date.now())
  const remaining = Math.max(0, data.plan.limits.requests - data.usage.requests)
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
        </div>
        <Button
          render={<a href={`/t/${workspace.slug}/`} />}
          nativeButton={false}
          role='link'
          disabled={inactive}
        >
          {t('Enter workspace')}
        </Button>
      </header>
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
              <dd>{data.plan.limits.users.toLocaleString()}</dd>
              <dt className='text-muted-foreground'>{t('Tokens')}</dt>
              <dd>{data.plan.limits.tokens.toLocaleString()}</dd>
              <dt className='text-muted-foreground'>{t('Channels')}</dt>
              <dd>{data.plan.limits.channels.toLocaleString()}</dd>
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
                {remaining.toLocaleString()}
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
      {admin && (
        <div className='flex justify-end border-t pt-5'>
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
          workspace={workspace}
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
    </section>
  )
}
