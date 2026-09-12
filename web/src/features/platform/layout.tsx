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
import { Outlet, useRouter, useRouterState } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LanguageSwitcher } from '@/components/language-switcher'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'

import { platformLogout, platformSessionQuery } from './api'
import { PlatformAuthForm } from './auth-form'
import { HostingPlans } from './hosting-plans'
import { PlatformAccessDenied, PlatformLink } from './navigation'
import { PlatformPasswordDialog } from './password-dialog'

export function PlatformLayout() {
  const { t } = useTranslation()
  const router = useRouter()
  const [passwordOpen, setPasswordOpen] = useState(false)
  const queryClient = useQueryClient()
  const session = useQuery(platformSessionQuery)
  const path = useRouterState({ select: (state) => state.location.pathname })
  const admin =
    !!session.data && ['admin', 'root'].includes(session.data.user.role)
  const passwordRequired = session.data?.user.must_change_password
  const logout = useMutation({
    mutationFn: platformLogout,
    onSuccess: async () => {
      await queryClient.cancelQueries({ queryKey: ['platform'] })
      queryClient.removeQueries({
        queryKey: ['platform'],
        predicate: (query) => query.queryKey[1] !== 'session',
      })
      queryClient.setQueryData(['platform', 'session'], null)
    },
  })
  return (
    <main className='mx-auto flex min-h-svh max-w-7xl flex-col gap-6 px-4 py-6 sm:px-8'>
      <header className='flex flex-wrap items-center justify-between gap-4 border-b pb-5'>
        <div>
          <h1 className='text-2xl font-semibold'>New API SaaS</h1>
          <p className='text-muted-foreground'>
            {t('Your independent API workspaces')}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <LanguageSwitcher />
          {session.data && (
            <Button variant='outline' onClick={() => setPasswordOpen(true)}>
              {t('Change platform password')}
            </Button>
          )}
          {session.data && (
            <Button
              variant='outline'
              onClick={() => logout.mutate()}
              disabled={logout.isPending}
            >
              {t('Sign out')}
            </Button>
          )}
        </div>
      </header>
      {session.isPending && <LoadingState />}
      {session.isError && (
        <ErrorState
          title={t('Unable to complete the request')}
          onRetry={() => void session.refetch()}
        />
      )}
      {session.data === null && (
        <>
          <PlatformAuthForm onSignedIn={() => void router.invalidate()} />
          <HostingPlans />
        </>
      )}
      {session.data && (
        <>
          <nav
            aria-label={t('Platform navigation')}
            className='flex flex-wrap gap-2'
          >
            <PlatformLink to='/platform'>{t('My workspaces')}</PlatformLink>
            {admin && !passwordRequired && (
              <PlatformLink to='/platform/admin/users'>
                {t('Administration')}
              </PlatformLink>
            )}
          </nav>
          {(passwordOpen || passwordRequired) && (
            <PlatformPasswordDialog
              open
              required={passwordRequired}
              onOpenChange={setPasswordOpen}
            />
          )}
          {!passwordRequired &&
            (path.startsWith('/platform/admin') && !admin ? (
              <PlatformAccessDenied />
            ) : (
              <Outlet />
            ))}
        </>
      )}
      <footer className='text-muted-foreground mt-auto pt-6 text-center text-sm'>
        <a
          href='https://github.com/QuantumNous/new-api'
          target='_blank'
          rel='noreferrer'
        >
          New API · QuantumNous
        </a>
      </footer>
      <Toaster />
    </main>
  )
}
