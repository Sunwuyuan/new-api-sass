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
export type PlatformUser = {
  id: number
  email: string
  role: 'admin' | 'user'
  tenant_count: number
}

export type PlatformSession = {
  user: PlatformUser
  csrf_token: string
}

export type HostingPlan = {
  id: number
  name: string
  price: string
  limits: { requests: number; users: number; tokens: number; channels: number }
  capabilities: { remove_platform_footer: boolean }
}

export type Workspace = {
  id: number
  slug: string
  name: string
  owner_platform_user_id: number
  plan_id: number
  status: 'active' | 'suspended'
  plan_expires_at: string | null
}

export type WorkspaceUsage = {
  tenant: Workspace
  usage: { month: string; requests: number }
}
