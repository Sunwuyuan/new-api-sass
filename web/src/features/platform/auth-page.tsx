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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRouter, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LanguageSwitcher } from '@/components/language-switcher'
import { LoadingState } from '@/components/loading-state'
import { AuthLayoutFrame } from '@/features/auth/auth-layout'
import { TermsFooter } from '@/features/auth/components/terms-footer'

import { platformStatusQuery, updatePlatformSession } from './auth-api'
import { PlatformAuthForm } from './auth-form'
import { PlatformBrand } from './brand'
import { safePlatformRedirect } from './lib/auth-redirect'
import { PlatformLink } from './navigation'
import type { PlatformSession } from './types'

export function PlatformSignIn() {
  return <PlatformAuthPage mode='sign-in' />
}
export function PlatformSignUp() {
  return <PlatformAuthPage mode='sign-up' />
}

function PlatformAuthPage(props: { mode: 'sign-in' | 'sign-up' }) {
  const { t } = useTranslation()
  const status = useQuery(platformStatusQuery)
  const queryClient = useQueryClient()
  const router = useRouter()
  const search = useSearch({ strict: false }) as {
    redirect?: string
    registered?: boolean
  }
  const target = safePlatformRedirect(search.redirect)
  const signUp = props.mode === 'sign-up'
  async function signedIn(session: PlatformSession) {
    await updatePlatformSession(queryClient, session)
    await router.navigate({ href: target, replace: true })
    await router.invalidate()
  }
  return (
    <AuthLayoutFrame brand={<PlatformBrand />} actions={<LanguageSwitcher />}>
      {status.isPending && <LoadingState />}
      {status.isError && <ErrorState onRetry={() => void status.refetch()} />}
      {status.data && (
        <div className='w-full space-y-8'>
          <div className='space-y-2'>
            <h1 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
              {signUp ? t('Create an account') : t('Sign in')}
            </h1>
            {(signUp || status.data.register_enabled) && (
              <p className='text-muted-foreground text-left text-sm sm:text-base'>
                {signUp
                  ? t('Already have an account?')
                  : t("Don't have an account?")}{' '}
                <PlatformLink
                  to={signUp ? '/platform/sign-in' : '/platform/sign-up'}
                  search={{ redirect: target }}
                  className='hover:text-primary font-medium underline underline-offset-4'
                >
                  {signUp ? t('Sign in') : t('Sign up')}
                </PlatformLink>
                .
              </p>
            )}
          </div>
          {search.registered && !signUp && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t('Registration submitted. Sign in with your credentials.')}
            </p>
          )}
          {signUp && !status.data.register_enabled ? (
            <ErrorState title={t('Registration is disabled')} />
          ) : (
            <PlatformAuthForm
              mode={props.mode}
              status={status.data}
              redirectTo={target}
              onSignedIn={signedIn}
              onRegistered={() =>
                router.navigate({
                  href: `/platform/sign-in?registered=true&redirect=${encodeURIComponent(target)}`,
                  replace: true,
                })
              }
            />
          )}
          <TermsFooter
            variant={props.mode}
            status={status.data}
            agreementUrl={status.data.user_agreement_url}
            privacyUrl={status.data.privacy_policy_url}
          />
          <p className='text-muted-foreground text-center text-xs'>
            {t('Platform and workspace accounts are separate.')}
          </p>
        </div>
      )}
    </AuthLayoutFrame>
  )
}
