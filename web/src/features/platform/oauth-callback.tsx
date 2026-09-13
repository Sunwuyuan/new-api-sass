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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useRouter } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { AuthLayoutFrame } from '@/features/auth/auth-layout'

import { finishPlatformOAuth, updatePlatformSession } from './auth-api'
import { PlatformBrand } from './brand'
import { safePlatformRedirect } from './lib/auth-redirect'
import { PlatformLink } from './navigation'

export default function PlatformOAuthCallback() {
  const { t } = useTranslation()
  const router = useRouter()
  const queryClient = useQueryClient()
  const params = useParams({ strict: false }) as { provider: string }
  const started = useRef(false)
  const complete = useMutation({
    meta: { errorToast: false },
    retry: false,
    mutationFn: async () => {
      // The server moves callback parameters into the fragment before serving
      // HTML, so proxy-injected resources cannot receive them in a Referer.
      const search = new URLSearchParams(
        window.location.hash.slice(1) || window.location.search
      )
      const state = search.get('state') || ''
      const code = search.get('code') || ''
      const error = search.get('error') || ''
      // Authorization codes stay out of router caches and subsequent referrers.
      window.history.replaceState(
        window.history.state,
        '',
        window.location.pathname
      )
      return finishPlatformOAuth(params.provider, { state, code, error })
    },
    onSuccess: async (session) => {
      await updatePlatformSession(queryClient, session)
      await router.navigate({
        href: safePlatformRedirect(session.redirect),
        replace: true,
      })
      await router.invalidate()
    },
  })
  const finish = complete.mutate
  useEffect(() => {
    if (started.current) return
    started.current = true
    finish()
  }, [finish])
  return (
    <AuthLayoutFrame brand={<PlatformBrand />}>
      {complete.isError ? (
        <ErrorState
          title={t('Login failed')}
          description={complete.error.message}
          action={
            <PlatformLink to='/platform/sign-in'>
              {t('Back to sign in')}
            </PlatformLink>
          }
        />
      ) : (
        <LoadingState message={t('Signing in...')} />
      )}
    </AuthLayoutFrame>
  )
}
