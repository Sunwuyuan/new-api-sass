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
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { finishPlatformEmailChange, startPlatformEmailChange } from './api'

export function PlatformEmailChangeDialog(props: { onClose: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [step, setStep] = useState<'email' | 'code'>('email')
  const schema = z.object({
    email: z
      .string()
      .trim()
      .max(254)
      .refine(
        (value) => z.email().safeParse(value).success,
        t('Invalid email address')
      ),
    code: z
      .string()
      .trim()
      .regex(/^\d{6}$/, t('Enter the 6-digit verification code.')),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', code: '' },
  })
  const pending = () => start.isPending || finish.isPending
  const start = useMutation({
    meta: { errorToast: false },
    mutationFn: (email: string) => startPlatformEmailChange(email),
    onSuccess: () => setStep('code'),
  })
  const finish = useMutation({
    meta: { errorToast: false },
    mutationFn: (input: { email: string; code: string }) =>
      finishPlatformEmailChange(input),
    onSuccess: async () => {
      toast.success(t('Email updated.'))
      await queryClient.invalidateQueries({
        queryKey: ['platform', 'session'],
      })
      props.onClose()
    },
  })
  async function sendCode() {
    start.reset()
    finish.reset()
    if (await form.trigger('email')) {
      start.mutate(form.getValues('email').trim())
    }
  }
  async function confirm() {
    finish.reset()
    if (await form.trigger('code')) {
      finish.mutate({
        email: form.getValues('email').trim(),
        code: form.getValues('code').trim(),
      })
    }
  }
  const failure = step === 'email' ? start.error : finish.error
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending()) props.onClose()
      }}
      title={t('Change email')}
      description={t(
        'A verification code is sent to the new address. Your sessions stay signed in.'
      )}
      contentClassName='sm:max-w-md'
    >
      <FieldGroup>
        {step === 'email' ? (
          <Field data-invalid={!!form.formState.errors.email}>
            <FieldLabel htmlFor='platform-email-change-address'>
              {t('New email')}
            </FieldLabel>
            <Input
              id='platform-email-change-address'
              type='email'
              autoComplete='email'
              {...form.register('email')}
              aria-invalid={!!form.formState.errors.email}
              disabled={pending()}
            />
            <FieldError>{form.formState.errors.email?.message}</FieldError>
          </Field>
        ) : (
          <>
            <p className='text-muted-foreground text-sm'>
              {t('Enter the verification code sent to {{email}}.', {
                email: form.getValues('email').trim(),
              })}
            </p>
            <Field data-invalid={!!form.formState.errors.code}>
              <FieldLabel htmlFor='platform-email-change-code'>
                {t('Verification code')}
              </FieldLabel>
              <Input
                id='platform-email-change-code'
                inputMode='numeric'
                autoComplete='one-time-code'
                {...form.register('code')}
                aria-invalid={!!form.formState.errors.code}
                disabled={pending()}
              />
              <FieldError>{form.formState.errors.code?.message}</FieldError>
            </Field>
          </>
        )}
        {failure && <p role='alert'>{failure.message}</p>}
        <div className='flex flex-wrap items-center gap-2'>
          {step === 'email' ? (
            <Button onClick={() => void sendCode()} disabled={pending()}>
              {start.isPending && <LoadingState inline size='sm' />}
              {t('Send code')}
            </Button>
          ) : (
            <>
              <Button onClick={() => void confirm()} disabled={pending()}>
                {finish.isPending && <LoadingState inline size='sm' />}
                {t('Change email')}
              </Button>
              <Button
                variant='ghost'
                disabled={pending()}
                onClick={() => {
                  finish.reset()
                  setStep('email')
                }}
              >
                {t('Use a different email')}
              </Button>
            </>
          )}
        </div>
      </FieldGroup>
    </Dialog>
  )
}
