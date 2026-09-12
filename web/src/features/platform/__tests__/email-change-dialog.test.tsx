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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { finishPlatformEmailChange, startPlatformEmailChange } from '../api'
import { PlatformEmailChangeDialog } from '../email-change-dialog'

vi.mock('../api', () => ({
  finishPlatformEmailChange: vi.fn(),
  startPlatformEmailChange: vi.fn(),
}))

const clients: QueryClient[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
})

function renderDialog(content: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  })
  clients.push(client)
  render(<QueryClientProvider client={client}>{content}</QueryClientProvider>)
  return client
}

test('email change validates input, sends the code and confirms the new address', async () => {
  const user = userEvent.setup()
  const onClose = vi.fn()
  vi.mocked(startPlatformEmailChange).mockResolvedValue()
  vi.mocked(finishPlatformEmailChange)
    .mockRejectedValueOnce(
      new Error('That verification code is invalid or expired.')
    )
    .mockResolvedValueOnce()
  const client = renderDialog(<PlatformEmailChangeDialog onClose={onClose} />)
  client.setQueryData(['platform', 'session'], { user: { id: 1 } })

  const dialog = await screen.findByRole('dialog')
  await user.type(within(dialog).getByLabelText('New email'), 'not-an-email')
  await user.click(within(dialog).getByRole('button', { name: 'Send code' }))
  expect(
    await within(dialog).findByText('Invalid email address', {
      selector: '[data-slot="field-error"]',
    })
  ).toBeVisible()
  expect(startPlatformEmailChange).not.toHaveBeenCalled()

  await user.clear(within(dialog).getByLabelText('New email'))
  await user.type(
    within(dialog).getByLabelText('New email'),
    'renamed@example.test'
  )
  await user.click(within(dialog).getByRole('button', { name: 'Send code' }))
  await waitFor(() =>
    expect(startPlatformEmailChange).toHaveBeenCalledWith(
      'renamed@example.test'
    )
  )
  expect(
    await within(dialog).findByText(
      'Enter the verification code sent to renamed@example.test.'
    )
  ).toBeVisible()

  await user.type(within(dialog).getByLabelText('Verification code'), '12')
  await user.click(within(dialog).getByRole('button', { name: 'Change email' }))
  expect(
    await within(dialog).findByText('Enter the 6-digit verification code.', {
      selector: '[data-slot="field-error"]',
    })
  ).toBeVisible()
  expect(finishPlatformEmailChange).not.toHaveBeenCalled()

  await user.type(within(dialog).getByLabelText('Verification code'), '3456')
  await user.click(within(dialog).getByRole('button', { name: 'Change email' }))
  expect(await within(dialog).findByRole('alert')).toHaveTextContent(
    'That verification code is invalid or expired.'
  )
  expect(onClose).not.toHaveBeenCalled()

  await user.click(within(dialog).getByRole('button', { name: 'Change email' }))
  await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
  expect(finishPlatformEmailChange).toHaveBeenCalledWith({
    email: 'renamed@example.test',
    code: '123456',
  })
  expect(client.getQueryState(['platform', 'session'])?.isInvalidated).toBe(
    true
  )
})

test('email change allows returning to the address step without resending the code', async () => {
  const user = userEvent.setup()
  vi.mocked(startPlatformEmailChange).mockResolvedValue()
  renderDialog(<PlatformEmailChangeDialog onClose={vi.fn()} />)

  const dialog = await screen.findByRole('dialog')
  await user.type(
    within(dialog).getByLabelText('New email'),
    'renamed@example.test'
  )
  await user.click(within(dialog).getByRole('button', { name: 'Send code' }))
  expect(
    await within(dialog).findByLabelText('Verification code')
  ).toBeVisible()
  await user.click(
    within(dialog).getByRole('button', { name: 'Use a different email' })
  )
  expect(await within(dialog).findByLabelText('New email')).toHaveValue(
    'renamed@example.test'
  )
  expect(startPlatformEmailChange).toHaveBeenCalledOnce()
})
