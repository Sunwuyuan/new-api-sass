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
import { authStatus } from './fixtures'

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
  has_password: true,
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
  if (config.url === '/status') return reply(config, authStatus)
  if (config.url === '/auth-methods') {
    return reply(config, { providers: [], passkeys: [], has_password: true })
  }
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
  vi.unstubAllGlobals()
  window.history.replaceState(null, '', '/')
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
  await waitFor(() =>
    expect(
      screen.getByText('Platform administrator access required')
    ).toBeVisible()
  )
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
  expect(
    await within(await screen.findByRole('table')).findByText(member.email)
  ).toBeVisible()
})

test('administrator user filters reach the API and dangerous actions require confirmation', async () => {
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/admin/users/7') return reply(config, { success: true })
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/users')
  await within(await screen.findByRole('table')).findByText(member.email)
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

test('platform administrators can change users without a second identity check', async () => {
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/admin/users/7') {
      return reply(config, { success: true })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/users')
  await within(await screen.findByRole('table')).findByText(member.email)
  expect(
    screen.queryByRole('button', { name: 'Verify administrator access' })
  ).not.toBeInTheDocument()
  await user.click(
    screen.getByRole('button', { name: `Actions for ${member.email}` })
  )
  await user.click(
    await screen.findByRole('menuitem', { name: 'Revoke platform sessions' })
  )
  const confirmation = await screen.findByRole('alertdialog')
  await user.click(
    within(confirmation).getByRole('button', { name: 'Continue' })
  )
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  const changes = network.adapter.mock.calls.filter(
    ([config]) => config.url === '/admin/users/7'
  )
  expect(changes).toHaveLength(1)
  expect(changes[0][0].headers.get('X-CSRF-Token')).toBe('test-csrf')
})

test.each(['/platform', '/platform/workspaces/42', '/platform/admin/usage'])(
  'anonymous access to %s goes to the independent sign-in route',
  async (path) => {
    session = null
    const { router } = await renderPlatform(path)
    await screen.findByRole('heading', { name: 'Sign in' })
    expect(router.state.location.pathname).toBe('/platform/sign-in')
    expect(router.state.location.search).toEqual(
      expect.objectContaining({ redirect: path })
    )
    expect(
      network.adapter.mock.calls.some(
        ([config]) =>
          config.url === '/tenants' || config.url?.startsWith('/admin/')
      )
    ).toBe(false)
  }
)

test('sign-up is a separate page with confirmation and returns to sign-in without creating a session', async () => {
  session = null
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/register') return reply(config, { success: true })
    return defaultAdapter(config)
  })
  const { router } = await renderPlatform(
    '/platform/sign-up?redirect=%2Fplatform%2Fusage'
  )
  expect(
    await screen.findByRole('heading', { name: 'Create an account' })
  ).toBeVisible()
  await user.type(screen.getByLabelText('Email'), 'new@example.test')
  await user.type(
    screen.getByLabelText('Password', { exact: true }),
    'a synthetic registration phrase'
  )
  await user.type(
    screen.getByLabelText('Confirm password'),
    'a synthetic registration phrase'
  )
  await user.click(screen.getByRole('button', { name: 'Create account' }))
  expect(await screen.findByRole('status')).toHaveTextContent(
    'Registration submitted. Sign in with your credentials.'
  )
  expect(router.state.location.pathname).toBe('/platform/sign-in')
  expect(router.state.location.search).toEqual(
    expect.objectContaining({ redirect: '/platform/usage' })
  )
  expect(session).toBeNull()
  expect(
    network.adapter.mock.calls.some(([config]) => config.url === '/login')
  ).toBe(false)
})

test('authenticated users leave sign-in and the workspace page contains no embedded auth or pricing wall', async () => {
  session = { ...administratorSession, user: member }
  const { router } = await renderPlatform('/platform/sign-in')
  await screen.findByRole('heading', { name: 'My workspaces' })
  expect(router.state.location.pathname).toBe('/platform')
  expect(
    screen.queryByLabelText('Password', { exact: true })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Hosting plans' })
  ).not.toBeInTheDocument()
})

test('platform status controls provider visibility and OAuth starts only through the platform API', async () => {
  session = null
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/status') {
      return reply(config, {
        ...authStatus,
        github_oauth: true,
        discord_oauth: true,
        linuxdo_oauth: true,
        oidc_enabled: true,
        oidc_display_name: 'Company SSO',
        telegram_oauth: true,
        wechat_login: true,
        passkey_login: true,
        custom_oauth_providers: [{ slug: 'custom', name: 'Custom SSO' }],
      })
    }
    if (config.url === '/oauth/github/start') {
      throw new axios.AxiosError('Unavailable', undefined, config, undefined, {
        ...reply(config, { code: 'authentication_failed' }),
        status: 401,
      })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/sign-in?redirect=%2Fplatform%2Fusage')
  for (const provider of [
    'GitHub',
    'Discord',
    'LinuxDO',
    'Company SSO',
    'Telegram',
    'WeChat',
    'Custom SSO',
  ]) {
    expect(
      await screen.findByRole('button', { name: `Continue with ${provider}` })
    ).toBeVisible()
  }
  expect(
    screen.getByRole('button', { name: 'Sign in with Passkey' })
  ).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Continue with GitHub' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Authentication failed. Start again or use another sign-in method.'
  )
  expect(network.adapter).toHaveBeenCalledWith(
    expect.objectContaining({
      baseURL: '/platform/api',
      url: '/oauth/github/start',
      data: JSON.stringify({ redirect: '/platform/usage' }),
    })
  )
})

test('disabled registration and sign-in providers are not offered', async () => {
  session = null
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/status') {
      return reply(config, {
        ...authStatus,
        register_enabled: false,
        password_register_enabled: false,
      })
    }
    return defaultAdapter(config)
  })
  const { router } = await renderPlatform('/platform/sign-in')
  await screen.findByLabelText('Email')
  expect(
    screen.queryByRole('button', { name: /Continue with/ })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Sign in with Passkey' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('link', { name: 'Sign up' })
  ).not.toBeInTheDocument()
  await act(async () => {
    await router.navigate({ href: '/platform/sign-up' })
  })
  expect(await screen.findByText('Registration is disabled')).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Create account' })
  ).not.toBeInTheDocument()
})

test.each(['?', '#'])(
  'OAuth %s callback sends one completion, clears the code, and accepts only a platform redirect',
  async (separator) => {
    session = null
    const code = 'test-authorization/code+with=symbols &space'
    const query = new URLSearchParams({ state: 'test-state', code }).toString()
    const callback = `/platform/oauth/custom${separator}${query}`
    window.history.replaceState(null, '', callback)
    network.adapter.mockImplementation(async (config) => {
      if (config.url === '/oauth/custom/finish') {
        session = { ...administratorSession, user: member }
        return reply(config, {
          ...session,
          redirect: 'https://attacker.example.test',
        })
      }
      return defaultAdapter(config)
    })
    const { router } = await renderPlatform(callback)
    await screen.findByRole('heading', { name: 'My workspaces' })
    expect(router.state.location.pathname).toBe('/platform')
    expect(window.location.search).toBe('')
    expect(window.location.hash).toBe('')
    const completions = network.adapter.mock.calls.filter(
      ([config]) => config.url === '/oauth/custom/finish'
    )
    expect(completions).toHaveLength(1)
    expect(JSON.parse(completions[0][0].data)).toEqual({
      state: 'test-state',
      code,
      error: '',
    })
  }
)

test('WeChat uses the shared dialog and a platform browser flow, then enters the workspace page', async () => {
  session = null
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/status') {
      return reply(config, {
        ...authStatus,
        wechat_login: true,
        wechat_qrcode: '/test-qr.png',
      })
    }
    if (config.url === '/wechat/start') {
      return reply(config, { flow_token: 'wechat-test-flow' })
    }
    if (config.url === '/wechat/finish') {
      session = { ...administratorSession, user: member }
      return reply(config, session)
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/sign-in')
  await user.click(
    await screen.findByRole('button', { name: 'Continue with WeChat' })
  )
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByRole('img')).toHaveAttribute('src', '/test-qr.png')
  expect(within(dialog).getByRole('button', { name: 'Confirm' })).toBeDisabled()
  await user.type(
    within(dialog).getByLabelText('Verification code'),
    'test-code{Enter}'
  )
  await screen.findByRole('heading', { name: 'My workspaces' })
  expect(network.adapter).toHaveBeenCalledWith(
    expect.objectContaining({
      url: '/wechat/finish',
      data: JSON.stringify({
        code: 'test-code',
        flow_token: 'wechat-test-flow',
      }),
    })
  )
})

test('Passkey assertion uses platform endpoints and preserves required user verification', async () => {
  session = null
  const user = userEvent.setup()
  const getCredential = vi.fn().mockResolvedValue({
    id: 'test-credential',
    rawId: new Uint8Array([1, 2]).buffer,
    type: 'public-key',
    response: {
      clientDataJSON: new Uint8Array([3]).buffer,
      authenticatorData: new Uint8Array([4]).buffer,
      signature: new Uint8Array([5]).buffer,
      userHandle: new Uint8Array([6]).buffer,
    },
    getClientExtensionResults: () => ({}),
  })
  vi.stubGlobal('PublicKeyCredential', class {})
  Object.defineProperty(navigator, 'credentials', {
    configurable: true,
    value: { get: getCredential },
  })
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/status') {
      return reply(config, { ...authStatus, passkey_login: true })
    }
    if (config.url === '/passkey/login/begin') {
      return reply(config, {
        flow_token: 'passkey-test-flow',
        options: {
          publicKey: {
            rpId: 'localhost',
            challenge: 'AQID',
            userVerification: 'required',
          },
        },
      })
    }
    if (config.url === '/passkey/login/finish') {
      session = { ...administratorSession, user: member }
      return reply(config, session)
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/sign-in')
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Sign in with Passkey' })
    ).toBeEnabled()
  )
  await user.click(screen.getByRole('button', { name: 'Sign in with Passkey' }))
  await screen.findByRole('heading', { name: 'My workspaces' })
  expect(getCredential).toHaveBeenCalledOnce()
  expect(getCredential.mock.calls[0][0].publicKey).toEqual(
    expect.objectContaining({ rpId: 'localhost', userVerification: 'required' })
  )
  const finish = network.adapter.mock.calls.find(
    ([config]) => config.url === '/passkey/login/finish'
  )
  expect(JSON.parse(finish?.[0].data ?? 'null')).toEqual(
    expect.objectContaining({
      flow_token: 'passkey-test-flow',
      credential: expect.objectContaining({ id: 'test-credential' }),
    })
  )
})

const hostingPlan = {
  id: 1,
  name: 'Lite',
  price: 'Free',
  limits: { requests: 1000, users: 5, tokens: 20, channels: 3 },
  capabilities: {
    max_workspaces: 1,
    custom_branding: false,
    remove_platform_footer: false,
  },
}
const tenant = {
  id: 42,
  name: 'Alpha instance',
  slug: 'alpha',
  owner_platform_user_id: member.id,
  plan_id: 1,
  status: 'active',
  plan_expires_at: null,
}
const usage = { month: '2026-09', requests: 750 }

test.each([false, true])(
  'workspace details show their own usage and scoped management actions (admin=%s)',
  async (admin) => {
    session = {
      ...administratorSession,
      user: admin ? administratorSession.user : member,
    }
    network.adapter.mockImplementation(async (config) => {
      if (config.url === '/plans') {
        return reply(config, { plans: [hostingPlan] })
      }
      if (config.url === `${admin ? '/admin' : ''}/tenants/42`) {
        return reply(config, {
          tenant,
          owner_name: member.email,
          plan: hostingPlan,
          usage,
          history: [usage],
          assignments: [],
        })
      }
      return defaultAdapter(config)
    })
    await renderPlatform(`/platform${admin ? '/admin' : ''}/workspaces/42`)
    expect(
      await screen.findByRole('heading', { name: tenant.name })
    ).toBeVisible()
    expect(
      screen.getByRole('progressbar', {
        name: `Monthly usage for ${tenant.name}`,
      })
    ).toHaveAttribute('aria-valuenow', '75')
    expect(screen.getByText('250')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Redeem hosting plan' })
    ).toBeEnabled()
    if (admin) {
      expect(
        screen.getByRole('button', { name: 'Activate plan manually' })
      ).toBeEnabled()
      expect(
        screen.getByRole('button', { name: 'Suspend workspace' })
      ).toBeEnabled()
    } else {
      expect(
        screen.queryByRole('button', { name: 'Suspend workspace' })
      ).not.toBeInTheDocument()
    }
  }
)

test('usage analytics renders real totals, scoped workspace links, and an empty recorded history', async () => {
  session = { ...administratorSession, user: member }
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/plans') return reply(config, { plans: [hostingPlan] })
    if (config.url === '/usage') {
      return reply(config, {
        month: '2026-09',
        summary: {
          workspaces: 1,
          active: 1,
          suspended: 0,
          expired: 0,
          expiring_soon: 0,
          exhausted: 0,
          requests: 750,
        },
        plans: [{ plan_id: 1, name: 'Lite', workspaces: 1, requests: 750 }],
        history: [],
      })
    }
    if (config.url === '/tenants') {
      return reply(config, {
        tenants: [{ tenant, usage }],
        pagination: { page: 1, page_size: 20, total: 1 },
      })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/usage')
  expect(
    await screen.findByRole('heading', { name: 'Usage analytics' })
  ).toBeVisible()
  expect(
    await screen.findByRole('link', { name: tenant.name })
  ).toHaveAttribute('href', '/platform/workspaces/42')
  expect(
    screen.getByRole('progressbar', {
      name: `Monthly usage for ${tenant.name}`,
    })
  ).toHaveAttribute('aria-valuenow', '75')
  expect(screen.getByText('No recorded usage yet')).toBeVisible()
  expect(
    network.adapter.mock.calls.some(([config]) =>
      config.url?.startsWith('/admin/')
    )
  ).toBe(false)
})

test('plans page shows hosting cards and a workspace redemption form', async () => {
  const user = userEvent.setup()
  session = { ...administratorSession, user: member }
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/plans') {
      return reply(config, {
        plans: [
          {
            id: 1,
            name: 'Lite',
            price: 'Free',
            limits: { requests: 1000, users: 5, tokens: 20, channels: 3 },
            capabilities: {
              remove_platform_footer: false,
              custom_branding: false,
              max_workspaces: 1,
            },
          },
          {
            id: 2,
            name: 'Standard',
            price: 'Contact administrator',
            limits: { requests: 50000, users: 50, tokens: 200, channels: 20 },
            capabilities: {
              remove_platform_footer: false,
              custom_branding: true,
              max_workspaces: 3,
              task_plugins: true,
            },
          },
          {
            id: 3,
            name: 'Pro',
            price: 'Contact administrator',
            limits: {
              requests: 100000,
              users: 1000,
              tokens: 10000,
              channels: 100,
            },
            capabilities: {
              remove_platform_footer: true,
              custom_branding: true,
              max_workspaces: 10,
              task_plugins: true,
            },
          },
        ],
      })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/plans')
  expect(await screen.findByRole('heading', { name: 'Plans' })).toBeVisible()
  const cards = within(
    await screen.findByRole('region', { name: 'Hosting plans' })
  )
  expect(cards.getByText('Lite')).toBeVisible()
  expect(cards.getByText('Standard')).toBeVisible()
  expect(cards.getByText('Pro')).toBeVisible()
  expect(cards.getByText('Free')).toBeVisible()
  expect(cards.getAllByText('Contact administrator')).toHaveLength(2)
  expect(screen.queryByText('Recommended')).not.toBeInTheDocument()
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
  expect(screen.queryByText('Tokens')).not.toBeInTheDocument()
  expect(
    cards.getByRole('link', { name: 'Create free workspace' })
  ).toHaveAttribute('href', '/platform/workspaces/new')
  const chrome = screen
    .getByRole('button', { name: 'Toggle Sidebar' })
    .closest('header')
  if (!(chrome instanceof HTMLElement)) {
    throw new Error('plans page is missing the console header')
  }
  expect(within(chrome).queryByText('Workspace')).not.toBeInTheDocument()
  const redeemButtons = screen.getAllByRole('button', { name: 'Redeem a code' })
  expect(redeemButtons).toHaveLength(3)
  expect(redeemButtons[0]).toBeDisabled()
  await user.click(redeemButtons[1])
  const dialog = await screen.findByRole('alertdialog')
  expect(
    within(dialog).getByRole('combobox', { name: 'Workspace' })
  ).toBeVisible()
  expect(within(dialog).getByLabelText('Redemption code')).toBeVisible()
})

test('administrators edit plan limits and features in one comparison table', async () => {
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/plans') {
      return reply(config, {
        plans: [
          {
            id: 1,
            name: 'Lite',
            price: 'Free',
            limits: { requests: 10000, users: 1, tokens: 50, channels: 10 },
            capabilities: {
              remove_platform_footer: false,
              custom_branding: false,
              max_workspaces: 1,
            },
          },
          {
            id: 2,
            name: 'Standard',
            price: 'Contact administrator',
            limits: { requests: 100000, users: 1000, tokens: 0, channels: 0 },
            capabilities: {
              remove_platform_footer: false,
              custom_branding: true,
              max_workspaces: 5,
            },
          },
          {
            id: 3,
            name: 'Pro',
            price: 'Contact administrator',
            limits: { requests: 0, users: 0, tokens: 0, channels: 0 },
            capabilities: {
              remove_platform_footer: true,
              custom_branding: true,
              max_workspaces: 20,
            },
          },
        ],
      })
    }
    if (config.url === '/admin/plans/1') {
      return reply(config, { success: true })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/admin/plans')
  expect(
    await screen.findByRole('heading', { name: 'Hosting plans' })
  ).toBeVisible()
  expect(screen.getByRole('columnheader', { name: 'Lite' })).toBeVisible()
  expect(screen.getByRole('columnheader', { name: 'Standard' })).toBeVisible()
  expect(screen.getByRole('columnheader', { name: 'Pro' })).toBeVisible()
  expect(screen.queryByText('Tokens')).not.toBeInTheDocument()
  const requests = screen.getByRole('spinbutton', {
    name: 'Lite: Monthly requests',
  })
  await user.clear(requests)
  await user.type(requests, '20000')
  await user.click(screen.getByRole('checkbox', { name: 'Lite: Custom branding' }))
  const saveButtons = screen.getAllByRole('button', { name: 'Save changes' })
  expect(saveButtons[0]).toBeEnabled()
  expect(saveButtons[1]).toBeDisabled()
  await user.click(saveButtons[0])
  await waitFor(() =>
    expect(
      network.adapter.mock.calls.some(([config]) => {
        if (config.url !== '/admin/plans/1') return false
        const body = JSON.parse(config.data) as {
          limits: { requests: number; tokens: number }
          capabilities: { custom_branding: boolean }
        }
        return (
          body.limits.requests === 20000 &&
          body.limits.tokens === 0 &&
          body.capabilities.custom_branding
        )
      })
    ).toBe(true)
  )
})

test('workspace administrators page lists accounts and saves edits', async () => {
  const user = userEvent.setup()
  network.adapter.mockImplementation(async (config) => {
    if (config.url === '/tenants/1') {
      return reply(config, {
        tenant: {
          id: 1,
          slug: 'alpha',
          name: 'Alpha',
          owner_platform_user_id: 7,
          plan_id: 1,
          status: 'active',
          plan_expires_at: null,
        },
        owner_name: member.email,
        plan: {
          id: 1,
          name: 'Lite',
          price: 'Free',
          limits: { requests: 5000, users: 1, tokens: 50, channels: 10 },
          capabilities: { max_workspaces: 1 },
        },
        usage: { month: '2026-09', requests: 0 },
        history: [],
        assignments: [],
        administrators: [
          {
            id: 3,
            username: 'root',
            display_name: 'Owner',
            email: 'root@example.test',
            status: 1,
            role: 'root',
          },
        ],
      })
    }
    if (config.url === '/tenants/1/administrator') {
      return reply(config, { success: true })
    }
    return defaultAdapter(config)
  })
  await renderPlatform('/platform/workspaces/1/administrators')
  expect(
    await screen.findByRole('heading', { name: 'Workspace administrators' })
  ).toBeVisible()
  expect(screen.getByText('root')).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Edit' }))
  const dialog = await screen.findByRole('alertdialog')
  await user.clear(within(dialog).getByLabelText('Username'))
  await user.type(within(dialog).getByLabelText('Username'), 'adminroot')
  await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
  await waitFor(() =>
    expect(
      network.adapter.mock.calls.some(
        ([config]) =>
          config.url === '/tenants/1/administrator' &&
          JSON.parse(config.data).username === 'adminroot'
      )
    ).toBe(true)
  )
})
