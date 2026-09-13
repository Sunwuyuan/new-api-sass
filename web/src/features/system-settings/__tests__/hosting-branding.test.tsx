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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { STATUS_QUERY_KEY } from '@/lib/status-query'

import { SystemInfoSection } from '../general/system-info-section'

const clients: QueryClient[] = []
afterEach(() => clients.splice(0).forEach((client) => client.clear()))

function renderBranding(status: {
  platform_branding_locked?: boolean
  platform_footer_locked?: boolean
}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  clients.push(client)
  client.setQueryData(STATUS_QUERY_KEY, status)
  const route = createRootRoute({
    component: () => (
      <SystemInfoSection
        defaultValues={{
          SystemName: 'Existing brand',
          ServerAddress: '',
          TaskPublicAddress: '',
          Logo: 'https://example.test/logo.png',
          Footer: 'Existing footer',
          legal: { user_agreement: '', privacy_policy: '' },
        }}
      />
    ),
  })
  const router = createRouter({
    routeTree: route,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return client
}

test('missing hosting capabilities keep branding and footer fields locked', async () => {
  renderBranding({})
  expect(await screen.findByLabelText('System Name')).toBeDisabled()
  expect(screen.getByLabelText('Logo URL')).toBeDisabled()
  expect(screen.getByLabelText('Footer')).toBeDisabled()
  expect(
    screen.getByText('Custom branding requires an eligible hosting plan.')
  ).toBeVisible()
})

test('Standard enables branding, Pro enables the footer, and a downgrade locks both immediately', async () => {
  const client = renderBranding({
    platform_branding_locked: false,
    platform_footer_locked: true,
  })
  expect(await screen.findByLabelText('System Name')).toBeEnabled()
  expect(screen.getByLabelText('Logo URL')).toBeEnabled()
  expect(screen.getByLabelText('Footer')).toBeDisabled()
  await act(async () => {
    client.setQueryData(STATUS_QUERY_KEY, {
      platform_branding_locked: false,
      platform_footer_locked: false,
    })
  })
  await waitFor(() => expect(screen.getByLabelText('Footer')).toBeEnabled())
  await act(async () => {
    client.setQueryData(STATUS_QUERY_KEY, {
      platform_branding_locked: true,
      platform_footer_locked: true,
    })
  })
  await waitFor(() =>
    expect(screen.getByLabelText('System Name')).toBeDisabled()
  )
  expect(screen.getByLabelText('Logo URL')).toBeDisabled()
  expect(screen.getByLabelText('Footer')).toBeDisabled()
})
