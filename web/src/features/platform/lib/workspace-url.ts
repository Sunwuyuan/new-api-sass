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
import type { WorkspaceHost } from '../types'

export function workspaceHref(
  source: { primary_url?: string },
  path = '/'
): string {
  const base = source.primary_url?.trim()
  if (!base) return ''
  if (path === '/' || path === '') return base
  const root = base.endsWith('/') ? base : `${base}/`
  return `${root}${path.replace(/^\//, '')}`
}

export function workspaceHostLabel(source: {
  primary_url?: string
  hosts?: WorkspaceHost[]
  tenant?: { slug: string }
}): string {
  if (source.primary_url) {
    try {
      return new URL(source.primary_url).host
    } catch {
      return source.primary_url.replace(/^https?:\/\//, '').replace(/\/$/, '')
    }
  }
  const verified = source.hosts?.find((host) => host.status === 'verified')
  if (verified?.host) return verified.host
  return source.tenant?.slug ?? ''
}
