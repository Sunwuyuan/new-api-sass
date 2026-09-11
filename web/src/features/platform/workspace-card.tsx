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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { assignHostingPlan, setWorkspaceStatus } from './api'
import type { HostingPlan, WorkspaceUsage } from './types'

export function WorkspaceCard(props: {
  item: WorkspaceUsage
  plans: HostingPlan[]
  admin: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirm, setConfirm] = useState(false)
  const workspace = props.item.tenant
  const currentPlan = props.plans.find((plan) => plan.id === workspace.plan_id)
  const expired =
    workspace.plan_expires_at !== null &&
    new Date(workspace.plan_expires_at).getTime() <= Date.now()
  let statusText = t('Active')
  if (expired) statusText = t('Expired')
  if (workspace.status === 'suspended') statusText = t('Suspended')
  const schema = z.object({
    planId: z.number().int().positive(),
    months: z
      .number()
      .int()
      .min(1, t('Choose between 1 and 36 months.'))
      .max(36, t('Choose between 1 and 36 months.')),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { planId: workspace.plan_id, months: 1 },
  })
  const assign = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      assignHostingPlan(workspace.id, values.planId, values.months),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['platform', 'tenants'] })
      toast.success(t('Hosting plan updated'))
    },
  })
  const status = useMutation({
    meta: { errorToast: false },
    mutationFn: () =>
      setWorkspaceStatus(
        workspace.id,
        workspace.status === 'active' ? 'suspended' : 'active'
      ),
    onSuccess: () => {
      setConfirm(false)
      void queryClient.invalidateQueries({ queryKey: ['platform', 'tenants'] })
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle className='break-words'>{workspace.name}</CardTitle>
        <CardDescription className='break-words'>
          /t/{workspace.slug} · {currentPlan?.name} · {statusText}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <p>
          {t('Monthly requests')}: {props.item.usage.requests.toLocaleString()}{' '}
          / {currentPlan?.limits.requests.toLocaleString()}
        </p>
        <p>
          {t('Plan expires')}:{' '}
          {workspace.plan_expires_at
            ? new Date(workspace.plan_expires_at).toLocaleString()
            : t('No expiry')}
        </p>
        <Button
          render={<a href={`/t/${workspace.slug}/`} />}
          nativeButton={false}
          role='link'
          disabled={expired || workspace.status === 'suspended'}
        >
          {t('Enter workspace')}
        </Button>
        {props.admin && (
          <>
            <form
              onSubmit={form.handleSubmit((values) => assign.mutate(values))}
            >
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor={`plan-${workspace.id}`}>
                    {t('Hosting plan')}
                  </FieldLabel>
                  <NativeSelect
                    id={`plan-${workspace.id}`}
                    {...form.register('planId', { valueAsNumber: true })}
                    disabled={assign.isPending || props.plans.length === 0}
                  >
                    {props.plans.map((plan) => (
                      <NativeSelectOption key={plan.id} value={plan.id}>
                        {plan.name}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </Field>
                <Field data-invalid={!!form.formState.errors.months}>
                  <FieldLabel htmlFor={`months-${workspace.id}`}>
                    {t('Months')}
                  </FieldLabel>
                  <Input
                    id={`months-${workspace.id}`}
                    type='number'
                    min={1}
                    max={36}
                    {...form.register('months', { valueAsNumber: true })}
                    aria-invalid={!!form.formState.errors.months}
                  />
                  <FieldError>
                    {form.formState.errors.months?.message}
                  </FieldError>
                </Field>
                {assign.isError && <p role='alert'>{assign.error.message}</p>}
                <Button
                  type='submit'
                  disabled={
                    assign.isPending ||
                    status.isPending ||
                    props.plans.length === 0
                  }
                >
                  {assign.isPending && <LoadingState inline size='sm' />}
                  {t('Activate plan manually')}
                </Button>
              </FieldGroup>
            </form>
            {status.isError && <p role='alert'>{status.error.message}</p>}
            <Button
              variant='outline'
              disabled={assign.isPending || status.isPending}
              onClick={() => setConfirm(true)}
            >
              {workspace.status === 'active'
                ? t('Suspend workspace')
                : t('Enable workspace')}
            </Button>
            <ConfirmDialog
              open={confirm}
              onOpenChange={setConfirm}
              title={
                workspace.status === 'active'
                  ? t('Suspend workspace')
                  : t('Enable workspace')
              }
              desc={t(
                'Suspended workspaces cannot access the gateway or their dashboard.'
              )}
              handleConfirm={() => status.mutate()}
              isLoading={status.isPending}
              destructive={workspace.status === 'active'}
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
