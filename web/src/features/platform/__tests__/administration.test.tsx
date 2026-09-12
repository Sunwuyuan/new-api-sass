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
import { createMemoryHistory, RouterProvider } from '@tanstack/react-router'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import axios, {
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { createPlatformRouter } from '../router'
import type { PlatformSession } from '../types'

const network = vi.hoisted(() => ({ adapter: vi.fn<AxiosAdapter>() }))
vi.mock('axios', async (importOriginal) => {
  const original = await importOriginal<typeof import('axios')>()
  return {
    ...original,
    default: {
      ...original.default,
      create: (config: Parameters<typeof original.default.create>[0]) =>
        original.default.create({ ...config, adapter: network.adapter }),
    },
  }
})

let session: PlatformSession | null
let clients: QueryClient[] = []
const member = {
  id: 7,
  email: 'owner@example.test',
  role: 'user' as const,
  status: 'active' as const,
  must_change_password: false,
  tenant_count: 1,
}

function reply(config: InternalAxiosRequestConfig, data: unknown) {
  return { config, data, status: 200, statusText: 'OK', headers: {} }
}

const administratorSession: PlatformSession = {
  user: { ...member, role: 'admin' },
  csrf_token: 'test-csrf',
  authenticated_at: '2026-09-12T00:00:00Z',
}

const defaultAdapter: AxiosAdapter = async (config) => {
  if (config.url === '/session') {
    if (!session) {
      throw new axios.AxiosError('Unauthorized', undefined, config, undefined, {
        ...reply(config, {}),
        status: 401,
      })
    }
    return reply(config, session)
  }
  if (config.url === '/plans') return reply(config, { plans: [] })
  if (config.url === '/admin/users') {
    return reply(config, {
      users: [member],
      pagination: { total: 1, page: 1, page_size: 20 },
    })
  }
  if (config.url === '/tenants') {
    return reply(config, {
      tenants: [],
      max_workspaces: 1,
      workspace_count: 0,
      pagination: { total: 0, page: 1, page_size: 20 },
    })
  }
  throw new Error(`Unexpected platform request: ${config.method} ${config.url}`)
}

beforeEach(() => {
  session = structuredClone(administratorSession)
  network.adapter.mockImplementation(defaultAdapter)
})

afterEach(() => {
  clients.forEach((client) => client.clear())
  clients = []
  network.adapter.mockReset()
})

async function renderPlatform(path: string) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  })
  clients.push(client)
  const router = createPlatformRouter(
    client,
    createMemoryHistory({ initialEntries: [path] })
  )
  await act(async () => {
    await router.load()
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return { router, client }
}

test('ordinary users opening an admin URL see access denied without fetching admin data', async () => {
  session = { ...administratorSession, user: member }
  await renderPlatform('/platform/admin/users')
  expect(
    await screen.findByText('Platform administrator access required')
  ).toBeVisible()
  expect(
    screen.queryByRole('link', { name: 'Administration' })
  ).not.toBeInTheDocument()
  expect(
    network.adapter.mock.calls.some(([config]) =>
      config.url?.startsWith('/admin/')
    )
  ).toBe(false)
})

test('signing in as an administrator on a protected URL loads that page', async () => {
  const user = userEvent.setup()
  const signedIn = session
  session = null
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/login') {
      session = signedIn
      return reply(config, session)
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/users')
  await user.type(await screen.findByLabelText('Email'), member.email)
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'a valid testing passphrase'
  )
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(
    await screen.findByRole('link', { name: 'Platform users' })
  ).toBeVisible()
  expect(await screen.findByText(member.email)).toBeVisible()
})

test('administrator user filters reach the API and dangerous actions require confirmation', async () => {
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/admin/users/7') return reply(config, { success: true })
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/users')
  await screen.findByText(member.email)
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Role' }),
    'user'
  )
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Status' }),
    'active'
  )
  await waitFor(() =>
    expect(network.adapter).toHaveBeenCalledWith(
      expect.objectContaining({
        url: '/admin/users',
        params: expect.objectContaining({
          role: 'user',
          status: 'active',
          page: 1,
        }),
      })
    )
  )
  await user.click(
    screen.getByRole('button', { name: `Actions for ${member.email}` })
  )
  await user.click(
    await screen.findByRole('menuitem', { name: 'Disable user' })
  )
  expect(
    network.adapter.mock.calls.some(([config]) => config.method === 'post')
  ).toBe(false)
  const dialog = await screen.findByRole('alertdialog')
  expect(within(dialog).getByText(member.email)).toBeVisible()
  await user.click(within(dialog).getByRole('button', { name: 'Continue' }))
  await waitFor(() =>
    expect(network.adapter).toHaveBeenCalledWith(
      expect.objectContaining({
        url: '/admin/users/7',
        data: JSON.stringify({ action: 'disable' }),
      })
    )
  )
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
})

test('forced password changes hide workspace and administration controls', async () => {
  session = {
    ...administratorSession,
    user: { ...administratorSession.user, must_change_password: true },
  }
  await renderPlatform('/platform/admin/users')
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByLabelText('Current Password')).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Change your password before continuing.'
  )
  expect(
    screen.queryByRole('link', { name: 'Administration' })
  ).not.toBeInTheDocument()
  expect(
    network.adapter.mock.calls.some(
      ([config]) =>
        config.url?.startsWith('/admin/') || config.url === '/tenants'
    )
  ).toBe(false)
})

test('reauthentication rotates CSRF without replaying a rejected administrative change', async () => {
  const user = userEvent.setup()
  let verified = false
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/reauthenticate') {
      if (
        JSON.parse(config.data).password !== 'the correct testing passphrase'
      ) {
        throw new axios.AxiosError(
          'Unauthorized',
          undefined,
          config,
          undefined,
          { ...reply(config, { code: 'invalid_credentials' }), status: 401 }
        )
      }
      verified = true
      session = { ...administratorSession, csrf_token: 'rotated-test-csrf' }
      return reply(config, session)
    }
    if (config.url === '/admin/users/7') {
      if (!verified) {
        throw new axios.AxiosError(
          'Unauthorized',
          undefined,
          config,
          undefined,
          { ...reply(config, { code: 'recent_login_required' }), status: 401 }
        )
      }
      return reply(config, { success: true })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/users')
  await screen.findByText(member.email)
  await user.click(
    screen.getByRole('button', { name: `Actions for ${member.email}` })
  )
  await user.click(
    await screen.findByRole('menuitem', { name: 'Revoke platform sessions' })
  )
  let confirmation = await screen.findByRole('alertdialog')
  await user.click(
    within(confirmation).getByRole('button', { name: 'Continue' })
  )
  expect(await within(confirmation).findByRole('alert')).toHaveTextContent(
    'Verify administrator access, then try again.'
  )
  await user.click(within(confirmation).getByRole('button', { name: 'Cancel' }))
  await user.click(
    screen.getByRole('button', { name: 'Verify administrator access' })
  )
  const dialog = await screen.findByRole('dialog')
  await user.type(
    within(dialog).getByLabelText('Current Password'),
    'wrong testing passphrase'
  )
  await user.click(
    within(dialog).getByRole('button', { name: 'Verify administrator access' })
  )
  expect(await within(dialog).findByRole('alert')).toHaveTextContent(
    'Invalid email or password'
  )
  await user.clear(within(dialog).getByLabelText('Current Password'))
  await user.type(
    within(dialog).getByLabelText('Current Password'),
    'the correct testing passphrase'
  )
  await user.click(
    within(dialog).getByRole('button', { name: 'Verify administrator access' })
  )
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(
    network.adapter.mock.calls.filter(
      ([config]) => config.url === '/admin/users/7'
    )
  ).toHaveLength(1)
  await user.click(
    screen.getByRole('button', { name: `Actions for ${member.email}` })
  )
  await user.click(
    await screen.findByRole('menuitem', { name: 'Revoke platform sessions' })
  )
  confirmation = await screen.findByRole('alertdialog')
  await user.click(
    within(confirmation).getByRole('button', { name: 'Continue' })
  )
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  const changes = network.adapter.mock.calls.filter(
    ([config]) => config.url === '/admin/users/7'
  )
  expect(changes).toHaveLength(2)
  expect(changes[1][0].headers.get('X-CSRF-Token')).toBe('rotated-test-csrf')
})
