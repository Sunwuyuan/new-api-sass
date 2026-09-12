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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getHostingPlans, updateHostingPlan } from '../api'
import type { HostingPlan } from '../types'

type CapabilityKey = Exclude<
  keyof HostingPlan['capabilities'],
  'max_workspaces' | 'data_export'
>

function sortPlans(plans: HostingPlan[]) {
  return [...plans].sort((a, b) => {
    const left = a.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : a.limits.requests
    const right = b.limits.requests === 0 ? Number.MAX_SAFE_INTEGER : b.limits.requests
    return left - right
  })
}

function payload(plan: HostingPlan): HostingPlan {
  return {
    ...plan,
    limits: {
      requests: plan.limits.requests,
      users: plan.limits.users,
      tokens: 0,
      channels: 0,
      emails: 0,
    },
    capabilities: { ...plan.capabilities, data_export: true },
  }
}

function samePlan(left: HostingPlan, right: HostingPlan) {
  return JSON.stringify(payload(left)) === JSON.stringify(payload(right))
}

function parseCount(value: string) {
  const count = Number(value)
  return Number.isFinite(count) && count >= 0 ? Math.trunc(count) : 0
}

export default function PlatformPlans() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const [drafts, setDrafts] = useState<HostingPlan[] | null>(null)
  useEffect(() => {
    if (!query.data) return
    setDrafts((current) => current ?? sortPlans(query.data))
  }, [query.data])
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: updateHostingPlan,
    onSuccess: async (_result, plan) => {
      toast.success(t('Hosting plan updated'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
      setDrafts((current) =>
        current?.map((item) => (item.id === plan.id ? plan : item)) ?? current
      )
    },
  })
  const features: { key: CapabilityKey; label: string }[] = [
    { key: 'custom_branding', label: t('Custom branding') },
    {
      key: 'remove_platform_footer',
      label: t('Allow removing the platform footer'),
    },
    { key: 'platform_email', label: t('Platform email') },
    { key: 'task_plugins', label: t('Image, video and task plugins') },
    { key: 'workspace_oauth', label: t('Workspace sign-in providers') },
    { key: 'topup', label: t('Wallet top-up') },
    { key: 'affiliate', label: t('Affiliate rewards') },
    { key: 'passkey', label: 'Passkey' },
    { key: 'custom_models', label: t('Custom model pricing') },
  ]
  const update = (id: number, next: (plan: HostingPlan) => HostingPlan) => {
    setDrafts(
      (current) =>
        current?.map((plan) => (plan.id === id ? next(plan) : plan)) ?? current
    )
  }
  if (query.isPending) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  if (!drafts) return <LoadingState />
  return (
    <section className='space-y-4'>
      <header className='space-y-1'>
        <h1 className='text-2xl font-semibold'>{t('Hosting plans')}</h1>
        <p className='text-muted-foreground text-sm'>
          {t('Plan changes apply immediately to every workspace on that plan.')}{' '}
          {t('Use 0 for unlimited.')}
        </p>
      </header>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='min-w-48' />
            {drafts.map((plan) => (
              <TableHead key={plan.id}>{plan.name}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell className='whitespace-normal'>{t('Price')}</TableCell>
            {drafts.map((plan) => (
              <TableCell key={plan.id}>
                <Input
                  aria-label={`${plan.name}: ${t('Price')}`}
                  value={plan.price}
                  onChange={(event) =>
                    update(plan.id, (item) => ({
                      ...item,
                      price: event.target.value,
                    }))
                  }
                  disabled={mutation.isPending}
                />
              </TableCell>
            ))}
          </TableRow>
          <TableRow>
            <TableCell className='whitespace-normal'>
              {t('Monthly requests')}
            </TableCell>
            {drafts.map((plan) => (
              <TableCell key={plan.id}>
                <Input
                  type='number'
                  min={0}
                  max={1000000000}
                  aria-label={`${plan.name}: ${t('Monthly requests')}`}
                  value={plan.limits.requests}
                  onChange={(event) =>
                    update(plan.id, (item) => ({
                      ...item,
                      limits: {
                        ...item.limits,
                        requests: parseCount(event.target.value),
                      },
                    }))
                  }
                  disabled={mutation.isPending}
                />
              </TableCell>
            ))}
          </TableRow>
          <TableRow>
            <TableCell className='whitespace-normal'>{t('Users')}</TableCell>
            {drafts.map((plan) => (
              <TableCell key={plan.id}>
                <Input
                  type='number'
                  min={0}
                  max={1000000000}
                  aria-label={`${plan.name}: ${t('Users')}`}
                  value={plan.limits.users}
                  onChange={(event) =>
                    update(plan.id, (item) => ({
                      ...item,
                      limits: {
                        ...item.limits,
                        users: parseCount(event.target.value),
                      },
                    }))
                  }
                  disabled={mutation.isPending}
                />
              </TableCell>
            ))}
          </TableRow>
          {features.map((feature) => (
            <TableRow key={feature.key}>
              <TableCell className='whitespace-normal'>{feature.label}</TableCell>
              {drafts.map((plan) => (
                <TableCell key={plan.id}>
                  <Checkbox
                    checked={Boolean(plan.capabilities[feature.key])}
                    aria-label={`${plan.name}: ${feature.label}`}
                    onCheckedChange={(checked) =>
                      update(plan.id, (item) => ({
                        ...item,
                        capabilities: {
                          ...item.capabilities,
                          [feature.key]: checked === true,
                        },
                      }))
                    }
                    disabled={mutation.isPending}
                  />
                </TableCell>
              ))}
            </TableRow>
          ))}
          <TableRow>
            <TableCell />
            {drafts.map((plan) => {
              const original = query.data.find((item) => item.id === plan.id)
              const dirty = !original || !samePlan(plan, original)
              return (
                <TableCell key={plan.id}>
                  <Button
                    type='button'
                    disabled={!dirty || mutation.isPending}
                    onClick={() => mutation.mutate(payload(plan))}
                  >
                    {t('Save changes')}
                  </Button>
                </TableCell>
              )
            })}
          </TableRow>
        </TableBody>
      </Table>
      {mutation.isError && (
        <p role='alert' className='text-destructive text-sm'>
          {mutation.error.message}
        </p>
      )}
    </section>
  )
}
