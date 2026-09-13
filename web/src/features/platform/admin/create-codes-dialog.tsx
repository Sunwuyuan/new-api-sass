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
import { z } from 'zod'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

import { createPlatformRedemptions } from '../api'
import { platformCountSchema } from '../lib/schema'
import type { HostingPlan } from '../types'

export function CreateCodesDialog(props: {
  plans: HostingPlan[]
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [codes, setCodes] = useState<string[] | null>(null)
  const schema = z.object({
    plan_id: platformCountSchema(t, Number.MAX_SAFE_INTEGER),
    duration_months: platformCountSchema(t, 36),
    count: platformCountSchema(t, 100),
    max_uses: platformCountSchema(t, 1000),
    expires_at: z
      .string()
      .refine(
        (value) =>
          value === '' ||
          (!Number.isNaN(Date.parse(value)) && Date.parse(value) > Date.now()),
        t('Expiry must be in the future.')
      ),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      plan_id: props.plans[0]?.id,
      duration_months: 1,
      count: 1,
      max_uses: 1,
      expires_at: '',
    },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    gcTime: 0,
    mutationFn: (values: z.infer<typeof schema>) =>
      createPlatformRedemptions({
        ...values,
        expires_at: values.expires_at
          ? new Date(values.expires_at).toISOString()
          : null,
      }),
    onSuccess: (data) => {
      setCodes(data.codes)
      mutation.reset()
      void queryClient.invalidateQueries({
        queryKey: ['platform', 'admin', 'redemptions'],
      })
    },
  })
  if (codes) {
    return (
      <Dialog
        open
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
        title={t('Redemption codes created')}
        description={t('Copy these codes now. Full codes are only shown once.')}
        footer={<Button onClick={props.onClose}>{t('Done')}</Button>}
      >
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='platform-created-codes'>
              {t('Platform redemption codes')}
            </FieldLabel>
            <Textarea
              id='platform-created-codes'
              readOnly
              value={codes.join('\n')}
              rows={8}
              className='font-mono'
            />
          </Field>
          <CopyButton
            value={codes.join('\n')}
            size='default'
            variant='outline'
            aria-label={t('Copy all codes')}
          >
            {t('Copy all codes')}
          </CopyButton>
        </FieldGroup>
      </Dialog>
    )
  }
  const fields = [
    { name: 'duration_months', label: t('Months'), max: 36 },
    { name: 'count', label: t('Code count'), max: 100 },
    { name: 'max_uses', label: t('Maximum uses per code'), max: 1000 },
  ] as const
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={t('Generate platform codes')}
      description={t(
        'Codes activate hosting plans for selected workspaces. Each workspace can redeem a code once.'
      )}
    >
      <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='platform-code-plan'>
              {t('Hosting plan')}
            </FieldLabel>
            <NativeSelect
              id='platform-code-plan'
              {...form.register('plan_id', { valueAsNumber: true })}
              disabled={mutation.isPending}
            >
              {props.plans.map((plan) => (
                <NativeSelectOption key={plan.id} value={plan.id}>
                  {plan.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </Field>
          {fields.map((field) => (
            <Field
              key={field.name}
              data-invalid={!!form.formState.errors[field.name]}
            >
              <FieldLabel htmlFor={`platform-code-${field.name}`}>
                {field.label}
              </FieldLabel>
              <Input
                id={`platform-code-${field.name}`}
                type='number'
                min={1}
                max={field.max}
                {...form.register(field.name, { valueAsNumber: true })}
                aria-invalid={!!form.formState.errors[field.name]}
                disabled={mutation.isPending}
              />
              <FieldError>
                {form.formState.errors[field.name]?.message}
              </FieldError>
            </Field>
          ))}
          <Field data-invalid={!!form.formState.errors.expires_at}>
            <FieldLabel htmlFor='platform-code-expiry'>
              {t('Expiry (optional)')}
            </FieldLabel>
            <Input
              id='platform-code-expiry'
              type='datetime-local'
              {...form.register('expires_at')}
              disabled={mutation.isPending}
              aria-invalid={!!form.formState.errors.expires_at}
            />
            <FieldError>{form.formState.errors.expires_at?.message}</FieldError>
          </Field>
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
          <Button
            type='submit'
            disabled={mutation.isPending || props.plans.length === 0}
          >
            {t('Generate platform codes')}
          </Button>
        </FieldGroup>
      </form>
    </Dialog>
  )
}
