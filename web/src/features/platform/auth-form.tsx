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
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { platformLogin, platformRegister } from './api'
import { platformPasswordSchema } from './lib/schema'

export function PlatformAuthForm() {
  const { t } = useTranslation()
  const [register, setRegister] = useState(false)
  const [registered, setRegistered] = useState(false)
  const queryClient = useQueryClient()
  const schema = z.object({
    email: z.email(t('Invalid email address')),
    password: register
      ? platformPasswordSchema(t)
      : z.string().min(1, t('Required')),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: async (values: z.infer<typeof schema>) => {
      if (register) {
        await platformRegister(values)
        setRegistered(true)
        setRegister(false)
        form.resetField('password')
        return
      }
      const session = await platformLogin(values)
      queryClient.setQueryData(['platform', 'session'], session)
      form.reset()
    },
  })
  return (
    <Card className='mx-auto w-full max-w-md'>
      <CardHeader>
        <CardTitle>
          {register ? t('Create platform account') : t('Platform sign in')}
        </CardTitle>
        <CardDescription>
          {t('Platform and workspace accounts are separate.')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
          <FieldGroup>
            <Field data-invalid={!!form.formState.errors.email}>
              <FieldLabel htmlFor='platform-email'>{t('Email')}</FieldLabel>
              <Input
                id='platform-email'
                type='email'
                autoComplete='email'
                {...form.register('email')}
                aria-invalid={!!form.formState.errors.email}
              />
              <FieldError>{form.formState.errors.email?.message}</FieldError>
            </Field>
            <Field data-invalid={!!form.formState.errors.password}>
              <FieldLabel htmlFor='platform-password'>
                {t('Password')}
              </FieldLabel>
              <PasswordInput
                id='platform-password'
                autoComplete={register ? 'new-password' : 'current-password'}
                {...form.register('password')}
                aria-invalid={!!form.formState.errors.password}
              />
              <FieldDescription>
                {t('Use 15 to 128 characters for a new password.')}
              </FieldDescription>
              <FieldError>{form.formState.errors.password?.message}</FieldError>
            </Field>
            {registered && (
              <p role='status'>
                {t('Registration submitted. Sign in with your credentials.')}
              </p>
            )}
            {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
            <Button type='submit' disabled={mutation.isPending}>
              {mutation.isPending && <LoadingState inline size='sm' />}
              {register ? t('Register') : t('Sign in')}
            </Button>
            <Button
              type='button'
              variant='ghost'
              onClick={() => setRegister(!register)}
              disabled={mutation.isPending}
            >
              {register ? t('Back to sign in') : t('Create platform account')}
            </Button>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
