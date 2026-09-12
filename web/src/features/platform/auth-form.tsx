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
import { Loading03Icon, Login01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useIsMutating, useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { LegalConsent } from '@/features/auth/components/legal-consent'

import { platformLogin, platformRegister } from './api'
import type { PlatformAuthStatus } from './auth-api'
import { platformPasswordSchema } from './lib/schema'
import { PlatformLoginMethods } from './login-methods'
import type { PlatformSession } from './types'

export function PlatformAuthForm(props: {
  mode: 'sign-in' | 'sign-up'
  status: PlatformAuthStatus
  redirectTo: string
  onSignedIn: (session: PlatformSession) => Promise<void> | void
  onRegistered?: () => Promise<void> | void
}) {
  const { t } = useTranslation()
  const signUp = props.mode === 'sign-up'
  const [agreed, setAgreed] = useState(false)
  const busy = useIsMutating({ mutationKey: ['platform', 'authenticate'] }) > 0
  const requiresConsent =
    props.status.user_agreement_enabled || props.status.privacy_policy_enabled
  const disabled = busy || (requiresConsent && !agreed)
  const schema = z
    .object({
      email: z.email(t('Invalid email address')).max(254),
      password: signUp
        ? platformPasswordSchema(t)
        : z.string().min(1, t('Required')).max(512),
      confirmation: z.string(),
    })
    .refine((data) => !signUp || data.password === data.confirmation, {
      message: t('Passwords do not match'),
      path: ['confirmation'],
    })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '', confirmation: '' },
  })
  const mutation = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: async (values: z.infer<typeof schema>) => {
      if (requiresConsent && !agreed) {
        throw new Error(t('Please agree to the legal terms first'))
      }
      const input = { email: values.email, password: values.password }
      if (signUp) {
        await platformRegister(input)
        form.reset()
        await props.onRegistered?.()
      } else {
        const session = await platformLogin(input)
        form.reset()
        await props.onSignedIn(session)
      }
    },
  })
  const methods = (
    <PlatformLoginMethods
      status={props.status}
      redirectTo={props.redirectTo}
      disabled={disabled}
      showPasskey={!signUp}
      onSuccess={props.onSignedIn}
    />
  )
  const passwordEnabled = signUp
    ? props.status.password_register_enabled
    : props.status.password_login_enabled
  return (
    <div className='grid gap-4'>
      {!signUp && methods}
      <form
        onSubmit={form.handleSubmit((values) => {
          if (!disabled) mutation.mutate(values)
        })}
      >
        <FieldGroup className='gap-4'>
          {passwordEnabled && (
            <>
              <Field data-invalid={!!form.formState.errors.email}>
                <FieldLabel htmlFor='platform-email'>{t('Email')}</FieldLabel>
                <Input
                  id='platform-email'
                  type='email'
                  autoComplete='username'
                  placeholder={t('name@example.com')}
                  {...form.register('email')}
                  aria-invalid={!!form.formState.errors.email}
                  disabled={busy}
                />
                <FieldError>{form.formState.errors.email?.message}</FieldError>
              </Field>
              <Field data-invalid={!!form.formState.errors.password}>
                <FieldLabel htmlFor='platform-password'>
                  {t('Password')}
                </FieldLabel>
                <PasswordInput
                  id='platform-password'
                  autoComplete={signUp ? 'new-password' : 'current-password'}
                  placeholder={t('Enter password')}
                  {...form.register('password')}
                  aria-invalid={!!form.formState.errors.password}
                  disabled={busy}
                />
                {signUp && (
                  <FieldDescription>
                    {t('Use 15 to 128 characters for a new password.')}
                  </FieldDescription>
                )}
                <FieldError>
                  {form.formState.errors.password?.message}
                </FieldError>
              </Field>
              {signUp && (
                <Field data-invalid={!!form.formState.errors.confirmation}>
                  <FieldLabel htmlFor='platform-confirmation'>
                    {t('Confirm password')}
                  </FieldLabel>
                  <PasswordInput
                    id='platform-confirmation'
                    autoComplete='new-password'
                    placeholder={t('Confirm password')}
                    {...form.register('confirmation')}
                    aria-invalid={!!form.formState.errors.confirmation}
                    disabled={busy}
                  />
                  <FieldError>
                    {form.formState.errors.confirmation?.message}
                  </FieldError>
                </Field>
              )}
            </>
          )}
          <LegalConsent
            status={props.status}
            agreementUrl={props.status.user_agreement_url}
            privacyUrl={props.status.privacy_policy_url}
            checked={agreed}
            onCheckedChange={setAgreed}
          />
          {mutation.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {mutation.error.message}
            </p>
          )}
          {passwordEnabled && (
            <Button
              type='submit'
              disabled={disabled}
              className='mt-2 w-full justify-center gap-2'
            >
              {mutation.isPending ? (
                <HugeiconsIcon
                  icon={Loading03Icon}
                  className='animate-spin'
                  data-icon='inline-start'
                />
              ) : (
                !signUp && (
                  <HugeiconsIcon icon={Login01Icon} data-icon='inline-start' />
                )
              )}
              {signUp ? t('Create account') : t('Sign in')}
            </Button>
          )}
        </FieldGroup>
      </form>
      {signUp && props.status.oauth_register_enabled && methods}
    </div>
  )
}
