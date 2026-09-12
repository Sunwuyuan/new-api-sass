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
import { z } from 'zod'

export function safePlatformRedirect(value?: string): string {
  if (
    !value ||
    value.length > 1024 ||
    /[\\\r\n]/.test(value) ||
    !value.startsWith('/platform')
  ) {
    return '/platform'
  }
  try {
    const url = new URL(value, 'https://platform.invalid')
    if (url.origin !== 'https://platform.invalid' || url.hash) {
      return '/platform'
    }
    if (
      [
        '/platform',
        '/platform/security',
        '/platform/plans',
        '/platform/usage',
        '/platform/workspaces/new',
        '/platform/redeem',
        '/platform/admin',
      ].includes(url.pathname) ||
      url.pathname.startsWith('/platform/admin/') ||
      /^\/platform\/workspaces\/[1-9]\d*$/.test(url.pathname)
    ) {
      return url.pathname + url.search
    }
  } catch {
    /* Invalid redirect targets return to the workspace list. */
  }
  return '/platform'
}

export const platformAuthSearch = z.object({
  redirect: z.string().optional().transform(safePlatformRedirect),
  registered: z.boolean().optional().catch(false),
})
