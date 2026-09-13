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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRouterState } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'

import { platformLogout, reauthenticatePlatform } from './api'
import {
  platformAuthMethodsQuery,
  platformStatusQuery,
  platformVerificationMethods,
  updatePlatformSession,
} from './auth-api'
import { PlatformLoginMethods } from './login-methods'
import type { PlatformSession } from './types'

export function ReauthenticateDialog(props: {
  onClose: () => void
  title?: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const status = useQuery(platformStatusQuery)
  const methods = useQuery(platformAuthMethodsQuery)
  const redirectTo = useRouterState({ select: (state) => state.location.href })
  const logout = useMutation({
    mutationFn: platformLogout,
    onSuccess: async () => {
      await updatePlatformSession(queryClient, null)
      props.onClose()
    },
  })
  async function verified(session: PlatformSession) {
    await updatePlatformSession(queryClient, session)
    form.reset()
    props.onClose()
    toast.success(t('Identity verified. You can continue.'))
  }
  const schema = z.object({
    password: z.string().min(1, t('Required')).max(512),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { password: '' },
  })
  const mutation = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) =>
      reauthenticatePlatform(values.password),
    onSuccess: verified,
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
      title={props.title ?? t('Verify administrator access')}
      description={t(
        'Verify your identity to continue with sensitive changes for five minutes.'
      )}
    >
      {(status.isPending || methods.isPending) && <LoadingState />}
      {(status.isError || methods.isError) && (
        <ErrorState
          onRetry={() => {
            void status.refetch()
            void methods.refetch()
          }}
        />
      )}
      {status.data && methods.data && (
        <PlatformLoginMethods
          status={platformVerificationMethods(status.data, methods.data)}
          intent='verify'
          redirectTo={redirectTo}
          onSuccess={verified}
        />
      )}
      {methods.data?.has_password && (
        <form
          className='mt-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
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
              {props.title ?? t('Verify administrator access')}
            </Button>
          </FieldGroup>
        </form>
      )}
      {methods.data && !methods.data.has_password && (
        <div className='mt-4 space-y-3'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'If no verification method is available, sign out and sign in again. Add a Passkey from account security after signing in.'
            )}
          </p>
          <Button
            variant='outline'
            disabled={logout.isPending}
            onClick={() => logout.mutate()}
          >
            {t('Sign out and sign in again')}
          </Button>
        </div>
      )}
    </Dialog>
  )
}
