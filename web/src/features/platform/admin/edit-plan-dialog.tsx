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
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { updateHostingPlan } from '../api'
import { platformCountSchema } from '../lib/schema'
import type { HostingPlan } from '../types'

export function EditPlanDialog(props: {
  plan: HostingPlan
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const limit = platformCountSchema(t, 1000000000)
  const schema = z.object({
    id: z.number(),
    name: z.string(),
    price: z
      .string()
      .trim()
      .min(1, t('Required'))
      .max(64, t('Check the plan limits and capabilities.')),
    limits: z.object({
      requests: limit,
      users: limit,
      tokens: limit,
      channels: limit,
    }),
    capabilities: z.object({
      max_workspaces: platformCountSchema(t, 1000),
      remove_platform_footer: z.boolean(),
      custom_branding: z.boolean(),
    }),
  })
  const form = useForm<HostingPlan>({
    resolver: zodResolver(schema),
    defaultValues: props.plan,
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: updateHostingPlan,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['platform'] })
      toast.success(t('Hosting plan updated'))
      props.onClose()
    },
  })
  const numericFields = [
    { name: 'limits.requests', label: t('Monthly requests') },
    { name: 'limits.users', label: t('Users') },
    { name: 'limits.tokens', label: t('Tokens') },
    { name: 'limits.channels', label: t('Channels') },
    { name: 'capabilities.max_workspaces', label: t('Workspaces per account') },
  ] as const
  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={`${t('Edit plan')} · ${props.plan.name}`}
      desc={t(
        'Plan changes apply immediately to every workspace on that plan.'
      )}
      confirmText={t('Save changes')}
      className='max-h-[90svh] overflow-y-auto sm:max-w-xl'
      handleConfirm={form.handleSubmit((values) => mutation.mutate(values))}
      isLoading={mutation.isPending}
    >
      <FieldGroup>
        <Field data-invalid={!!form.formState.errors.price}>
          <FieldLabel htmlFor='platform-plan-price'>{t('Price')}</FieldLabel>
          <Input
            id='platform-plan-price'
            {...form.register('price')}
            disabled={mutation.isPending}
            aria-invalid={!!form.formState.errors.price}
          />
          <FieldError>{form.formState.errors.price?.message}</FieldError>
        </Field>
        <div className='grid gap-4 sm:grid-cols-2'>
          {numericFields.map((field) => (
            <Field
              key={field.name}
              data-invalid={
                !!form.getFieldState(field.name, form.formState).error
              }
            >
              <FieldLabel htmlFor={`platform-${field.name}`}>
                {field.label}
              </FieldLabel>
              <Input
                id={`platform-${field.name}`}
                type='number'
                min={1}
                max={
                  field.name === 'capabilities.max_workspaces'
                    ? 1000
                    : 1000000000
                }
                {...form.register(field.name, { valueAsNumber: true })}
                disabled={mutation.isPending}
                aria-invalid={
                  !!form.getFieldState(field.name, form.formState).error
                }
              />
              <FieldError>
                {form.getFieldState(field.name, form.formState).error?.message}
              </FieldError>
            </Field>
          ))}
        </div>
        <Controller
          control={form.control}
          name='capabilities.custom_branding'
          render={({ field }) => (
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-custom-branding'
                checked={field.value}
                onCheckedChange={field.onChange}
                disabled={mutation.isPending}
              />
              <FieldLabel htmlFor='platform-custom-branding'>
                {t('Custom branding')}
              </FieldLabel>
            </Field>
          )}
        />
        <Controller
          control={form.control}
          name='capabilities.remove_platform_footer'
          render={({ field }) => (
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-remove-footer'
                checked={field.value}
                onCheckedChange={field.onChange}
                disabled={mutation.isPending}
              />
              <FieldLabel htmlFor='platform-remove-footer'>
                {t('Allow removing the platform footer')}
              </FieldLabel>
            </Field>
          )}
        />
        {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
      </FieldGroup>
    </ConfirmDialog>
  )
}
