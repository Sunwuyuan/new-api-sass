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
import { Logout01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Outlet, useRouter, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LanguageSwitcher } from '@/components/language-switcher'
import { LoadingState } from '@/components/loading-state'
import { SkipToMain } from '@/components/skip-to-main'
import { Button } from '@/components/ui/button'
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import { Toaster } from '@/components/ui/sonner'

import { platformLogout, platformSessionQuery } from './api'
import { updatePlatformSession } from './auth-api'
import { PlatformPasswordDialog } from './password-dialog'
import { PlatformSidebar } from './sidebar'

export function PlatformRoot() {
  return (
    <>
      <Outlet />
      <Toaster />
    </>
  )
}

export function PlatformLayout() {
  const { t } = useTranslation()
  const router = useRouter()
  const queryClient = useQueryClient()
  const session = useQuery(platformSessionQuery)
  const path = useRouterState({ select: (state) => state.location.pathname })
  useEffect(() => {
    if (session.data === null) void router.invalidate()
  }, [session.data, router])
  const logout = useMutation({
    mutationFn: platformLogout,
    onSuccess: async () => {
      await updatePlatformSession(queryClient, null)
      await router.navigate({ href: '/platform/sign-in', replace: true })
      await router.invalidate()
    },
  })
  if (session.isError) {
    return <ErrorState onRetry={() => void session.refetch()} />
  }
  if (!session.data) return <LoadingState />
  return (
    <SidebarProvider>
      <SkipToMain />
      <PlatformSidebar user={session.data.user} />
      <SidebarInset className='min-w-0'>
        <header className='bg-background/95 sticky top-0 z-20 flex h-14 items-center justify-between gap-3 border-b px-4 sm:px-6'>
          <div className='flex min-w-0 items-center gap-3'>
            <SidebarTrigger />
            <span className='text-muted-foreground truncate text-sm'>
              {path.startsWith('/platform/admin')
                ? t('Administration')
                : t('Workspace')}
            </span>
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            <LanguageSwitcher />
            <Button
              variant='ghost'
              size='sm'
              onClick={() => logout.mutate()}
              disabled={logout.isPending}
            >
              <HugeiconsIcon icon={Logout01Icon} data-icon='inline-start' />
              {t('Sign out')}
            </Button>
          </div>
        </header>
        <div
          id='content'
          className='mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 p-4 sm:p-6 lg:p-8'
        >
          {session.data.user.must_change_password ? (
            <PlatformPasswordDialog
              open
              required
              onOpenChange={() => undefined}
            />
          ) : (
            <Outlet />
          )}
        </div>
        <footer className='text-muted-foreground px-6 py-4 text-xs'>
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noreferrer'
          >
            New API · QuantumNous
          </a>
        </footer>
      </SidebarInset>
    </SidebarProvider>
  )
}
