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

import { Dialog } from '@/components/dialog'
import { LoadingState } from '@/components/loading-state'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'

import { changePlatformPassword } from './api'
import { platformPasswordSchema } from './lib/schema'

export function PlatformPasswordDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = z
    .object({
      current_password: z.string().min(1, t('Required')),
      new_password: platformPasswordSchema(t),
      confirmation: z.string(),
    })
    .refine((values) => values.new_password === values.confirmation, {
      message: t('Passwords do not match'),
      path: ['confirmation'],
    })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: '', new_password: '', confirmation: '' },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      changePlatformPassword({
        current_password: values.current_password,
        new_password: values.new_password,
      }),
    onSuccess: () => {
      form.reset()
      props.onOpenChange(false)
      queryClient.removeQueries({ queryKey: ['platform', 'tenants'] })
      queryClient.removeQueries({ queryKey: ['platform', 'users'] })
      queryClient.setQueryData(['platform', 'session'], null)
      toast.success(
        t('Password changed. Sign in again with your new password.')
      )
    },
  })
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Change platform password')}
      description={t(
        'Changing your platform password signs out all platform sessions.'
      )}
      contentClassName='sm:max-w-md'
    >
      <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
        <FieldGroup>
          <Field data-invalid={!!form.formState.errors.current_password}>
            <FieldLabel htmlFor='platform-current-password'>
              {t('Current Password')}
            </FieldLabel>
            <PasswordInput
              id='platform-current-password'
              autoComplete='current-password'
              {...form.register('current_password')}
              aria-invalid={!!form.formState.errors.current_password}
              disabled={mutation.isPending}
            />
            <FieldError>
              {form.formState.errors.current_password?.message}
            </FieldError>
          </Field>
          <Field data-invalid={!!form.formState.errors.new_password}>
            <FieldLabel htmlFor='platform-new-password'>
              {t('New Password')}
            </FieldLabel>
            <PasswordInput
              id='platform-new-password'
              autoComplete='new-password'
              {...form.register('new_password')}
              aria-invalid={!!form.formState.errors.new_password}
              disabled={mutation.isPending}
            />
            <FieldError>
              {form.formState.errors.new_password?.message}
            </FieldError>
          </Field>
          <Field data-invalid={!!form.formState.errors.confirmation}>
            <FieldLabel htmlFor='platform-confirm-password'>
              {t('Confirm New Password')}
            </FieldLabel>
            <PasswordInput
              id='platform-confirm-password'
              autoComplete='new-password'
              {...form.register('confirmation')}
              aria-invalid={!!form.formState.errors.confirmation}
              disabled={mutation.isPending}
            />
            <FieldError>
              {form.formState.errors.confirmation?.message}
            </FieldError>
          </Field>
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
          <Button type='submit' disabled={mutation.isPending}>
            {mutation.isPending && <LoadingState inline size='sm' />}
            {t('Change platform password')}
          </Button>
        </FieldGroup>
      </form>
    </Dialog>
  )
}
