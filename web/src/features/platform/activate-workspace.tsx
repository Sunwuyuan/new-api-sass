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
import { useMutation } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { LoadingState } from '@/components/loading-state'
import { PasswordInput } from '@/components/password-input'
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
import { Toaster } from '@/components/ui/sonner'
import { tenantPath } from '@/lib/tenant'

import { activateWorkspaceRoot } from './api'
import { platformPasswordSchema } from './lib/schema'

export default function ActivateWorkspace() {
  const { t } = useTranslation()
  const [activationToken] = useState(
    () => new URLSearchParams(window.location.hash.slice(1)).get('token') ?? ''
  )
  useEffect(() => {
    window.history.replaceState(
      window.history.state,
      '',
      window.location.pathname
    )
  }, [])
  const schema = z.object({ password: platformPasswordSchema(t) })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { password: '' },
  })
  const activation = useMutation({
    meta: { errorToast: false },
    mutationFn: async (values: z.infer<typeof schema>) => {
      await activateWorkspaceRoot(activationToken, values.password)
      form.reset()
    },
  })
  return (
    <main className='mx-auto max-w-md px-4 py-16'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Activate workspace root')}</CardTitle>
          <CardDescription>
            {t('Choose the password for the root account in this workspace.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {activation.isSuccess ? (
            <Button
              render={<a href={tenantPath('/sign-in')} />}
              nativeButton={false}
              role='link'
            >
              {t('Sign in as root')}
            </Button>
          ) : (
            <form
              onSubmit={form.handleSubmit((values) =>
                activation.mutate(values)
              )}
            >
              <FieldGroup>
                <Field data-invalid={!!form.formState.errors.password}>
                  <FieldLabel htmlFor='root-password'>
                    {t('Password')}
                  </FieldLabel>
                  <PasswordInput
                    id='root-password'
                    autoComplete='new-password'
                    {...form.register('password')}
                    aria-invalid={!!form.formState.errors.password}
                  />
                  <FieldError>
                    {form.formState.errors.password?.message}
                  </FieldError>
                </Field>
                {!activationToken && (
                  <p role='alert'>
                    {t('The activation link is missing or expired.')}
                  </p>
                )}
                {activation.isError && (
                  <p role='alert'>{activation.error.message}</p>
                )}
                <Button
                  type='submit'
                  disabled={activation.isPending || !activationToken}
                >
                  {activation.isPending && <LoadingState inline size='sm' />}
                  {t('Activate workspace root')}
                </Button>
              </FieldGroup>
            </form>
          )}
        </CardContent>
      </Card>
      <Toaster />
    </main>
  )
}
