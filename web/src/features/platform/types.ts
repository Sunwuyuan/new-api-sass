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
  email: string | null
  display_name?: string
  has_password?: boolean
  role: 'admin' | 'root' | 'user'
  status: 'active' | 'disabled'
  must_change_password: boolean
  tenant_count: number
}

export type PlatformSession = {
  user: PlatformUser
  csrf_token: string
  authenticated_at: string
  redirect?: string
}

export type HostingPlan = {
  id: number
  name: string
  price: string
  limits: { requests: number; users: number; tokens: number; channels: number }
  capabilities: {
    remove_platform_footer: boolean
    custom_branding: boolean
    max_workspaces: number
  }
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
  owner_email: string
}

export type PageParams = {
  page: number
  page_size: number
  search?: string
  status?: string
  role?: string
}

export type PageResult = {
  pagination: { page: number; page_size: number; total: number }
}

export type WorkspacePage = PageResult & {
  tenants: WorkspaceUsage[]
  max_workspaces: number
  workspace_count: number
}

export type PlatformRedemption = {
  id: number
  code_hint: string
  plan_id: number
  duration_months: number
  status: 'active' | 'disabled'
  max_uses: number
  used: number
  expires_at: string | null
  created_by: number
  created_at: string
}

export type RedemptionBatch = {
  plan_id: number
  duration_months: number
  count: number
  max_uses: number
  expires_at: string | null
}

export type RedemptionUse = {
  id: number
  redemption_id: number
  tenant_id: number
  platform_user_id: number
  assignment_id: number
  created_at: string
}

export type PlatformAudit = {
  id: number
  actor_id: number
  action: string
  target_id: number
  details: string
  created_at: string
}

export type PlanAssignment = {
  id: number
  tenant_id: number
  plan_id: number
  source: 'manual' | 'redeem'
  expires_at: string
}

export type UserAction =
  | 'enable'
  | 'disable'
  | 'promote'
  | 'demote'
  | 'require_password_change'
  | 'revoke_sessions'
