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
import { lazy, type ReactNode } from 'react'

import { isPlatformShell } from '@/lib/tenant'

const PlatformApp = lazy(() => import('./index'))
const ActivateWorkspace = lazy(() => import('./activate-workspace'))

// Entering another workspace reloads the document so its router, query cache,
// and in-memory authentication state start independently.
export function PlatformEntry(props: { children: ReactNode }) {
  if (isPlatformShell()) return <PlatformApp />
  if (window.location.pathname === '/activate') {
    return <ActivateWorkspace />
  }
  return props.children
}
