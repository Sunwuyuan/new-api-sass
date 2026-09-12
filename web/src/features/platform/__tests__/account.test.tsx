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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import axios from 'axios'
import type { ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import {
  changePlatformPassword,
  platformLogin,
  platformRegister,
  resendPlatformVerification,
  verifyPlatformEmail,
} from '../api'
import { PlatformAuthForm } from '../auth-form'
import { PlatformPasswordDialog } from '../password-dialog'
import { authStatus } from './fixtures'

vi.mock('../api', () => ({
  changePlatformPassword: vi.fn(),
  platformLogin: vi.fn(),
  platformRegister: vi.fn(),
  resendPlatformVerification: vi.fn(),
  verifyPlatformEmail: vi.fn(),
}))

const clients: QueryClient[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
})

function renderAccount(content: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  })
  clients.push(client)
  render(<QueryClientProvider client={client}>{content}</QueryClientProvider>)
  return client
}

test('registration validates password length and directs the user to sign in', async () => {
  const user = userEvent.setup()
  vi.mocked(platformRegister).mockResolvedValue()
  const onRegistered = vi.fn()
  renderAccount(
    <PlatformAuthForm
      mode='sign-up'
      status={authStatus}
      redirectTo='/platform'
      onSignedIn={vi.fn()}
      onRegistered={onRegistered}
    />
  )
  await user.type(screen.getByLabelText('Email'), 'owner@example.test')
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'too-short'
  )
  await user.click(screen.getByRole('button', { name: 'Create account' }))
  expect(
    await screen.findByText('Use 10 to 128 characters for a new password.', {
      selector: '[data-slot="field-error"]',
    })
  ).toBeVisible()
  expect(platformRegister).not.toHaveBeenCalled()
  await user.clear(screen.getByLabelText('Password', { exact: true }))
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'a long phrase for this test'
  )
  await user.click(screen.getByRole('button', { name: 'Create account' }))
  await user.type(
    screen.getByLabelText('Confirm password'),
    'a long phrase for this test'
  )
  await user.click(screen.getByRole('button', { name: 'Create account' }))
  await waitFor(() => expect(onRegistered).toHaveBeenCalledOnce())
  expect(screen.getByLabelText('Password', { exact: true })).toHaveValue('')
  expect(screen.getByRole('button', { name: 'Create account' })).toBeEnabled()
})

test('failed sign in displays a safe error and permits retry without creating a session', async () => {
  const user = userEvent.setup()
  vi.mocked(platformLogin).mockRejectedValue(
    new Error('Invalid email or password')
  )
  const client = renderAccount(
    <PlatformAuthForm
      mode='sign-in'
      status={authStatus}
      redirectTo='/platform'
      onSignedIn={vi.fn()}
    />
  )
  await user.type(screen.getByLabelText('Email'), 'owner@example.test')
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'a long phrase for this test'
  )
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Invalid email or password'
  )
  expect(screen.getByRole('button', { name: 'Sign in' })).toBeEnabled()
  expect(client.getQueryData(['platform', 'session'])).toBeUndefined()
})

test('password change disables resubmission while pending and clears the platform session on success', async () => {
  const user = userEvent.setup()
  const onOpenChange = vi.fn()
  let finish: () => void = () => undefined
  vi.mocked(changePlatformPassword).mockReturnValue(
    new Promise<void>((resolve) => {
      finish = resolve
    })
  )
  const client = renderAccount(
    <PlatformPasswordDialog open onOpenChange={onOpenChange} />
  )
  client.setQueryData(['platform', 'session'], { user: { id: 1 } })
  client.setQueryData(['platform', 'tenants', 1], [{ tenant: { id: 1 } }])
  const dialog = await screen.findByRole('dialog')
  await user.type(
    within(dialog).getByLabelText('Current Password'),
    'the current test phrase'
  )
  await user.type(
    within(dialog).getByLabelText('New Password', { exact: true }),
    'the replacement test phrase'
  )
  await user.type(
    within(dialog).getByLabelText('Confirm New Password'),
    'the replacement test phrase'
  )
  await user.click(
    within(dialog).getByRole('button', { name: 'Change platform password' })
  )
  await waitFor(() =>
    expect(
      within(dialog).getByRole('button', { name: 'Change platform password' })
    ).toBeDisabled()
  )
  await act(async () => finish())
  await waitFor(() =>
    expect(client.getQueryData(['platform', 'session'])).toBeNull()
  )
  expect(client.getQueryData(['platform', 'tenants', 1])).toBeUndefined()
  expect(onOpenChange).toHaveBeenCalledWith(false)
})

test('unverified sign in asks for an email code and then creates a session', async () => {
  const user = userEvent.setup()
  const axiosError = new axios.AxiosError('Unauthorized')
  axiosError.response = {
    data: { code: 'email_not_verified' },
    status: 401,
    statusText: 'Unauthorized',
    headers: {},
    config: { headers: new axios.AxiosHeaders() },
  }
  vi.mocked(platformLogin)
    .mockRejectedValueOnce(
      new Error('Verify your email before signing in.', { cause: axiosError })
    )
    .mockResolvedValueOnce({
      user: {
        id: 1,
        email: 'owner@example.test',
        role: 'user',
        status: 'active',
        must_change_password: false,
        tenant_count: 0,
      },
      csrf_token: 'csrf',
      authenticated_at: '2026-09-12T00:00:00Z',
    })
  vi.mocked(verifyPlatformEmail).mockResolvedValue()
  const onSignedIn = vi.fn()
  renderAccount(
    <PlatformAuthForm
      mode='sign-in'
      status={authStatus}
      redirectTo='/platform'
      onSignedIn={onSignedIn}
    />
  )
  await user.type(screen.getByLabelText('Email'), 'owner@example.test')
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'a long phrase for this test'
  )
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(await screen.findByLabelText('Verification code')).toBeVisible()
  expect(
    screen.getByText('Enter the verification code sent to owner@example.test.')
  ).toBeVisible()
  await user.type(screen.getByLabelText('Verification code'), '123456')
  await user.click(screen.getByRole('button', { name: 'Verify email' }))
  await waitFor(() => expect(onSignedIn).toHaveBeenCalledOnce())
  expect(verifyPlatformEmail).toHaveBeenCalledWith({
    email: 'owner@example.test',
    code: '123456',
  })
  expect(resendPlatformVerification).not.toHaveBeenCalled()
})
