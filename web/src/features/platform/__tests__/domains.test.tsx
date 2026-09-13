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
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import {
  createWorkspaceHost,
  getWildcardDomains,
  verifyWorkspaceHost,
} from '../api'
import { workspaceHref, workspaceHostLabel } from '../lib/workspace-url'
import type { WorkspaceHost } from '../types'
import { WorkspaceHosts } from '../workspace-hosts'

vi.mock('../api', () => ({
  getWildcardDomains: vi.fn(),
  createWorkspaceHost: vi.fn(),
  verifyWorkspaceHost: vi.fn(),
  deleteWorkspaceHost: vi.fn(),
}))

test('workspace links use the verified host url', () => {
  expect(
    workspaceHref({ primary_url: 'http://alpha.example.test/' }, '/setup')
  ).toBe('http://alpha.example.test/setup')
  expect(
    workspaceHostLabel({ primary_url: 'http://alpha.example.test/' })
  ).toBe('alpha.example.test')
})

const clients: QueryClient[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
})

async function renderHosts(hosts: WorkspaceHost[]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  await act(async () => {
    render(
      <QueryClientProvider client={client}>
        <WorkspaceHosts workspaceId={1} admin={false} hosts={hosts} />
      </QueryClientProvider>
    )
  })
}

const pending: WorkspaceHost = {
  id: 9,
  tenant_id: 1,
  kind: 'custom',
  host: 'api.customer.test',
  status: 'pending',
  verification_method: 'txt',
  txt_name: '_newapi-verify.api.customer.test',
  txt_value: 'site-token',
}

test('a custom domain shows TXT records and checks DNS', async () => {
  const user = userEvent.setup()
  vi.mocked(getWildcardDomains).mockResolvedValue([
    { id: 1, domain: 'example.test', enabled: true },
  ])
  vi.mocked(verifyWorkspaceHost).mockResolvedValue({
    ...pending,
    status: 'verified',
  })
  await renderHosts([pending])
  expect(screen.getByText(/_newapi-verify\.api\.customer\.test/)).toBeVisible()
  expect(screen.getByText(/site-token/)).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Check DNS' }))
  await waitFor(() =>
    expect(verifyWorkspaceHost).toHaveBeenCalledWith(1, 9, false)
  )
})

test('a prefix can be added independently of the workspace id', async () => {
  const user = userEvent.setup()
  vi.mocked(getWildcardDomains).mockResolvedValue([
    { id: 1, domain: 'example.test', enabled: true },
  ])
  vi.mocked(createWorkspaceHost).mockResolvedValue({
    id: 2,
    tenant_id: 1,
    kind: 'wildcard',
    host: 'docs.example.test',
    prefix: 'docs',
    status: 'verified',
    url: 'http://docs.example.test/',
  })
  await renderHosts([])
  await user.type(await screen.findByLabelText('Workspace prefix'), 'docs')
  await user.click(screen.getByRole('button', { name: 'Add prefix' }))
  await waitFor(() =>
    expect(createWorkspaceHost).toHaveBeenCalledWith(
      1,
      { kind: 'wildcard', prefix: 'docs', wildcard_domain_id: 1 },
      false
    )
  )
})
