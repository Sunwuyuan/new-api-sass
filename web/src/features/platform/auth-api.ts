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
import type { QueryClient } from '@tanstack/react-query'

import type { OAuthProviderStatus } from '@/features/auth/components/oauth-providers'
import type { PasskeyOptionsPayload } from '@/features/auth/passkey/types'

import {
  getPlatformSession,
  platformClient,
  setPlatformSessionCredentials,
} from './api'
import type { PlatformSession } from './types'

export type PlatformAuthStatus = OAuthProviderStatus & {
  system_name: string
  logo: string
  register_enabled: boolean
  password_login_enabled: boolean
  password_register_enabled: boolean
  oauth_register_enabled: boolean
  passkey_login: boolean
  reauthentication_providers: string[]
  wechat_qrcode?: string
  user_agreement_enabled: boolean
  privacy_policy_enabled: boolean
  user_agreement_url: string
  privacy_policy_url: string
}

export type PlatformAuthIntent = 'login' | 'link' | 'verify'
export type PlatformAuthMethods = {
  providers: string[]
  has_password: boolean
  passkeys: Array<{
    id: string
    created_at: string
    last_used_at: string | null
  }>
}

export function platformVerificationMethods(
  status: PlatformAuthStatus,
  methods: PlatformAuthMethods
): PlatformAuthStatus {
  return {
    ...status,
    github_oauth: false,
    discord_oauth: false,
    linuxdo_oauth: false,
    oidc_enabled:
      status.oidc_enabled &&
      status.reauthentication_providers.includes('oidc') &&
      methods.providers.includes('oidc'),
    telegram_oauth:
      status.telegram_oauth &&
      status.reauthentication_providers.includes('telegram') &&
      methods.providers.includes('telegram'),
    wechat_login: status.wechat_login && methods.providers.includes('wechat'),
    passkey_login: status.passkey_login && methods.passkeys.length > 0,
    custom_oauth_providers: status.custom_oauth_providers?.filter(
      (provider) =>
        status.reauthentication_providers.includes(provider.slug) &&
        methods.providers.includes(provider.slug)
    ),
  }
}

export const platformStatusQuery = {
  queryKey: ['platform', 'status'],
  queryFn: async (): Promise<PlatformAuthStatus> =>
    (await platformClient.get<PlatformAuthStatus>('/status')).data,
  staleTime: 60_000,
}

export const platformAuthMethodsQuery = {
  queryKey: ['platform', 'auth-methods'],
  queryFn: async (): Promise<PlatformAuthMethods> =>
    (await platformClient.get<PlatformAuthMethods>('/auth-methods')).data,
}

export async function updatePlatformSession(
  queryClient: QueryClient,
  session: PlatformSession | null
) {
  await queryClient.cancelQueries({ queryKey: ['platform'] })
  queryClient.removeQueries({
    queryKey: ['platform'],
    predicate: (query) =>
      !['session', 'status'].includes(String(query.queryKey[1])),
  })
  // The API client keeps the rotated CSRF token only in memory.
  setPlatformSessionCredentials(session)
  queryClient.setQueryData(['platform', 'session'], session)
}

export async function beginPlatformOAuth(
  provider: string,
  intent: PlatformAuthIntent,
  redirect: string
): Promise<string> {
  const action = intent === 'login' ? 'start' : intent
  const response = await platformClient.post<{ authorization_url: string }>(
    `/oauth/${encodeURIComponent(provider)}/${action}`,
    { redirect }
  )
  return response.data.authorization_url
}

export async function finishPlatformOAuth(
  provider: string,
  input: { state: string; code?: string; error?: string }
): Promise<PlatformSession> {
  // Authenticated linking/verification callbacks also require the current CSRF token.
  await getPlatformSession()
  return (
    await platformClient.post<PlatformSession>(
      `/oauth/${encodeURIComponent(provider)}/finish`,
      input
    )
  ).data
}

export async function platformWeChatLogin(
  code: string,
  intent: PlatformAuthIntent,
  redirect: string
): Promise<PlatformSession> {
  const action = intent === 'login' ? 'start' : intent
  const flow = await platformClient.post<{ flow_token: string }>(
    `/wechat/${action}`,
    { redirect }
  )
  return (
    await platformClient.post<PlatformSession>('/wechat/finish', {
      code,
      flow_token: flow.data.flow_token,
    })
  ).data
}

export async function beginPlatformPasskey(
  intent: 'login' | 'verify' | 'register',
  signal?: AbortSignal
): Promise<PasskeyOptionsPayload> {
  return (
    await platformClient.post<PasskeyOptionsPayload>(
      `/passkey/${intent}/begin`,
      {},
      { signal }
    )
  ).data
}

export async function finishPlatformPasskey(
  intent: 'login' | 'verify' | 'register',
  flowToken: string,
  credential: Record<string, unknown>,
  signal?: AbortSignal
): Promise<PlatformSession> {
  return (
    await platformClient.post<PlatformSession>(
      `/passkey/${intent}/finish`,
      { flow_token: flowToken, credential },
      { signal }
    )
  ).data
}

export async function removePlatformPasskey(
  id: string
): Promise<PlatformSession> {
  return (
    await platformClient.post<PlatformSession>(
      `/passkey/${encodeURIComponent(id)}/delete`
    )
  ).data
}
