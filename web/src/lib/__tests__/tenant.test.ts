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

import {
  applyTenantRequestURL,
  getTenantBasePath,
  isPlatformPath,
  isPlatformShell,
  scopedStorage,
  tenantPath,
} from '@/lib/tenant'

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

test('path prefixes are not used as a workspace address', () => {
  expect(getTenantBasePath('/t/alpha/dashboard')).toBe('')
  expect(getTenantBasePath('/platform')).toBe('')
  expect(isPlatformPath('/platform')).toBe(true)
  expect(isPlatformPath('/platform/workspaces/1')).toBe(true)
  expect(isPlatformPath('/dashboard')).toBe(false)
})

test('workspace requests stay on the current host', () => {
  expect(tenantPath('/api/status')).toBe('/api/status')
  expect(applyTenantRequestURL({ url: '/api/status' })).toEqual({
    url: '/api/status',
    baseURL: '/',
  })
  expect(
    applyTenantRequestURL({
      url: '/api/user/self',
      baseURL: '/t/alpha',
    })
  ).toEqual({
    url: '/api/user/self',
    baseURL: '/',
  })
  expect(
    applyTenantRequestURL({ url: 'https://example.test/api/status' })
  ).toEqual({
    url: 'https://example.test/api/status',
  })
})

test('the platform console is selected from the path or localhost root', () => {
  expect(isPlatformShell('/platform')).toBe(true)
  expect(isPlatformShell('/dashboard')).toBe(false)
  expect(isPlatformShell('/')).toBe(true)
})
