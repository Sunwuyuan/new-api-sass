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
import { beforeEach, describe, expect, test } from 'vitest'

import { getTenantBasePath, scopedStorage } from '@/lib/tenant'

beforeEach(() => localStorage.clear())

describe('workspace storage', () => {
  test('the same key in two workspaces stays independent after one is cleared', () => {
    const alpha = scopedStorage(localStorage, 'tenant:alpha:')
    const beta = scopedStorage(localStorage, 'tenant:beta:')
    alpha.setItem('status', 'Alpha')
    beta.setItem('status', 'Beta')
    expect(alpha.getItem('status')).toBe('Alpha')
    expect(beta.getItem('status')).toBe('Beta')
    expect(alpha.length).toBe(1)
    expect(alpha.key(0)).toBe('status')
    alpha.clear()
    expect(alpha.getItem('status')).toBeNull()
    expect(beta.getItem('status')).toBe('Beta')
  })

  test('denied browser storage does not prevent opening a workspace', () => {
    const storage = scopedStorage(() => {
      throw new DOMException('Denied', 'SecurityError')
    }, 'tenant:alpha:')
    expect(() => storage.setItem('status', 'Alpha')).not.toThrow()
    expect(storage.getItem('status')).toBeNull()
    expect(storage.length).toBe(0)
  })
})

test('only a valid workspace segment becomes the router base', () => {
  expect(getTenantBasePath('/t/alpha/dashboard')).toBe('/t/alpha')
  expect(getTenantBasePath('/t/beta')).toBe('/t/beta')
  expect(getTenantBasePath('/platform')).toBe('')
  expect(getTenantBasePath('/t/alpha%2Fbeta/dashboard')).toBe('')
})
