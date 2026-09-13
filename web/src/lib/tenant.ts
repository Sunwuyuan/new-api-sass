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
const ABSOLUTE_URL = /^([a-z][a-z\d+\-.]*:)?\/\//i

export function isPlatformPath(pathname: string): boolean {
  return pathname === '/platform' || pathname.startsWith('/platform/')
}

export function isPlatformShell(
  pathname = typeof window === 'undefined' ? '' : window.location.pathname
): boolean {
  if (isPlatformPath(pathname)) return true
  if (typeof document !== 'undefined') {
    const shell = document
      .querySelector('meta[name="new-api-shell"]')
      ?.getAttribute('content')
    if (shell === 'platform') return true
  }
  if (pathname !== '/' && pathname !== '') return false
  if (typeof window === 'undefined') return true
  const hostname = window.location.hostname
  return (
    hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]'
  )
}

export function getTenantBasePath(_pathname?: string): string {
  return ''
}

export const tenantBasePath = ''

export function currentTenantBasePath(): string {
  return ''
}

export function tenantPath(path: string): string {
  if (!path.startsWith('/') || path.startsWith('//')) return path
  return path
}

export function applyTenantRequestURL<
  T extends { url?: string; baseURL?: string },
>(config: T): T {
  const url = config.url ?? ''
  if (!url || ABSOLUTE_URL.test(url)) return config
  if (config.baseURL && ABSOLUTE_URL.test(config.baseURL)) return config
  const path = url.startsWith('/') ? url : `/${url}`
  config.url = path
  // Keep a root base so Axios cannot re-apply a stale instance baseURL.
  config.baseURL = '/'
  return config
}

function tenantStorageHost(): string {
  if (typeof window === 'undefined') return 'platform'
  return window.location.host || 'platform'
}

export function tenantKey(key: string): string {
  return `new-api:${tenantStorageHost()}:${key}`
}

// All persisted workspace data uses this facade, including key enumeration and
// cleanup. Navigating between workspaces performs a full page navigation so
// in-memory auth and query caches have the same lifetime as the workspace.
export function scopedStorage(
  source: Storage | (() => Storage | undefined),
  prefix: string
): Storage {
  const storage = () => {
    try {
      return typeof source === 'function' ? source() : source
    } catch {
      return undefined
    }
  }
  const keys = () => {
    const current = storage()
    if (!current) return []
    return Array.from({ length: current.length }, (_, i) =>
      current.key(i)
    ).filter((key): key is string => key !== null && key.startsWith(prefix))
  }
  return {
    get length() {
      return keys().length
    },
    key(index: number) {
      return keys()[index]?.slice(prefix.length) ?? null
    },
    getItem(key: string) {
      return storage()?.getItem(prefix + key) ?? null
    },
    setItem(key: string, value: string) {
      storage()?.setItem(prefix + key, value)
    },
    removeItem(key: string) {
      storage()?.removeItem(prefix + key)
    },
    clear() {
      keys().forEach((key) => storage()?.removeItem(key))
    },
  }
}

export const tenantStorage = scopedStorage(
  () => globalThis.localStorage,
  tenantKey('')
)
export const tenantSessionStorage = scopedStorage(
  () => globalThis.sessionStorage,
  tenantKey('')
)
