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
import axios from 'axios'
import { t } from 'i18next'

import { safeServerErrorMessage } from '@/lib/server-error-message'
import { tenantPath } from '@/lib/tenant'

import type {
  HostingPlan,
  PlatformSession,
  PlatformUser,
  Workspace,
  WorkspaceUsage,
} from './types'

let csrfToken = ''

// Platform sessions have their own cookie and CSRF contract. Gateway JWTs are
// never attached to platform requests.
const client = axios.create({
  baseURL: '/platform/api',
  withCredentials: true,
  headers: { 'X-Requested-With': 'NewAPIPlatform' },
})

client.interceptors.request.use((config) => {
  config.headers.set('X-CSRF-Token', csrfToken)
  return config
})

client.interceptors.response.use(undefined, (error: unknown) => {
  const code: unknown = axios.isAxiosError(error)
    ? error.response?.data?.code
    : undefined
  let message = t('Unable to complete the request')
  if (code === 'invalid_credentials') message = t('Invalid email or password')
  if (code === 'authentication_rate_limited') {
    message = t('Too many attempts. Please try again later.')
  }
  if (code === 'recent_login_required') {
    message = t('Please sign in again before changing a workspace.')
  }
  if (code === 'workspace_unavailable_or_limit_reached') {
    message = t(
      'This workspace address is unavailable or your workspace limit has been reached.'
    )
  }
  if (code === 'invalid_email_or_password_length') {
    message = t('Enter a valid email and a password of 15 to 128 characters.')
  }
  if (code === 'invalid_new_password') {
    message = t('Use 15 to 128 characters and choose a less common password.')
  }
  if (code === 'password_unchanged') {
    message = t('New password must be different from current password')
  }
  if (code === 'invalid_activation') {
    message = t(
      'The activation link is invalid or expired, or the password is too common.'
    )
  }
  if (code === 'platform_login_required') message = t('Please sign in again.')
  const failure = new Error(message, { cause: error })
  Object.defineProperty(failure, safeServerErrorMessage, { value: true })
  throw failure
})

export async function getPlatformSession(): Promise<PlatformSession | null> {
  try {
    const { data } = await client.get<PlatformSession>('/session')
    csrfToken = data.csrf_token
    return data
  } catch (error) {
    const cause = error instanceof Error ? error.cause : error
    if (axios.isAxiosError(cause) && cause.response?.status === 401) return null
    throw error
  }
}

export async function platformLogin(input: {
  email: string
  password: string
}): Promise<PlatformSession> {
  const { data } = await client.post<PlatformSession>('/login', input)
  csrfToken = data.csrf_token
  return data
}

export async function platformRegister(input: {
  email: string
  password: string
}): Promise<void> {
  await client.post('/register', input)
}

export async function platformLogout(): Promise<void> {
  await client.post('/logout')
  csrfToken = ''
}

export async function changePlatformPassword(input: {
  current_password: string
  new_password: string
}): Promise<void> {
  await client.post('/password', input)
  csrfToken = ''
}

export async function activateWorkspaceRoot(
  token: string,
  password: string
): Promise<void> {
  await client.post(
    tenantPath('/api/saas/activate'),
    { token, password },
    { baseURL: '' }
  )
}

export async function getHostingPlans(): Promise<HostingPlan[]> {
  return (await client.get<{ plans: HostingPlan[] }>('/plans')).data.plans
}

export async function getWorkspaces(admin: boolean): Promise<WorkspaceUsage[]> {
  return (
    await client.get<{ tenants: WorkspaceUsage[] }>(
      admin ? '/admin/tenants' : '/tenants'
    )
  ).data.tenants
}

export async function getPlatformUsers(): Promise<PlatformUser[]> {
  return (await client.get<{ users: PlatformUser[] }>('/admin/users')).data
    .users
}

export async function createWorkspace(input: {
  name: string
  slug: string
}): Promise<{ tenant: Workspace; root_activation_url: string }> {
  return (await client.post('/tenants', input)).data
}

export async function assignHostingPlan(
  id: number,
  planId: number,
  months: number
): Promise<void> {
  await client.post(`/admin/tenants/${id}/plan`, { plan_id: planId, months })
}

export async function setWorkspaceStatus(
  id: number,
  status: Workspace['status']
): Promise<void> {
  await client.post(`/admin/tenants/${id}/status`, { status })
}
