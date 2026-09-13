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
import {
  createRootRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import ActivateWorkspace from '../activate-workspace'
import { AssignPlanDialog } from '../admin/assign-plan-dialog'
import {
  createWorkspace,
  assignHostingPlan,
  setWorkspaceStatus,
  activateWorkspaceRoot,
} from '../api'
import { CreateWorkspace } from '../create-workspace'
import type { HostingPlan, WorkspaceUsage } from '../types'
import { WorkspaceCard } from '../workspace-card'

vi.mock('../api', () => ({
  createWorkspace: vi.fn(),
  assignHostingPlan: vi.fn(),
  setWorkspaceStatus: vi.fn(),
  activateWorkspaceRoot: vi.fn(),
}))

const clients: QueryClient[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
  window.history.replaceState(null, '', '/')
})

async function renderWorkspaces(content: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  clients.push(client)
  const root = createRootRoute({ component: () => content })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await act(async () => {
    await router.load()
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

const workspace: WorkspaceUsage = {
  tenant: {
    id: 1,
    slug: 'alpha',
    name: 'Alpha',
    owner_platform_user_id: 1,
    plan_id: 1,
    status: 'active',
    plan_expires_at: null,
  },
  usage: { month: '2026-09', requests: 1000 },
  owner_email: 'owner@example.test',
  primary_url: 'http://alpha.example.test/',
}
const wildcards = [{ id: 1, domain: 'example.test', enabled: true }]
const plans: HostingPlan[] = [
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
    name: 'Pro',
    price: 'Contact administrator',
    limits: { requests: 100000, users: 1000, tokens: 10000, channels: 100 },
    capabilities: {
      remove_platform_footer: true,
      custom_branding: true,
      max_workspaces: 10,
    },
  },
]

test('workspace creation rejects invalid slugs and opens the workspace after success', async () => {
  const user = userEvent.setup()
  vi.mocked(createWorkspace).mockResolvedValue({
    tenant: workspace.tenant,
    primary_url: 'http://alpha.example.test/',
    setup_url: 'http://alpha.example.test/setup',
  })
  await renderWorkspaces(
    <CreateWorkspace plans={plans} liteAvailable wildcardDomains={wildcards} />
  )
  await user.type(screen.getByLabelText('Name'), 'Alpha')
  await user.type(screen.getByLabelText('Workspace prefix'), 'Invalid/slug')
  await user.type(screen.getByLabelText('Username'), 'root')
  await user.type(
    screen.getByLabelText('Password'),
    'a synthetic testing passphrase'
  )
  await user.click(screen.getByRole('button', { name: 'Create workspace' }))
  await waitFor(() =>
    expect(screen.getByLabelText('Workspace prefix')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  )
  expect(createWorkspace).not.toHaveBeenCalled()
  await user.clear(screen.getByLabelText('Workspace prefix'))
  await user.type(screen.getByLabelText('Workspace prefix'), 'alpha')
  await user.click(screen.getByRole('button', { name: 'Create workspace' }))
  expect(
    await screen.findByRole('link', { name: 'Enter workspace' })
  ).toHaveAttribute('href', 'http://alpha.example.test/')
  expect(screen.getByRole('link', { name: 'Finish setup' })).toHaveAttribute(
    'href',
    'http://alpha.example.test/setup'
  )
  expect(vi.mocked(createWorkspace).mock.calls[0][0]).toEqual({
    name: 'Alpha',
    slug: 'alpha',
    prefix: 'alpha',
    wildcard_domain_id: 1,
    username: 'root',
    display_name: '',
    email: '',
    password: 'a synthetic testing passphrase',
    plan_id: 1,
    code: undefined,
  })
})

test('owners see usage while plan administration is reserved for administrators', async () => {
  await renderWorkspaces(<WorkspaceCard item={workspace} plans={plans} />)
  expect(
    screen.getByRole('progressbar', { name: 'Monthly usage for Alpha' })
  ).toHaveAttribute('aria-valuenow', '100')
  expect(
    screen.getByText('1,000', { exact: true }).parentElement
  ).toHaveTextContent('1,000 / 1,000')
  expect(
    screen.getByRole('link', { name: 'Manage workspace' })
  ).toHaveAttribute('href', '/platform/workspaces/1')
  expect(screen.getByRole('link', { name: 'Enter workspace' })).toHaveAttribute(
    'href',
    'http://alpha.example.test/'
  )
  expect(
    screen.queryByRole('button', { name: 'Activate plan manually' })
  ).not.toBeInTheDocument()
})

test('an administrator can manually select Pro and a failed activation remains retryable', async () => {
  const user = userEvent.setup()
  vi.mocked(assignHostingPlan).mockRejectedValue(
    new Error('Please sign in again.')
  )
  await renderWorkspaces(
    <AssignPlanDialog
      workspace={workspace.tenant}
      plans={plans}
      onClose={() => undefined}
    />
  )
  await user.selectOptions(screen.getByLabelText('Hosting plan'), '2')
  await user.click(
    screen.getByRole('button', { name: 'Activate plan manually' })
  )
  expect(assignHostingPlan).toHaveBeenCalledWith(1, 2, 1)
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Please sign in again.'
  )
  expect(
    screen.getByRole('button', { name: 'Activate plan manually' })
  ).toBeEnabled()
  expect(setWorkspaceStatus).not.toHaveBeenCalled()
})

test('expired workspaces show their state and stay enterable', async () => {
  await renderWorkspaces(
    <WorkspaceCard
      item={{
        ...workspace,
        tenant: {
          ...workspace.tenant,
          plan_id: 2,
          plan_expires_at: '2020-01-01T00:00:00Z',
        },
      }}
      plans={plans}
    />
  )
  expect(screen.getByText(/Expired/)).toBeVisible()
  expect(screen.getByText('alpha.example.test · Lite')).toBeVisible()
  expect(
    screen.getByText('1,000', { exact: true }).parentElement
  ).toHaveTextContent('1,000 / 1,000')
  expect(screen.getByRole('link', { name: 'Enter workspace' })).toHaveAttribute(
    'href',
    'http://alpha.example.test/'
  )
  expect(
    screen.getByRole('link', { name: 'Enter workspace' })
  ).not.toHaveAttribute('aria-disabled', 'true')
})

test('a second free Lite workspace is blocked until a paid plan is chosen', async () => {
  const user = userEvent.setup()
  await renderWorkspaces(
    <CreateWorkspace
      plans={plans}
      liteAvailable={false}
      wildcardDomains={wildcards}
    />
  )
  expect(
    screen.getByText(
      'You already have a free Lite workspace. Redeem a code to create another workspace.'
    )
  ).toBeVisible()
  expect(screen.getByRole('radio', { name: /Lite/ })).toHaveAttribute(
    'aria-disabled',
    'true'
  )
  expect(screen.getByRole('radio', { name: /Pro/ })).toBeChecked()
  expect(screen.getByLabelText('Redemption code')).toBeVisible()
  await user.type(screen.getByLabelText('Name'), 'Beta')
  await user.type(screen.getByLabelText('Workspace prefix'), 'beta')
  await user.type(screen.getByLabelText('Username'), 'root')
  await user.type(
    screen.getByLabelText('Password'),
    'a synthetic testing passphrase'
  )
  await user.click(screen.getByRole('button', { name: 'Create workspace' }))
  await waitFor(() =>
    expect(screen.getByLabelText('Redemption code')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  )
  expect(createWorkspace).not.toHaveBeenCalled()
})

test('root activation removes the secret from browser history and rejects a missing link', async () => {
  await renderWorkspaces(<ActivateWorkspace />)
  expect(
    screen.getByRole('button', { name: 'Activate workspace root' })
  ).toBeDisabled()
  expect(screen.getByRole('alert')).toHaveTextContent(
    'The activation link is missing or expired.'
  )
  expect(window.location.hash).toBe('')
  expect(activateWorkspaceRoot).not.toHaveBeenCalled()
})
