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
import axios, { type AxiosAdapter } from 'axios'
import type { ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { CreateCodesDialog } from '../admin/create-codes-dialog'
import { RedeemPlanDialog } from '../redeem-plan-dialog'
import type { HostingPlan, Workspace } from '../types'

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

const standard: HostingPlan = {
  id: 3,
  name: 'Standard',
  price: 'Contact administrator',
  limits: { requests: 20000, users: 50, tokens: 200, channels: 20 },
  capabilities: {
    max_workspaces: 3,
    custom_branding: true,
    remove_platform_footer: false,
  },
}
const workspace: Workspace = {
  id: 42,
  name: 'Selected workspace',
  slug: 'selected',
  owner_platform_user_id: 7,
  plan_id: 1,
  status: 'active',
  plan_expires_at: null,
}
const clients: QueryClient[] = []

afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
  network.adapter.mockReset()
})

function renderDialog(content: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  const result = render(
    <QueryClientProvider client={client}>{content}</QueryClientProvider>
  )
  return { ...result, client }
}

test('redeeming validates the code and preserves the selected workspace when retrying an unavailable code', async () => {
  const user = userEvent.setup()
  const onClose = vi.fn()
  network.adapter.mockImplementation(async (config) => {
    throw new axios.AxiosError('Unavailable', undefined, config, undefined, {
      config,
      status: 400,
      statusText: 'Bad Request',
      headers: {},
      data: { code: 'redemption_unavailable' },
    })
  })
  const { client } = renderDialog(
    <RedeemPlanDialog workspace={workspace} onClose={onClose} />
  )
  client.setQueryData(['platform', 'tenants', 7], { workspace_count: 1 })
  const dialog = await screen.findByRole('alertdialog')
  await user.type(within(dialog).getByLabelText('Redemption code'), 'invalid')
  await user.click(within(dialog).getByRole('button', { name: 'Redeem' }))
  expect(
    await within(dialog).findByText('Enter a valid platform redemption code.')
  ).toBeVisible()
  expect(network.adapter).not.toHaveBeenCalled()
  const code = 'a'.repeat(64)
  await user.clear(within(dialog).getByLabelText('Redemption code'))
  await user.type(within(dialog).getByLabelText('Redemption code'), code)
  await user.click(within(dialog).getByRole('button', { name: 'Redeem' }))
  expect(await within(dialog).findByRole('alert')).toHaveTextContent(
    'This code cannot be redeemed for the selected workspace.'
  )
  expect(onClose).not.toHaveBeenCalled()
  expect(network.adapter).toHaveBeenCalledWith(
    expect.objectContaining({
      url: '/redeem',
      data: JSON.stringify({ tenant_id: 42, code }),
    })
  )
  network.adapter.mockImplementation(async (config) => ({
    config,
    status: 200,
    statusText: 'OK',
    headers: {},
    data: {
      assignment: {
        id: 12,
        tenant_id: 42,
        plan_id: 3,
        source: 'redeem',
        expires_at: '2027-01-15T12:00:00Z',
      },
    },
  }))
  await user.click(within(dialog).getByRole('button', { name: 'Redeem' }))
  await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
  expect(client.getQueryState(['platform', 'tenants', 7])?.isInvalidated).toBe(
    true
  )
})

test('batch generation disables duplicate submissions and displays full codes only in the creation result', async () => {
  const user = userEvent.setup()
  const onClose = vi.fn()
  let complete: () => void = () => undefined
  const codes = ['b'.repeat(64), 'c'.repeat(64)]
  network.adapter.mockImplementation(
    (config) =>
      new Promise((resolve) => {
        complete = () =>
          resolve({
            config,
            status: 201,
            statusText: 'Created',
            headers: {},
            data: { codes },
          })
      })
  )
  const { unmount, client } = renderDialog(
    <CreateCodesDialog plans={[standard]} onClose={onClose} />
  )
  const dialog = await screen.findByRole('dialog')
  await user.clear(within(dialog).getByLabelText('Code count'))
  await user.type(within(dialog).getByLabelText('Code count'), '2')
  await user.click(
    within(dialog).getByRole('button', { name: 'Generate platform codes' })
  )
  await waitFor(() =>
    expect(
      within(dialog).getByRole('button', { name: 'Generate platform codes' })
    ).toBeDisabled()
  )
  expect(network.adapter).toHaveBeenCalledOnce()
  expect(network.adapter).toHaveBeenCalledWith(
    expect.objectContaining({
      url: '/admin/redemptions',
      data: JSON.stringify({
        plan_id: 3,
        duration_months: 1,
        count: 2,
        max_uses: 1,
        expires_at: null,
      }),
    })
  )
  await act(async () => complete())
  expect(await screen.findByLabelText('Platform redemption codes')).toHaveValue(
    codes.join('\n')
  )
  expect(
    screen.getByText('Copy these codes now. Full codes are only shown once.')
  ).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Copy all codes' }))
  expect(await navigator.clipboard.readText()).toBe(codes.join('\n'))
  expect(JSON.stringify(client.getQueryCache().getAll())).not.toContain(
    codes[0]
  )
  await user.click(screen.getByRole('button', { name: 'Done' }))
  expect(onClose).toHaveBeenCalledOnce()
  unmount()
  renderDialog(<CreateCodesDialog plans={[standard]} onClose={onClose} />)
  expect(
    screen.queryByLabelText('Platform redemption codes')
  ).not.toBeInTheDocument()
  expect(screen.getByLabelText('Code count')).toHaveValue(1)
})
