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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LanguageSwitcher } from '@/components/language-switcher'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Toaster } from '@/components/ui/sonner'

import {
  getHostingPlans,
  getPlatformSession,
  getPlatformUsers,
  getWorkspaces,
  platformLogout,
} from './api'
import { PlatformAuthForm } from './auth-form'
import { CreateWorkspace } from './create-workspace'
import { PlatformPasswordDialog } from './password-dialog'
import { WorkspaceCard } from './workspace-card'

export default function PlatformApp() {
  const { t } = useTranslation()
  const [passwordOpen, setPasswordOpen] = useState(false)
  const queryClient = useQueryClient()
  const session = useQuery({
    queryKey: ['platform', 'session'],
    queryFn: getPlatformSession,
    retry: false,
  })
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const admin = session.data?.user.role === 'admin'
  const workspaces = useQuery({
    queryKey: ['platform', 'tenants', session.data?.user.id, admin],
    queryFn: () => getWorkspaces(admin),
    enabled: !!session.data,
  })
  const users = useQuery({
    queryKey: ['platform', 'users', session.data?.user.id],
    queryFn: getPlatformUsers,
    enabled: admin,
  })
  const logout = useMutation({
    mutationFn: platformLogout,
    onSuccess: () => {
      queryClient.setQueryData(['platform', 'session'], null)
      queryClient.removeQueries({ queryKey: ['platform', 'tenants'] })
      queryClient.removeQueries({ queryKey: ['platform', 'users'] })
    },
  })
  return (
    <main className='mx-auto flex min-h-svh max-w-6xl flex-col gap-8 px-4 py-8 sm:px-8'>
      <header className='flex flex-wrap items-center justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>New API SaaS</h1>
          <p className='text-muted-foreground'>
            {t('Your independent API workspaces')}
          </p>
        </div>
        <div className='flex items-center gap-3'>
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
      {session.data === null && <PlatformAuthForm />}
      <section
        aria-label={t('Hosting plans')}
        className='grid gap-4 sm:grid-cols-2'
      >
        {plans.isPending && <LoadingState />}
        {plans.isError && (
          <ErrorState
            title={t('Unable to complete the request')}
            onRetry={() => void plans.refetch()}
          />
        )}
        {plans.data?.map((plan) => (
          <Card key={plan.id}>
            <CardHeader>
              <CardTitle>
                {plan.name} · {t(plan.price)}
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-2'>
              <p>
                {t('Monthly requests')}: {plan.limits.requests.toLocaleString()}
              </p>
              <p>
                {t('Users')}: {plan.limits.users} · {t('Tokens')}:{' '}
                {plan.limits.tokens} · {t('Channels')}: {plan.limits.channels}
              </p>
              <p>
                {plan.capabilities.remove_platform_footer
                  ? t('Custom platform footer')
                  : t('Platform footer required')}
              </p>
            </CardContent>
          </Card>
        ))}
      </section>
      {session.data && (
        <>
          <CreateWorkspace key={session.data.user.id} />
          {passwordOpen && (
            <PlatformPasswordDialog open onOpenChange={setPasswordOpen} />
          )}
          <section
            aria-label={t('Workspaces')}
            className='grid gap-4 md:grid-cols-2'
          >
            {workspaces.isPending && <LoadingState />}
            {workspaces.isError && (
              <ErrorState
                title={t('Unable to complete the request')}
                onRetry={() => void workspaces.refetch()}
              />
            )}
            {workspaces.data?.length === 0 && (
              <EmptyState title={t('No workspaces yet')} />
            )}
            {workspaces.data?.map((item) => (
              <WorkspaceCard
                key={item.tenant.id}
                item={item}
                plans={plans.data ?? []}
                admin={admin}
              />
            ))}
          </section>
          {admin && (
            <section aria-label={t('Platform users')} className='space-y-3'>
              <h2 className='text-lg font-semibold'>{t('Platform users')}</h2>
              {users.isPending && <LoadingState />}
              {users.isError && (
                <ErrorState
                  title={t('Unable to complete the request')}
                  onRetry={() => void users.refetch()}
                />
              )}
              {users.data?.map((user) => (
                <p key={user.id}>
                  {user.email} ·{' '}
                  {user.role === 'admin' ? t('Administrator') : t('User')} ·{' '}
                  {t('Workspaces')}: {user.tenant_count}
                </p>
              ))}
            </section>
          )}
        </>
      )}
      <footer className='text-muted-foreground mt-auto text-center text-sm'>
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
