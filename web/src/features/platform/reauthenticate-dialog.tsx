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
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'

import { reauthenticatePlatform } from './api'

export function ReauthenticateDialog(props: { onClose: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = z.object({
    password: z.string().min(1, t('Required')).max(512),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { password: '' },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      reauthenticatePlatform(values.password),
    onSuccess: (session) => {
      queryClient.setQueryData(['platform', 'session'], session)
      form.reset()
      props.onClose()
      toast.success(t('Administrator access verified. Retry your action.'))
    },
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={t('Verify administrator access')}
      description={t(
        'Confirm your password to enable administrative changes for five minutes.'
      )}
    >
      <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
        <FieldGroup>
          <Field data-invalid={!!form.formState.errors.password}>
            <FieldLabel htmlFor='platform-verify-password'>
              {t('Current Password')}
            </FieldLabel>
            <PasswordInput
              id='platform-verify-password'
              autoComplete='current-password'
              {...form.register('password')}
              disabled={mutation.isPending}
              aria-invalid={!!form.formState.errors.password}
            />
            <FieldError>{form.formState.errors.password?.message}</FieldError>
          </Field>
          {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
          <Button type='submit' disabled={mutation.isPending}>
            {t('Verify administrator access')}
          </Button>
        </FieldGroup>
      </form>
    </Dialog>
  )
}
