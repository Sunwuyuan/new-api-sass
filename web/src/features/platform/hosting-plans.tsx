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
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from '@/components/ui/card'

import { getHostingPlans } from './api'
import type { HostingPlan } from './types'

function included(t: (key: string) => string, on: boolean | undefined) {
  return on ? t('Included') : t('Not included')
}

function limitItems(plan: HostingPlan, t: (key: string) => string) {
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
      label: t('Redemption code'),
      value:
        plan.name === 'Lite' ? t('Not required') : t('Required'),
    },
  ]
}

function capabilityItems(plan: HostingPlan, t: (key: string) => string) {
  return [
    {
      label: t('Branding'),
      value: plan.capabilities.custom_branding
        ? t('Custom branding')
        : t('Basic branding'),
      on: plan.capabilities.custom_branding,
    },
    {
      label: t('Platform footer'),
      value: plan.capabilities.remove_platform_footer
        ? t('Custom platform footer')
        : t('Platform footer required'),
      on: plan.capabilities.remove_platform_footer,
    },
    {
      label: t('Platform email'),
      value: included(t, plan.capabilities.platform_email),
      on: plan.capabilities.platform_email,
    },
    {
      label: t('Image, video and task plugins'),
      value: included(t, plan.capabilities.task_plugins),
      on: plan.capabilities.task_plugins,
    },
    {
      label: t('Workspace sign-in providers'),
      value: included(t, plan.capabilities.workspace_oauth),
      on: plan.capabilities.workspace_oauth,
    },
    {
      label: t('Wallet top-up'),
      value: included(t, plan.capabilities.topup),
      on: plan.capabilities.topup,
    },
    {
      label: t('Affiliate rewards'),
      value: included(t, plan.capabilities.affiliate),
      on: plan.capabilities.affiliate,
    },
    {
      label: 'Passkey',
      value: included(t, plan.capabilities.passkey),
      on: plan.capabilities.passkey,
    },
    {
      label: t('Custom model pricing'),
      value: included(t, plan.capabilities.custom_models),
      on: plan.capabilities.custom_models,
    },
  ]
}

export function HostingPlans() {
  const { t } = useTranslation()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  if (plans.isPending) return <LoadingState />
  if (plans.isError) return <ErrorState onRetry={() => void plans.refetch()} />
  const ordered = [...plans.data].sort((a, b) => {
    const left = a.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : a.limits.requests
    const right = b.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : b.limits.requests
    return left - right
  })
  return (
    <section
      aria-label={t('Hosting plans')}
      className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'
    >
      {ordered.map((plan) => (
          <Card key={plan.id}>
            <CardHeader className='gap-2'>
              <h2 className='text-xl font-medium'>{plan.name}</h2>
              <CardDescription>{t(plan.price)}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-5'>
              <dl className='grid grid-cols-2 gap-3'>
                {limitItems(plan, t).map((item) => (
                  <div key={item.label}>
                    <dt className='text-muted-foreground text-xs'>{item.label}</dt>
                    <dd className='mt-1 text-base font-semibold tabular-nums'>
                      {item.value}
                    </dd>
                  </div>
                ))}
              </dl>
              <ul className='space-y-2 text-sm'>
                {capabilityItems(plan, t).map((item) => (
                  <li
                    key={item.label}
                    className={item.on ? undefined : 'text-muted-foreground'}
                  >
                    {item.label}
                    {item.value !== t('Included')
                      ? ` · ${item.value}`
                      : null}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
      ))}
    </section>
  )
}
