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
import {
  MinusSignIcon,
  Tick02Icon,
  Ticket02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

import { getHostingPlans } from './api'
import { PlatformLink } from './navigation'
import type { HostingPlan, Workspace } from './types'

function quotaItems(plan: HostingPlan, t: (key: string) => string) {
  return [
    {
      label: t('Monthly requests'),
      value:
        plan.limits.requests === 0
          ? t('Unlimited')
          : plan.limits.requests.toLocaleString(),
    },
    {
      label: t('Users'),
      value:
        plan.limits.users === 0
          ? t('Unlimited')
          : plan.limits.users.toLocaleString(),
    },
    {
      label: t('Monthly platform emails'),
      value:
        (plan.limits.emails ?? 0) === 0
          ? t('Unlimited')
          : (plan.limits.emails ?? 0).toLocaleString(),
    },
    {
      label: t('Workspaces'),
      value: plan.capabilities.max_workspaces.toLocaleString(),
    },
  ]
}

function capabilityItems(plan: HostingPlan, t: (key: string) => string) {
  return [
    { label: t('Custom branding'), on: plan.capabilities.custom_branding },
    {
      label: t('No platform footer'),
      on: plan.capabilities.remove_platform_footer,
    },
    { label: t('Platform email'), on: plan.capabilities.platform_email },
    {
      label: t('Image, video and task plugins'),
      on: plan.capabilities.task_plugins,
    },
    {
      label: t('Workspace sign-in providers'),
      on: plan.capabilities.workspace_oauth,
    },
    { label: t('Wallet top-up'), on: plan.capabilities.topup },
    { label: t('Affiliate rewards'), on: plan.capabilities.affiliate },
    { label: 'Passkey', on: plan.capabilities.passkey },
    { label: t('Custom model pricing'), on: plan.capabilities.custom_models },
  ]
}

export function HostingPlans(props: {
  workspaces: Workspace[]
  onRedeem: () => void
}) {
  const { t } = useTranslation()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  if (plans.isPending) return <LoadingState />
  if (plans.isError) return <ErrorState onRetry={() => void plans.refetch()} />
  const now = Date.now()
  const currentPlanIds = new Set(
    props.workspaces
      .filter(
        (workspace) =>
          workspace.status === 'active' &&
          (!workspace.plan_expires_at ||
            Date.parse(workspace.plan_expires_at) > now)
      )
      .map((workspace) => workspace.plan_id)
  )
  const ordered = [...plans.data].sort((a, b) => {
    const left =
      a.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : a.limits.requests
    const right =
      b.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : b.limits.requests
    return left - right
  })
  return (
    <section
      aria-label={t('Hosting plans')}
      className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'
    >
      {ordered.map((plan) => {
        const current = currentPlanIds.has(plan.id)
        return (
          <Card key={plan.id} className='h-full'>
            <CardHeader>
              <CardTitle className='text-lg'>{plan.name}</CardTitle>
              {current && (
                <CardAction>
                  <Badge variant='secondary'>{t('Current')}</Badge>
                </CardAction>
              )}
              <p className='text-2xl font-semibold tracking-tight'>
                {t(plan.price)}
              </p>
            </CardHeader>
            <CardContent className='flex flex-1 flex-col gap-4'>
              <dl className='space-y-2 text-sm'>
                {quotaItems(plan, t).map((item) => (
                  <div
                    key={item.label}
                    className='flex items-center justify-between gap-2'
                  >
                    <dt className='text-muted-foreground'>{item.label}</dt>
                    <dd className='font-medium tabular-nums'>{item.value}</dd>
                  </div>
                ))}
              </dl>
              <Separator />
              <ul className='space-y-2 text-sm'>
                {capabilityItems(plan, t).map((item) => (
                  <li
                    key={item.label}
                    className={cn(
                      'flex items-center gap-2',
                      !item.on && 'text-muted-foreground'
                    )}
                  >
                    <HugeiconsIcon
                      icon={item.on ? Tick02Icon : MinusSignIcon}
                      className={cn(
                        'size-4 shrink-0',
                        item.on ? 'text-primary' : 'text-muted-foreground/50'
                      )}
                      aria-hidden='true'
                    />
                    {item.label}
                  </li>
                ))}
              </ul>
            </CardContent>
            <CardFooter className='mt-auto'>
              {plan.name === 'Lite' ? (
                <PlatformLink
                  to='/platform/workspaces/new'
                  className={cn(
                    buttonVariants({ variant: 'outline' }),
                    'w-full'
                  )}
                >
                  {t('Create free workspace')}
                </PlatformLink>
              ) : (
                <Button
                  variant='outline'
                  className='w-full'
                  onClick={props.onRedeem}
                >
                  <HugeiconsIcon icon={Ticket02Icon} data-icon='inline-start' />
                  {t('Redeem a code')}
                </Button>
              )}
            </CardFooter>
          </Card>
        )
      })}
    </section>
  )
}
