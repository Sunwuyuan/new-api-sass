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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import {
  getPlatformSettings,
  updatePlatformSettings,
  type PlatformOAuthProviderSetting,
} from '../api'
import { AdminWildcardDomains } from './wildcard-domains'

type ProviderDraft = PlatformOAuthProviderSetting & { client_secret: string }

export default function PlatformAdminSettings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['platform', 'admin', 'settings'],
    queryFn: getPlatformSettings,
  })
  const [registration, setRegistration] = useState(true)
  const [passwordLogin, setPasswordLogin] = useState(true)
  const [oauthRegistration, setOauthRegistration] = useState(true)
  const [passkey, setPasskey] = useState(false)
  const [providers, setProviders] = useState<ProviderDraft[]>([])
  const [mailEnabled, setMailEnabled] = useState(false)
  const [mailFrom, setMailFrom] = useState('')
  const [mailFromName, setMailFromName] = useState('')
  const [mailBase, setMailBase] = useState('')
  const [mailProvider, setMailProvider] = useState('auto')
  const [mailKey, setMailKey] = useState('')
  const [emailVerification, setEmailVerification] = useState(false)
  const [notifications, setNotifications] = useState(false)
  useEffect(() => {
    if (!query.data) return
    setRegistration(query.data.auth.registration)
    setPasswordLogin(query.data.auth.password_login)
    setOauthRegistration(query.data.auth.oauth_registration)
    setPasskey(query.data.auth.passkey)
    setProviders(
      query.data.auth.providers.map((provider) => ({
        ...provider,
        client_secret: '',
      }))
    )
    setMailEnabled(query.data.mail.enabled)
    setMailFrom(query.data.mail.from)
    setMailFromName(query.data.mail.from_name)
    setMailBase(query.data.mail.base_url)
    setMailProvider(query.data.mail.provider_id)
    setEmailVerification(query.data.mail.email_verification)
    setNotifications(query.data.mail.notifications)
  }, [query.data])
  const mutation = useMutation({
    meta: { errorToast: false },
    mutationFn: () =>
      updatePlatformSettings({
        auth: {
          registration,
          password_login: passwordLogin,
          oauth_registration: oauthRegistration,
          passkey,
          providers: providers.map((provider) => ({
            slug: provider.slug,
            name: provider.name,
            enabled: provider.enabled,
            client_id: provider.client_id,
            issuer: provider.issuer,
            configured: provider.configured,
            client_secret: provider.client_secret,
          })),
        },
        mail: {
          enabled: mailEnabled,
          from: mailFrom,
          from_name: mailFromName,
          base_url: mailBase,
          provider_id: mailProvider || 'auto',
          api_key: mailKey,
          email_verification: emailVerification,
          notifications,
        },
      }),
    onSuccess: async () => {
      setMailKey('')
      toast.success(t('Platform settings saved'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  if (query.isPending) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  return (
    <div className='max-w-3xl space-y-6'>
      <header className='space-y-1'>
        <h1 className='text-2xl font-semibold'>{t('Platform settings')}</h1>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Configure how people sign in to the platform, and how the platform sends mail.'
          )}
        </p>
      </header>
      <AdminWildcardDomains />
      <Card>
        <CardHeader>
          <CardTitle>{t('Sign-in')}</CardTitle>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-registration'
                checked={registration}
                onCheckedChange={(value) => setRegistration(value === true)}
              />
              <FieldLabel htmlFor='platform-registration'>
                {t('Allow registration')}
              </FieldLabel>
            </Field>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-password-login'
                checked={passwordLogin}
                onCheckedChange={(value) => setPasswordLogin(value === true)}
              />
              <FieldLabel htmlFor='platform-password-login'>
                {t('Password sign-in')}
              </FieldLabel>
            </Field>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-oauth-registration'
                checked={oauthRegistration}
                onCheckedChange={(value) =>
                  setOauthRegistration(value === true)
                }
              />
              <FieldLabel htmlFor='platform-oauth-registration'>
                {t('Allow creating accounts with OAuth')}
              </FieldLabel>
            </Field>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-passkey'
                checked={passkey}
                onCheckedChange={(value) => setPasskey(value === true)}
              />
              <FieldLabel htmlFor='platform-passkey'>Passkey</FieldLabel>
            </Field>
          </FieldGroup>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('OAuth providers')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='space-y-6'>
            {providers.map((provider, index) => (
              <FieldGroup key={provider.slug} className='rounded-md border p-4'>
                <Field orientation='horizontal'>
                  <Checkbox
                    id={`platform-oauth-${provider.slug}`}
                    checked={provider.enabled}
                    onCheckedChange={(value) => {
                      const next = [...providers]
                      next[index] = { ...provider, enabled: value === true }
                      setProviders(next)
                    }}
                  />
                  <FieldLabel htmlFor={`platform-oauth-${provider.slug}`}>
                    {provider.name}
                  </FieldLabel>
                </Field>
                <div className='grid gap-4 sm:grid-cols-2'>
                  <Field>
                    <FieldLabel htmlFor={`platform-oauth-${provider.slug}-id`}>
                      {t('Client ID')}
                    </FieldLabel>
                    <Input
                      id={`platform-oauth-${provider.slug}-id`}
                      value={provider.client_id}
                      autoComplete='off'
                      onChange={(event) => {
                        const next = [...providers]
                        next[index] = {
                          ...provider,
                          client_id: event.target.value,
                        }
                        setProviders(next)
                      }}
                    />
                  </Field>
                  <Field>
                    <FieldLabel
                      htmlFor={`platform-oauth-${provider.slug}-secret`}
                    >
                      {t('Client secret')}
                    </FieldLabel>
                    <Input
                      id={`platform-oauth-${provider.slug}-secret`}
                      type='password'
                      autoComplete='off'
                      placeholder={
                        provider.configured
                          ? t('Leave blank to keep the existing credential')
                          : undefined
                      }
                      value={provider.client_secret}
                      onChange={(event) => {
                        const next = [...providers]
                        next[index] = {
                          ...provider,
                          client_secret: event.target.value,
                        }
                        setProviders(next)
                      }}
                    />
                  </Field>
                  {(provider.slug === 'logto' || provider.slug === 'oidc') && (
                    <Field className='sm:col-span-2'>
                      <FieldLabel
                        htmlFor={`platform-oauth-${provider.slug}-issuer`}
                      >
                        {t('Issuer URL')}
                      </FieldLabel>
                      <Input
                        id={`platform-oauth-${provider.slug}-issuer`}
                        value={provider.issuer}
                        placeholder='https://auth.example.com'
                        autoComplete='off'
                        onChange={(event) => {
                          const next = [...providers]
                          next[index] = {
                            ...provider,
                            issuer: event.target.value,
                          }
                          setProviders(next)
                        }}
                      />
                    </Field>
                  )}
                </div>
              </FieldGroup>
            ))}
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('Platform mail')}</CardTitle>
          <CardDescription>
            {t(
              'Amail sends verification, notices, and workspace mail when a plan includes the platform email quota.'
            )}{' '}
            <a
              href='https://amail.wuyuan.dev/'
              className='underline underline-offset-4'
              target='_blank'
              rel='noreferrer'
            >
              amail.wuyuan.dev
            </a>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-mail-enabled'
                checked={mailEnabled}
                onCheckedChange={(value) => setMailEnabled(value === true)}
              />
              <FieldLabel htmlFor='platform-mail-enabled'>
                {t('Enable platform mail')}
              </FieldLabel>
            </Field>
            <div className='grid gap-4 sm:grid-cols-2'>
              <Field>
                <FieldLabel htmlFor='platform-mail-from'>
                  {t('From Address')}
                </FieldLabel>
                <Input
                  id='platform-mail-from'
                  value={mailFrom}
                  autoComplete='off'
                  onChange={(event) => setMailFrom(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='platform-mail-from-name'>
                  {t('From name')}
                </FieldLabel>
                <Input
                  id='platform-mail-from-name'
                  value={mailFromName}
                  autoComplete='off'
                  onChange={(event) => setMailFromName(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='platform-mail-base'>
                  {t('API URL')}
                </FieldLabel>
                <Input
                  id='platform-mail-base'
                  value={mailBase}
                  autoComplete='off'
                  onChange={(event) => setMailBase(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='platform-mail-provider'>
                  {t('Provider ID')}
                </FieldLabel>
                <Input
                  id='platform-mail-provider'
                  value={mailProvider}
                  autoComplete='off'
                  onChange={(event) => setMailProvider(event.target.value)}
                />
              </Field>
              <Field className='sm:col-span-2'>
                <FieldLabel htmlFor='platform-mail-key'>
                  {t('API Key')}
                </FieldLabel>
                <Input
                  id='platform-mail-key'
                  type='password'
                  autoComplete='off'
                  placeholder={
                    query.data.mail.configured
                      ? t('Leave blank to keep the existing credential')
                      : undefined
                  }
                  value={mailKey}
                  onChange={(event) => setMailKey(event.target.value)}
                />
              </Field>
            </div>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-mail-verify'
                checked={emailVerification}
                onCheckedChange={(value) =>
                  setEmailVerification(value === true)
                }
              />
              <FieldLabel htmlFor='platform-mail-verify'>
                {t('Require email verification')}
              </FieldLabel>
            </Field>
            <Field orientation='horizontal'>
              <Checkbox
                id='platform-mail-notify'
                checked={notifications}
                onCheckedChange={(value) => setNotifications(value === true)}
              />
              <FieldLabel htmlFor='platform-mail-notify'>
                {t('Send account notices')}
              </FieldLabel>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter className='justify-end gap-2'>
          {mutation.isError && (
            <p role='alert' className='text-destructive mr-auto text-sm'>
              {mutation.error.message}
            </p>
          )}
          <Button
            type='button'
            disabled={mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending && <LoadingState inline size='sm' />}
            {t('Save changes')}
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}
