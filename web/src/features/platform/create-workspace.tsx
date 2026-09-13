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
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'

import { createWorkspace } from './api'
import { platformPasswordSchema } from './lib/schema'
import { workspaceHref } from './lib/workspace-url'
import { WorkspaceLink } from './navigation'
import type { HostingPlan, WildcardDomain } from './types'

const prefixPattern = /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/

export function CreateWorkspace(props: {
  plans: HostingPlan[]
  liteAvailable: boolean
  wildcardDomains: WildcardDomain[]
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [setupURL, setSetupURL] = useState('')
  const [createdID, setCreatedID] = useState<number>()
  const [createdURL, setCreatedURL] = useState('')
  const wildcards = props.wildcardDomains
  const lite = props.plans.find((plan) => plan.name === 'Lite')
  const initialPlan =
    lite && props.liteAvailable
      ? lite
      : (props.plans.find((plan) => plan.name !== 'Lite') ?? props.plans[0])
  const schema = z
    .object({
      name: z.string().trim().min(1, t('Required')).max(128),
      slug:
        wildcards.length === 0
          ? z
              .string()
              .regex(
                prefixPattern,
                t(
                  'Use lowercase letters, numbers and hyphens, up to 48 characters.'
                )
              )
          : z.string(),
      prefix:
        wildcards.length > 0
          ? z
              .string()
              .regex(
                prefixPattern,
                t(
                  'Use lowercase letters, numbers and hyphens, up to 48 characters.'
                )
              )
          : z.string(),
      wildcard_domain_id: z.number().int(),
      username: z.string().trim().min(1, t('Required')).max(20),
      display_name: z.string().trim().max(20),
      email: z
        .string()
        .trim()
        .max(50)
        .refine(
          (value) => value === '' || z.email().safeParse(value).success,
          t('Invalid email address')
        ),
      password: platformPasswordSchema(t),
      plan_id: z.number().int().positive(),
      code: z.string().trim(),
    })
    .superRefine((values, ctx) => {
      const selected = props.plans.find((plan) => plan.id === values.plan_id)
      if (!selected) {
        ctx.addIssue({
          code: 'custom',
          path: ['plan_id'],
          message: t('Choose a hosting plan'),
        })
        return
      }
      if (selected.name === 'Lite') {
        if (!props.liteAvailable) {
          ctx.addIssue({
            code: 'custom',
            path: ['plan_id'],
            message: t('You already have a free Lite workspace.'),
          })
        }
        return
      }
      if (!/^[a-fA-F0-9]{64}$/.test(values.code)) {
        ctx.addIssue({
          code: 'custom',
          path: ['code'],
          message: t('Enter a valid platform redemption code.'),
        })
      }
    })
    .superRefine((values, ctx) => {
      if (wildcards.length === 0) return
      if (
        !wildcards.some((domain) => domain.id === values.wildcard_domain_id)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['wildcard_domain_id'],
          message: t('Choose an available wildcard domain.'),
        })
      }
    })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
      slug: '',
      prefix: '',
      wildcard_domain_id: wildcards[0]?.id ?? 0,
      username: '',
      display_name: '',
      email: '',
      password: '',
      plan_id: initialPlan?.id ?? 0,
      code: '',
    },
  })
  const selectedPlan = props.plans.find(
    (plan) => plan.id === form.watch('plan_id')
  )
  const paid = selectedPlan !== undefined && selectedPlan.name !== 'Lite'
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: (values: z.infer<typeof schema>) => {
      const selected = props.plans.find((plan) => plan.id === values.plan_id)
      return createWorkspace({
        name: values.name,
        slug: wildcards.length > 0 ? values.prefix : values.slug,
        prefix: wildcards.length > 0 ? values.prefix : undefined,
        wildcard_domain_id:
          wildcards.length > 0 ? values.wildcard_domain_id : undefined,
        username: values.username,
        display_name: values.display_name,
        email: values.email,
        password: values.password,
        plan_id: values.plan_id,
        code: selected?.name === 'Lite' ? undefined : values.code,
      })
    },
    onSuccess: (data) => {
      setSetupURL(
        data.setup_url
          ? new URL(data.setup_url, window.location.origin).toString()
          : ''
      )
      setCreatedID(data.tenant.id)
      setCreatedURL(workspaceHref(data, '/'))
      form.reset()
      void queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Create workspace')}</CardTitle>
        <CardDescription>
          {t(
            'Each account can create one free Lite workspace. Other plans need a redemption code.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!props.liteAvailable && (
          <p className='text-muted-foreground mb-4'>
            {t(
              'You already have a free Lite workspace. Redeem a code to create another workspace.'
            )}
          </p>
        )}
        <form onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
          <FieldGroup>
            <Field data-invalid={!!form.formState.errors.plan_id}>
              <FieldLabel>{t('Hosting plan')}</FieldLabel>
              <RadioGroup
                className='grid gap-3 sm:grid-cols-3'
                value={selectedPlan ? String(selectedPlan.id) : ''}
                onValueChange={(value) =>
                  form.setValue('plan_id', Number(value), {
                    shouldValidate: true,
                  })
                }
                disabled={mutation.isPending}
              >
                {props.plans.map((plan) => {
                  const unavailable =
                    plan.name === 'Lite' && !props.liteAvailable
                  return (
                    <Label
                      key={plan.id}
                      htmlFor={`create-plan-${plan.id}`}
                      className={cn(
                        'border-muted bg-card hover:border-primary/40 focus-within:border-primary/50 has-data-[checked]:border-primary has-data-[checked]:ring-primary/20 flex cursor-pointer flex-col gap-2 rounded-lg border p-3 font-normal has-data-[checked]:ring-2',
                        unavailable && 'cursor-not-allowed opacity-60'
                      )}
                    >
                      <div className='flex items-start gap-2'>
                        <RadioGroupItem
                          id={`create-plan-${plan.id}`}
                          value={String(plan.id)}
                          disabled={unavailable || mutation.isPending}
                        />
                        <div>
                          <p className='font-medium'>{plan.name}</p>
                          <p className='text-muted-foreground text-xs'>
                            {t(plan.price)}
                          </p>
                        </div>
                      </div>
                      <p className='text-muted-foreground text-xs'>
                        {t('Monthly requests')}:{' '}
                        {plan.limits.requests === 0
                          ? t('Unlimited')
                          : plan.limits.requests.toLocaleString()}
                      </p>
                    </Label>
                  )
                })}
              </RadioGroup>
              <FieldError>{form.formState.errors.plan_id?.message}</FieldError>
            </Field>
            {paid && (
              <Field data-invalid={!!form.formState.errors.code}>
                <FieldLabel htmlFor='workspace-plan-code'>
                  {t('Redemption code')}
                </FieldLabel>
                <Input
                  id='workspace-plan-code'
                  autoComplete='off'
                  {...form.register('code')}
                  aria-invalid={!!form.formState.errors.code}
                  disabled={mutation.isPending}
                />
                <FieldError>{form.formState.errors.code?.message}</FieldError>
              </Field>
            )}
            <FieldGroup className='md:grid md:grid-cols-2'>
              <Field data-invalid={!!form.formState.errors.name}>
                <FieldLabel htmlFor='workspace-name'>{t('Name')}</FieldLabel>
                <Input
                  id='workspace-name'
                  {...form.register('name')}
                  aria-invalid={!!form.formState.errors.name}
                  disabled={mutation.isPending}
                />
                <FieldError>{form.formState.errors.name?.message}</FieldError>
              </Field>
              {wildcards.length === 0 ? (
                <Field data-invalid={!!form.formState.errors.slug}>
                  <FieldLabel htmlFor='workspace-slug'>
                    {t('Workspace ID')}
                  </FieldLabel>
                  <Input
                    id='workspace-slug'
                    {...form.register('slug')}
                    aria-invalid={!!form.formState.errors.slug}
                    disabled={mutation.isPending}
                    placeholder='my-workspace'
                  />
                  <FieldDescription>
                    {t(
                      'No wildcard domain is configured. You can bind a custom domain after creating the workspace.'
                    )}
                  </FieldDescription>
                  <FieldError>{form.formState.errors.slug?.message}</FieldError>
                </Field>
              ) : (
                <Field data-invalid={!!form.formState.errors.prefix}>
                  <FieldLabel htmlFor='workspace-prefix'>
                    {t('Workspace prefix')}
                  </FieldLabel>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Input
                      id='workspace-prefix'
                      {...form.register('prefix')}
                      aria-invalid={!!form.formState.errors.prefix}
                      disabled={mutation.isPending}
                      placeholder='docs'
                      className='min-w-0 flex-1'
                    />
                    <span className='text-muted-foreground'>.</span>
                    {wildcards.length === 1 ? (
                      <span className='text-muted-foreground'>
                        {wildcards[0].domain}
                      </span>
                    ) : (
                      <NativeSelect
                        id='workspace-wildcard-domain'
                        aria-label={t('Wildcard domain')}
                        value={String(form.watch('wildcard_domain_id') || '')}
                        onChange={(event) =>
                          form.setValue(
                            'wildcard_domain_id',
                            Number(event.target.value),
                            { shouldValidate: true }
                          )
                        }
                        disabled={mutation.isPending}
                      >
                        {wildcards.map((domain) => (
                          <NativeSelectOption
                            key={domain.id}
                            value={String(domain.id)}
                          >
                            {domain.domain}
                          </NativeSelectOption>
                        ))}
                      </NativeSelect>
                    )}
                  </div>
                  <FieldDescription>
                    {t(
                      'This prefix is independent of the workspace name. You can add more prefixes later.'
                    )}
                  </FieldDescription>
                  <FieldError>
                    {form.formState.errors.prefix?.message ??
                      form.formState.errors.wildcard_domain_id?.message}
                  </FieldError>
                </Field>
              )}
              <Field data-invalid={!!form.formState.errors.username}>
                <FieldLabel htmlFor='workspace-admin-username'>
                  {t('Username')}
                </FieldLabel>
                <Input
                  id='workspace-admin-username'
                  autoComplete='off'
                  {...form.register('username')}
                  aria-invalid={!!form.formState.errors.username}
                  disabled={mutation.isPending}
                />
                <FieldError>
                  {form.formState.errors.username?.message}
                </FieldError>
              </Field>
              <Field data-invalid={!!form.formState.errors.display_name}>
                <FieldLabel htmlFor='workspace-admin-display'>
                  {t('Display Name')}
                </FieldLabel>
                <Input
                  id='workspace-admin-display'
                  autoComplete='off'
                  {...form.register('display_name')}
                  aria-invalid={!!form.formState.errors.display_name}
                  disabled={mutation.isPending}
                />
                <FieldError>
                  {form.formState.errors.display_name?.message}
                </FieldError>
              </Field>
              <Field data-invalid={!!form.formState.errors.email}>
                <FieldLabel htmlFor='workspace-admin-email'>
                  {t('Email')}
                </FieldLabel>
                <Input
                  id='workspace-admin-email'
                  type='email'
                  autoComplete='off'
                  {...form.register('email')}
                  aria-invalid={!!form.formState.errors.email}
                  disabled={mutation.isPending}
                />
                <FieldError>{form.formState.errors.email?.message}</FieldError>
              </Field>
              <Field data-invalid={!!form.formState.errors.password}>
                <FieldLabel htmlFor='workspace-admin-password'>
                  {t('Password')}
                </FieldLabel>
                <PasswordInput
                  id='workspace-admin-password'
                  autoComplete='new-password'
                  {...form.register('password')}
                  aria-invalid={!!form.formState.errors.password}
                  disabled={mutation.isPending}
                />
                <FieldError>
                  {form.formState.errors.password?.message}
                </FieldError>
              </Field>
              {mutation.isError && <p role='alert'>{mutation.error.message}</p>}
              <Button type='submit' disabled={mutation.isPending}>
                {mutation.isPending && <LoadingState inline size='sm' />}
                {t('Create workspace')}
              </Button>
            </FieldGroup>
          </FieldGroup>
        </form>
        {createdID && (
          <div className='mt-6 flex flex-wrap items-center gap-3' role='status'>
            <p className='w-full'>{t('Workspace created.')}</p>
            {createdURL ? (
              <Button
                render={<a href={createdURL} />}
                nativeButton={false}
                role='link'
              >
                {t('Enter workspace')}
              </Button>
            ) : (
              <Button disabled>{t('Enter workspace')}</Button>
            )}
            {setupURL && (
              <Button
                variant='outline'
                render={<a href={setupURL} />}
                nativeButton={false}
                role='link'
              >
                {t('Finish setup')}
              </Button>
            )}
            <WorkspaceLink id={createdID}>
              {t('Manage workspace')}
            </WorkspaceLink>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
