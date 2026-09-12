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
import { LoadingState } from '@/components/loading-state'
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
import { Input } from '@/components/ui/input'

import { createWorkspace } from './api'

export function CreateWorkspace(props: { disabled?: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activationURL, setActivationURL] = useState('')
  const schema = z.object({
    name: z.string().trim().min(1, t('Required')).max(128),
    slug: z
      .string()
      .regex(
        /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/,
        t('Use lowercase letters, numbers and hyphens, up to 48 characters.')
      ),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', slug: '' },
  })
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: createWorkspace,
    onSuccess: (data) => {
      setActivationURL(
        new URL(data.root_activation_url, window.location.origin).toString()
      )
      form.reset()
      void queryClient.invalidateQueries({ queryKey: ['platform', 'tenants'] })
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Create workspace')}</CardTitle>
        <CardDescription>
          {t(
            'New workspaces start on Lite. Redeem a code or contact an administrator to activate a hosting plan.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {props.disabled && (
          <p className='text-muted-foreground mb-4'>
            {t(
              'Your workspace limit has been reached. Upgrade an active workspace to increase your capacity.'
            )}
          </p>
        )}
        <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
          <FieldGroup className='md:grid md:grid-cols-2'>
            <Field data-invalid={!!form.formState.errors.name}>
              <FieldLabel htmlFor='workspace-name'>{t('Name')}</FieldLabel>
              <Input
                id='workspace-name'
                {...form.register('name')}
                aria-invalid={!!form.formState.errors.name}
                disabled={props.disabled || mutation.isPending}
              />
              <FieldError>{form.formState.errors.name?.message}</FieldError>
            </Field>
            <Field data-invalid={!!form.formState.errors.slug}>
              <FieldLabel htmlFor='workspace-slug'>
                {t('Workspace address')}
              </FieldLabel>
              <Input
                id='workspace-slug'
                {...form.register('slug')}
                aria-invalid={!!form.formState.errors.slug}
                disabled={props.disabled || mutation.isPending}
                placeholder='my-workspace'
              />
              <FieldError>{form.formState.errors.slug?.message}</FieldError>
            </Field>
            {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
            <Button
              type='submit'
              disabled={props.disabled || mutation.isPending}
            >
              {mutation.isPending && <LoadingState inline size='sm' />}
              {t('Create workspace')}
            </Button>
          </FieldGroup>
        </form>
        {activationURL && (
          <div className='mt-6 flex flex-wrap items-center gap-3' role='status'>
            <p className='w-full'>
              {t(
                'Save this root activation link. It expires in 30 minutes and can be used once.'
              )}
            </p>
            <Button
              render={
                <a href={activationURL} target='_blank' rel='noreferrer' />
              }
              nativeButton={false}
              role='link'
            >
              {t('Activate workspace root')}
            </Button>
            <CopyButton
              value={activationURL}
              size='default'
              variant='outline'
              aria-label={t('Copy activation link')}
            >
              {t('Copy activation link')}
            </CopyButton>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
