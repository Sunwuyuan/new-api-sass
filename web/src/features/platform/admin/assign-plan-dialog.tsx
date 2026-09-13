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
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { assignHostingPlan } from '../api'
import { platformCountSchema } from '../lib/schema'
import type { HostingPlan, Workspace } from '../types'

export function AssignPlanDialog(props: {
  workspace: Workspace
  plans: HostingPlan[]
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = z.object({
    planId: platformCountSchema(t, Number.MAX_SAFE_INTEGER),
    months: platformCountSchema(t, 36),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { planId: props.workspace.plan_id, months: 1 },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      assignHostingPlan(props.workspace.id, values.planId, values.months),
    onSuccess: (assignment) => {
      void queryClient.invalidateQueries({ queryKey: ['platform'] })
      toast.success(
        t('Hosting plan activated until {{date}}.', {
          date: new Date(assignment.expires_at).toLocaleString(),
        })
      )
      props.onClose()
    },
  })
  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={t('Activate plan manually')}
      desc={
        <>
          <p className='font-medium break-words'>
            {props.workspace.name} · /t/{props.workspace.slug}
          </p>
          <p>
            {t(
              'Renewing the same plan extends its expiry. Switching plans starts a new term today. Suspended workspaces must be enabled separately.'
            )}
          </p>
        </>
      }
      confirmText={t('Activate plan manually')}
      handleConfirm={form.handleSubmit((values) => mutation.mutate(values))}
      isLoading={mutation.isPending}
      disabled={props.plans.length === 0}
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='platform-assign-plan'>
            {t('Hosting plan')}
          </FieldLabel>
          <NativeSelect
            id='platform-assign-plan'
            {...form.register('planId', { valueAsNumber: true })}
            disabled={mutation.isPending}
          >
            {props.plans.map((plan) => (
              <NativeSelectOption key={plan.id} value={plan.id}>
                {plan.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
        <Field data-invalid={!!form.formState.errors.months}>
          <FieldLabel htmlFor='platform-assign-months'>
            {t('Months')}
          </FieldLabel>
          <Input
            id='platform-assign-months'
            type='number'
            min={1}
            max={36}
            {...form.register('months', { valueAsNumber: true })}
            disabled={mutation.isPending}
            aria-invalid={!!form.formState.errors.months}
          />
          <FieldError>{form.formState.errors.months?.message}</FieldError>
        </Field>
        {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
      </FieldGroup>
    </ConfirmDialog>
  )
}
