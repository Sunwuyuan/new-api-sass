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
import { render, screen, waitFor } from '@testing-library/react'
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

function renderWorkspaces(content: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  clients.push(client)
  render(<QueryClientProvider client={client}>{content}</QueryClientProvider>)
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
}
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

test('workspace creation rejects invalid slugs and shows the one-time activation link after success', async () => {
  const user = userEvent.setup()
  vi.mocked(createWorkspace).mockResolvedValue({
    tenant: workspace.tenant,
    root_activation_url: '/t/alpha/activate#token=test-activation',
  })
  renderWorkspaces(<CreateWorkspace />)
  await user.type(screen.getByLabelText('Name'), 'Alpha')
  await user.type(screen.getByLabelText('Workspace address'), 'Invalid/slug')
  await user.click(screen.getByRole('button', { name: 'Create workspace' }))
  await waitFor(() =>
    expect(screen.getByLabelText('Workspace address')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  )
  expect(createWorkspace).not.toHaveBeenCalled()
  await user.clear(screen.getByLabelText('Workspace address'))
  await user.type(screen.getByLabelText('Workspace address'), 'alpha')
  await user.click(screen.getByRole('button', { name: 'Create workspace' }))
  expect(
    await screen.findByRole('link', { name: 'Activate workspace root' })
  ).toHaveAttribute('href', expect.stringContaining('/t/alpha/activate#token='))
  expect(
    screen.getByRole('button', { name: 'Copy activation link' })
  ).toBeVisible()
})

test('owners see usage while plan administration is reserved for administrators', () => {
  renderWorkspaces(<WorkspaceCard item={workspace} plans={plans} />)
  expect(screen.getByText(/Monthly requests/)).toHaveTextContent(
    '1,000 / 1,000'
  )
  expect(screen.getByRole('link', { name: 'Enter workspace' })).toHaveAttribute(
    'href',
    '/t/alpha/'
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
  renderWorkspaces(
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

test('expired workspaces show their state and disable entry', () => {
  renderWorkspaces(
    <WorkspaceCard
      item={{
        ...workspace,
        tenant: {
          ...workspace.tenant,
          plan_expires_at: '2020-01-01T00:00:00Z',
        },
      }}
      plans={plans}
    />
  )
  expect(screen.getByText(/Expired/)).toBeVisible()
  expect(screen.getByRole('link', { name: 'Enter workspace' })).toHaveAttribute(
    'aria-disabled',
    'true'
  )
})

test('root activation removes the secret from browser history and rejects a missing link', async () => {
  renderWorkspaces(<ActivateWorkspace />)
  expect(
    screen.getByRole('button', { name: 'Activate workspace root' })
  ).toBeDisabled()
  expect(screen.getByRole('alert')).toHaveTextContent(
    'The activation link is missing or expired.'
  )
  expect(window.location.hash).toBe('')
  expect(activateWorkspaceRoot).not.toHaveBeenCalled()
})
