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

import { redeemHostingPlan } from './api'
import type { Workspace } from './types'

export function RedeemPlanDialog(props: {
  workspace: Workspace
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = z.object({
    code: z
      .string()
      .trim()
      .regex(/^[a-fA-F0-9]{64}$/, t('Enter a valid platform redemption code.')),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { code: '' },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      redeemHostingPlan(props.workspace.id, values.code),
    onSuccess: (assignment) => {
      form.reset()
      void queryClient.invalidateQueries({ queryKey: ['platform', 'tenants'] })
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
      title={t('Redeem hosting plan')}
      desc={
        <>
          <p className='font-medium break-words'>
            {props.workspace.name} · /t/{props.workspace.slug}
          </p>
          <p>
            {t(
              'Renewing the same plan extends its expiry. Switching plans starts a new term today. Each code can be used once per workspace.'
            )}
          </p>
        </>
      }
      confirmText={t('Redeem')}
      handleConfirm={form.handleSubmit((values) => mutation.mutate(values))}
      isLoading={mutation.isPending}
    >
      <FieldGroup>
        <Field data-invalid={!!form.formState.errors.code}>
          <FieldLabel htmlFor='platform-redeem-code'>
            {t('Redemption code')}
          </FieldLabel>
          <Input
            id='platform-redeem-code'
            autoComplete='off'
            {...form.register('code')}
            aria-invalid={!!form.formState.errors.code}
            disabled={mutation.isPending}
          />
          <FieldError>{form.formState.errors.code?.message}</FieldError>
        </Field>
        {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
      </FieldGroup>
    </ConfirmDialog>
  )
}
