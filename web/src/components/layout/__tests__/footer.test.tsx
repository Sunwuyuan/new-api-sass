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
import { render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { Footer } from '@/components/layout/components/footer'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useSystemConfigStore } from '@/stores/system-config-store'

const clients: QueryClient[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
})

function renderFooter(locked: boolean) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  client.setQueryData(STATUS_QUERY_KEY, { platform_footer_locked: locked })
  useSystemConfigStore.getState().setConfig({
    systemName: 'Workspace',
    footerHtml:
      '<p>Custom brand</p><script>window.bad = true</script><img src=x onerror=alert(1)>',
  })
  const route = createRootRoute({ component: () => <Footer /> })
  const router = createRouter({
    routeTree: route,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

test('Lite keeps platform attribution even when a custom footer is cached', async () => {
  renderFooter(true)
  expect(
    await screen.findByRole('link', { name: 'Hosted with New API SaaS' })
  ).toHaveAttribute('href', '/platform')
  expect(screen.queryByText('Custom brand')).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'New API' })).toBeInTheDocument()
})

test('Pro displays sanitized custom content and preserves project attribution', async () => {
  const view = renderFooter(false)
  expect(await screen.findByText('Custom brand')).toBeInTheDocument()
  expect(
    screen.queryByRole('link', { name: 'Hosted with New API SaaS' })
  ).not.toBeInTheDocument()
  expect(view.container.querySelector('script')).toBeNull()
  expect(view.container.querySelector('[onerror]')).toBeNull()
  expect(screen.getByRole('link', { name: 'New API' })).toBeInTheDocument()
})
