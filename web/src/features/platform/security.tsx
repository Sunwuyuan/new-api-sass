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
import { Key01Icon, Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  buildRegistrationResult,
  createCredential,
  prepareCredentialCreationOptions,
  isPasskeySupported,
} from '@/lib/passkey'
import { AuthOperationError } from '@/lib/secure-verification'

import {
  beginPlatformPasskey,
  finishPlatformPasskey,
  platformAuthMethodsQuery,
  platformStatusQuery,
  removePlatformPasskey,
  updatePlatformSession,
} from './auth-api'
import { PlatformLoginMethods } from './login-methods'
import { PlatformPasswordDialog } from './password-dialog'
import { ReauthenticateDialog } from './reauthenticate-dialog'
import type { PlatformSession } from './types'

export default function PlatformSecurity() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const busy = useIsMutating({ mutationKey: ['platform', 'authenticate'] }) > 0
  const status = useQuery(platformStatusQuery)
  const methods = useQuery(platformAuthMethodsQuery)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [verifyOpen, setVerifyOpen] = useState(false)
  const [deleteID, setDeleteID] = useState<string | null>(null)
  const [supported, setSupported] = useState(false)
  const operation = useRef<AbortController | null>(null)
  useEffect(() => {
    let active = true
    void isPasskeySupported()
      .then((value) => {
        if (active) setSupported(value)
      })
      .catch(() => {
        if (active) setSupported(false)
      })
    return () => {
      active = false
      operation.current?.abort()
    }
  }, [])
  async function refresh(session: PlatformSession) {
    await updatePlatformSession(queryClient, session)
    await methods.refetch()
  }
  const registration = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: async () => {
      if (operation.current) {
        throw new Error(t('A security operation is already in progress.'))
      }
      const controller = new AbortController()
      operation.current = controller
      try {
        const flow = await beginPlatformPasskey('register', controller.signal)
        if (!flow.flow_token) {
          throw new Error(t('Registration flow expired. Please try again.'))
        }
        const credential = (await createCredential(
          prepareCredentialCreationOptions(flow.options ?? flow),
          controller.signal
        )) as PublicKeyCredential | null
        if (!credential) {
          throw new Error(t('Passkey registration was cancelled'))
        }
        const result = buildRegistrationResult(credential)
        if (!result) throw new Error(t('Invalid Passkey registration response'))
        return await finishPlatformPasskey(
          'register',
          flow.flow_token,
          result,
          controller.signal
        )
      } catch (error) {
        throw AuthOperationError.from(error)
      } finally {
        operation.current = null
      }
    },
    onSuccess: refresh,
  })
  const removal = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: removePlatformPasskey,
    onSuccess: async (session) => {
      setDeleteID(null)
      await refresh(session)
    },
  })
  if (status.isPending || methods.isPending) return <LoadingState />
  if (status.isError || methods.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void status.refetch()
          void methods.refetch()
        }}
      />
    )
  }
  const linked = methods.data.providers
  const linkable = {
    ...status.data,
    github_oauth: status.data.github_oauth && !linked.includes('github'),
    discord_oauth: status.data.discord_oauth && !linked.includes('discord'),
    oidc_enabled: status.data.oidc_enabled && !linked.includes('oidc'),
    linuxdo_oauth: status.data.linuxdo_oauth && !linked.includes('linuxdo'),
    telegram_oauth: status.data.telegram_oauth && !linked.includes('telegram'),
    wechat_login: status.data.wechat_login && !linked.includes('wechat'),
    custom_oauth_providers: status.data.custom_oauth_providers?.filter(
      (provider) => !linked.includes(provider.slug)
    ),
  }
  return (
    <section className='space-y-6'>
      <header className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('Account security')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Manage sign-in methods for your platform account.')}
          </p>
        </div>
        <Button
          variant='outline'
          disabled={busy}
          onClick={() => setVerifyOpen(true)}
        >
          {t('Verify your identity')}
        </Button>
      </header>
      <Card>
        <CardHeader>
          <CardTitle>{t('Password')}</CardTitle>
          <CardDescription>
            {t('Platform and workspace accounts are separate.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {methods.data.has_password ? (
            <Button
              variant='outline'
              disabled={busy}
              onClick={() => setPasswordOpen(true)}
            >
              {t('Change platform password')}
            </Button>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('This account uses an external sign-in method.')}
            </p>
          )}
        </CardContent>
      </Card>
      {status.data.passkey_login && (
        <Card>
          <CardHeader>
            <CardTitle>Passkey</CardTitle>
            <CardDescription>
              {t('Use your device or a security key to sign in.')}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            {methods.data.passkeys.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('No Passkeys registered')}
              </p>
            ) : (
              <ul className='divide-y'>
                {methods.data.passkeys.map((passkey, index) => (
                  <li
                    key={passkey.id}
                    className='flex flex-wrap items-center justify-between gap-3 py-3'
                  >
                    <div>
                      <p className='text-sm font-medium'>
                        {t('Passkey {{number}}', { number: index + 1 })}
                      </p>
                      <p className='text-muted-foreground text-xs'>
                        {t('Created at')}:{' '}
                        {new Date(passkey.created_at).toLocaleString()}
                      </p>
                    </div>
                    <Button
                      variant='outline'
                      size='sm'
                      disabled={busy}
                      onClick={() => {
                        removal.reset()
                        setDeleteID(passkey.id)
                      }}
                    >
                      {t('Delete')}
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            <Button
              variant='outline'
              disabled={
                !supported || busy || methods.data.passkeys.length >= 10
              }
              onClick={() => registration.mutate()}
            >
              <HugeiconsIcon
                icon={registration.isPending ? Loading03Icon : Key01Icon}
                className={registration.isPending ? 'animate-spin' : undefined}
                data-icon='inline-start'
              />
              {t('Register Passkey')}
            </Button>
            {!supported && (
              <p className='text-muted-foreground text-xs'>
                {t('Passkey is not supported on this device.')}
              </p>
            )}
            {registration.isError && (
              <p role='alert' className='text-destructive text-sm'>
                {registration.error.message}
              </p>
            )}
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle>{t('Connected accounts')}</CardTitle>
          <CardDescription>
            {t('Verify your identity before connecting a sign-in method.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-5'>
          {linked.length ? (
            <div className='flex flex-wrap gap-2'>
              {linked.map((provider) => (
                <Badge key={provider} variant='secondary'>
                  {status.data.custom_oauth_providers?.find(
                    (item) => item.slug === provider
                  )?.name || provider}
                </Badge>
              ))}
            </div>
          ) : (
            <EmptyState
              title={t('No connected accounts')}
              className='min-h-20'
            />
          )}
          <div className='max-w-sm'>
            <PlatformLoginMethods
              status={linkable}
              intent='link'
              redirectTo='/platform/security'
              showPasskey={false}
              onSuccess={refresh}
            />
          </div>
        </CardContent>
      </Card>
      {passwordOpen && (
        <PlatformPasswordDialog open onOpenChange={setPasswordOpen} />
      )}
      {verifyOpen && (
        <ReauthenticateDialog
          title={t('Verify your identity')}
          onClose={() => setVerifyOpen(false)}
        />
      )}
      {deleteID && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !removal.isPending) setDeleteID(null)
          }}
          title={t('Delete Passkey')}
          desc={t(
            'This removes the Passkey and signs out other platform sessions.'
          )}
          confirmText={t('Delete')}
          destructive
          handleConfirm={() => removal.mutate(deleteID)}
          isLoading={removal.isPending}
        >
          {removal.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {removal.error.message}
            </p>
          )}
        </ConfirmDialog>
      )}
    </section>
  )
}
